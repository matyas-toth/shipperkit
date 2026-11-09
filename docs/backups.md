# Backups and recovery

Phase 8 adds instance recovery and protection for declared application data. The
same leased worker runs deployment, task, backup and restore operations. A backup
or restore blocks other operations on its application. It adds no daemon or queue
service.

## Protect an installation

1. Open **Settings → Recovery** and **Download key**. Save that file in a password
   manager or another location separate from both the VPS and its backup objects.
2. Create an **Instance recovery** backup plan. Choose a schedule if needed.
3. Under **Off-server storage**, configure an S3-compatible destination. Edit the
   plan's **Storage and retention** to keep local copies and upload encrypted copies
   there. A local-only backup cannot survive loss of the VPS.
4. Run the plan and download an archive, or retain the object in its S3 bucket.

An instance archive contains an online SQLite snapshot, the **original**
`encryption.key`, encrypted source/registry/database credentials, applications,
release/task/schedule definitions, instance identity, proxy configuration and ACME
state when present. The original key allows existing encrypted values to be read
after recovery. The separate age X25519 recovery identity decrypts the archive
without depending on a key on the lost host. Anyone with both can access the
installation's credentials.

Instance archives do **not** contain Docker images, BuildKit caches, Git working
copies, every volume or a database dump. Protect workload data with separate plans.
Retain pinned images/source revisions and any external secrets/files that the
application requires. A metadata backup cannot recreate an image that its registry
or source no longer supplies.

## Recover on an empty installation

Download the encrypted instance archive from local storage or its S3 destination
using separately retained storage access. Install the same Shipper release, stop
the control plane and choose a new, empty data directory:

```sh
shipper restore --archive instance.age --identity shipper-recovery-key.txt --data-dir /new/shipper
SHIPPER_DATA_DIR=/new/shipper shipper
```

The CLI authenticates the complete archive, verifies file hashes, checks SQLite
integrity and schema compatibility, and preserves the original instance/key. It
never overwrites a populated target directory. The recovery key is read from a
file, not a command argument or environment variable.

Sessions, provider flows, deploy tokens and captured push intents are revoked.
Pending deployment/backup work is interrupted. Applications start with desired
state **stopped**, task/backup schedules disabled and push auto-deploy disabled.
Sign in with the original administrator credentials, or use `reset-password`.
Review the recovered applications, restore their data, and deliberately restart or
redeploy them and re-enable schedules. Do not run the original and recovered
installation against the same Docker daemon or application storage.

Static proxy settings are retained in `proxy-recovery.json`; the dashboard
hostname is stored in SQLite and recovered with the database. Explicit `SHIPPER_*`
environment values take precedence. Update old host-specific bindings/upstreams
for the new machine. Recovered ACME state is installed privately into the owned
proxy data volume before the proxy starts. Conflicting existing certificate state
is rejected instead of overwritten. Public DNS/ports still need to reach the new
host. The managed installer and public dashboard/application acceptance are
complete for the current candidate; controlled renewal and broader host coverage
remain release gates.

## Protect application data

Use **Backups** on an application or an individual Compose service. Each plan has
one source, a destination and retention count. Connection and storage options are
collapsed; commands/passwords do not appear in history responses.

| Source | Supported format and consistency |
| --- | --- |
| PostgreSQL | Native `pg_dump` custom archive of one database, consistent snapshot, no ownership/ACL replay |
| MySQL | Native SQL dump of one database, global read lock with routines/events/triggers and binary data |
| MariaDB | Native `mariadb-dump`, with the same explicit global-read-lock boundary |
| MongoDB | Native `mongodump` gzip archive of one database, restored under a new namespace |
| ClickHouse, SQLite, Redis | Cold archive of the engine data volume, including numeric ownership and modes |
| Named volume | Cold archive of regular files, directories and internal relative symlinks, including numeric ownership and modes |

Database tools execute **inside the one selected Shipper-owned service** against
its loopback endpoint. Default users/passwords can come from the service's actual
`POSTGRES_*`, `MYSQL_*`, `MARIADB_*` or `MONGO_INITDB_*` environment. Explicit
overrides are encrypted at rest. Initialization environment values are not proof that a password remains
current; supply the current credential if it has changed. `_FILE` secrets and
custom database images may require explicit credentials. The selected engine must
match the native client tools available in that image. Confirm the engine in the
plan; backup failure is surfaced if it does not match.

Port and TLS mode are configurable. PostgreSQL uses `PGSSLMODE`, MySQL uses
`--ssl-mode`, and MariaDB uses its native SSL controls. Certificate verification
uses trust material available to the client inside that service. This does not
expose a database port to the Internet. MySQL caching-SHA authentication can require
TLS even on loopback; leave TLS enabled unless the server's authentication setup
explicitly supports an unencrypted connection.

MySQL/MariaDB dumps can briefly block writes. Pause schema changes during a native
dump. Live raw archives of database directories are **not** supported. For a cold
volume backup, stop every mounted writer through Shipper first so normal shutdown
hooks run. Shared/foreign mounts, running or paused containers, escaping links,
devices and hard links are rejected. Bind mounts, sockets, device files, extended
attributes and external volume-driver guarantees are outside this release's
archive boundary.

