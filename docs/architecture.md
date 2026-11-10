# Architecture

Shipper is one Go control-plane process with an embedded SolidJS SPA, SQLite WAL,
an official Docker SDK adapter, durable deployment jobs, and authenticated SSE.
Apache site containers and Traefik run independently on the Docker host.

## Boundaries

- `cmd/shipper`: configuration, lifecycle, local password recovery.
- `internal/store`: transactional SQL migrations, accounts, roles, sessions,
  provider identities and OAuth state, observed snapshots, application
  configuration, deployments, logs, and worker lease.
- `internal/auth`: bcrypt, random capabilities/session tokens, TOTP and PKCE helpers,
  constant-time checks.
- `internal/engine`: official SDK inventory/event projection, instance filtering.
- `internal/inventory`: debounced reconciliation, reconnect, connection health.
- `internal/deploy`: hardened public Git checkout, Apache archive/image strategy,
  candidate lifecycle, proxy adapter, durable worker, recovery and deletion.
- `internal/events`: bounded nonblocking SSE invalidations.
- `internal/httpapi`: authenticated REST contract, role gates, TOTP and provider
  sign-in, CSRF/origin checks, domain probes.
- `internal/ui`: embedded production assets and SPA fallback.
- `web/src`: persistent shell, cached routed application views, virtual tables/logs.

## Persistence and identity

SQLite uses WAL, foreign keys, busy timeout, and one pooled connection. Migrations
are transactional and additive. Application source (`git`) and method (`static`, `dockerfile`, `railpack`, `auto`)
remain separate from each other and from observed containers. Multiple containers
share one application ID. Snapshot replacement and state aggregation commit
together; a failed Docker read retains the last complete snapshot.

Each installation persists an instance ID. Mutations verify both instance and
application ownership. New Docker resources also carry deployment IDs, making
candidate/active/retired releases distinguishable. Legacy ownership-labeled
applications remain inspectable. Unrelated workloads, infrastructure containers,
and other installations' resources are excluded from the dashboard.

An application's desired configuration may differ from its serving release after
a failed update. Runtime routing points to the active deployment's immutable image/port snapshot.
Domain policies are independent authoritative routing metadata with unique claims;
route-only jobs reserve both old and proposed hosts until acknowledgment or rollback. Deletion leaves a
tombstone, preventing late inventory events from recreating the application.

## Accounts and collaboration

The first setup capability creates the first administrator. Further accounts are
only created by an admin. The `administrators` table stores the immutable user ID,
case-insensitive email, optional password hash, role (`admin`, `developer` or
`viewer`), disabled flag, temporary-password flag and last sign-in time. There is
no public registration path. Migration 014 rebuilds the original single-owner
table and preserves sessions, factors, recovery codes, terminals and operation
tokens.

Provider identities live in `user_identities`, keyed by provider and stable
subject with one identity per provider for each account. A new identity is
resolved by verified email to exactly one active provisioned account and is
bound only after every onboarding gate passes. Once bound, the stable provider
subject wins even if the provider later reports a different address. Provider
configuration is stored in settings; client secrets are sealed with the instance
key and never returned by an API.

OAuth authorization uses a hashed one-use state with a ten-minute expiry, an
HttpOnly SameSite state cookie and PKCE for Google and Discord. Callbacks are
constructed from the canonical public origin, never request headers. Google ID
tokens are verified against cached JWKS keys and checked for issuer, audience,
expiry, nonce and verified email. GitHub identities use the numeric user ID and
the authenticated email endpoint. Discord requires a verified email. Login
creates a restricted session until a required password change and factor
enrollment or verification finish.

Role and account checks are server-side and evaluated on every request. Disabling
or removing an account deletes its sessions, operation tokens and terminal
sessions. A transaction guard rejects role, password, provider or account changes
that would leave no active administrator with a working sign-in method. Account
changes, provider changes, factor changes and recovery are written to the bounded
account audit table without credentials, OTPs or tokens.

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

The dashboard hostname is a SQLite setting (`dashboard_domain`) read by the proxy
adapter and the HTTP origin policy. Saving it from Settings updates the setting,
reserves the hostname, updates the server-side canonical URL, and queues the same
serialized proxy repair used by route reconciliation. On restart the setting is
authoritative, so the dashboard remains on its domain without a Compose variable.
Upgrades from the earlier environment-based setup seed this setting from the old
`SHIPPER_DOMAIN` value before the old environment is removed.

## Authentication and isolation

First-admin creation requires a local capability and is atomic. Passwords use
bcrypt; session tokens are stored only as SHA-256 hashes. Cookies are HttpOnly,
SameSite Strict, and Secure on HTTPS requests. Mutations require JSON,
same-origin requests, and CSRF tokens. Expensive password work and mutation rates
are bounded. Local password recovery revokes all sessions.

