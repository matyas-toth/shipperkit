# Operations and local deployments

1.0.0-rc.6. The dashboard and focused Go CLI use the same
application identities, validation, reviewed configuration and durable worker.
No model, MCP server, separate collector or additional database is required.

## Diagnostics

Application Overview prioritizes availability, the public address and an action
when attention is needed. Source, runtime and health details expand on request.
Current health checks are separate from recorded deployment results and show
their own timestamps. Serving state is independent of the latest deployment
result, so a failed attempt can coexist with a live previous release. Failed
deployments offer a structured diagnostic context export for external agents.
Runtime and deployment logs support pause/resume,
search, filtered copy and text download; retained client windows remain bounded.
Opening a Compose service from Overview keeps its log connection across unrelated
inventory refreshes, just like selecting that service in the Logs tab.

The leased worker owns a bounded observation loop: 15-second intervals, a
12-second cycle budget, and at most 32 container inspections/stats per cycle with
rotation. Samples carry their own times, age and a stale flag after 60 seconds.
Unavailable statistics remain unobserved. CPU needs two valid samples; memory is
Docker's reported usage, not an estimate of free host RAM. Inspect verifies exact
instance/application ownership before projecting state, exit code, restarts or OOM.

The data-filesystem check measures the filesystem containing Shipper's metadata.
It does not claim to measure a remote Docker daemon's disk or the Docker Desktop
VM. Low space warns below 512 MiB or 10%; a fresh sample below 128 MiB blocks new
build preparation. Builds remain serialized by the existing worker.

Compose dependency checks compare fresh running-service network membership from
the recorded plan; completed one-shot dependencies use their completion result.
A shared network is a prerequisite, not proof of a successful TCP/database
transaction. Application protocol health needs declared readiness checks.
Route/TLS checks retain their observation time and control-plane probe vantage.
Independent external availability and installed boot-service readiness require
the Phase 10 VPS acceptance work. Unknown checks never become synthetic passes.

## System, notifications and retention

System shows daemon/proxy observations, data-filesystem headroom, runtime versions,
recovery access, notification delivery state and expiring/revocable access tokens.
Recheck requests inventory reconciliation. Repair proxy reviews current effects,
queues work through the existing leased worker, checks for a changed review and
reuses normal proxy ownership/configuration validation. It preserves application
containers and certificate storage; it cannot run arbitrary host commands or
repair unsupported Docker/OS configurations.

Webhook or email delivery is optional. Settings/credentials are encrypted at rest;
ordinary reads hide the webhook URL and SMTP password. Webhooks require HTTPS by
default, pin DNS resolution at connection time, reject private addresses unless
explicitly permitted and never follow redirects. SMTP uses trusted TLS on 465 or
STARTTLS on 587. Test delivery is deliberate. Live mailbox/provider acceptance is
separate from local transport tests.

Observation transitions and their outbox records commit together. Unchanged
conditions refresh their timestamps without sending another alert. Distinct
operations remain distinct events; bounded batches and five attempts with minute
backoff prevent unbounded delivery loops. Delivery is at least once, so webhook
receivers can deduplicate by event sequence. Disabled/unselected notifications
are consumed instead of replayed when the channel is later enabled. Incident
history retains at most 10,000 entries and no more than 30 days.

Cleanup is reviewed, queued and revalidated immediately before execution. Daily
eligible cleanup is explicitly opt-in. It covers abandoned uploads older than a
day, provably orphaned owned images older than a day and deployment logs older
than 30 days. Serving-release logs, the newest ten operations per application and
uncertain/interrupted task evidence are retained. All recorded image references,
containers, volumes and certificates remain protected. Image deletion is withheld
while Compose history makes image references ambiguous. No global Docker prune
or direct manipulation of Docker log files occurs. Existing per-instance BuildKit
GC retains its declared 256 MB reserve / 4 GB usage / 1 GB free-space targets;
this is a cache policy, not proof of reclaimed host bytes. SQLite log deletion
reuses database pages rather than promising an immediate filesystem shrink.

## Focused CLI

Build the existing Go executable. In System, create an access token with the
needed capabilities and application scope. CLI connection accepts HTTPS, or HTTP
only on loopback. Tokens go through stdin or the environment, never a command
argument. Browser cookies are not reused.

```sh
shipper connect https://shipper.example.com --token-stdin
shipper apps --json
shipper plan . --name shop --method dockerfile --domain shop.example.com
shipper deploy --plan PLAN_ID --wait
shipper doctor shop --json
shipper logs shop --operation OPERATION_ID --after 0 --json
shipper watch shop --operation OPERATION_ID --timeout 10m
```

