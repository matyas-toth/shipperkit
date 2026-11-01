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
certificate validation. Build commands, Railpack, and Compose execution remain
in their respective later phases.

## Phase 3: Full applications

Railpack/BuildKit, Dockerfile builds, build configuration, runtime variables,
ports, and streaming logs.

## Phase 4: Compose

Official Compose execution, Git/pasted YAML, multi-service state, persistent
volumes, service logs, and safe updates.

## Phase 5: Release hardening

Disposable Linux VPS integration suite, TLS/ACME staging, backups, upgrades,
installer/distribution decisions, resource measurements, and security review.

