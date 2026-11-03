# Roadmap

Phases 1–5 and the Phase 6 1.0 core are implemented. Later milestones follow
the [Coolify parity audit and acceptance milestones](../feature_parity.md).
Capability parity is an eventual goal; optional integrations must not enlarge the
default application workflow or require extra platform services.

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

GitHub private repositories and signed push webhooks are available in Phase 6.
Provider login remains separate from connecting deployment sources.
Public ACME issuance needs the routing and reachable VPS/domain checks in Phases 7 and 10.

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

## Phase 5: Lifecycle automation and configuration UX

- [x] P5.1: durable lifecycle tasks, first-deploy migration jobs, startup hooks
- [x] P5.2: saved manual tasks, schedules, timezone/overlap policies and run history
- [x] P5.3: pending changes, masked diffs, runtime-versus-build change planning
- [x] Preserve existing log, .env, static-status, naming and navigation QoL
- [x] Verify task recovery, failure ordering, secret redaction and no-shell images

Exit: migrations complete before consumers start, failed tasks block the new
release, restarts honor the correct hooks, and everyday edits apply with the
smallest required operation. See the P5 contract in the parity audit and [commands and tasks](commands.md).

The supported boundary is explicit: one leased operation worker, bounded logs,
startup attempts in runtime logs, and uncertainty for unprovable exec outcomes.
Compose interpolation/build-input edits conservatively rebuild.

## 1.0 release boundary

Phase 10 is the 1.0 release gate for a dependable single-server platform. It is
not a requirement to finish every breadth milestone below. Within Phases 6–9:

| Phase | Required for 1.0 | Later parity |
| --- | --- | --- |
| 6 | GitHub private repositories, repository/revision pickers, signed push deployments, predictable cancel/retry, image workflow | Additional Git providers, PR previews, broad source/build compatibility |
| 7 | VPS/dashboard HTTPS and renewal, common domains/routing, per-service controls, explicit previous-release recovery | Wildcards/custom certificates, broad proxy/Compose/device/network compatibility |
| 8 | Shipper/key backup and recovery, declared supported application-data backups and tested restores | Every database adapter, Coolify import, broad portability features |
| 9 | Health/resource diagnostics, actionable notifications, scoped automation API | Full CLI/MCP, file browser, log-drain/channel/traffic breadth |
| 10 | Tested Linux install, update, reboot, ACME renewal, security/recovery, performance and documentation | Provider/catalog/collaboration/remote-server breadth |

Phase 5 is required in full. Teams, remote servers, scaling and the optional
integration/catalog work in Phases 11–14 remain after 1.0. A feature must pass its
acceptance scenario before it counts toward the release gate.

## Phase 6: Sources, images and Git automation

- [x] GitHub App/token connections, private repository and revision pickers
- [x] Direct images, inline Dockerfiles, registry authentication and digest recording
- [x] Signed push webhooks, durable coalescing, path filters and pause controls
- [x] Application-scoped deploy tokens, cancellation and pinned retries
- [x] Isolated Docker source workflows and desktop/mobile browser checks
- [ ] Later parity: GitLab/SSH/private Git hosts, submodules, LFS, publishing
- [ ] Later parity: isolated trusted PR previews and optional provider sign-in

The 1.0 core is implemented. See [sources and triggers](sources.md) for behavior
and the verification boundary: real GitHub onboarding/private-clone and public
webhook delivery still need a reachable installation and an operator account.
Full P6 parity, especially preview isolation, is not claimed.

## Phase 7: Routing, Compose compatibility and release control

- [ ] P7.1: public/dashboard HTTPS, generated domains, certificate diagnostics
- [ ] DNS-01/wildcards/custom certificates, scoped Traefik settings and maintenance
- [ ] Controlled TCP/UDP exposure, cross-app networking and optional managed DNS
- [ ] P7.2: Compose profiles/files, safe includes/extends, build secrets/contexts
- [ ] Explicit trusted-workload options for needed host mounts, devices and GPU
- [ ] P7.3: manual release rollback, comparison, retention, readiness and draining

Exit: advanced settings are validated and reviewable, route edits remain lightweight,
operator rollback works, and Compose's stateful update limits remain explicit.

## Phase 8: Data protection, databases and portability

