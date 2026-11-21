## Database connections

Every application and managed database on an instance joins one private Docker
network, `shipper-<instance>-private`. A release answers on it as `app-<id>` and,
for databases, `db-<id>`, once it is promoted; candidates only answer their
release name, `site-<release>`. The proxy is the only infrastructure container
on the network. Releases created by earlier versions joined a network per
application; routing moves them onto the shared network and removes the old one.

**Connection string** on a database's Overview copies the private string, which
is plain for every engine (no `sslmode`, `redis://` rather than `rediss://`).
PostgreSQL, MySQL and MariaDB still offer TLS with the instance's private CA, so
strings that ask for it keep working, but none require it. Redis is plain only.
A container created before this change carries no `io.shipper.database.transport`
label and still requires TLS, so its connection string and Redis backups keep the
TLS form until the database is deployed again.

PostgreSQL, Redis and MongoDB can also answer on a public domain
(`config.database.domain`). Traefik binds the engine's standard port (5432, 6379,
27017) only while an active database of that engine has a domain, terminates TLS
with the domain's Let's Encrypt certificate, routes by SNI and forwards plain
traffic to `site-<release>`. Changing the set of engines recreates the proxy on
its volume; if the new proxy cannot bind a port, the previous one is restored.
MySQL, MariaDB and ClickHouse negotiate TLS in-protocol and stay on published
ports. `POST /applications/{id}/database/connection` with `{"public": true}`
returns the public string.

Credentials follow the official image's runtime initialization variables and
defaults. Custom start commands, file-based secrets and Redis authentication or
configuration mounts require manual connection details rather than guessed
passwords.

Run `npm run test:database` for the opt-in real Docker connection round trip, and
`SHIPPER_TEST_NETWORKING=1 go test -run TestDockerNetworkingAcceptance ./internal/deploy`
for the shared network, the retirement of per-application networks and public
domains through the proxy.
