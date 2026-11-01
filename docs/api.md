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

Log responses contain `items: [{sequence, step, line, createdAt}]` and `state`.
Use the last returned sequence as `after` until a page has fewer than 500 items,
then wait for the next invalidation. Logs contain at most 10,000 lines per job,
with each line bounded to 2 KiB. Text is rendered without HTML interpretation.

Deletion accepts `{"confirmName":"Website"}`. It withdraws routing, removes
only this instance's application containers, and soft-deletes the application
record. Image history, proxy data, and application volumes are retained.
There are no arbitrary container IDs or Docker command endpoints.

