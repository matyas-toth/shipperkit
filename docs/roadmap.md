# Roadmap

Updated 2026-11-05. Phases 1–9 core are implemented in 0.9.0, including
shared diagnostics/operations, a focused CLI and immutable local-project deployment.
The next deliverables are the mandatory UI refactor and a tested installation and
server-care workflow. Phase 10 remains the single-server 1.0 release gate.

The [Coolify parity audit](../feature_parity.md) tracks eventual capability breadth.
A smaller, dependable 1.0 takes priority over completing every parity milestone.
The planned [VPS test lab](vps-test-plan.md) and [account/authentication contract](authentication.md)
record the real-system acceptance resources and the future access model.
Implemented code, locally verified behavior, and public-VPS acceptance are separate
statuses; GitHub onboarding/private checkout/public push delivery and public ACME
issuance/renewal still require external acceptance before release.

## Product and operating principles

The target experience is: buy a supported Linux VPS, run one installer command,
open Shipper, connect a source, and deploy. Routine deployment and server care
should happen through Shipper without requiring Docker commands or repeated SSH
sessions. This is planned behavior, not the current installation experience.

The lightweight, information-dense Coolify alternative and agent-accessible
deployment environment are complementary product paths. The dashboard remains a
complete, fast way to perform supported everyday work by hand. A developer or
external coding agent should also be able to take an ordinary local project to a
verified, recoverable deployment without first opening the dashboard or connecting
a Git provider. Both paths use the same application model and operation contracts;
neither requires an AI subscription, embedded model or new infrastructure hierarchy.

- Keep one implicit local server and one application model. An application can
  contain web services, workers and databases without becoming several resources.
- Manage Shipper applications and the infrastructure required to run them. Do not
  adopt, expose or remove unrelated containers and data.
- Keep Dockerfiles and Compose as ordinary, editable application definitions.
  Support agent-authored definitions through the same validation as human input;
  retain source detection, maintained examples and optional recipes for people
  deploying by hand. A large catalog is not a prerequisite for broad app support.
- Let external agents inspect projects, propose configuration and investigate
  evidence. Shipper deterministically validates, executes, verifies and recovers
  operations; deployment ordering and data protection never depend on a model.
- Give UI, API, CLI and eventual MCP clients shared identities, permissions,
  reviewed effects and durable results. Adapters must not introduce separate
  deployment engines or let a token bypass application ownership checks.
- Treat HTTPS as built-in infrastructure: seed the ACME contact automatically from
  the first administrator email, then handle issuance, renewal and recovery without
  a separate email field, certificate upload or recurring operator task.
- Keep access admin provisioned. There is no public signup or automatic account
  creation through OAuth; the token-protected first-admin bootstrap is the exception.
- Automate dependency setup, networking, HTTPS, boot recovery, declared backups,
  eligible cleanup and tested updates within an explicit ownership boundary.
- Keep privileged host operations narrow and permission checked. Application tasks
  execute inside their workloads, never through an arbitrary host shell API.
- Preserve Go, SQLite, the embedded SolidJS frontend and existing Docker/proxy
  components. Reuse durable operations, schedules and events instead of adding a
  daemon, queue or monitoring platform for each feature.
- Make perceived speed an acceptance criterion: cached navigation, stable layouts,
  bounded data, incremental logs, immediate queue feedback and background work.
- Make agent interactions efficient too: bounded summaries first, selective detail,
  stable machine-readable output and operation watches instead of repeated full
  inventory/log dumps. Collect only useful history within explicit resource budgets.
- Follow DESIGN.md and AGENTS.md. Use components suited to the decision, concise
  labels and progressive disclosure. Do not compensate for confusing layouts with
  taglines or walls of helper text. Shared validation and operation contracts must
  make the same workflow understandable to users, contributors and agents.

Shipper can take responsibility for routine host setup and care. The operator
still owns the VPS account, billing, capacity decisions, domain ownership and
application/data compatibility. Provider outages and unsupported host repairs
need explicit diagnostics and a documented recovery path. DNS changes require
operator action or a separately connected provider integration.

### Complementary journeys and scope

- Human: install, claim the administrator account, open the dashboard, select a
  source, review relevant essentials and deploy; configure and operate everything
  in application/service context without writing prompts or installing a CLI.
- Developer/agent: securely connect to a supported installation, inspect or submit
  a local project, review effects, deploy, watch the operation and verify its result.
  P10 adds bootstrap of a fresh authorized VPS through the same installer.
- Keep the existing source/provider, routing, recovery, collaboration, remote-host,
  specialist compatibility and QoL backlog. The new path changes priority and
  shared foundations without removing those commitments.
