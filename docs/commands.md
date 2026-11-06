# Commands and tasks

Phase 5 adds lifecycle automation without another daemon, queue, or database.
Commands run in application-owned containers, with the release's runtime
configuration and encrypted environment. They never run on the Shipper host.

## Everyday workflow

Set install/build/start commands in application configuration when detection is
not sufficient. Dockerfile install/build steps remain in the Dockerfile. The
optional **Detect commands** action reads supported package scripts and lockfiles
without executing repository code. Existing command overrides are preserved.

Set **Before deploy** under Deployment commands for a migration. It runs from the
candidate image before consumers start, including the first deployment. In
Compose, use the service tabs to edit **All services** or one service at a time.
An all-services command runs in every service container, and it runs before that
service's own command at the same stage. Required dependencies start first using
their saved Compose health/completion conditions. Cycles that would start the
consumer are rejected. Compose updates can have downtime, and container recovery
does not undo schema or data changes. Use a migration framework's database lock
when several applications share one database.

Other stages are under **More commands**:

| Stage | Contract |
| --- | --- |
| Before start | Runs inside the container before its foreground process on every real start, including Docker-managed restarts. Failure prevents the main process from starting. |
| After deploy | Runs after healthy promotion. Failure is recorded as a warning and failed task run; it does not undo the healthy release. |
| Before stop | Best-effort work for Shipper-controlled stop/restart, replacement, or deletion. Failure is recorded and shutdown continues. It is not a crash hook or a substitute for backups. |

The original image entrypoint and command are preserved unless Start command is
explicitly overridden. Startup hooks use a small static Linux helper installed in
a cached image layer. It replaces itself with the original foreground process,
so that process receives container signals directly. Docker restart policy still
belongs to the application. Startup output is in runtime logs; deploy/manual
commands have separate durable attempt records. Shipper does not need to be
running for an already-configured startup hook to execute.

## Command options

The default command format uses `/bin/sh -c`. **Argument list** accepts a JSON
array, for example `["/app/migrate", "up"]`, and does not require a shell. An image
without a shell fails shell mode explicitly. Argument lists do not expand shell
variables, pipes, or redirections.

Working directory and user default to the image. A command can override them.
Startup commands run under the container's security policy; an unprivileged main
user cannot elevate a startup command to root. Prefer a numeric UID:GID for
minimal images. Commands inherit the effective runtime environment, including
service overrides; build-only variables are not supplied to runtime jobs.

Timeouts are 1–3600 seconds, default 300. Job timeouts remove the owned job
container after saving its failed outcome. Running-service execution uses Docker
exec: a timeout or interrupted attachment can leave its process running, so the
result is marked **Uncertain** and the process is not automatically retried or
killed by guessing its PID. Inspect its effects before explicitly running again.
Deployment work has an overall 30-minute budget. Standalone manual/scheduled tasks
receive their configured timeout plus bounded setup time. Startup readiness waits
include the configured startup-hook deadline.

## Saved tasks

Open **Tasks**, choose **New task**, enter its name and command, then Save task.
**Run now** records a deployment operation and an immutable command snapshot.
Editing or deleting the definition does not change an already-queued run.
Disable a task to prevent both manual and scheduled launches.

A fresh job uses the active image, runtime environment, networks and declared
volumes. Its entrypoint is the command rather than the application's startup
hook. It has no public ports, HTTP route, restart policy, or application-inventory
entry. **Running service** executes in the selected owned live container instead.
Compose jobs can address their application's dependency services on its networks;
there is no automatic cross-application network access.

Run history records attempt ID, stage/service, start/end, state and exit status.
Select a run to see its timestamped deployment logs. The owning deployment records
the authenticated actor or `schedule`. Command definitions, queue snapshots,
lifecycle commands and run snapshots are encrypted with the installation key.
Known secret values are redacted before logs and errors enter SQLite.

## Schedules

Schedules are optional and use five fields: minute, hour, day of month, month,
weekday. Supported expressions include wildcards, numeric lists, ranges and
steps. Sunday is 0 or 7. Restricted day-of-month and weekday fields use cron's OR
rule. Names, seconds, `@every`, and extended cron syntax are not supported.

Use an IANA timezone, such as `Europe/Budapest`, and preview the next three runs.
Timezones are bundled so the production image needs no external timezone data.
Calendar iteration follows actual UTC minutes: skipped local times do not run,
and a repeated local time can occur twice during a DST transition.

**When busy** controls this application's active operation: skip this occurrence,
or retain one waiting occurrence. Waiting is persisted, including across control
plane restarts. **After downtime** skips missed occurrences or runs once to catch
up; it never queues a burst of every missed interval. A stopped application does
not launch scheduled jobs. An idempotency key per task and scheduled instant
prevents duplicate queue insertion. Editing a schedule recalculates its next run.

One leased worker processes deployment and task operations serially. Long tasks
can delay other operations. Schedules are checked while the worker is available,
with the selected missed-run policy applied afterward. There are at most 64 saved
tasks per application and the UI returns the newest 100 run records.

## Recovery and uncertainty

Each deploy hook gets one durable attempt per deployment/stage/service. A fresh
job has a deterministic owned container name. Its container identity is saved
before start, and recovery inspects that exact container's created/running/exited
state. Successful and failed attempts are not rerun during controller recovery.
If a previously recorded container disappears, or an exec outcome cannot be
proven, Shipper reports uncertainty. It does not promise exactly-once execution
across arbitrary external deletion or daemon failures.

Startup hooks follow actual container starts, not deployment attempt counts.
An OS/Docker restart can repeat a startup hook. Make startup hooks safe to repeat;
put once-per-deployment migrations in Before deploy.

## Editing configuration

Edits remain drafts until Review changes and Apply changes. New secret values
survive tab navigation in a bounded in-memory draft, are cleared on sign-out,
and are never written to session storage. Reloading the page discards unsaved
secret values. Review lists changed
fields without exposing secret values and identifies recreate versus rebuild.
A signed revision rejects stale reviews after another configuration change.

Runtime-only variables, start commands, health/readiness, memory, volumes and
lifecycle commands can reuse the active single-container image. Compose service
runtime overrides reuse pinned images and its normalized model. Changes that can
affect Compose interpolation/build inputs still rebuild; removing certain
inherited overrides also reloads the source so its original values are restored.
Source/build changes rebuild. Rename and domain edits retain their existing
metadata-only and routing-only operations. Redeploy always refreshes the source.

**Find missing variables** reads names from bounded `.env.example`, `.env.sample`,
`.env.template`, and `.env.dist` files. It does not import template values or
inspect a real `.env`. Enter the missing values before applying. A variable row’s **Generate value**
action creates a 256-bit random value, marks it secret and keeps it masked.

Service action menus provide start/stop/restart. Shipper checks transitive
Compose dependencies before controlled shutdown and refuses to stop a dependency
while its consumer is running. Start uses the active saved model and its declared
dependencies. Starting a service in a stopped application requires starting the
application first. A later deployment reconciles the complete application model.

## Build and validation

`npm run dev`, `npm run build`, `npm test`, and the Dockerfile generate and embed
startup helpers for Linux amd64 and arm64. If building directly with Go, run:

```sh
go generate ./internal/startup
go build ./cmd/shipper
```

Generated binaries are ignored by Git. Other container architectures receive an
explicit unsupported-helper error when a startup hook is requested.

```sh
npm run check
npm test
npm run test:e2e
npm run test:tasks
```
