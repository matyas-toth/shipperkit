# Security

Shipper controls security-critical infrastructure. Phase 1 is development
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

Scope currently includes the control plane, authentication, inventory projection,
and read-only SDK integration. Build execution, Git credentials, runtime secrets,
proxy routing, and deployment mutations need separate review as they are added.