- Prioritize a few complete workflows before broad CLI/MCP coverage. Specialized
  data-consistency/restore adapters remain necessary even when agents can generate
  database services. State clearly which recovery formats are actually supported.
- Embedded AI chat, model hosting, arbitrary generated dashboard scripts and a
  custom micro-VM/isolate runtime are outside the 1.0 core. Optional integrations
  can extend the same contracts later without adding idle services by default.

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

1.0 must support the complete lifecycle of a dedicated, supported Linux VPS:
install, deploy, configure, observe, back up, reboot, update, recover and uninstall.
The operator should normally use Shipper for that lifecycle. Exceptional recovery
can still require SSH or a provider console and must be documented.

| Milestone | Required for 1.0 | Later parity |
| --- | --- | --- |
| 5 | Lifecycle commands, saved/scheduled tasks and reviewed changes | Broader command inference and adapters |
| 6 | GitHub private source/revision selection, signed push, cancel/retry and image workflows, with real-account acceptance | Additional providers, SSH/private Git hosts, previews and broad build inputs |
| 7 | Common per-service routing/runtime controls, recorded rollback and VPS/dashboard HTTPS | Wildcard/custom certificates, general proxy overrides and specialist Docker/Compose/network compatibility |
| 8 | Recoverable Shipper state/key/certificates, supported application-data backups, off-server copies, retention and tested restores | Every database adapter, general portability and Coolify migration |
| 9 | Focused diagnostics/evidence, quiet actionable alerts, safe retention/server care, scoped operations API, a focused CLI and immutable local-project deployment | Full CLI/MCP coverage, file browser, broad collectors/channels/traffic analytics and bulk operations |
| 9.5 | Adaptive creation flow, meaningful choice components, contextual settings, guided schedule picker and UI/performance regression checks | Broader optional organisation; mandatory hierarchy remains excluded |
| 10 | One-command installation and narrow authorized SSH bootstrap, first-admin ACME setup and built-in HTTPS, TOTP/enforcement, reboot recovery, tested updates/maintenance, human/agent acceptance, recovery and data-preserving uninstall | Universal Linux support, VPS purchasing/provisioning, remote fleets and general host administration |

Phase 9.5 is mandatory even if every backend feature already works. Phase 10 is
an acceptance gate, not just publishing an installer. Teams, remote targets,
scaling and integration/catalog breadth in Phases 11–14 remain after 1.0.
A setting or adapter does not count as complete until its acceptance scenario passes.

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

- [x] Packaged VPS/dashboard HTTPS route, automatic ACME renewal configuration
- [x] DNS/TLS/identity verification and observed certificate issuer/expiry
- [x] Per-domain path/strip-prefix routing, response headers, compression, maintenance
- [x] Per-service CPU/PID/restart/startup/stop controls, single-container traffic drain
- [x] Reviewed recorded-release rollback, config comparison and retained-artifact checks
- [x] Preserve current domains/data and skip deployment migrations on restoration
- [ ] Public ACME issuance/renewal acceptance on a real VPS remains a Phase 10 gate
- [ ] Later parity: wildcard/custom certificates, raw proxy plans, managed DNS
- [ ] Later parity: TCP/UDP exposure, cross-app networks, profiles/multiple files
- [ ] Later parity: trusted host mounts, devices/GPU, build secrets/extra contexts

The 1.0 core is implemented within the [routing and release support boundary](routing-and-releases.md).
Compose retains stateful reconciliation and pinned failure recovery; data/schema
reversal is not implied by rolling back containers. Specialist compatibility and
full P7 parity remain open. Static proxy setting changes require explicit
container recreation while preserving certificate storage.

## Phase 8: Data protection and recoverable databases

Outcome: an application and its data, and the Shipper installation managing them,
can be recovered on a fresh host. Database rollback remains distinct from restoring
an application release.

### P8.1: Instance and storage recovery, required for 1.0

- [x] Versioned backup of SQLite state, original encryption material, source/registry
  credentials, task/schedule definitions and relevant proxy certificate state
- [x] Portable encrypted recovery bundle with separately held recovery material;
  restoring it must not depend on an encryption key available only on the lost VPS
- [x] Local and S3-compatible off-server destinations, integrity checks, bounded
  archive size, retention and deliberate download/reveal of sensitive material
- [x] Backup eligible application volumes/files with a declared consistency policy;
  stop writes or use native backup operations when a live archive would be unsafe
- [x] Backup status, freshness, failures and restore actions in application/service
  context; use the shared schedule picker from P9.5 for recurrence
- [x] Safe retained-volume discovery/reattachment and separate explicit data deletion

### P8.2: Database recovery, required for the declared 1.0 engines

