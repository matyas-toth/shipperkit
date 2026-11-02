# Phase 3 implementation decisions

Full application deployment extends the existing application model and durable
worker. Public Git remains the source, while static, Dockerfile and Railpack are
independent deployment methods. Compose execution remains Phase 4.

BuildKit runs in an explicitly instance-owned container with its own cache volume.
Buildx uses a private client configuration and the remote docker-container
transport, so no BuildKit port is published and no existing/default builder is
changed. Ordinary Docker inventory and mutations continue through the official
SDK. The Docker CLI is used only for the supported BuildKit client protocol.

Railpack prepare runs in a bounded helper container without a Docker socket or
host mounts. Its version and frontend are pinned together. Build commands execute
inside BuildKit; repository commands never execute in the control-plane process.
Build contexts exclude Git metadata and environment/key files, reject escaping
links, and honor Docker's ignore rules after the platform's own filtering.

Runtime port, health path, start command, memory limit, variables and named volumes
belong to application configuration. Candidates join the proxy network without
published ports. Health checks run from the proxy so images do not need wget or
curl. Failed build/health/promotion retains the old serving release. Lifecycle
operations are durable and application-scoped, and never remove data volumes.

Variables are encrypted in SQLite using a private per-installation AES-GCM key.
Secrets are masked in ordinary API responses, never saved in browser drafts, and
redacted from deployment/runtime logs. Explicit authenticated reveal is separate.
Build secrets use BuildKit secret mounts. Application configuration snapshots and
the encryption key must be backed up together.

Domains become independent routing metadata with unique host claims. Route-only
jobs persist changes and rollback state, atomically publish the full Traefik
configuration, and wait for acknowledgment without rebuilding/replacing the app.
Each hostname has its own HTTPS, redirect and verification controls. The existing
single-domain API fields remain compatible. An optional www expansion uses the
public suffix list rather than counting dots. Per-service routing fields are
reserved for Compose; ephemeral container IDs do not become configuration keys.

The frontend keeps DESIGN.md tokens and the persistent shell. This phase adds
public branch selection, method suggestions with explicit override, build fields,
masked variables with .env paste, runtime logs, draft preservation without
secrets, independent rename, and multi-domain editing. There is no provider-login
UI, placeholder private-repository support, webhook setup, or public installer.

Validation uses isolated SQLite and Docker fixtures, real BuildKit Dockerfile and
Railpack deployments, a failed build and unhealthy candidate, routing changes
without image/container replacement, encrypted-secret round trips, runtime-log
streaming, session/restart persistence, and desktop/mobile browser flows.
