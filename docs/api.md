# HTTP API

All routes are same-origin and prefixed with `/api`. JSON errors have the shape:

```json
{"error":{"code":"unauthenticated","message":"Sign in to continue"}}
```

| Method | Route | Authentication | Result |
| --- | --- | --- | --- |
| GET | /health | Public | Process liveness |
| GET | /auth/session | Optional cookie | `setupRequired`, `user`, and authenticated `csrfToken` |
| POST | /auth/setup | First-run setup capability | Create the singleton administrator and session |
| POST | /auth/login | Email/password | Create a session |
| POST | /auth/logout | Cookie + CSRF | Revoke current session |
| GET | /containers | Cookie | Shipper-owned container projection and Docker connection health |
| GET | /applications | Cookie | Persistent owned application records and their containers |
| GET | /system | Cookie | Curated engine, database, and process information |
| GET | /events | Cookie | SSE inventory invalidations |
| POST | /refresh | Cookie + CSRF | Queue coalesced reconciliation; return 202 |
| GET | /applications/{id} | Cookie | `application` and newest 100 `deployments` |
| POST | /applications | Cookie + CSRF | Create application and enqueue deployment atomically; return 202 |
| POST | /applications/{id}/deployments | Cookie + CSRF | Redeploy, optionally update `name`/`config`; return 202 |
| POST | /applications/{id}/delete | Cookie + CSRF | Confirm by `confirmName`, enqueue route/container removal; return 202 |
| GET | /applications/{id}/deployments/{deployment}/logs?after=0 | Cookie | Up to 500 ordered log lines after the sequence cursor |
| POST | /applications/{id}/domain-check | Cookie + CSRF | DNS, reachability, release identity, and TLS verification |
| GET | /proxy | Cookie | Proxy ports, bind address, ACME email and CA directory |
| POST | /repositories/branches | Cookie + CSRF | Public repository `items` and `default` branch |
| GET | /domain-options?hostname=example.com | Cookie | `canIncludeWWW` using the public suffix list |
| POST | /variables/parse | Cookie + CSRF | Parse `{content}` into variable `items` |
| POST | /applications/{id}/variables/reveal | Cookie + CSRF | Reveal saved `{name}` as `{value}` |
| POST | /applications/{id}/name | Cookie + CSRF | Save `{name}` without deploying |
| POST | /applications/{id}/domains | Cookie + CSRF | Queue `{domains, includeWWW?}` without rebuilding |
| POST | /applications/{id}/lifecycle | Cookie + CSRF | Queue `{action: start|stop|restart, service?}` |
| GET | /applications/{id}/logs?container=ID&tail=200 | Cookie | Runtime SSE stream |

POST bodies require `Content-Type: application/json`. Setup accepts
`email`, `password`, and `setupToken`; login accepts `email` and `password`.
Passwords must be between 12 and 72 UTF-8 bytes. Session cookies are HttpOnly,
SameSite=Strict, and Secure when the canonical public URL uses HTTPS.
Sessions expire after seven days and are not extended by background traffic.

Authenticated mutations need `X-CSRF-Token` from the session response.
All supplied Origin headers must match the canonical public origin or direct
request origin. Cross-site Fetch Metadata is rejected. No CORS policy is enabled.
Login/setup/refresh are rate-limited by the direct peer and route.

The container projection includes ID, name, image, state, human Docker status,
health, creation time, published/internal ports, ownership, and Compose grouping.
Environment values and arbitrary Docker labels are excluded.
`/system` also exposes the stable, non-secret `instanceId` for ownership checks.

`docker.observedAt` is the last successful snapshot time, while `checkedAt`
includes failed checks. When `connected` is false, inventory is cached and may
be stale. A successful empty inventory is represented as `items: []`.

SSE messages are small invalidations, not complete inventory records:

```text
id: 12
event: inventory
data: {"revision":12}

```