- [ ] P8.1: instance/storage backups, S3 copies, retention and verified recovery
- [ ] P8.2: database connection/TLS controls and engine-aware backup/restore adapters
- [ ] Start with PostgreSQL and MySQL/MariaDB, then extend supported engines
- [ ] P8.3: portable export/import, stopped config clones and Coolify migration mapping
- [ ] Retained-volume reattachment and separate, explicit persistent-data deletion

Exit: restore a disposable application and its data, recover an installation with
its original encryption material, and prove that ordinary cleanup preserves data.

## Phase 9: Operations, observability and agent interfaces

- [ ] P9.1: managed-service terminal/files, focused metrics and connectivity diagnostics
- [ ] P9.2: event outbox, notifications, optional log forwarding and bounded retention
- [ ] Favorites/tags, command palette, log controls, stable navigation and bulk previews
- [ ] P9.3: scoped versioned API, Go CLI, JSON output and permission-checked MCP adapter
- [ ] Measure per-feature resource cost and keep optional collectors disabled by default

Exit: an operator or agent can diagnose and act through the same authorized
operations, with bounded logs/metrics and no unrelated-container access.

## Phase 10: Production release hardening

- [ ] Disposable Linux VPS, public ACME renewal, reboot and failure matrix
- [ ] Architecture/engine compatibility and reproducible resource/performance checks
- [ ] One-command setup after local workflows are stable, pinned updates and recovery
- [ ] Account/session controls, 2FA/recovery, security and dependency review
- [ ] Versioned API/schema, examples, migration/recovery guides and release documentation
- [ ] Data-preserving uninstall and a documented support boundary

Exit: install, deploy, back up, reboot, update, recover and uninstall on a tested
host. This is the single-server release gate, not a claim of complete Coolify parity.

## Phase 11: Collaboration, environments and reusable configuration

- [ ] Invitations and simple roles with one implicit workspace for solo installations
- [ ] Optional groups, application variants, linked values and propagation previews
- [ ] Passkeys/OIDC and additional OAuth adapters
- [ ] Doppler/Infisical/Vault references, rotation and team isolation
- [ ] Expanded audit and permission tests across API, terminal, tasks and data

Exit: collaboration works without adding mandatory hierarchy to a solo user's flow.

## Phase 12: Remote targets, build servers and scale

- [ ] P12.1: SSH targets, host-key validation, connection profiles and scoped inventory
- [ ] P12.2: remote builds, registry transport, capacity and explicit fallback policy
- [ ] P12.3: stateless replicas/multi-target releases, target results and external LB checks
- [ ] Data-aware clone/move and encrypted, single-owner instance transfer
- [ ] Dedicated isolation targets for untrusted preview/CI execution

Exit: remote execution preserves ownership and reports partial failure correctly;
adding replicas does not imply database or control-plane high availability.

## Phase 13: Optional catalog and integration breadth

- [ ] Small tested recipes plus an on-demand catalog and Coolify recipe translation
- [ ] Named notification/log-drain adapters and opt-in traffic analytics
- [ ] Cloudflare tunnels, provider provisioning/cloud-init and optional host patch jobs
- [ ] Dedicated GitHub Actions runner integration
- [ ] Alternate proxy handoff and lazy-loaded relationship/canvas views
- [ ] Per-recipe/provider support declarations instead of broad unsupported parity claims

Exit: enabling an optional capability does not add dependencies or workflow steps
for installations that do not use it.

## Phase 14: Exact legacy and specialist compatibility

- [ ] Optional pinned Nixpacks compatibility builder and migration guidance
- [ ] Explicitly tracked legacy Swarm adapter, separate from ordinary Compose/replicas
- [ ] Remaining concrete runtime/device/network/storage and proxy compatibility cases
- [ ] Field-by-field Coolify import reports with unresolved gaps kept visible

Exit: specialist fixtures work with declared backend versions while the ordinary
single-server product keeps the same small operational footprint.

## Progress and parity rules

A planned field or adapter is not completion. Each capability needs a support
boundary and an acceptance scenario, linked by its F-ID in
[feature_parity.md](../feature_parity.md). Preserve the Q-ID QoL register as phases
ship. Re-audit a pinned Coolify release before claiming current parity, and keep
native workflows, partial support and Compose workarounds distinct.