- [x] Start with PostgreSQL and MySQL/MariaDB native backup/restore adapters
- [x] Confirm engine/version and expose connection, credential and relevant TLS
  controls on the existing service, without mandatory database-resource hierarchy
- [x] Restore from declared file/object-storage formats with compatibility checks,
  reviewed restore into a new copy, a separate reviewed application switch and a recoverable failure result
- [x] Restore into a disposable copy and verify a real application read/write
- [x] Distinguish a successful upload, a fresh backup and a verified restore;
  show missing, stale or failing protection in context; notification policy continues in P9

### P8.3: Later parity, preserve the backlog

- [ ] MongoDB, SQLite, ClickHouse and engine-specific Redis-family protection
- [ ] Portable application export/import, stopped clones with fresh storage/domains
  and reviewed credential copying, and Coolify migration reports
- [ ] Broader database recipes and portability beyond the declared restore boundary

Acceptance: recover Shipper with its original secrets and serving configuration on
a fresh host; restore a supported database and eligible files from an off-server
copy; verify application behavior; prove updates, ordinary cleanup and app deletion
preserve data. Interrupted or inconsistent restores must never report success.

Implementation boundary: [backup and recovery operations](backups.md) declares the
supported formats, local/native consistency policies, copy-first restore and
separate application switch. PostgreSQL 17, MySQL 8.4 and MariaDB 11 are the initial
acceptance engines. Generic destructive SQL import is later parity. Real public
ACME/domain recovery and distribution acceptance continue in the Phase 10 lab.

## Phase 9: Operations, diagnostics and bounded server care

Outcome: explain a failure near the affected application, give an appropriate
next action, and support the same authorized operations through the UI, API and a
focused CLI. Existing manual workflows remain first-class throughout this phase.

Implementation: the 0.9.0 core is available within the [operations support
boundary](operations.md). Dependency checks establish declared network/readiness
prerequisites, not arbitrary application-protocol transactions. Proxy repair is
reviewed and leased; boot/OS readiness and disruptive host maintenance are verified
with the P10 installer. Local tests do not replace the real-VPS performance,
SMTP/provider, public HTTPS or installation acceptance gate. Full CLI/MCP and the
wider terminal/collector/channel backlog remain open.

### P9.1: Focused operations, required for 1.0

- [x] Bounded CPU/RAM/restart/OOM/disk observations and dependency connectivity checks
- [x] Distinguish build, task, readiness, routing, certificate, daemon and storage
  failures; keep serving state separate from the latest deployment result
- [x] Focused failed-stage view with logs, exit status and duration
- [x] Pause/resume log following, useful search and bounded copy/export
- [x] A focused System area for the implicit host's health, storage, installed
  versions and recovery, without adding server-selection steps to apps
- [ ] Complete update availability and installed boot-service reporting with P10
- [x] Recheck Docker/proxy readiness and review bounded proxy repairs with actual
  effects and results; installed boot checks and disruptive host care follow P10
- [x] One bounded diagnostic report for desired configuration, serving release,
  latest operation/failed stage, service health, dependencies, routing/DNS/TLS,
  relevant resource pressure, backup freshness and changes since last success
- [x] Summary-first JSON and human views with selective detail, stable evidence
  references, observation timestamps, freshness and explicit unknown/unobserved
  states. Separate observed facts from possible causes; redact sensitive material
- [x] Separate build, start, readiness, route and endpoint verification results.
  Respect workers/private services and configured authenticated health checks;
  distinguish host-origin probes from independent external reachability evidence
- [x] Versioned diagnostic playbooks describing how to gather and interpret facts;
  playbooks do not replace deterministic validation, deployment or repair code.
  Treat repository content and workload logs as untrusted data, not instructions

### P9.2: Events, protection and retention, required for 1.0

- [x] One durable event/outbox path for deploy, task, health, backup, disk,
  certificate and update failures, including recovery events
- [x] Webhook/email first, configurable event choices, local delivery tests,
  transition deduplication, quiet unchanged state and bounded retry batches
- [ ] Verify real mailbox/provider delivery and repeated-failure aggregation
- [x] Disk-pressure detection and previewable retention for eligible images, build
  cache, logs, samples and generated files, with narrowly scoped opt-in automation
- [x] Preserve active releases, configured rollback retention, live resources and
  persistent data; refuse cleanup when ownership or references cannot be proven
- [x] Bound observation cycles, sample counts, delivery retries and retained history
- [ ] Record idle cost, API latency and event delay on the offered VPS lab
- [x] Correlate bounded operation, service-exit/restart/OOM, readiness, route,
  protection and host-pressure history by stable instance/app/service/operation IDs;
  support incident windows and last-success comparisons without a metrics platform
