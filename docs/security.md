# Security model

Shipper controls security-critical infrastructure and has not had an independent security review. To report a vulnerability, follow the [security policy](../SECURITY.md).

## Deployment

Keep the dashboard on loopback or behind authenticated access. Configure the canonical HTTPS public origin when using a reverse proxy. A Docker socket grants control of the host, and a read-only mount does not restrict Docker API operations. Do not expose an unauthenticated Docker TCP daemon.

Protect the data directory and backups. POSIX permissions are restrictive, and Windows operators must also enforce appropriate directory ACLs. The setup token becomes unusable once an administrator exists, but should still be treated as sensitive. Back up `encryption.key` together with the database.

## Sessions and setup

Passwords use bcrypt. Session tokens are random, hashed in SQLite, HttpOnly, SameSite=Strict and Secure under HTTPS. Authenticated mutations require CSRF tokens. Managed first-run setup uses `shipperctl claim` through host access, and a manually hosted first run uses a private expiring local token. Offline owner recovery revokes existing sessions.

## Workload isolation

Mutations are restricted to exact instance and application ownership. Traefik receives no Docker socket. Application containers receive neither a socket nor host mounts and drop Linux capabilities. Static containers use an unprivileged user, and full application images keep their configured user, so prefer an explicit non-root `USER` in Dockerfiles. HTTP and HTTPS proxy publication is separate from the private management listener, and public ACME issuance depends on the operator's DNS and firewall.

Repository content is served as a website. Do not share the management origin with an untrusted application, and deploy only content you intend to publish. BuildKit runs as a privileged, instance-owned daemon. Railpack preparation is bounded and has no socket or host mounts. Variables are encrypted in SQLite and masked in ordinary responses, though Docker administrators can still inspect a container's runtime environment.

## Sources

Git sources use a hardened HTTPS checkout. Public repositories and connected private GitHub repositories are supported. Source credentials are encrypted and kept out of ordinary responses and command arguments. Redirects, Git hooks, submodules, Git LFS, private Git hosts and control-plane repository command execution are outside the supported source path. Registry credentials are bound to their host, and private registry addresses require explicit opt-in. Build commands run inside the dedicated BuildKit daemon. Publish archives exclude metadata and secrets, reject links and path escapes, and use a restricted Apache `.htaccess` allowlist. See [static site operations](static-sites.md), [full application operations](full-applications.md) and [sources](sources.md).

Signed GitHub deliveries and application-scoped deploy tokens do not grant dashboard access.

## Compose

Compose sources and resolved plans are encrypted and hidden from ordinary API responses. Referenced paths cannot leave the checkout, repository binds become Docker volume snapshots and host ports are removed. Compose operations verify instance and application ownership before orchestration. Explicit external volumes are used without adoption or deletion. Compose is an administrator-only workload capability, not a sandbox for hostile tenants. See [Compose operations](compose.md).

## Accounts and sign-in

The [authentication contract](authentication.md) describes the account model. There is no public registration. Admins provision accounts and assign the Admin, Developer or Viewer role, and the private first-admin capability is the only bootstrap exception.

Email and password, Google, GitHub and Discord sign-in are enabled independently. Accounts without a password have no password credential, and admin-supplied temporary passwords must be replaced before full access. Verified-email admission links a provider only to an existing active account. Stable provider subjects identify established links, and unverified, ambiguous or non-authoritative claims cannot silently link or create users. Login credentials are separate from repository connections.

OAuth uses expiring one-use state bound to an HttpOnly cookie, exact canonical redirect URIs, PKCE for Google and Discord, and server-side code exchange and identity retrieval. Google ID tokens are checked for signature, issuer, audience, expiry, nonce and verified email. Provider client secrets are sealed with the instance key and excluded from ordinary API responses.

Role checks and account assurance gates run on the server. A temporary password, required factor enrollment or factor challenge restricts the session to the onboarding endpoints. Every cookie mutation requires the session CSRF token. Disabling or removing an account revokes its sessions, personal operation tokens and terminal sessions. A transaction guard prevents any change that would leave no active administrator with a working sign-in method.

## Two-factor authentication

Every enrolled interactive login needs Shipper's own TOTP factor, and administrators can require it for the whole instance. Enrollment and factor gates restrict UI, API and stream access, including active sessions. Operation and deploy tokens are automation identities: they cannot create browser sessions or change the account policy. Factor resets and offline recovery are audited, and ordinary APIs never expose TOTP secrets or recovery codes.

## Certificates

Managed HTTPS seeds the ACME contact from the first administrator's email. Traefik issues and renews certificates, and Shipper observes readiness and expiry. Preserve deliberate CA and contact overrides during migration and recovery.

## Backups and recovery

Recovery archives use age authenticated encryption. Keep the recovery identity outside the server and separate from its encrypted archives, because together they expose the original instance encryption material and credentials. Archives are size-bounded and fully authenticated and inventoried before a restore. Instance recovery is offline, refuses populated targets, revokes sessions and tokens and pauses captured work. Database and volume restores create a new copy and leave the original in place. S3 credentials and per-run destination snapshots are encrypted at rest, and signed requests use HTTPS by default and reject redirects and private network targets unless opted in. See [backups and recovery](backups.md).
