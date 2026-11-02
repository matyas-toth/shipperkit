# Roadmap

## Phase 1: Foundation

- [x] Go HTTP API using chi
- [x] SolidJS/TypeScript/Vite SPA with Solid Router and TanStack Query
- [x] SQLite WAL persistence and transactional migrations
- [x] First-admin capability, hashed passwords, sessions, CSRF, local recovery
- [x] Official Docker Engine SDK integration
- [x] Separate application source/method model and explicit container ownership
- [x] Container event subscription, reconnect, reconciliation, authenticated SSE
- [x] Embedded production frontend and development runner
- [x] Non-root development Docker image and Compose packaging
- [x] Unit/integration tests and isolated browser tests

The foundation is complete. Unrelated workloads are excluded from the application
dashboard and remain outside Shipper's mutation boundary.

## Phase 2: Static sites

Public Git -> Apache runtime image -> Docker -> Traefik -> HTTP/HTTPS.

- [x] Public HTTPS Git checkout with recorded commit SHA
- [x] SQLite deployment queue, exclusive application operations, worker lease
- [x] Persisted state transitions, history, incremental logs, restart recovery
- [x] Digest-pinned Apache base and deployment-specific image versions
- [x] File-based, SPA fallback, strict routing, opt-in `.htaccess` allowlist
- [x] Candidate HTTP checks and acknowledged proxy promotion
- [x] Failed deployment preserves the serving release
- [x] Traefik file provider, persistent routes and certificate volume
- [x] Custom domain validation, DNS/reachability checks, ACME resolver configuration
- [x] Confirmed, ownership-scoped deletion without removing application volumes
- [x] Application dashboard, multi-container detail, creation/configuration/history UI
- [x] Tabler outline icons with individual tree-shaken imports
- [x] Real public-Git/Docker/Apache/Traefik HTTP and trusted local TLS tests
- [x] Desktop/mobile browser tests, 600 applications, incremental log pagination

Public ACME issuance is configured but needs a reachable domain for live
verification. The local suite uses a generated trust anchor without disabling
certificate validation.

## Phase 3: Full applications

- [x] Isolated instance-owned BuildKit with private Buildx configuration
- [x] Dockerfile and version-matched Railpack frontend builds
- [x] Automatic method detection and explicit override
- [x] Repository subdirectories, branch suggestions, commands and uncached rebuild
- [x] Runtime port detection, custom HTTP health paths and resource limits
- [x] Encrypted variables, masked responses, explicit reveal, build secret mounts
- [x] .env paste, safe session drafts, independent rename
- [x] Persistent named volumes and durable start/stop/restart
- [x] Bounded authenticated runtime log streaming with filtering/copy
- [x] Multi-domain route-only jobs, primary/redirect aliases and www expansion
- [x] Per-domain HTTPS and persisted verification
- [x] Real Dockerfile/Railpack and static regression coverage
- [x] Desktop/mobile editing and large-list/log browser checks
- [x] Live deployment duration, timestamped logs and per-deployment line numbers
- [x] Static serving state, legacy runtime visibility and readable container names

Git provider login/private repositories and signed push webhooks remain planned.
Public ACME issuance needs the reachable VPS/domain check in Phase 5.

## Phase 4: Compose

- [x] Reference Compose loader and official CLI orchestration
- [x] Git/pasted YAML, file discovery and project-directory selection
- [x] Stable project identities and preserved named/external volumes
- [x] Repository build contexts, env files and portable config/file mounts
- [x] Multi-service status and focused service logs, .env and settings
- [x] Per-service multi-domain routing, HTTPS and verification
- [x] Build-before-update, pinned release recovery and scoped lifecycle actions
- [x] Encrypted pasted source/resolved plans and inherited variable editing
- [x] Real PostgreSQL persistence and desktop/mobile browser coverage

See [Compose operations and supported boundary](compose.md). Stateful updates can
involve downtime; restoring containers does not reverse database migrations.

## Phase 5: Release hardening

Disposable Linux VPS integration suite, TLS/ACME staging, backups, upgrades,
installer/distribution decisions, resource measurements, and security review.