- [x] Preflight observed data-filesystem capacity and serialize expensive builds;
  expose container memory observations without inventing host memory headroom
- [ ] Verify memory/disk pressure behavior on inexpensive VPSs with P10

### P9.3: Automation contract, required core and later adapters

- [x] Required: scoped, versioned operations API with revocable/expiring tokens,
  structured results/errors, idempotency and operation/event watches
- [x] Required: separate read, secret reveal, execution, destructive and host-admin
  capabilities; UI and agents use the same ownership and permission checks
- [x] Required: extend existing reviewed changes to record rebuild/restart/routing,
  commands, storage effects, expected downtime and additional capability needs.
  Bind apply to reviewed source/configuration identities; reject stale plans
- [x] Required: stable application/service identities independent of container
  names, documented schemas/error codes, bounded pagination/log cursors, evidence
  links and a version/capability discovery response for compatible clients
- [x] Required: idempotent submissions return the existing operation; distinguish
  accepted/queued, running, succeeded, failed, cancelled and uncertain results.
  Reconnect/watch after interruption without blindly rerunning migrations or tasks
- [x] Required: record actor and operation provenance. Support prior authorization
  for routine scoped actions; consequential actions outside that authority require
  effect review. Keep secret reveal, workload execution and host powers separate;
  Compose/build access is not a hostile-workload sandbox
- [ ] Later parity: full Go CLI/JSON command coverage and a thin MCP adapter over
  these same operations, with focused discovery rather than an enormous tool list
- [ ] Later parity: managed-service interactive terminal/file tooling, broad
  notification/log-drain adapters, traffic analytics and optional collectors
- [ ] Later parity: favorites/tags, command palette and reviewed bulk operations;
  none may introduce mandatory organisation or separate execution semantics

### P9.4: Focused CLI and local-project deployment, required for 1.0

- [x] Extend the Go client/executable with secure connection/pairing, application
  lookup, inspect/diagnose, plan, deploy, operation watch and focused logs; include
  supported reviewed rollback and saved-task execution through explicit capabilities
- [x] Human output and versioned JSON, meaningful exit codes, noninteractive input,
  bounded waits and resumable operation references. Enqueue success is distinct
  from deployment success; timeout/disconnection does not imply cancellation
- [x] Store credentials outside repositories with restrictive local permissions.
  A non-secret project association can remember the instance/application identity;
  resolve names unambiguously and require explicit target changes
- [x] Add local-directory upload as an optional source alongside Git, images and
  pasted definitions. Support declared Dockerfile, Compose, static and automatic
  build paths using existing builders/workers; no Git connection or push required
- [x] Preview selected files/size and apply documented ignore rules and safe secret
  defaults. Exclude credential/private-key material by default, handle required
  secrets deliberately, reject unsafe paths/links and bound archive/extraction sizes
- [x] Snapshot content immutably with a digest and provenance, including Git commit
  and dirty state when available; retain required build inputs under declared
  retention. Apply uses exactly the reviewed snapshot, not a changed working tree
- [x] Upload authentication, integrity, quotas, interruption/retry and abandoned
  upload cleanup share ownership/resource limits. Reuse immutable input on an
  eligible retry and do not silently substitute a newer local tree or Git revision
- [x] Expose local-source identity/history in the dashboard and allow subsequent
  manual operation. An explicit reviewed transition can connect Git automation
  later; the CLI and upload path do not become dashboard prerequisites

The focused command/source contracts are documented in [operations](operations.md)
and the [API](api.md). Broader command coverage and MCP remain later adapters
rather than prerequisites for ordinary dashboard use.

Acceptance: diagnose representative deployment, dependency, low-disk and daemon
failures; receive one useful failure/recovery notification; inspect/wait/retry an
operation through an appropriately scoped API token. A read-only token cannot run
commands, reveal secrets or administer the host. Cleanup preserves data and retained
releases; disabled optional integrations add no collector or mandatory service.
Also complete local project -> plan -> deploy -> watch -> evidence-backed result
without a browser or Git connection. Exercise stale plans, changed local files,
interrupted uploads/watches and idempotent retries. Diagnose a failed update while
the previous release serves; preserve data and report uncertain command outcomes.


## Phase 9.5: UI refactor and guided scheduling, mandatory before 1.0

Outcome: common tasks require fewer decisions and less navigation while advanced
configuration remains available in the relevant application/service context.
This is an interaction-design pass using the existing visual identity. Shared
components, including the schedule picker, can land alongside P8/P9 features;
P9.5 completes the cross-flow audit and acceptance before P10.

### P9.5.1: Shared choice and information components