`deploy .` can plan and submit in one operation under the supplied token's prior
deployment authority. `plan` followed by `deploy --plan` provides a review boundary.
An optional `--config` JSON file supplies existing application configuration fields;
it is not a mandatory new deployment language. Compose files are detected for a
new automatic local deployment. `--app` selects an existing application; an exact
name/ID must resolve unambiguously. `--worker` uses process readiness. The first
deployment remembers a local project association outside the repository. Use
`--new` to create another application from the same directory. `--domain` sets a
new application's hostname; change existing multi-domain policies through Domains
or the scoped domains API. Revoking a token blocks further requests and stream
heartbeats; it does not cancel already authorized durable work.

`inspect`, `doctor`, operation `watch`, bounded deployment logs, runtime logs with
`--service`, saved-task execution and reviewed recorded rollback are supported.
For rollback, first use `rollback APP --operation RELEASE_ID`, then apply with
the returned `--revision` and `--apply`. A task runs with `task APP TASK_ID`.
Runtime logs use Docker's bounded tail/SSE format. Full CLI coverage, additional
contexts, general terminal/files and MCP remain later breadth.

Requests return operation IDs immediately. `--wait` watches separately; a timeout
or disconnection does not cancel the operation. Applying the same saved plan uses
a stable idempotency key, including after promotion changes the serving revision.
Changed inputs and stale reviews fail explicitly. JSON results go to stdout;
structured errors go to stderr and failures exit nonzero.

Credentials, plans and project associations live under the user's configuration
directory in `shipper/`, with private directory/file modes where supported by the
OS. `SHIPPER_CLIENT_DIR` overrides that location. `SHIPPER_TOKEN` temporarily
overrides a saved credential without silently replacing it when associations save.
Protect that directory with the account's normal filesystem access controls.

## Immutable local source

Local files are another optional source beside Git, images and pasted definitions.
The dashboard can choose/review a folder; the CLI can package its working tree.
No Git account or push is needed. Uploads are authenticated, serialized with other
archive operations, bounded to 64 MiB of content / 10,000 regular files and stored
with an identity, digest and provenance. CLI plans preserve executable bits;
browser folder uploads use regular 0644 files and portable 100-byte tar paths.

Default exclusions include Git metadata, dependency directories, `.env` variants,
SSH/cloud credential directories, private keys and PEM files. `.env.example`,
`.env.sample` and `.env.template` remain source files. Review selected files;
filename rules do not establish that arbitrary source content is secret-free.
Use Shipper variables/build secret mounts deliberately. Symlinks, devices, unsafe
paths and duplicate archive entries are rejected. CLI `.shipperignore` adds
simple path globs, one per line; it does not claim full Git/Docker ignore syntax.
`.dockerignore` continues to apply at Docker build time. Browser upload applies
the built-in exclusions and displays its selected file list.

The instance permits up to 1,000 snapshots, stopping new uploads at 896 MiB of
recorded archive usage so a final bounded upload still fits the recovery budget.
Referenced snapshots remain retained for recorded releases. Unreferenced uploads
become eligible after 24 hours; completed release history is never silently pruned
to make room. Transfer interruption discards partial files; retry uploads a new
validated snapshot. Range-resuming partial uploads is not implemented.

Apply and worker retries materialize the reviewed snapshot and verify its digest;
editing local files afterward cannot change it. Successful local releases remain
visible and operable in the dashboard. Source settings can explicitly switch back
to Git and enable provider automation. Instance recovery includes referenced local
snapshots and verifies them before accepting a restore. Restoring revokes operations
tokens and disables notification delivery/automatic cleanup until reviewed.

See [API contracts](api.md), [diagnostic playbook](agent-diagnostics.md),
[recovery](backups.md) and [release acceptance](vps-test-plan.md).

### Discord webhooks

Paste a Discord channel webhook URL into **System > Notifications > Webhook**.
Shipper recognizes official Discord webhook URLs automatically, including versioned
API paths and thread query parameters. Test and queued notifications send a readable
message with the event state, summary, application and reference. Messages respect
Discord's 2,000-character limit and disable all automatic mentions.

Other webhook destinations retain the versioned `{ "version": "1", "event": ... }`
JSON contract. Private destination opt-in, redirect rejection, retries and encrypted
endpoint storage apply to both formats. Slack and Teams payloads are not inferred.
