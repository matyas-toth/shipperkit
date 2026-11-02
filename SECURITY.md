# Security

Shipper controls security-critical infrastructure. Phase 3 is development
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

Only public HTTPS Git sources are supported. Credentials, redirects, hooks,
submodules, private-network destinations, and control-plane repository command execution are
disabled. Build commands execute inside the dedicated BuildKit daemon. Publish archives exclude metadata/secrets, reject links and escapes,
and use a restricted Apache `.htaccess` allowlist. Limits and operational caveats
are documented in [static site operations](docs/static-sites.md).

Repository content is served as a website. Do not share the management origin
with an untrusted application, and deploy only content you intend to publish.
BuildKit requires a privileged, instance-owned daemon; Railpack preparation is
bounded and has no socket/host mounts. Variables are encrypted in SQLite and
masked in ordinary responses; Docker administrators can still inspect runtime
environment. Back up encryption.key with the database. See
[full application operations](docs/full-applications.md). Private Git credentials
remain unimplemented. A formal security audit and disposable-VPS ACME/reboot
checks remain release prerequisites.


Compose sources and resolved plans are encrypted and hidden from ordinary API
responses. Referenced paths cannot leave the checkout, repository binds become
Docker volume snapshots, and host ports are removed. Compose project operations
verify instance/application ownership before orchestration. Explicit external
volumes are used without adoption or deletion. Compose is an administrator-only
workload capability, not a sandbox for hostile tenants. Supported options and
stateful rollback limits are documented in [Compose operations](docs/compose.md).