- [ ] Audit creation, configuration, .env, domains, services, tasks, backups,
  updates and recovery against actual user journeys and DESIGN.md
- [ ] Choice cards for meaningful alternatives, service summaries for stacks,
  compact tables for dense history, and structured status/review panels for
  diagnostics and consequential changes; avoid presenting everything as a list
- [ ] Separate source type, build method and runtime role. Automatic is a selection
  mode that recommends a detected method, rather than a peer of every build method
- [ ] Make selected/recommended states, defaults, inheritance and overrides clear
  through layout and controls; keep advanced options contextual
- [ ] Keep one primary action per task and preserve the minimal .env editor,
  lightweight log toolbar, multi-domain controls and existing QoL register
- [ ] Share components, validation and operation contracts for consistent UX/DX/AX;
  avoid a generic form framework or large new frontend dependency
- [ ] Present the same reviewed effects, diagnostic facts and verification states
  used by automation in compact contextual components. Keep manual creation,
  editing and recovery complete without an agent, CLI or prompt-writing step

### P9.5.2: Adaptive application creation

- [ ] Source choices for Git, container image and Compose, with inline Dockerfile
  and specialist input paths available through additional options
- [ ] Source-specific connection/repository/revision or image/Compose fields
- [ ] Include the local-upload source where supported, with file review and recorded
  snapshot identity; preserve the quick Git/image/pasted-Compose paths
- [ ] Inspect source without executing it, present a detected recommendation and
  clear build-method cards for deliberate overrides, and preview Compose services
- [ ] Configure only relevant essentials: inferred/overridden ports, optional
  domains, required .env values and persistent storage; workers skip HTTP fields
- [ ] Review source/revision, deployment method, public services, storage and
  commands before enqueueing the first deploy
- [ ] A quick path for experienced users; allow direct section edits from review,
  preserve drafts when going back, and keep secrets out of persistent browser storage
- [ ] Do not turn a trivial deploy into mandatory empty steps. Domain setup can
  happen later and a single-container app does not need service selection

### P9.5.3: Intuitive task recurrence picker (Q39)

Implemented early in Phase 8 for Tasks and Backups; future maintenance flows reuse
the same component and parser. Guided intervals divide their hour/day evenly;
other supported cron steps stay in Custom mode.

- [x] Guided scheduling is the default when a schedule is enabled. Offer common
  minute/hour intervals, daily times, selected weekdays/weekly times and monthly
  dates, with relevant time/day/month controls instead of five raw cron fields
- [x] Show the generated five-field cron expression continuously in a read-only,
  focusable, copyable input. Keep it visible so users can learn the resulting pattern
- [x] An explicit Custom cron mode makes the expression editable. Switching from
  the picker retains its current expression; editing a manual expression never
  silently simplifies it to fit a preset
- [x] Return to guided controls only when they can represent the expression exactly;
  otherwise preserve Custom mode. Supported raw lists, ranges and steps remain usable
- [x] Show the saved IANA timezone and the next three real run dates/times using
  the existing backend preview/validation, including DST gaps and repeated times
- [x] Preserve current five-field numeric cron semantics, overlap/missed-run policy
  and task history, including day-of-month/weekday OR semantics. Do not add another
  scheduler daemon or change execution behavior
- [x] Offer only clock patterns that the existing model can represent. Do not label
  a cron step as an arbitrary elapsed interval or promise every-other-week/last-day/
  paired-time rules that a single supported expression cannot express accurately
- [x] Reuse the picker for task, backup and maintenance schedules where those
  features use the same recurrence model; keep timeout/overlap options separate

Examples, interpreted in the selected timezone:

| Picker selection | Generated cron |
| --- | --- |
| Every 15 minutes | `*/15 * * * *` |
| Every hour at minute 10 | `10 * * * *` |
| Every day at 02:30 | `30 2 * * *` |
| Every week on Monday at 09:00 | `0 9 * * 1` |
| Weekdays at 09:00 | `0 9 * * 1-5` |
| First day of every month at 03:00 | `0 3 1 * *` |

Acceptance: create public/private Git, image, static, worker and Compose apps on
desktop/mobile without irrelevant settings; complete routine .env/domain/service
edits; create a weekly task without knowing cron; see/copy its expression and next
runs; edit it manually and round-trip supported patterns without changing meaning.
Test short months, invalid patterns and both DST transitions. Keyboard/focus access,
draft preservation, stable navigation, responsive layouts and measured perceived
speed must pass. A prettier form with the same overload does not satisfy this gate.

## Phase 10: VPS provisioning, ongoing care and the 1.0 release gate

