# Security

Shipper controls security-critical infrastructure. The 1.0 beta has not had an
independent security review.

Keep it on loopback or behind authenticated private access. Configure the
canonical HTTPS public origin when using a reverse proxy. A Docker socket grants
host-level control; a read-only mount does not restrict Docker API operations.

Administrator passwords use bcrypt. Session tokens are random, hashed in SQLite,
HttpOnly, SameSite=Strict, and Secure under HTTPS. Authenticated mutations require
CSRF tokens. Managed first-run setup uses `shipperctl claim` through host access.
Manually hosted first-run setup uses a private expiring local token. Offline
owner recovery revokes existing sessions.

Protect the data directory and backups. POSIX permissions are restrictive;
Windows operators must also enforce appropriate directory ACLs. The setup-token
file becomes unusable once an administrator exists, but should still be treated
as sensitive. Do not expose an unauthenticated Docker TCP daemon.

Do not post credentials or exploit details in public issues. Until a private
reporting channel is established, use the repository host's private vulnerability
reporting mechanism when enabled, or contact the maintainers privately.

Scope includes authentication, inventory, public Git transport, static packaging,
deployment mutations, and proxy configuration. Mutations are restricted to exact
instance/application ownership. Traefik receives no Docker socket; application
containers receive neither a socket nor host mounts and drop Linux capabilities.
Static containers use an unprivileged user; full application images retain their
configured user. Prefer an explicit non-root USER in Dockerfiles. HTTP/HTTPS proxy publication is separate from the private management
listener. Public ACME issuance depends on operator DNS and firewall configuration.

Git sources use hardened HTTPS checkout. Public repositories and connected private
GitHub repositories are supported; source credentials are encrypted and excluded
from ordinary responses and command arguments. Redirects, Git hooks, submodules,
Git LFS, private Git hosts and control-plane repository command execution remain
outside the supported source path. Registry credentials are bound to their host;
private registry addresses require explicit opt-in. Build commands execute inside
the dedicated BuildKit daemon. Publish archives exclude metadata/secrets, reject links and escapes,
and use a restricted Apache `.htaccess` allowlist. Limits and operational caveats
are documented in [static site operations](docs/static-sites.md).

Repository content is served as a website. Do not share the management origin
with an untrusted application, and deploy only content you intend to publish.
BuildKit requires a privileged, instance-owned daemon; Railpack preparation is
bounded and has no socket/host mounts. Variables are encrypted in SQLite and
masked in ordinary responses; Docker administrators can still inspect runtime
environment. Back up encryption.key with the database. See
[full application operations](docs/full-applications.md) and
[source credentials and triggers](docs/sources.md). Signed GitHub deliveries and
application-scoped deploy tokens do not grant dashboard access. A formal security
audit and the outstanding [real-system acceptance checks](docs/release-acceptance.md)
remain release prerequisites.


Compose sources and resolved plans are encrypted and hidden from ordinary API
responses. Referenced paths cannot leave the checkout, repository binds become
Docker volume snapshots, and host ports are removed. Compose project operations
verify instance/application ownership before orchestration. Explicit external
volumes are used without adoption or deletion. Compose is an administrator-only
workload capability, not a sandbox for hostile tenants. Supported options and
stateful rollback limits are documented in [Compose operations](docs/compose.md).

## Authentication and HTTPS policy

The [authentication contract](docs/authentication.md) describes the implemented
account model. There is no public registration. Admins provision accounts and
assign Admin, Developer or Viewer roles; the private first-admin capability
remains the bootstrap exception.

Email and password, Google, GitHub and Discord sign-in are enabled independently.
Empty-password accounts have no password credential; admin-supplied temporary
passwords require replacement before full access. Verified-email admission links
a provider only to an existing active provisioned account. Stable provider
subjects identify established links, and unverified, ambiguous or
non-authoritative claims cannot silently link or create users. Login credentials
remain separate from repository connections.

OAuth uses expiring one-use state bound to an HttpOnly cookie, exact canonical
redirect URIs, PKCE for Google and Discord, server-side code exchange and
server-side identity retrieval. Google ID tokens are checked for signature,
issuer, audience, expiry, nonce and verified email. Provider client secrets are
sealed with the instance key and excluded from ordinary API responses.

Role checks and account assurance gates run server-side. A temporary password,
required factor enrollment or factor challenge restricts the session to the
onboarding endpoints. Every cookie mutation requires the session CSRF token.
Disabling or removing an account revokes sessions, personal operation tokens and
terminal sessions. A transaction guard prevents a change that would leave no
active administrator with a working sign-in method.

Personal TOTP, recovery and instance-wide enforcement are implemented. Every
enrolled interactive login needs Shipper's own second factor. Enrollment and
factor gates restrict UI, API and stream access, including active sessions.
Separately scoped operations and deploy tokens remain automation identities; they
cannot create browser sessions or change the account policy. Factor resets and
offline recovery are audited. Ordinary APIs do not expose TOTP secrets or
recovery codes.

Managed HTTPS seeds the ACME contact from the first administrator's email.
Traefik issues and renews certificates; Shipper observes readiness and expiry.
The beta preserved the certificate store through a real reboot; actual renewal is
a separate acceptance check. Preserve deliberate CA and contact overrides during
migration and recovery.


Phase 8 recovery archives use age authenticated encryption. Keep the separately
downloaded recovery identity outside the VPS and separate from its encrypted archives;
together they expose the original instance encryption material and credentials.
Private staging is necessary for database tools and SQLite snapshots. Archives are
size-bounded and fully authenticated/inventoried before restore. Instance recovery
is offline, refuses populated targets, revokes sessions/tokens and pauses captured
work. Database/volume restoration creates a new copy, leaving originals available.
S3 credentials and per-run destination snapshots are encrypted at rest; signed requests
use HTTPS by default and reject redirects/private network targets unless opted in.
See [backup boundaries and recovery](docs/backups.md).
