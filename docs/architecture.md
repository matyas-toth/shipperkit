# Architecture

Phase 1 is a single Go process with an embedded SolidJS SPA, SQLite metadata,
a read-only Docker SDK adapter, a reconciliation loop, and authenticated SSE.
Deployment creation and lifecycle mutations begin in later phases.

## Boundaries

- `cmd/shipper`: configuration, process lifecycle, local password recovery.
- `internal/store`: versioned SQL migrations, administrator/session persistence,
  observed container snapshots, and independently persisted application identities.
- `internal/auth`: password hashing, random tokens, setup token lifecycle.
- `internal/engine`: the official Docker SDK and a narrow inventory/events interface.
- `internal/inventory`: bounded, coalesced reconciliation and connection health.
- `internal/events`: bounded nonblocking fan-out with slow-client disconnection.
- `internal/httpapi`: authenticated REST resources, CSRF/origin enforcement, SSE.
- `internal/ui`: embedded production assets with SPA fallback.
- `web/src`: persistent shell, routed pages, query cache, shared UI primitives.

SQLite uses WAL, foreign keys, a busy timeout, and one pooled connection.
Migrations run transactionally before serving HTTP. Snapshot replacement and
application reconciliation commit together. A failed Docker request preserves the
last complete snapshot and marks it stale. A successful empty snapshot clears
the inventory. Application records survive container disappearance.

Container ownership is explicit: `io.shipper.managed=true` and
`io.shipper.application.id=<stable-id>`. Discovery never adopts an unlabeled
container. Phase 1 performs no Docker mutations. Container environment values,
arbitrary labels, registry credentials, and raw inspect responses are not exposed.

## Authentication

First-admin setup needs a random token stored in the private data directory.
Creating the singleton administrator and consuming setup access is atomic.
Passwords use bcrypt. Random session tokens are stored only as SHA-256 hashes.
Sessions have an absolute expiry, HttpOnly/SameSite=Strict cookies, and a separate
CSRF token. HTTPS public URLs enable Secure cookies. State-changing browser
requests validate the origin and require JSON; authenticated mutations also
require the CSRF header. Rate limiting applies before expensive password work.
Password recovery requires local access and revokes every existing session.

The default listener is loopback. Reverse-proxy operators must set the canonical
public URL. Forwarded headers are not trusted. Docker TCP connections require TLS
verification; unauthenticated plaintext endpoints are rejected outside tests.
Mounting a Docker socket read-only does not make its API read-only. The socket
still grants host-level power, so keep this management interface private.

## State and event flow

Docker event stream -> short debounce -> complete inventory -> SQLite transaction
-> small SSE invalidation -> TanStack Query refetch -> targeted Solid updates.

Subscribe before the first list, relist on reconnect, and periodically reconcile
to repair missed events. Reconnect with bounded exponential backoff and jitter.
Events are invalidations rather than a second database. Reconnection and missed
SSE delivery trigger a fresh snapshot. Slow browser subscribers are disconnected
instead of blocking Docker or growing an unbounded queue.

The UI retains cached rows during refresh, reports stale/disconnected state, and
uses a fixed-height virtual window for large inventories. Local search/filtering
does not wait for the network. The shell survives route transitions. No timers
poll Docker from the browser.

## Future deployment boundary

Source (`git`, `compose`, eventually `image`) is independent from build method
(`static`, `railpack`, `dockerfile`, `compose`). The initial application schema
stores these separately; Phase 1 does not pretend it can deploy them.

Phase 2 will add SQLite-backed operations with transitions:
queued -> cloning -> building -> starting -> verifying -> succeeded.
Cancellation and failure are terminal, with explicit recovery after interruption.
Keep the healthy container until the candidate passes verification, and never
remove persistent volumes during ordinary updates.

Build strategies should implement a small image-producing interface only when
the first static-site pipeline needs it. Compose should delegate to its official
implementation. Traefik routing should be isolated behind domain configuration
and use ownership labels. Neither is provisioned during Phase 1.

## Local and release packaging

Vite builds into the Go embed directory. Production has no Node server.
The development runner starts Go and Vite with same-origin API proxying.
The multi-stage Dockerfile builds both and runs a non-root, filesystem-restricted
control plane. Compose binds the UI to loopback, persists only Shipper metadata,
and maps the Docker socket group. Distribution scripts and a hosted installer
are deferred until the local foundation is proven.

## Validation

Go tests use isolated SQLite directories and an HTTP Docker API fixture through
the real SDK. Browser tests cover setup, login, filtering, drawers, settings,
CSRF, event-driven updates, and reconnect states. A separate opt-in real-Docker
smoke test creates and removes only its own labeled container.

Real Linux VPS deployment, TLS/ACME, build failure recovery, durable build jobs,
and upgrade migrations remain later-phase work. Memory and navigation targets
are aspirations until measured on representative hardware.

## Reference

Coolify's ownership labels, status aggregation, and preservation of ordinary
Docker workloads are useful concepts. Shipper's code is original; its control
plane uses the Docker Engine API directly rather than adopting Coolify's stack.
See the upstream [repository](https://github.com/coollabsio/coolify) and
[Docker SDK examples](https://docs.docker.com/reference/api/engine/sdk/examples/).

