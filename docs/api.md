# HTTP API, Phase 1

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
| GET | /containers | Cookie | Projected container inventory and Docker connection health |
| GET | /applications | Cookie | Persistent owned application records and their containers |
| GET | /system | Cookie | Curated engine, database, and process information |
| GET | /events | Cookie | SSE inventory invalidations |
| POST | /refresh | Cookie + CSRF | Queue coalesced reconciliation; return 202 |

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

The API intentionally has no deployment or container mutation endpoints yet.