Fetch the relevant resource again on every event and stream reconnection.
Revisions are process-local and are not a durable replay log. The stream sends an
initial invalidation and periodic heartbeats. A bounded fan-out disconnects
slow clients, and session revocation/expiry is checked on heartbeats.

## Static application creation

```json
{
  "name": "Website",
  "config": {
    "repository": "https://github.com/mdn/beginner-html-site.git",
    "branch": "",
    "publishDirectory": ".",
    "routing": "files",
    "htaccess": false,
    "domain": "",
    "https": false
  }
}
```

An empty name uses the repository name. An empty branch uses the remote default.
An empty domain allocates `<application-id>.localhost` over HTTP. `routing` is
`files`, `spa`, or `strict`. Public HTTPS needs an enabled ACME resolver.

Creation and redeploy accept `Idempotency-Key` (1–128 letters, digits, `._:-`).
Replay returns the original operation; reusing a key for a different request
returns `409 idempotency_conflict`. One unfinished operation per application is
allowed; competing operations return `409 operation_in_progress`. Domain claims
cover both desired configuration and the currently serving release.

The 202 response is a deployment resource with a stable `id`, `applicationId`,
`operation`, `state`, configuration snapshot, commit SHA, image/container IDs,
timestamps, and error. Follow the Location header to the application and refetch
on SSE invalidations. Operations outlive the HTTP request. A shutdown leaves
unfinished work resumable; it does not imply cancellation.

Log responses contain `items: [{sequence, number, step, line, createdAt}]` and `state`.
Use the last returned sequence as `after` until a page has fewer than 500 items,
then wait for the next invalidation. Logs contain at most 10,000 lines per job,
with each line bounded to 2 KiB. Text is rendered without HTML interpretation.

## Full application configuration

Creation and redeploy additionally accept `method` (`auto`, `static`, `dockerfile`,
`railpack`), `rootDirectory`, `dockerfile`, `installCommand`, `buildCommand`,
`startCommand`, `port`, `healthPath`, `memoryMB`, `variables`, and `volumes` inside
`config`. Missing method retains the static API default for older clients. The
dashboard sends `auto` explicitly. Redeploy supports top-level `noCache: true`.
Deployments expose `resolvedMethod` and `runtimePort`. Application detail also
includes `activeDeployment` separately from the newest 100 operation records.

A variable is `{name,value,secret,scope}` with scope `runtime`, `build`, or `both`.
Secret values are empty in ordinary responses and carry `keep: true` and
`hasValue: true`; round-trip `keep: true` to retain the saved value, or send
`keep: false` and a new value. An empty value is a valid replacement. Remove an
entry to delete it. Include `keepFrom` with the original name to rename a masked
saved variable while keeping its value. At-rest encryption applies to all values. A volume is
`{name,path}` with a stable lowercase name and an absolute non-system container path.

An application carries authoritative `domains` plus `desiredState`. A domain is
`{id,hostname,https,primary,redirect,verificationState?,message?,checkedAt?}`.
POST domains accepts the complete list (including an empty list). `includeWWW`
optionally contains root-domain IDs to expand server-side. POST domain-check
accepts `{domainId}`; omission checks the primary hostname. Legacy `config.domain`
and `config.https` mirror the primary hostname. Routing changes are `domains`
operations with their own queue/history record and do not replace `activeDeployment`.

Runtime SSE uses `reset`, `log` (`{line}`), `failure` (`{message}`), `end`, and
heartbeat events. It checks container/application/instance ownership, permits
tails of 1–500, masks exact secret values from desired and serving configuration,
and closes on session expiry/revocation. It has no durable cursor; reconnect reads
a fresh Docker tail. Container streams never accept arbitrary Docker identifiers.

Deletion accepts `{"confirmName":"Website"}`. It withdraws routing, removes
only this instance's application containers, and soft-deletes the application
record. Image history, proxy data, and application volumes are retained.
There are no arbitrary container IDs or Docker command endpoints.