The management listener defaults to loopback for local and manual runs. The
managed installer publishes it on `0.0.0.0:8080` for first-run claim and recovery,
then introduces the dashboard domain through Settings and Traefik. The listener
does not trust forwarded host headers for origin checks; it accepts the configured
canonical origin or the direct request origin, and uses the forwarded protocol
only as a transport-security hint for Secure cookies and HSTS. Remote Docker
requires verified TLS. A mounted Docker socket grants host-level control even when
mounted `:ro`; keep the management plane private. The Git and publish isolation
policy is documented in [static site operations](static-sites.md).

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
The managed installer publishes the control plane on 8080, binds Traefik on 80
and 443, and stores the dashboard domain in SQLite. Native development uses Go,
Node, Git, and Docker CLI/Buildx; runtime Docker packaging includes Git and Docker
CLI/Buildx.

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

## Full application extensions

Phase 3 reuses the durable queue for deploy, domains, start, stop, restart and
Delete operations. The application stores authoritative domain policies and a
desired running/stopped state separately from immutable deployment configuration.
Domain changes wait for proxy acknowledgment without replacing the serving image.

BuildKit has a private instance-owned daemon/cache and private Buildx configuration.
The CLI handles the BuildKit client protocol; inventory/lifecycle/ownership checks
use the official Docker SDK. Railpack prepares a pinned plan in an unprivileged
helper and builds it through a matching frontend. Runtime image configuration is
preserved while Shipper adds ownership labels. Secret mounts keep build values out
of host command arguments. Variables use AES-GCM at rest; API masking, explicit
reveal and runtime/deployment log redaction are separate boundaries. See
[Phase 3 decisions](phase-3-decisions.md) and [operations](full-applications.md).

## Compose applications

The reference compose-go loader validates and normalizes a source with a closed
interpolation environment. Referenced paths are checked before environment files
are opened. The official CLI performs builds and project orchestration; SDK
checks enforce resource ownership and populate portable file volumes. Project
identity stays fixed for the lifetime of an application. Runtime plans use pinned
image IDs and volume names, are AES-GCM encrypted in SQLite, and do not appear in
public deployment responses. A failed build preserves the serving stack. Apply
failures attempt to restore the previous model without touching persistent data.
The service UI addresses stable Compose service names rather than container IDs.
See [Compose operations](compose.md) for stateful update and isolation boundaries.

## Lifecycle automation

Migration 007 stores encrypted task definitions and attempt snapshots, operation
actor/service identity and a pending scheduled occurrence. These share the leased
application operation queue. Prepared jobs persist their container identity before
start and recover by inspecting the same owned container. Docker exec has no
atomic completion record, so ambiguous recovery is surfaced instead of retried.

The small `cmd/shipper-start` Linux executable is generated for amd64/arm64 and
embedded compressed in the control plane. Only applications with a Before start
hook receive its cached layer and wrapped entrypoint. It enforces the hook deadline
and execs the original process. It adds no resident Shipper sidecar. Successful
job cleanup retains declared volumes. Startup hook logs remain runtime logs.

Reviewed applies classify changes from the active release, reuse safe immutable
image/model snapshots, and verify a signed desired-config/active-release revision.
Public diffs contain field names, never values. Source suggestions use the existing
public Git boundary and inspect bounded regular files without executing source.


## Data protection

Migration 010 adds encrypted backup plans, immutable credential/destination snapshots
and durable attempts. The existing renewable worker lease serializes execution;
transactional application locks cover deployment, task and protection queues. No
new runner service is introduced. Binary database output streams through the Docker
SDK to private bounded staging rather than deployment log buffers. Native adapters
run inside the selected owned service, while cold file archives use temporary owned
helpers with no network.

Versioned age X25519 bundles include a checked SHA-256 file inventory. SQLite uses
VACUUM INTO for a consistent online snapshot, and instance archives retain the original
encryption key and certificate state. The independently saved recovery identity
decrypts a lost-host archive. S3 uses the official AWS SigV4 signer and streaming
requests with explicit private-endpoint policy and no redirects. Restore authenticates
and validates the whole archive before any workload mutation. Database copies and
new retained volumes preserve originals; reviewed configuration changes perform the
application switch. Offline instance recovery only installs into an empty directory
and pauses schedules, push automation and captured work. See [operations](backups.md).

## Phase 9 shared operations

`internal/operations` collects bounded Docker/data-filesystem facts and delivers
transition notifications inside the existing control plane. Its observation loop
belongs to the deployment worker's renewable lease and stops before that lease is
released. Durable maintenance records use the same serialized worker, rather than
introducing another daemon or execution queue service. Observation transitions and
the notification outbox share a transaction. Deployment state remains independently
recoverable; periodic reconciliation repairs the latest observed condition.

`internal/cli` is a thin client of allowlisted `/api/v1` handlers. Expiring hashed
operations tokens carry capabilities and optional application scope. Session UI
and automation share validation, source/configuration review, idempotency and job
execution. Sensitive command projections use retention metadata, like masked
variables, so a permitted configuration round trip does not accidentally remove
commands the caller cannot read.

Immutable local-source adapters reuse existing builders and Compose loading.
Archive authentication/ownership, regular-file/path bounds, quotas and content
digests precede use. Retained inputs participate in instance recovery. Source
cleanup and operation enqueue recheck references under serialized SQLite writes.
The diagnostic playbook guides investigation; deterministic code retains all
execution, permission and data-consistency decisions. See [the supported
operations boundary](operations.md).