Outcome: a fresh dedicated VPS becomes a working application host through one
installer command, and routine care remains available through Shipper.
Start with a small, explicitly tested Linux distribution/architecture matrix.

### P10.1: One-command bootstrap and ownership

- [ ] Detect supported OS/architecture, resource availability, privilege requirements,
  package-manager locks, port/network conflicts and incompatible existing software
- [ ] Install supported Docker Engine, Compose/build tooling and required system
  dependencies when absent; reuse compatible installations without silently
  overwriting daemon configuration, other workloads or host policy
- [ ] Establish Shipper directories, permissions, persistent state, managed networks,
  proxy and boot services; workloads get language runtimes through build images
- [ ] A resumable, idempotent installer with durable stage results, readable logs,
  versioned artifacts/integrity checks and useful retry/repair outcomes
- [ ] Secure first-admin setup and dashboard URL. Creating the first administrator
  automatically persists that email as the initial ACME contact and queues resolver
  setup; no separate ACME email prompt or environment variable is required
- [ ] Make fresh setup and ACME initialization recoverable across partial failure.
  Preserve existing CA accounts and deliberate legacy contact overrides on upgrade;
  later login-email changes do not silently replace the stored ACME contact
- [ ] Guide only domain/DNS prerequisites. Once a valid reachable hostname is
  configured, automatically issue and attach its dashboard/application certificate
  and redirect ordinary HTTP traffic to HTTPS; localhost/internal services keep
  their explicit supported behavior
- [ ] Default supported new public domains to HTTPS, with explicit HTTP/external
  termination choices under advanced settings; preserve deliberate existing route
  policy on upgrade rather than silently changing serving behavior
- [ ] Reconcile the proxy safely when automatic ACME setup is first enabled, with
  persisted intent, preserved routes/certificate storage and failure recovery;
  users must not remove proxy containers or hand-edit files to activate HTTPS
- [ ] Apply only reviewed/declared firewall changes, preserve SSH access, keep
  databases/internal services private, and report provider-firewall prerequisites
- [ ] Provision the dedicated local host by default. Existing-Docker/manual installs
  keep an explicit compatibility/ownership boundary and remain documented
- [ ] A narrow CLI bootstrap entry point can connect to one explicitly authorized
  fresh VPS over SSH and invoke this same installer. Verify host identity, review
  host effects/privileges, preserve key handling and resume recorded setup stages;
  it does not introduce remote deployment targets or a permanent SSH control plane
- [ ] Use an expiring single-use first-admin claim and deliberate secure CLI pairing
  to obtain scoped/revocable management credentials. Never scrape browser cookies,
  save SSH keys in project files, expose reusable setup credentials in logs or
  bypass interactive account/factor policy to establish automation access

### P10.2: Ongoing host care

- [ ] Restore Shipper, proxy, workers and each app's intended running/stopped state
  after reboot or a recoverable daemon restart; report components that cannot recover
- [ ] Own the certificate lifecycle: renew before expiry, retry transient failures
  with bounded backoff respecting CA retry/rate-limit instructions, observe expiry
  and recover after proxy/host restarts without losing ACME account or certificates
- [ ] Keep routine successful issuance/renewal quiet. Show pending/failed state in
  context and alert only on actionable DNS/reachability/issuance/expiry risk, with
  recovery notifications; never present an untrusted fallback as valid HTTPS
- [ ] Connect System diagnostics, backup health and safe disk retention to the
  operations/events introduced in P8/P9
- [ ] Narrow, audited privileged actions with permission checks, preflight, effect
  review, bounded execution and persisted results; never arbitrary host commands
- [ ] Version-pinned Shipper updates with compatibility checks, verified pre-update
  backups, migration handling and a tested recovery route; do not assume an old
  binary can read a newer database schema
- [ ] Explicit Docker/dependency/OS-security maintenance policy, maintenance windows
  and reboot-required status. Disruptive upgrades/reboots need an operator-approved
  action or previously configured policy, not a surprise during an app deployment
- [ ] Detect unsupported/foreign dependencies and preserve them; diagnose uncertain
  host states instead of trying destructive generic repair or global Docker pruning
- [ ] Data-preserving uninstall with separate explicit choices for Shipper state,
  application data and shared host dependencies; no implicit Docker removal

### P10.3: Security, compatibility and production acceptance

- [ ] Personal TOTP 2FA with generated secret, issuer Shipper, QR/manual setup and
  confirmation, hashed recovery codes and recent-auth protection for factor changes
- [ ] An administrator can require 2FA instance-wide. Existing sessions and new
  logins, including the administrator's, are restricted to enrollment/recovery
  until enrolled and verified; enforce this in APIs/streams as well as navigation
