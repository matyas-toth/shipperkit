# Security

Shipper controls security-critical infrastructure. Version 0.7.0 is development
software and has not had an independent security review.

Keep it on loopback or behind authenticated private access. Configure the
canonical HTTPS public origin when using a reverse proxy. A Docker socket grants
host-level control; a read-only mount does not restrict Docker API operations.

Administrator passwords use bcrypt. Session tokens are random, hashed in SQLite,
HttpOnly, SameSite=Strict, and Secure under HTTPS. Authenticated mutations require
CSRF tokens. First-run administrator creation requires a private local token.
Local password recovery revokes existing sessions.

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
audit and [real-system ACME/reboot/recovery acceptance](docs/vps-test-plan.md)
remain release prerequisites; offered lab access is not evidence of completed tests.


Compose sources and resolved plans are encrypted and hidden from ordinary API
responses. Referenced paths cannot leave the checkout, repository binds become
Docker volume snapshots, and host ports are removed. Compose project operations
verify instance/application ownership before orchestration. Explicit external
volumes are used without adoption or deletion. Compose is an administrator-only
workload capability, not a sandbox for hostile tenants. Supported options and
stateful rollback limits are documented in [Compose operations](docs/compose.md).

## Planned authentication and HTTPS policy

The [authentication contract](docs/authentication.md) and
[roadmap](docs/roadmap.md) are future requirements, not current OAuth/2FA support.
There is no planned public registration: admins provision accounts, while the
existing private first-admin capability remains the bootstrap exception.

Collaboration will support independently enabled Email + Password, Google,
GitHub and Discord sign-in. Empty-password accounts have no password credential;
admin-supplied temporary passwords require replacement before full access.
Verified-email admission may link a provider to an existing provisioned account,
but stable provider subjects identify established links. Unverified/ambiguous or
non-authoritative claims cannot silently link or create users. Login credentials
remain separate from repository connections.

Personal TOTP, recovery and instance-wide enforcement are required before 1.0.
Every enrolled interactive login needs Shipper's own second factor, including
future OAuth login. Password-change/enrollment/factor gates restrict both UI and
API/stream access. Forced enrollment covers active admin/user sessions and cannot
be bypassed by disabling a required factor or using an old personal token.
Audit factor resets and offline owner recovery; do not log or expose TOTP secrets
or recovery codes through ordinary APIs.

Planned managed HTTPS seeds ACME contact from the first administrator's email and
owns issuance, renewal, persistence, bounded retry and actionable failure reporting.
Current 0.7.0 still uses explicit proxy/ACME configuration. Preserve existing CA
accounts, contact overrides and certificate storage during migration and recovery.


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
