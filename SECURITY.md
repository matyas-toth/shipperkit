# Security

Shipper controls security-critical infrastructure. Phase 2 is development
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
containers receive neither a socket nor host mounts and run without root or Linux
capabilities. HTTP/HTTPS proxy publication is separate from the private management
listener. Public ACME issuance depends on operator DNS and firewall configuration.

Only public HTTPS Git sources are supported. Credentials, redirects, hooks,
submodules, private-network destinations, and arbitrary repository commands are
disabled. Publish archives exclude metadata/secrets, reject links and escapes,
and use a restricted Apache `.htaccess` allowlist. Limits and operational caveats
are documented in [static site operations](docs/static-sites.md).

Repository content is served as a website. Do not share the management origin
with an untrusted application, and deploy only content you intend to publish.
Build execution, private Git credentials, and runtime secrets require separate
review when introduced. A formal security audit and disposable-VPS ACME/reboot
checks remain release prerequisites.