- [ ] Enrolled users complete Shipper's own second-factor step at every interactive
  login. Optional 2FA can be disabled after verification; enforced 2FA cannot be
  disabled by the user. Preserve this gate for all P11 OAuth login methods
- [ ] Account/session management, audited offline owner recovery and credential/token
  lifecycle review, including bootstrap exposure, privileged operations, application
  execution, and separately scoped noninteractive automation identities
- [ ] Use the offered disposable Contabo VPS (8 GB RAM) and 3–4 test domains once
  access is supplied, reinstalling declared Linux distributions only on that host;
  maintain target/domain allowlists and keep redacted evidence off the machine
- [ ] Fresh VPS tests with Docker absent and compatible Docker present, interrupted
  install/rerun, low resources, port conflicts and unsupported-host outcomes
- [ ] Real-account GitHub onboarding, private repository/revision selection, signed
  public push delivery, credential revocation and meaningful failure results
- [ ] Verify first-admin email automatically initializes ACME without manual config;
  test dashboard and multi-service/multi-domain/www issuance, trusted chains and
  actual renewal, DNS/address changes and firewall/CA failures, then restart/reboot
  with persistent certificate state. Use staging for repeated CA exercises and
  record what genuinely proves renewal; a restart alone is not renewal evidence
- [ ] Deploy a web app, worker, static site and database stack; run migrations/tasks,
  reboot with a deliberately stopped app, update, inject failure, recover and restore
  supported data from an off-server backup on a fresh host
- [ ] Prove ordinary operation through Shipper without Docker commands or repeated
  SSH sessions; document exceptional SSH/provider-console recovery procedures
- [ ] Publish install/support matrices, measured resource/performance budgets,
  release/API/schema notes, examples, update/recovery/uninstall instructions and
  contributor guidance. Compare platforms only through equivalent measured workloads
- [ ] Complete the P9.5 UI gate and preserve all shipped QoL regression requirements
- [ ] Prove complementary end-to-end journeys on the authorized lab: a human uses
  the installer/dashboard, and an external agent with an ordinary repository and
  authorized SSH/DNS access bootstraps, deploys local code and verifies the result
  without manual Docker/GitHub/proxy setup or dashboard navigation
- [ ] Repeat with wrong ports, missing variables, dependency readiness failures,
  failed updates, low disk and interrupted connections. Record success rate,
  elapsed time, human interventions, tool calls/tokens when available and data/
  workload preservation. Separate required human authorization from avoidable friction
- [ ] Compare equivalent tasks against direct SSH/Docker and Coolify using declared
  workloads, agent/model versions and access. Publish measured results and failure
  cases; CLI/MCP availability alone is not evidence of a competitive advantage

Acceptance: repeat the full fresh-host lifecycle on every declared supported
configuration. No production-ready claim rests only on local mocks or generated
TLS certificates. 1.0 means a supportable single-server platform, with documented
limits; universal Linux administration, provider outages, fleets and complete
Coolify breadth are not part of that promise.

## Phase 11: Admin-provisioned collaboration and reusable configuration

Outcome: add collaborators to the same application model without public signup,
provider-specific duplicate accounts or mandatory team/project hierarchy. The
[authentication contract](authentication.md) defines setup, linking and login gates.

### P11.1: Accounts and enabled login methods

- [ ] Users live in an administrator-managed Settings list, with optional simple
  Viewer/Developer/Admin roles and one implicit workspace
- [ ] No public registration, signup UI, self-invitation, OAuth just-in-time account
  creation or domain-wide auto-admission; only admins provision additional users
- [ ] Enable Email + Password, Google, GitHub and Discord independently at instance
  level. Only configured/enabled methods appear on login; provider login does not
  grant repository access or a Shipper role
- [ ] Add email + temporary password: hash the password and require a different
  password on first successful password login before granting application access;
  never email or persist a readable password through Shipper
- [ ] Add email + no password: create no password credential. The account can log
  in only through an enabled verified matching provider; require password change
  if a temporary password is added later. Do not silently enable password login
- [ ] Keep the outstanding temporary-password gate when another login method is
  used; replacing/removing that credential through a verified/admin action clears
  it deliberately, so switching providers cannot bypass onboarding requirements
- [ ] One internal account for matching verified email across the enabled methods.
  Match only an active admin-provisioned account, then bind the provider's stable
  subject/user ID after all required gates; do not trust a typed email, display name,
  an unverified claim or email changes on an already linked provider identity
- [ ] Handle missing/private/unverified email and ambiguous identities explicitly;
  do not guess, create a new account or silently merge existing accounts. Where a
  provider cannot establish current mailbox ownership, require an additional proof
  or explicit administrator-approved identity binding
