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

The Phase 1 interface is an inventory and settings dashboard. Existing workloads
remain read-only, and no deployment controls are presented before they work.

## Phase 2: Static sites

Public Git -> Apache runtime image -> Docker -> Traefik -> HTTP/HTTPS.
Add durable deployment jobs, logs, domain validation, candidate health checks,
failure preservation, and safe deletion.

## Phase 3: Full applications

Railpack/BuildKit, Dockerfile builds, build configuration, runtime variables,
ports, and streaming logs.

## Phase 4: Compose

Official Compose execution, Git/pasted YAML, multi-service state, persistent
volumes, service logs, and safe updates.

## Phase 5: Release hardening

Disposable Linux VPS integration suite, TLS/ACME staging, backups, upgrades,
installer/distribution decisions, resource measurements, and security review.