Deployment log `sequence` remains a global pagination cursor. `number` is the
one-based line number within that deployment, including when records from other
jobs are interleaved. It remains stable while filtering and paging. The UI shows
local datetime before each log message. Deployment duration includes queue time,
from `createdAt` to `finishedAt`, or the live client clock for ongoing operations.
Applications expose `activeMethod` for the resolved method of the serving release.
Deployment `applicationName` is the name snapshot used for generated container
names; independent application renaming does not change an in-progress build's
recovery identity.

## Compose

`config.method` accepts `compose`; `composeSource` is `git` or `paste`.
Git uses the existing repository/branch plus `composeFile` and `rootDirectory`.
Paste uses `composeYAML`. `composeKeep` retains saved YAML on ordinary edits;
ordinary responses replace source with retention metadata. `sourceKind` is
`compose` for pasted applications and `git` for repository applications.

`POST /api/compose/preview` accepts `{config, applicationId?}` and returns
`{services, files, composeFile}` without creating Docker resources. Supplying an
application ID allows retained secret values to participate in interpolation.
`POST /api/applications/{id}/compose/source` explicitly reveals pasted source.
Both require authentication, CSRF and the normal request rate limits.

Deployment `services` contains names, source images, HTTP ports, health paths,
volume references, and retained environment metadata. It excludes resolved plans
and inherited environment values. `config.services` supplies overrides by name:
`port` (omitted inherits, 0 means internal), `healthPath`, `memoryMB` (0 inherits), `variables`, and
`unsetVariables`. Inherited retained variables resolve against the active encrypted
plan during an edit. Variable reveal accepts optional `service` alongside `name`.

Domains add `service`; Compose domains require a deployed HTTP service. Primary
and redirect policies are independent per service. Application `.env` supplies
interpolation, while service overrides change that service's runtime environment.
Compose and single-container methods cannot be switched within an existing app.

## Lifecycle automation and reviewed changes

All routes below require a session; POST additionally requires CSRF.

| Method | Route | Contract |
| --- | --- | --- |
| POST | /repositories/inspect | `{config}` returns method/command suggestions and env-template names without executing source code. |
| GET | /applications/{id}/deployments/{deployment} | One owned deployment, including task operations outside the newest-100 history window. |
| POST | /applications/{id}/changes | `{config}` returns `{operation, changes, revision}`; secret values are excluded from the change list. |
| POST | /applications/{id}/deployments | `{config, revision}` enqueues a reviewed `apply`; a stale revision returns 409. Without a revision, existing redeploy behavior is preserved. |
| GET | /applications/{id}/tasks | `{items, runs}` with saved definitions and newest 100 attempts. |
| POST | /applications/{id}/tasks | Create/update a task; omit `id` to create. |
| POST | /applications/{id}/tasks/{task}/run | Snapshot its command into an owned queued operation; return 202. |
| POST | /applications/{id}/tasks/{task}/delete | Delete its definition and retain execution history. |
| POST | /applications/{id}/schedule-preview | `{schedule, timezone}` returns the next three Unix-second instants. |

A command has `shell` or `argv`, optional `service`, `directory`, `user`, `timeout`
and `mode` (`job` or `exec`). Config `hooks` contains `{stage, command}` entries;
stages are `before_deploy`, `before_start`, `after_deploy`, and `before_stop`.
`startupPolicy` is `http` or `process`; process-only workers have no public domains.
Before deploy/start uses job mode. Defaults and limitations are documented in
[commands and tasks](commands.md).

A saved task adds `name`, `enabled`, optional five-field `schedule`, IANA `timezone`,
`overlap` (`skip` or `queue`), and `missed` (`skip` or `once`). Responses include
`nextAt`. Runs expose `id`, `deploymentId`, stage, command snapshot, state,
container/exec identity, exit code (`-1` when unknown), timestamps and a redacted
error. Their deployment logs use the existing cursor endpoint. Deployment records
include actor, optional task ID and optional service identity. Definitions are
application-scoped; an ID from another application returns 404.
