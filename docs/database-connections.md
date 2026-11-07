## Database image connections

A deployed official PostgreSQL, MySQL, MariaDB, MongoDB, Redis or Valkey image
shows **Connection string** on Overview. **Copy** fetches the URI without revealing
it on screen; the eye control reveals it for inspection. Credentials are requested
only by those actions, through the same secret-access boundary as environment
variable reveals. Ordinary application responses contain only engine, host and port.
Compose applications and unrecognized/custom images do not show this control.

The URI addresses the active release's `site-<release-id>` alias on the instance's
shared Docker network, using its resolved container port. Other single-container
Shipper apps and Compose services attached to the edge network can use it in their
runtime environment. No host database port is opened. This is an internal Docker
address, not an address for a laptop or another server. Copy the current URI after
a replacement deployment or rollback; each release has its own address.

Credentials follow the official image's runtime initialization variables and
defaults. PostgreSQL defaults its user/database to `postgres`; MongoDB root users
use `authSource=admin`; MariaDB uses a MySQL-compatible URI. Redis/Valkey default
to database 0 without credentials. Custom start commands, file-based secrets and
Redis authentication/configuration mounts require manual connection details rather
than guessed passwords. Images do not apply changed initialization credentials to
an existing data directory; keep saved variables aligned with the database's actual
users and passwords.

Recognized database images receive only the directory ownership and user-switching
capabilities their official entrypoints need (`CHOWN`, `DAC_OVERRIDE`, `FOWNER`,
`SETGID`, `SETUID`). Other capabilities stay dropped, `no-new-privileges` stays
on, and unrelated images and task containers keep their existing restrictions.
Run `npm run test:database` for the opt-in real Docker connection round trip.
It creates isolated Redis and PostgreSQL application pairs, verifies their URIs across the shared network
without a public port, and removes only its labeled containers, volumes and network.
