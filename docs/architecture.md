# Architecture

Shipper is one Go control-plane process with an embedded SolidJS SPA, SQLite WAL,
an official Docker SDK adapter, durable deployment jobs, and authenticated SSE.
Apache site containers and Traefik run independently on the Docker host.

## Boundaries

- `cmd/shipper`: configuration, lifecycle, local password recovery.
- `internal/store`: transactional SQL migrations, identities, sessions, observed
  snapshots, application configuration, deployments, logs, and worker lease.
- `internal/auth`: bcrypt, random capabilities/session tokens, constant-time checks.
- `internal/engine`: official SDK inventory/event projection, instance filtering.
- `internal/inventory`: debounced reconciliation, reconnect, connection health.
- `internal/deploy`: hardened public Git checkout, Apache archive/image strategy,
  candidate lifecycle, proxy adapter, durable worker, recovery and deletion.
- `internal/events`: bounded nonblocking SSE invalidations.
- `internal/httpapi`: authenticated REST contract, CSRF/origin checks, domain probes.
- `internal/ui`: embedded production assets and SPA fallback.
- `web/src`: persistent shell, cached routed application views, virtual tables/logs.

## Persistence and identity

SQLite uses WAL, foreign keys, busy timeout, and one pooled connection. Migrations
are transactional and additive. Application source (`git`) and method (`static`)
remain separate from each other and from observed containers. Multiple containers
share one application ID. Snapshot replacement and state aggregation commit
together; a failed Docker read retains the last complete snapshot.

Each installation persists an instance ID. Mutations verify both instance and
application ownership. New Docker resources also carry deployment IDs, making
candidate/active/retired releases distinguishable. Legacy ownership-labeled
applications remain inspectable. Unrelated workloads, infrastructure containers,
and other installations' resources are excluded from the dashboard.

An application's desired configuration may differ from its serving release after
a failed update. Routing always uses the active deployment's immutable snapshot.
Domain reservations cover both configurations so a failed domain change cannot
allow another application to steal the still-serving hostname. Deletion leaves a
tombstone, preventing late inventory events from recreating the application.

## Durable operations

Application creation/configuration and job enqueue are one transaction. A partial
unique index allows at most one unfinished operation per application. Optional
request keys provide idempotent retries. A renewable SQLite lease fences workers
sharing a metadata database; a replacement waits for an expired lease.

```text
queued -> cloning -> packaging -> starting -> checking -> promoting -> succeeded
   \---------------------------------------------------------------> failed
queued -> deleting -> succeeded / failed
```

One worker processes the persisted queue. Stages, commit SHA, image/container IDs,
previous release, timestamps, and text logs are persisted. Shutdown leaves the
current stage resumable. Names/tags are deterministic within a deployment, and
recovery checks ownership before reusing side effects. A changed branch during
incomplete packaging fails explicitly rather than relabeling a different commit.

The static strategy creates a bounded, filtered tar archive of the publish root
and a generated Apache configuration. It copies these into a stopped, networkless
container based on a digest-pinned Apache image and commits a versioned image
through the official SDK. This avoids invoking a builder or executing repository
code for plain static files. There is no dependency on BuildKit in Phase 2.

The candidate runs without host ports, host mounts, root privileges, or Linux
capabilities. An HTTP check exercises the real root page, including `.htaccess`.
Promotion persists intent, atomically publishes the complete Traefik routing
snapshot, and waits for proxy acknowledgment. Only then is the previous container
removed. Clone/package/health failures leave the active release untouched; failed
routing attempts restore its active pointer and configuration.

Deletion withdraws routing first, then removes only ownership-verified containers,
then soft-deletes the application. Volumes and image/history records are retained.
There is no automatic volume deletion, image pruning, stateful rollback, or
zero-downtime guarantee.

## Proxy and certificates

Traefik uses its file provider, with a persistent instance-owned Docker volume for
routing and ACME data. The worker uploads a temporary file and atomically renames
it inside the watched directory. JSON is emitted as valid YAML with a `.yaml`
extension; empty router/service maps are omitted for Traefik's decoder.

Traefik needs no Docker socket. Its API is reachable only inside the proxy
container and is used through the Docker SDK to verify routing. Only HTTP/HTTPS
entry points are published. Static proxy configuration is fingerprinted and an
unexpected configuration change fails explicitly instead of being silently ignored.

Local defaults bind to loopback and allocate `.localhost` HTTP domains. Public
domains can use the configured ACME HTTP-01 resolver. Public DNS, reachable ports
80/443, and a valid email are operator prerequisites. Domain verification pins
public DNS addresses, forbids redirects/private destinations, verifies TLS
normally, and checks the active release's response identity. A ready route and a
publicly issued certificate are separate states.

## Authentication and isolation

First-admin creation requires a local capability and is atomic. Passwords use
bcrypt; session tokens are stored only as SHA-256 hashes. Cookies are HttpOnly,
SameSite Strict, and Secure under a canonical HTTPS URL. Mutations require JSON,
same-origin requests, and CSRF tokens. Expensive password work and mutation rates
are bounded. Local password recovery revokes all sessions.

The management listener defaults to loopback and does not trust forwarded headers.
Remote Docker requires verified TLS. A mounted Docker socket grants host-level
control even when mounted `:ro`; keep the management plane private. The Git and
publish isolation policy is documented in [static site operations](static-sites.md).

## Realtime and perceived speed

Docker events -> debounce -> complete snapshot -> SQLite -> SSE invalidation ->
cached query refetch -> targeted Solid updates. Subscription precedes the initial
list. Reconnect and periodic reconciliation repair missed events. Slow SSE clients
are disconnected rather than blocking writers. Cached rows remain during refresh.

Deployment stages and logs use the same invalidation stream. Logs are fetched
incrementally by cursor, with a small polling fallback. SQLite bounds lines per
job, and the UI renders a virtual window. Application search/filtering happens
locally across cached records; list rows retain identity and keyboard focus during
live updates. The frontend uses Tabler Solid components with tree-shaken imports.

## Packaging and validation

Vite produces embedded assets; production requires no Node server. Docker builds
both layers and runs the control plane as UID/GID 65532 with a read-only root.
Native development uses Go, Node, and Git; runtime Docker packaging includes Git.

Tests cover auth/origin/CSRF, additive migrations, worker/domain exclusivity,
ownership, failure preservation, recovery, inventory events, and bounded streams.
Browser tests cover application grouping, creation, logs, errors, domain and
deletion flows, mobile layout, 600 applications, and paginated logs. Opt-in Docker
tests exercise public Git, real Apache/Traefik routing, trusted local TLS, proxy
restart, and scoped deletion. Public ACME issuance and host reboot/upgrade tests
still need a disposable Linux VPS and dedicated DNS.

Coolify was studied for concepts such as ownership and workload preservation;
Shipper includes no Coolify source. References: [Coolify](https://github.com/coollabsio/coolify),
[Docker SDK](https://docs.docker.com/reference/api/engine/sdk/examples/),
[Traefik file provider](https://doc.traefik.io/traefik/reference/install-configuration/providers/others/file/),
[Apache overrides](https://httpd.apache.org/docs/2.4/howto/htaccess.html).