- [ ] Disabling/removing an account revokes sessions and user credentials across all
  providers. Prevent changes that strand the last administrator without a usable
  login/recovery route; enabled methods and available account credentials both matter

### P11.2: Provider setup without guesswork

- [ ] Admins supply each provider's client ID and secret in Settings; encrypt/mask
  secrets, support deliberate rotation and validate configuration before enabling
- [ ] Show the exact callback derived from the canonical dashboard HTTPS origin,
  provide Copy and provider-specific setup steps, and test return-to-Shipper
- [ ] Google: configure a web OAuth/OIDC client and consent/audience settings with
  the exact redirect URI and minimal identity/email scopes
- [ ] GitHub: configure a sign-in OAuth client and exact callback with identity and
  verified/private email access; keep it separate from deployment GitHub Apps
- [ ] Discord: configure an OAuth2 application and exact redirect with identify/email
  scopes; no bot, guild membership or repository permissions required
- [ ] Protect redirects/code exchange with expiring one-use state and PKCE where
  supported; validate Google issuer/audience/signature/nonce and provider identities
  server-side; never use wildcard callbacks or trust browser-supplied profile data

### P11.3: Shared 2FA policy and collaboration isolation

- [ ] Reuse the P10 TOTP/account model. After successful Google/GitHub/Discord or
  password authentication, enrolled users must complete Shipper's own 2FA challenge;
  upstream provider 2FA claims do not bypass it
- [ ] A required password change, optional/forced factor enrollment or factor
  challenge grants only a restricted onboarding session, with no app/API access
  until all requirements pass. Enforce policy changes against existing sessions
- [ ] A fresh OAuth-only account under forced 2FA goes directly into setup; existing
  unenrolled users, including admins, also enroll before continuing. Users cannot
  remove their last required factor while instance enforcement is enabled
- [ ] Optional groups, application variants, linked values and reviewed propagation;
  external secret references, passkeys/general OIDC and wider role/workspace models
  can extend the same account/permission boundary later
- [ ] Audit account provisioning, role/provider/factor changes, recovery and revocation
  without storing credentials, OTPs or tokens in ordinary logs or browser drafts

Acceptance: reject unprovisioned sign-ins; exercise all four enabled methods on
one provisioned identity; prove blank-password and temporary-password semantics,
verified-email linking and stable provider identities, personal 2FA, enforcement
against existing admin/user sessions, recovery and revocation. Test cross-role
API/secret/terminal/data isolation and last-admin lockout prevention. A solo user's
creation path remains unchanged and a source connection never becomes login authority.

## Phase 12: Remote targets, build servers and scale

- [ ] P12.1: SSH targets, host-key validation, connection profiles and scoped inventory
- [ ] P12.2: remote builds, registry transport, capacity and explicit fallback policy
- [ ] P12.3: stateless replicas/multi-target releases, target results and external LB checks
- [ ] Data-aware clone/move and encrypted, single-owner instance transfer
- [ ] Dedicated isolation targets for untrusted preview/CI execution

Exit: remote execution preserves ownership and reports partial failure correctly;
adding replicas does not imply database or control-plane high availability.
P10 remote bootstrap installs one independent local Shipper instance; it does not
complete P12 multi-target management or isolation for untrusted agent workloads.

## Phase 13: Optional catalog and integration breadth

- [ ] Small tested recipes plus an on-demand catalog and Coolify recipe translation
- [ ] Named notification/log-drain adapters and opt-in traffic analytics
- [ ] Cloudflare tunnels, provider purchasing/provisioning/cloud-init and extended fleet patching
- [ ] Dedicated GitHub Actions runner integration
- [ ] Alternate proxy handoff and lazy-loaded relationship/canvas views
- [ ] Per-recipe/provider support declarations instead of broad unsupported parity claims
- [ ] Broader CLI coverage and a thin MCP adapter using P9 contracts, plus versioned
  external-agent skills/playbooks. No model, chatbot or separate execution engine
  is required on the VPS; optional adapters remain cheap when disabled
- [ ] Optional constrained incident charts/tables/timelines using existing evidence
  and UI components. Arbitrary generated HTML/scripts and a general canvas runtime
  are deferred until a concrete need and explicit isolation model are established

Exit: enabling an optional capability does not add dependencies or workflow steps
for installations that do not use it.
Keep the existing catalog/translation backlog optional. Improve generic Dockerfile/
Compose deployment and reliable operational adapters first; catalog size is not a
release gate. Provider provisioning starts with operator-owned accounts, declared
API/cloud-init support and explicit billing/creation authorization, not reseller
or provider signup/payment responsibilities.

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