## Restore and switch deliberately

Select a completed backup in **History → Restore a copy**. Review its source and
enter the application name. Database copies require a new database name; an
existing database is never silently replaced. Database engine and native tool
version must match the archive. Native restore errors, version mismatches and
interruption fail the operation. A partially restored copy is retained for
inspection, and retrying cannot overwrite it.

A database restore runs a create/insert/select check in the restored copy. This is
a database check, not proof that every application query works. Inspect restored
records and test the application before switching its connection using the usual
**.env → Review changes** flow. The previous database remains available for
recovery. In-place destructive replacement and arbitrary raw SQL import are not
part of the supported restore boundary.

Volume restore creates a **new** owned retained volume and compares its file
contents, modes, owners and links against the archive. Select **Attach**, choose
the application's logical volume, then review the configuration change. Native
and Compose applications retain stable logical volume names. The source volume
is preserved. Shipper rejects ownership mismatches and never adopts unrelated
Docker storage. Retained volumes, including those left by deleted applications,
are discoverable in Settings. **Delete data** is a separate named confirmation,
rejects any attached volume and protects volumes referenced by a live application's
recorded releases. Ordinary cleanup/application deletion never removes data.

**Import backup** accepts the encrypted Shipper `.age` format, including archives
downloaded from S3. Supply its recovery key if it differs from the current
instance's key. The archive's application identity must match; general cloning or
cross-application import is later parity. Imported archives never provide trusted
connection credentials; use the matching local plan or explicit restore inputs.
Restore only archives from a trusted source. Age authenticates encrypted contents,
not the identity of their creator; instance state and native database dumps can
contain executable behavior. Database account/global privilege recreation and
missing extensions remain explicit requirements of the target database image.

An absent local copy of a recorded S3 backup is retrieved from its recorded target
and checked against its ciphertext checksum before decryption.

## Scheduling, retention and results

The shared picker covers minute intervals, hourly, daily, selected weekdays and
monthly schedules. It shows a copyable read-only cron expression and the next three
runs in the selected IANA timezone. **Custom cron** preserves unsupported patterns
verbatim. Five-field cron uses UTC-minute iteration: missing DST times are skipped,
repeated times occur twice; a restricted day-of-month and day-of-week use cron OR
semantics. Monthly days that do not exist are skipped. The preview is the actual
backend schedule calculation. Backup schedules collapse missed runs into one
attempt and skip occurrences while the application is busy.

Plans retain 1–100 completed backups; the default is seven. Expiration runs only
after a new successful backup and removes only recorded objects beneath this
instance's prefix. Failed/interrupted attempts are retained as history. A failed upload removes its
new local artifact so scheduled failures cannot fill the disk with unretained
archives; previous completed recovery points are preserved. Deleting a
plan preserves its existing archives. Destination targets are captured per run;
changing a bucket/prefix does not redirect deletion of historical objects. Rotating
credentials for the same endpoint/bucket updates access to those objects.

Archives have a 2 GiB plaintext inventory limit and a 30-minute operation deadline.
Temporary files are private, streaming bounds avoid loading whole dumps into RAM,
and interrupted worker staging is cleaned on reacquisition. The archive format is
versioned, authenticated with age, and validates hashes, inventory, entry types and
paths before restoring. S3 uses AWS SigV4, HTTPS by default and no redirects.
Private endpoints/HTTP require explicit opt-in. ETags are never treated as content
checksums. Provider bucket policies/encryption/lifecycle remain operator-controlled;
give Shipper access only to its chosen backup prefix.

History distinguishes **Backed up**, an **Uploaded** off-server copy, and restore
verification. The latest failure and overdue/freshness information stay in context.
Phase 9 adds transition notifications and observed backup freshness through the
shared [operations](operations.md) collector/outbox.

## Validation

`npm run test:backups` creates isolated real Docker fixtures for PostgreSQL 17,
MySQL 8.4 and MariaDB 11, verifies restored application-shaped reads/writes and
unchanged originals, cold-volume contents/ownership/modes, and instance/key recovery.
Unit tests exercise corruption, truncation, wrong keys, unsafe paths/links,
encrypted credential storage, worker exclusion, offline recovery and signed S3
round trips/tampering/redirect rejection. Browser tests cover real metadata backup
and downloads, schedule editing, S3 configuration, mobile layout and existing Tasks.
Fixtures only remove resources with their own random instance identity.

Cloud-provider interoperability, real domain recovery after a host rebuild, and
the planned Contabo distribution matrix still require the real-system acceptance
lab. Public dashboard ACME issuance has passed on staging. Redis-family policies,
generic SQL imports, application cloning and Coolify migration remain P8.3 later
parity.

## Phase 9 local sources and automation

Version 0.9.0 instance bundles include referenced immutable local sources in an
integrity-checked inner archive. Recovery verifies every referenced source digest,
revokes operations tokens, interrupts maintenance and disables notifications and
automatic cleanup. Application image transport is still separate from source/state
recovery; rebuilding a local source no longer depends on a lost developer machine.
Use a matching archive/schema version for offline recovery, then follow the tested
upgrade path. The new operations outbox reports backup failure/recovery and overdue
protection; see [operations](operations.md).
