# Shipper and Coolify: feature parity and implementation roadmap

**Audit date:** 2026-11-02, Europe/Budapest.  
**Shipper baseline:** version `0.4.0`, commit `5831782a85a2431cc30baa9e07f72524c72103a7`, including Phases 1–4.  
**Shipper implementation update:** 2026-11-04, Phase 6–7 core / `0.7.0`; the original comparison baseline above remains pinned. See [commands and tasks][L05].
**Coolify baseline:** release tag `v4.6.0`, commit `0becfba9113063336b09860579627fd218cb3fc4`.  
**Additional source checked:** Coolify `main` at `67b29ff24af8d54905341f8de8585c3302f67311` and the production docs repository at `b4f93fae4a2417817d4159c0b2377f24a265e5b8`.

The inspected Coolify application code is unchanged between that release tag and
the inspected `main` commit. Their differences are release and publishing files.
Source references below use the release commit rather than a moving branch.
This is a documentation and source audit, not a live Coolify deployment test or
a comparative performance benchmark. Docs can describe beta features and differ
from an implementation; those qualifications are retained below.

## 1. Verdict

**Shipper has a credible deployment foundation, but it is not yet a general
replacement for Coolify.** It covers the central public-Git, Dockerfile,
Railpack, static-site, and basic Compose workflows. It also has application/service
ownership, persistent volumes, deployment history, encrypted configuration,
runtime logs, health-gated single-container promotion, and scoped routing.

Since the initial audit, Phases 5–7 added lifecycle automation, private GitHub
sources, signed push deployments, direct images, recorded rollback and common
routing/runtime controls. Coolify's largest remaining lead is databases and data
recovery, interactive operations, remote hosts, teams, additional providers and
broader specialist configuration. Running PostgreSQL in Compose still does not
equal having a database backup/restore workflow. The narrow deploy API does not
yet equal a general scoped operations API, CLI and MCP integration. Public VPS
issuance/renewal remains a release-gate check despite the local TLS coverage.

There is no honest single parity percentage. A template count, a deployment
backend, and a tested recovery workflow have different value and risk. The matrix
is the backlog; its acceptance scenarios determine progress.

The target is **the same operator outcomes with fewer concepts and decisions**:

- An application remains the main object, including a multi-container stack.
- A service is a component of that application, identified independently of its container.
- Databases, workers, scheduled work, previews, and catalog recipes fit that model.
- Ordinary deployment should not require creating a project, environment, server
  role, destination, and resource category first.
- Advanced capability remains available through explicit, validated configuration.
- Features that need extra processes or privileged infrastructure are opt-in.

The latest request broadens the original post-MVP scope. Remote servers, provider
integrations, richer access control, and legacy compatibility are therefore
tracked here rather than silently dismissed as old non-goals. Commercial Coolify
Cloud billing and hosting are business offerings, not required self-hosted
deployment capabilities.

## 2. How to read the audit

| Status | Meaning |
| --- | --- |
| **Present** | Shipper has the relevant native workflow in the inspected implementation. This does not assert identical semantics or production maturity. |
| **Partial** | Some of the capability exists, but important controls, cases, or verification are missing. |
| **Workaround** | An outcome can be assembled through Compose or a Dockerfile; first-class workflow parity is still missing. |
| **Missing** | No equivalent managed workflow was found. |

`P5` through `P14` refer to the phases in section 5. A late or optional target
remains an open parity commitment, not a completed feature. Coolify's documented
limits are part of the reference, not requirements to exceed accidentally.

### Important comparison corrections

1. **Compose is stateful in both products.** Coolify documents that its
   application-level rolling replacement does not apply to Compose. Shipper's
   current Compose update also allows downtime and preserves persistent data.
   Neither implies automatic database rollback. [D01] [D03]
2. **Coolify's existing pre-deployment command is not a universal migration job.**
   Its implementation selects a currently running application container and skips
   execution when none exists. Shipper should support the first deployment by
   running a job from the candidate image after dependencies are ready. [C01]
3. **Image rollback and full release rollback differ.** Coolify's documented
   image rollback uses the selected older image with current runtime settings.
   Shipper should offer an explicit choice between a complete recorded release
   and an old image with reviewed current settings. [D04]
4. **Nixpacks remains supported but is documented as maintenance-mode software.**
   Railpack is the forward default for Shipper; exact Nixpacks recipe support is
   a separate compatibility task. Coolify labels its Railpack integration Beta.
   [D05] [D06]
5. **Swarm is legacy compatibility.** Coolify documents experimental, deprecated
   v4 support and planned removal in v5. Replica and placement outcomes matter;
   exact existing-Swarm compatibility remains separately tracked. [D07]
6. **Multiple deployment servers do not create a complete HA system.** Coolify's
   documented path excludes Compose and applications with configured persistent
   storage, and requires an external traffic layer and shared state. [D08]
7. **Catalog size is not a reliability result.** The pinned
   `service-templates.json` contains 353 metadata entries; public product copy
   uses broader changing counts. Each imported recipe needs its own compatibility
   status and test, rather than a blanket “all templates supported” claim. [C08]
8. **Some advanced features are Beta.** The inspected docs label container files,
   canvas views, server transfer, server patching, and GitHub Actions runners
   accordingly. Source presence alone does not establish operational reliability.
   [D18] [D25] [D26] [D27] [D28]

## 3. Capability matrix

### A. Sources, builds, and deployment methods

| ID | Coolify capability | Shipper now | Shipper approach and completion target | Evidence |
| --- | --- | --- | --- | --- |
| F01 | Public Git source, revision, base directory | **Present** | Preserve the short public-repository flow and immutable commit recording. | [D02], [L01] |
| F02 | GitHub/GitLab App repository access and private repository selection | **Partial** | GitHub App/token connections and authorized repository lists ship in 0.6.0. GitLab remains later P6 parity. | [D09], [C02] |
| F03 | Deploy keys and generic Git providers, including Bitbucket/Gitea | **Missing** | One generic Git credential/transport path plus provider discovery adapters. Explicitly approve private source hosts. **P6** | [D09], [C02] |
| F04 | Repository/branch selection and tags or commit revisions | **Present** for GitHub/public Git | GitHub repository, branch/tag suggestions and manual commit input are available. Additional provider discovery remains later parity. | [D09], [L01] |
| F05 | Push deployments, manual deploy webhooks, monorepo watch paths | **Present** for GitHub and scoped deploy tokens | Signed pushes, branch/path filters, delivery deduplication, pause and persisted coalescing ship in 0.6.0. Provider status feedback remains later breadth. | [D10], [C02] |
| F06 | Pull/merge-request previews and automatic cleanup | **Missing** | Isolated application variants, separate domains/volumes/secrets, trusted-author defaults and expiry. **P6**, hardened isolation in **P12** | [D11] |
| F07 | Submodules, Git LFS, shallow/full-history selection | **Missing** | Advanced source options with scoped credentials and the same host validation as the parent source. **P6** | [D12] |
| F08 | Repository scan for methods, Dockerfiles, Compose, ports, env templates | **Partial** | Basic method detection and explicit Compose inspection exist; unify a read-only scan with editable suggestions. **P5–P6** | [D13], [L01], [L02] |
| F09 | Static deployment with routing configuration | **Present** | Preserve Apache, SPA/file routing and controlled `.htaccess`; add headers and caching through routing settings. **P7** extends this. | [D02], [L01] |
| F10 | Railpack automatic builds and command overrides | **Present** | Keep the existing isolated builder. Expand tested language/framework examples and version/config controls where needed. **P6/P10** | [D06], [L01] |
| F11 | Git Dockerfile builds, contexts and file selection | **Present** | Keep Dockerfile ownership of its build steps and predictable monorepo paths. | [D02], [L01] |
| F12 | Dockerfile pasted without a Git repository | **Present** | Direct encrypted inline-Dockerfile source uses the isolated BuildKit path without a repository context. | [D02], [C01], [L02] |
| F13 | Direct prebuilt image deployment, tags and digests | **Present** | Direct image source, registry authentication, platform choice, image ID/digest recording and ordinary application controls ship in 0.6.0. | [D14], [L02] |
| F14 | Nixpacks configuration and existing Nixpacks projects | **Missing** | Offer migration guidance and an opt-in pinned compatibility builder for recipes that require it. Railpack stays the default. **P14** | [D05] |
| F15 | Git and pasted Compose, multiple services and persistent volumes | **Present** for the supported subset | Retain one application model for both sources; do not reproduce Coolify's separate Application/Service hierarchy. | [D03], [L02] |
| F16 | Broader Compose definitions, file combinations and profiles | **Partial** | Add selected profiles, overrides, supported includes/extends, additional contexts and clear preflight errors incrementally. **P7** | [D03], [L02] |
| F17 | Raw Compose/custom Docker options, devices and GPU access | **Missing** | An explicit trusted-workload capability profile and reviewed overrides; preserve Shipper ownership. Default deployments remain restricted. **P7**, specialized adapters in **P14** | [D12], [C01], [L02] |
| F18 | Registry login, image publishing, build secrets and advanced build inputs | **Partial** | Registry pull credentials and Linux platform selection ship in 0.6.0. Publishing, Compose build secrets/SSH and additional contexts remain later parity. | [D15], [L01], [L02] |

### B. Runtime, automation, configuration, and releases

| ID | Coolify capability | Shipper now | Shipper approach and completion target | Evidence |
| --- | --- | --- | --- | --- |
| F19 | Install/build/start and pre/post deployment commands | **Present** for the declared command contract | Candidate-image migrations, startup hooks on actual starts, after-deploy and controlled before-stop tasks. Startup attempts use runtime logs; job attempts are durable. [Command boundary][L05]. | [C01], [L01], [L05] |
| F20 | Manual execution, scheduled tasks and execution history | **Present** for the native subset | Saved tasks, Run now, five-field cron/IANA timezones, next-run preview, persisted wait/skip and missed-run policies, timeout and redacted history. One leased worker; uncertain exec outcomes require inspection. | [D16], [C03], [L05] |
| F21 | Deployment queue, cancellation and operation results | **Present** within one-worker scope | Durable coalescing, pre-mutation cancellation, queued superseding and recorded-revision retry ship in 0.6.0. Workload-changing tasks are deliberately not cancellable. | [D01], [L03] |
| F22 | Runtime-only changes without source rebuilding | **Present** within the declared reuse boundary | Reviewed Apply reuses single-container images and Compose service-runtime models. Compose interpolation/build inputs and removal of certain inherited overrides reload source conservatively. Rename and domains retain lighter operations. | [D17], [L01], [L05] |
| F23 | Health checks, worker operation and startup/shutdown tuning | **Present** within declared readiness scope | HTTP/image/process readiness, startup deadline, graceful stop and bounded single-container traffic drain ship through 0.7.0. Compose remains stateful. | [D12], [D17], [L01] |
| F24 | CPU/memory/restart settings and advanced resource controls | **Present** for common limits | CPU/PID/restart/startup/stop controls are available per Compose service and single-container app. Specialist resources remain later parity. | [D12], [L01], [L02] |
| F25 | Component lifecycle actions in a stack | **Present** for start/stop/restart | Service menus share durable operations and reject shutdown while a transitive dependant is running. Complete application reconciliation remains the deployment default. Further restart/draining policy is P7. | [D17], [L02], [L05] |
| F26 | Recognizable or custom container names | **Partial** | Generated names already work. Prefer stable service DNS; offer exact names only when required and explain replacement constraints. **P7** | [D12], [L01] |
| F27 | Scoped variables, secrets, import/edit/reveal | **Present** for local application/service values | Preserve the minimal `.env` editor, encryption and inheritance. Add validation rather than more default controls. | [D17], [L01], [L02] |
| F28 | Shared team/project/environment/server values | **Missing** | Opt-in reusable value sets with visible provenance, explicit links and reviewed propagation. Optional grouping must not become a deployment prerequisite. **P11** | [D20] |
| F29 | External secrets from Doppler, Infisical and Vault | **Missing** | Credential-reference adapters using the same `.env` surface, scoped resolution and rotation controls. **P11/P13** | [C04] |
| F30 | Env-template suggestions and generated credentials/URLs | **Partial** | Paste/edit works. Add missing-key suggestions, generation in row menus and recipe variables with explicit review. **P5/P6/P13** | [D13], [C08], [L01] |
| F31 | Health-gated replacement, graceful stop and manual image rollback | **Partial** | Reviewed recorded-release restoration, config comparison, retained-artifact checks and single-container drain ship in 0.7.0. Missing artifacts reject recovery; automatic retention/pruning remains deferred. | [D01], [D04], [L03] |
| F32 | Configuration inspection, cloning, move and migration workflows | **Partial** | Snapshots exist. Add effective-config review, portable export/import and safe duplication before remote migration. **P5/P8/P11/P12** | [D17], [D25], [L03] |

### C. Domains, HTTPS, networking, and proxy controls

| ID | Coolify capability | Shipper now | Shipper approach and completion target | Evidence |
| --- | --- | --- | --- | --- |
| F33 | Multiple domains, service routing, primary aliases and redirects | **Present** | Preserve hostname rows, per-service ownership, root-domain www expansion, independent HTTPS and verification. | [D03], [D19], [L02] |
| F34 | Automatic public HTTPS, renewal and dashboard HTTPS | **Partial** pending public acceptance | Packaged dashboard HTTPS, ACME renewal configuration and certificate diagnostics ship in 0.7.0. Trusted local TLS and restart tests exist; live public issuance/renewal remains P10. | [D19], [L01] |
| F35 | Generated app domains and wildcard domain configuration | **Partial** | Single-container local addresses exist; add consistent local/service URLs and an optional operator-controlled wildcard base domain. **P7** | [D19], [L01] |
| F36 | DNS challenge, wildcard certificates, custom certificates/resolvers | **Missing** | Advanced certificate options on the same domain model; HTTP challenge remains the short default path. **P7** | [D19] |
| F37 | Traefik dynamic config, middleware, headers, auth, path routing and compression | **Partial** | Per-domain path/strip-prefix, compression and response headers ship in 0.7.0. Auth middleware, arbitrary scoped Traefik overrides and admin proxy plans remain later parity. | [D19], [C05] |
| F38 | Maintenance mode and custom fallback/error pages | **Partial** | Route-only HTTP 503 maintenance preserves running services. Custom fallback/error pages remain later parity. | [D19] |
| F39 | Public TCP/UDP ports, database access and internal connections | **Partial** | Private Compose networking works; add explicit exposure/binding, database TLS and narrow cross-application connections. **P7/P8** | [D03], [D21], [L02] |
| F40 | Managed Cloudflare DNS and tunnel integration | **Missing** | Optional Cloudflare credential capabilities; show a proposed DNS change before applying it. Tunnel mode is separate from DNS management. **P7/P13** | [C04], [C05] |
| F41 | Caddy and custom reverse proxy configurations | **Missing** | Optional routing adapters for Caddy/custom proxy handoff. Traefik stays the default. **P13/P14** | [D19] |

### D. Databases, storage, backups, and portability

| ID | Coolify capability | Shipper now | Shipper approach and completion target | Evidence |
| --- | --- | --- | --- | --- |
| F42 | Deploy PostgreSQL/MySQL/MariaDB/MongoDB/Redis/KeyDB/Dragonfly/ClickHouse/SQLite workloads | **Workaround** | Images/Compose can run workloads. Add reviewed database recipes and optional service database capabilities, not new mandatory resource hierarchies. **P8/P13** | [D21], [C06], [L02] |
| F43 | Database connection details, credentials, exposure and TLS controls | **Partial** | Environment overrides exist; add connection copy, credential management and explicit private/public exposure. **P8** | [D21] |
| F44 | Engine-aware scheduled backups | **Missing** | PostgreSQL, MySQL/MariaDB, MongoDB, ClickHouse and SQLite adapters as applicable; separate Redis-family persistence policies. **P8** | [D22], [C06] |
| F45 | Database import/restore from upload, host file or S3 | **Missing** | Engine/format-aware restore jobs, validation, explicit data replacement and disposable restore verification. **P8** | [D22], [C06] |
| F46 | Named volumes, file/directory/host mounts and advanced storage options | **Partial** | Named volumes and repository file snapshots exist. Add explicit approved host paths and needed external storage settings without weakening the portable default. **P7/P8** | [D23], [L02] |
| F47 | Scheduled storage archives, S3 copies and retention | **Missing** | Reuse task scheduling and backup destinations. Distinguish consistent DB backups from file archives. **P8** | [D23], [C07] |
| F48 | Instance backup, restore and disaster recovery | **Partial** | Manual procedures exist. Add a verified metadata/key/certificate backup bundle, CLI recovery and restore checks. **P8/P10** | [D24], [L01] |
| F49 | Clone/move resources without copying stored data | **Missing** | Duplicate configuration into a stopped application; new domains/storage by default and explicit secret/data choices. **P8/P11** | [D17] |
| F50 | Cross-instance resource/server transfer | **Missing** | Versioned encrypted transfer, explicit ownership handoff and separate data movement when hosts change. **P12/P13** | [D25], [C09] |

### E. Operations and visibility

| ID | Coolify capability | Shipper now | Shipper approach and completion target | Evidence |
| --- | --- | --- | --- | --- |
| F51 | Deployment logs/history and live workload output | **Present** for core workflows | Preserve per-deployment numbering, timestamps, duration, bounded rendering, service selection and compact search/copy. | [D01], [D17], [L03] |
| F52 | Interactive resource terminal | **Missing** | An authenticated, audited session scoped to a selected managed service, opened from its actions. **P9** | [D17], [C10] |
| F53 | Container file browsing, edit/upload/download/rename/permissions | **Missing** | An on-demand service tool with runtime-versus-persistent targets clearly distinguished; no host-wide browser. **P9** | [D18], [C10] |
| F54 | Server/container metrics, health and disk monitoring | **Partial** | Connection/inventory status exists; add bounded CPU/RAM/restarts/OOM/disk diagnostics and short retained samples. **P9** | [D29], [C11] |
| F55 | Proxy traffic analytics | **Missing** | Optional bounded request/error/latency aggregates; off by default and subject to privacy/retention settings. **P9/P13** | [D29] |
| F56 | External log drains | **Missing** | Per-service opt-in with adapters for webhook/custom collector, Axiom, New Relic and CloudWatch. Start collectors only where used. **P9/P13** | [D30], [C11] |
| F57 | Email, Discord, Slack/Mattermost, Telegram, Pushover and webhook notifications | **Missing** | One event/outbox system and selectable channel adapters. Failures/recoveries first, with throttling and tests. **P9/P13** | [D31], [C12] |
| F58 | Image retention, build-cache cleanup and disk thresholds | **Missing** as managed policy | Show owned disk use and a cleanup preview; protect active/rollback images and all persistent data by default. **P8/P9** | [D04], [D32] |
| F59 | Tags and deploy-by-tag/bulk operations | **Missing** | Optional tags and selection actions with effect previews, bounded concurrency and per-app results. **P9/P12** | [D16] |

### F. Servers, scale, and advanced infrastructure

| ID | Coolify capability | Shipper now | Shipper approach and completion target | Evidence |
| --- | --- | --- | --- | --- |
| F60 | Multiple SSH-connected hosts, validation, keys and destinations | **Missing** | The configured local Docker endpoint is not a multi-host manager. Add an implicit local target, then explicit remote targets and network choices. **P12** | [D32], [C13] |
| F61 | Separate build/deployment server roles and registry transport | **Partial** | A local isolated BuildKit daemon exists. Add remote builder selection, registry transfer and explicit fallback policy. **P12** | [D32], [L01] |
| F62 | Stateless multi-server deployment with external load balancing | **Missing** | Deploy one pinned image to selected targets; report each result and require declared external/shared state. **P12** | [D08] |
| F63 | Replica count and placement through legacy Swarm | **Missing** | First deliver replica/multi-host outcomes; retain an optional exact legacy adapter milestone. Do not claim Compose equals Swarm. **P12/P14** | [D07] |
| F64 | Hetzner, DigitalOcean, Vultr and Hostinger provisioning/cloud-init | **Missing** | Optional provider adapters around Add server, with price/action confirmation and reusable SSH validation. **P13** | [D26], [C13] |
| F65 | Server package updates, Docker preparation and security update configuration | **Missing** | Explicit host-administration jobs and maintenance windows. Existing unmanaged workloads remain outside application inventory. **P10/P13** | [D27], [C13] |
| F66 | Host web terminal and jump/private-network access | **Missing** | Remote connection profiles and an opt-in admin host tool, separate from service terminals and ordinary app permissions. **P12/P13** | [D32], [C10] |
| F67 | Ephemeral GitHub Actions runners on build hosts | **Missing** | Optional isolated CI-runner integration with dedicated capacity and credential handling. Never a prerequisite for deployments. **P13** | [D28], [C13] |

### G. Access, automation interfaces, organization, and distribution

| ID | Coolify capability | Shipper now | Shipper approach and completion target | Evidence |
| --- | --- | --- | --- | --- |
| F68 | Secure admin setup, password sessions and recovery | **Present** for a single-admin installation | Preserve capability-based bootstrap, CSRF, session checks and local recovery; add account/session management. **P10** extends this. | [D33], [L04] |
| F69 | Invitations, team separation and roles | **Missing** | One implicit workspace initially; invite, choose access, finish. Optional additional workspaces later. **P11** | [D33], [C14] |
| F70 | OAuth sign-in, OIDC SSO, 2FA and passkeys | **Missing** | GitHub sign-in can accompany P6; add 2FA/recovery before public release, then passkeys/OIDC and provider adapters. **P6/P10/P11** | [D33], [C14] |
| F71 | Security/configuration audit events | **Partial** | Deployment history exists; extend a bounded audit stream to credentials, auth, config, terminal, data and server actions. **P5–P11** | [D33], [C14] |
| F72 | Scoped REST API tokens, IP restrictions and machine-readable operations | **Partial** | Browser-session API exists. Add scoped tokens, versioned contracts, idempotency, operation watches and sensitive-read separation. **P6/P9** | [D34], [C15], [L04] |
| F73 | CLI resource management | **Missing** beyond maintenance subcommands | Extend the Go client/binary over the same API, with JSON output and contexts. **P9** | [D35] |
| F74 | MCP tools, resources and prompts | **Missing** | A thin adapter over the same permission-checked operations; no chatbot or separate business-logic engine. **P9** | [D36], [C15] |
| F75 | Projects, environments, optional canvas and relationship views | **Missing** | Saved views, optional groups and application variants. Canvas may become a lazy-loaded optional view, never the required deployment path. **P11/P13** | [D20], [D37] |
| F76 | One-click application/service catalog | **Workaround** through pasted Compose | A few tested recipes first, then an on-demand catalog and Coolify recipe translator with per-recipe compatibility results. **P8/P13** | [C08] |
| F77 | Installation, updates, downgrade/recovery, uninstall and instance settings | **Partial** | Local binary and non-root Docker package exist. Add tested Linux setup, pinned update/restore paths and data-preserving uninstall. **P10** | [D24], [D38], [L04] |
| F78 | Documented, reproducible user and automation workflows | **Partial** | Repository docs/tests exist; publish versioned API/schema, recovery guides, examples, migration guides and support diagnostics. **Every phase, release gate P10** | [D34], [D35], [L04] |

## 4. The architecture that keeps parity affordable

### Reuse a small set of mechanisms

| Mechanism | Capabilities it should serve | Boundary |
| --- | --- | --- |
| Existing application/service model | Web apps, workers, databases, recipes, previews | A database capability does not require a separate application hierarchy. |
| Durable operation/task runner | Deploy hooks, manual jobs, cron, backup/restore, cleanup, migrations | Persist intent, attempts, ownership, result and logs; execute user workload code in its target image. |
| Change planner | Env/config saves, route-only changes, runtime recreation, rebuild, rollback | Compute an explicit plan and the relevant Apply action. |
| Source and image adapters | Public/private Git, providers, inline Dockerfile, image registry | Credentials are scoped; the default public-source protections stay intact. |
| Routing model | Domains, TLS, aliases, middleware, maintenance and proxy adapters | Stable application/service identities, not generated container names. |
| Storage/backup adapters | Volumes, files, database dumps, S3 and restore tests | Keep data lifecycle separate from container lifecycle. |
| Target transport | Local Docker, SSH-connected hosts, build hosts and optional legacy targets | Keep one implicit local target; add explicit remote targets when required. |
| Permission-checked API and event stream | UI, CLI, MCP, webhooks, notifications and audit | Clients share operations and authorization rather than reimplementing them. |

Build each boundary when a phase needs it. This is not a request to implement a
generic plugin framework, workflow language, or every future DTO before P5.

### Default overhead budget

- Keep Go, SQLite, embedded SolidJS and the existing Docker/proxy boundary.
- Do not add Redis, an external queue database, Kubernetes, a global metrics
  platform, or an always-running worker for each new feature.
- Store durable schedules and operation state in SQLite. Use bounded workers and
  per-app/per-target locks rather than one new process per automation category.
- Reuse engine tools inside appropriate containers for database backup/restore.
- Run metrics collectors, log forwarders, CI runners and alternate proxies only
  on targets where they are explicitly enabled.
- Fetch recipe/provider metadata on demand; do not ship a large client bundle or
  render hundreds of catalog cards on normal navigation.
- Keep polling, log buffers, metric cardinality, audit retention, storage and
  background work bounded. Measure idle cost after each phase.
- Preserve cached screens, stable rows, SSE updates and immediate accepted-operation
  feedback. An API request should not block while an image builds or a restore runs.

### UI and configuration contract

The main path stays **source → inspect → deploy → domain**. An application shows
its running state and deployment state separately. A single-container app should
not gain a compulsory service-selection step.

Use the existing `.env` editor for local, inherited and linked values, with
advanced options in row actions. Preserve a source value unless an actual override
is saved. Configuration edits must not pin unrelated inferred ports or settings.

Tasks, backups, terminal, files, metrics and advanced routing appear in the
relevant application/service context. Avoid a new top-level sidebar area for each
capability. Keep common commands, domains, configuration and errors close to the
workload. Optional projects, tags, environments and canvas views organize existing
objects; they never become mandatory creation prerequisites.

Follow `DESIGN.md` and `AGENTS.md`: compact neutral surfaces, concise labels,
progressive disclosure, keyboard access, stable navigation and no explanatory
taglines added to compensate for a confusing layout.

## 5. Phases and milestones

Phases 1–4 remain complete historical milestones. The old single “Phase 5: Release
hardening” placeholder is expanded below. Reliability and security work occur in
every phase; P10 is the public-release gate. These are sequenced deliverables, not
calendar estimates or a promise that full Coolify breadth fits one small release.

### P5. Lifecycle automation and configuration UX

**Implemented in 0.5.0.** The detailed contract below is supported within the
[documented limits][L05]. The real-engine suite verifies first-deploy PostgreSQL
schema creation, Docker restart hooks, service-runtime image reuse, dependency
controls, known-job recovery after reopening SQLite, shell-free argv execution,
and failed-migration preservation. Unit/API/browser checks cover encrypted
snapshots, schedule policies, stale review rejection, redaction and desktop/mobile UX.

**Outcome:** deploy an app with a database migration, operate a saved task, and
edit configuration without rebuilding unnecessarily.

**P5.1: task engine and lifecycle commands**

| Command stage | Execution contract | Default failure handling |
| --- | --- | --- |
| Install/build steps | Run in the supported build environment; Dockerfile-controlled steps remain in the file. | Stop the image build and preserve the serving release. |
| Before deploy | Run once per deployment attempt from the candidate image after required dependencies are ready. Support the first deployment. | Block startup/promotion; record the precise failed task. |
| Before start | Run before the main process on every real container start, including a Docker-managed restart. | Fail startup and leave readiness false. |
| Start | Run the main foreground process, preserving entrypoint/CMD unless the user deliberately overrides them. | Report actual process/readiness failure. |
| After healthy deployment | Run after successful traffic promotion. | Record a warning by default; do not silently undo an otherwise healthy deployment. |
| Before stop | Best-effort controlled-shutdown work with a deadline. | Surface the result; never depend on it for the only backup or crash recovery path. |

The ordinary UI exposes build/start and an optional Before deploy command. Other
stages are advanced. Each task selects a stable service, working directory, user,
timeout and environment scope. Support explicit argv and shell commands where
the image provides a shell. Do not assume every image contains `/bin/sh`.

Task runs need durable attempt IDs, exit status, timing, timestamped redacted logs
and restart recovery. A per-app lock does not replace a migration framework's
database lock when multiple apps share a database. Do not promise exactly-once
execution across arbitrary crashes. Reattach to known runs; mark an ambiguous
result explicitly and avoid automatic blind migration retries.

**P5.2: manual and scheduled work**

- Save a task, Run now, inspect executions, disable it, or add a schedule.
- Support explicit timezone, next-run preview, overlap policy and missed-run policy.
- Distinguish a task in an existing running service from a fresh one-off job image.
- Preserve one task mechanism for future backups, cleanup and integration work.
- Add task-level audit identity and service lifecycle actions with dependency checks.

**P5.3: change planning and daily editing**

- Keep drafts until Apply; show pending changes and a concise masked diff.
- Classify metadata, routing, runtime recreation and rebuild changes correctly.
- Improve detected commands and missing env suggestions without importing secrets
  or forcing a development Compose file into production.
- Add configurable worker/non-HTTP startup policy and useful failure-stage links.
- Preserve the known duration/log-number/search/static-status/container-name fixes.

**Exit checks:** first deploy creates a database schema before starting consumers;
failed migration blocks the new app; ordinary restart does not rerun deploy-only
tasks; actual restart does run startup tasks; controller restart recovers a known
task or exposes uncertainty; overlapping runs are controlled; images without a
shell fail or execute predictably; a runtime-only env change avoids source rebuild;
logs do not leak a saved secret; desktop/mobile remain fast and uncluttered.

### P6. Git automation, source access, and image workflows

**1.0 core implemented in 0.6.0:** GitHub App/token connections and repository/ref
selection, encrypted credentials, signed push deployment with durable coalescing,
path filters, cancellation/pinned retry, scoped deploy tokens, direct images,
registry authentication and inline Dockerfiles. See [sources](docs/sources.md).
Protocol, isolated Docker and browser tests pass. Real GitHub installation and
public delivery remain an external acceptance check. The breadth milestones below
remain the parity target; GitLab/SSH, publishing and PR previews are not shipped.

**Outcome:** connect a private repository or image and deploy a selected revision
manually or on push.

**P6.1: source and registry credentials**

- GitHub App onboarding and selected-repository access, then GitLab App and
  generic HTTPS/deploy-key sources for Bitbucket, Gitea and other providers.
- Repository, branch and tag search with manual commit override and monorepo paths.
- Scoped credentials, rotation/revocation, SSH host-key verification and controlled
  private-host access. Do not globally disable source-host/SSRF protections.
- Direct image and inline-Dockerfile sources, registry authentication, digest
  recording, build publishing and supported platform selection.
- Advanced submodule, LFS and clone-history controls.
- Keep source-provider connection separate from optional “Sign in with GitHub.”

**P6.2: deploy triggers and queue controls**

- Signed Git webhooks, narrowly scoped deploy tokens and delivery deduplication.
- Branch/path filters, pause auto deploy, skip unsuitable events, commit links and
  provider status feedback.
- Cancel queued/building work, supersede stale queued commits, and retry with the
  recorded revision. State-changing task cancellation must show its actual result.
- Start a versioned programmatic API contract and token permissions here; P9
  extends it into the full CLI/MCP surface.

**P6.3: isolated previews**

- Opt-in trusted pull/merge-request previews with separate URLs and env values.
- Fresh storage/secrets by default, TTL and automatic close/merge cleanup.
- No production volume, database credential or broad provider-token inheritance.
- Untrusted public-fork execution stays disabled until explicit approval and an
  adequate isolated target are available. Rich remote isolation follows P12.

**Exit checks:** a private repo works without copying a URL/token into command
logs; revoked access fails clearly; branch lists match permissions; duplicate
webhooks create one accepted deployment; unrelated monorepo changes do not build;
the queued revision is the revision deployed; a preview cannot access production
data; an image-only application needs no Git or Compose knowledge.

### P7. Routing, Compose breadth, and deployment control

**1.0 core implemented in 0.7.0:** packaged dashboard HTTPS routing, ACME renewal
configuration, certificate diagnostics, per-domain path/header/compression and
maintenance controls, per-service runtime limits and reviewed recorded-release
recovery. Current domains/data are retained and deployment migration hooks skipped.
See [routing and releases](docs/routing-and-releases.md) for supported behavior and
verification. Public ACME issuance/renewal still needs the Phase 10 VPS gate.
The breadth milestones below remain open, including wildcard/custom certificates,
raw proxy configuration, specialist networking and broad Compose compatibility.

**Outcome:** common advanced workloads deploy without abandoning Shipper's model.

**P7.1: domains and networking**

- Retain per-service multi-domain rows, www expansion, independent HTTPS and checks.
- Add consistent generated local/service URLs and optional wildcard base domains.
- Public dashboard/application HTTPS setup, DNS diagnostics, renewal visibility,
  DNS-01/wildcard certificates, custom certificates and resolver controls.
- Path routes, redirects, headers, basic/forward authentication, compression,
  body limits, streaming/WebSocket-friendly timeouts and maintenance/error pages.
- App/service Traefik overrides plus a separate admin-level proxy configuration
  plan, validation, diff, acknowledgement and rollback.
- Explicit TCP/UDP exposure and bind address, declared cross-app connections and
  approved external networks. Private service access remains the default.
- Optional Cloudflare DNS changes with conflict review; tunnel setup can follow P13.

**P7.2: compatibility without a parallel orchestrator**

- Add Compose profiles, multiple files/overrides, safe includes/extends, build
  secrets/SSH and additional contexts using the reference parser and official CLI.
- Provide an expert trusted-workload profile for required devices, GPU, resource,
  network and host-mount options. Validate the selected target's paths and runtime.
- The profile must not let user YAML impersonate another application's ownership.
- Portable repository file snapshots remain the ordinary default; approved host
  paths are explicit target-host paths, not accidental control-plane bind paths.
- Provide the practical raw/custom configuration outcomes of Coolify without
  silently dropping requested fields. Unsupported fields remain visible gaps.

**P7.3: release actions**

- Operator rollback to a recorded release, or an explicitly reviewed old image
  with current settings; both require retained/retrievable artifacts.
- Release comparison, readiness policy, startup grace and graceful stop/draining.
- CPU/restart/PID settings and narrowly necessary fixed-name controls.
- Compose remains a stateful reconciliation path; avoid a zero-downtime claim.

**Exit checks:** two services route independently through aliases and TLS; route
edits do not rebuild; invalid proxy input leaves previous routing usable; a custom
certificate survives restart; a worker deploys without a fake HTTP endpoint;
Compose secrets are available to the intended build only; requested trusted
options work on an approved target; rollback identifies code/config/data boundaries.

### P8. Data protection and database workflows

**Outcome:** deploying persistent data includes a usable recovery path.

**P8.1: instance and storage recovery**

- A versioned encrypted backup bundle containing metadata, encryption material,
  credential references/settings and proxy certificate state as appropriate.
- Backup destinations with local and S3-compatible storage, retention, download,
  integrity checks, bounded compression and optional multiple copies.
- Storage archives for eligible volumes/directories, with a quiesce/task mechanism
  when consistency requires stopping writes.
- Retained-volume discovery, safe reattachment, disk-use inspection and separate
  explicit data deletion. App deletion continues to preserve persistent data.

**P8.2: database capabilities**

- Start with PostgreSQL and MySQL/MariaDB; extend MongoDB, SQLite and ClickHouse
  where their native backup operations fit. Add Redis/KeyDB/Dragonfly persistence
  recipes separately rather than calling every engine backup-identical.
- Show a Database capability on an appropriate service for connection details,
  credentials, TLS, backup and restore. A user can confirm engine detection.
- Restore from upload, approved target file and object storage, with engine/version/
  format validation and explicit replacement choices.
- Preserve the reference distinction: Coolify documents built-in imports for
  PostgreSQL, MySQL/MariaDB, MongoDB and SQLite, not every backed-up engine.
- Add recipe-based simple database creation, fresh data clones and backup health.

**P8.3: portability**

- Export application configuration and required external references safely.
- Duplicate into a stopped app with new domains/storage and reviewed secrets.
- Import/export does not imply copying data or authorizing a new public route.
- Offer a Coolify migration mapping for source, env, domains, commands, storage
  and schedules; flag unsupported settings before creating resources.

**Exit checks:** recover a fresh installation with its original key and credentials;
restore a disposable copy and verify an application query; demonstrate that a
running-database archive is not presented as a consistent dump; restore cancellation
cannot masquerade as success; normal updates/cleanup never erase live data.

### P9. Operations, visibility, and agent experience

**Outcome:** diagnose and operate an app from the UI, CLI or an agent with the same
permissions and operation results.

**P9.1: focused operations**

- Service terminal sessions and a container file tool for inspect/edit/upload/
  download/rename/permissions, with capability checks and audit.
- Distinguish runtime-layer edits from persistent storage and source-controlled
  files. Only expose the selected managed service's filesystem.
- Bounded CPU/RAM/restart/OOM/disk samples, dependency connectivity diagnostics and
  separate serving/deploying state. Optional traffic aggregates can follow.
- Pause/follow/search/export logs and jump to the failed task/stage.

**P9.2: alerts and retention**

- One durable event/outbox path for deployment, task, health, backup, disk and
  certificate events, including deduplicated recovery notifications.
- Webhook/email first; add Discord, Slack/Mattermost, Telegram and Pushover through
  the same channel interface. No channel should create a mandatory background service.
- Log forwarding starts with an external destination contract. Named destinations
  and optional collectors are adapters, not an embedded observability platform.
- Preview owned cleanup and retention for images, cache, logs, metrics and generated
  file volumes. Preserve active/rollback images and data volumes by default.
- Favorites, tags, command palette and safe bulk selection reuse normal app actions.

**P9.3: API, CLI and MCP**

- Complete token scopes, expiry/revocation, optional IP restrictions, structured
  errors, idempotent operations and operation/event watches.
- Extend the existing Go executable/client with deploy/status/logs/env/task/backup
  commands, JSON output and explicit target contexts.
- Add MCP tools/resources as a thin adapter over the same authorized operations.
- Sensitive reads, task execution, terminal access, deletion and administration
  are distinct capabilities. Never use browser-cookie scraping for automation.

**Exit checks:** an agent can deploy, wait, inspect a failure and retry without the
UI; a read-only token cannot execute or reveal secrets; a terminal/file request
cannot switch to another app by supplying its container ID; a notification storm
is bounded; disabled analytics/forwarding adds no collector; large lists/logs remain
responsive with measured idle and per-feature resource use.

### P10. Production release and installation hardening

**Outcome:** a reproducible, supportable single-server release.

- Test fresh supported Linux hosts, architecture/engine/Compose compatibility,
  setup recovery, public ACME issuance and renewal, restart/reboot and low-disk cases.
- Provide one-command installation after the local workflows are stable, preserving
  the user's request to defer distribution details until the app works locally.
- Add pinned updates, backup-before-migration, tested restore-based recovery,
  version/support diagnostics and data-preserving uninstall.
- Improve account/password/session controls, add 2FA with recovery codes, review
  management-plane exposure, source credentials, task/terminal privileges and tokens.
- Publish release notes, API schema, examples, disaster recovery and contributor
  documentation; maintain license notices and dependency/base-image updates.
- Establish reproducible performance budgets for idle CPU/RAM, API latency,
  navigation, event-to-UI delay, builds and log interaction. Proposed targets are
  not evidence of beating Coolify until measured on equivalent workloads.

**Exit checks:** install, deploy a private-source app and a database stack, migrate,
serve HTTPS, back up, reboot, update, simulate failure, restore and uninstall on
an isolated VPS. The release must document remaining single-admin/remote-host limits.

### P11. Collaboration, reusable configuration, and environments

**Outcome:** share an installation without making every solo user manage teams.

- One implicit workspace; optional additional workspaces for real isolation needs.
- Invite a person, choose Viewer/Developer/Admin access, finish. Preserve an owner
  recovery boundary and enforce all scope checks server-side.
- Add passkeys, OIDC SSO and further OAuth adapters through the same account model.
- Optional groups, app variants/environments, linked value sets and explicit
  propagation previews. Local values and ownership remain understandable.
- Integrate Doppler, Infisical and Vault as scoped secret references with rotation
  and resolution history. Values should not be copied into browser drafts.
- Expand audit events and team/source/storage credential separation.

**Exit checks:** a solo user's creation path is unchanged; a viewer cannot deploy,
open a terminal or reveal a secret; an app cannot resolve another workspace's values;
removing a member revokes effective access; changing a shared value shows affected
apps and does not silently redeploy them all.

### P12. Remote targets, separate builds, and scale

**Outcome:** operate multiple machines without making them mandatory concepts.

**P12.1: targets and remote operations**

- Keep an implicit local target; add SSH connection, host-key confirmation, allowed
  credentials, validation, non-root setup and optional jump/private connection profiles.
- Scope inventory and actions to managed Shipper applications on each target.
- Use the existing API/operation model with bounded remote transports. A new
  always-on metrics agent is optional, not a connection prerequisite.

**P12.2: build and deployment roles**

- Remote build target selection, registry push/pull and architecture compatibility.
- Explicit fallback policy and target capacity/concurrency, preserving the production
  machine when the operator selected an external builder for isolation.
- Dedicated targets for untrusted previews or CI-style jobs where required.

**P12.3: replication and movement**

- Stateless service replicas, multi-target application deployment, per-target results
  and external load-balancer integration/health checks.
- Storage/session/database prerequisites and clear limits; do not promise an entire
  Compose database stack becomes highly available by adding a replica count.
- Controlled clone/move/data migration and encrypted instance ownership transfer.
- An explicit handoff must prevent both controllers mutating the same workload.
  Shipper's instance ownership labels need a tested transfer design; accepting all
  containers with a similar label would violate the product boundary.
- Keep SQLite's single-controller authority. Real control-plane HA needs a separate
  consistency design; backup/restore or standby does not equal active-active HA.

**Exit checks:** deploy only the same pinned image to intended targets; lose a build
host without silently building on production; lose a deployment target and report
partial failure correctly; move data only when requested; transfer existing managed
apps without adopting unrelated containers or leaving two active owners.

### P13. Optional catalog and integration breadth

**Outcome:** reach the long tail of Coolify use cases with small reusable adapters.

- Keep a small tested built-in recipe set. Add an optional on-demand catalog and
  translator for pinned Coolify Compose recipes and their generated URL/secret
  placeholders. Preserve provenance, licenses and validation failures.
- Catalog applications use the same app/service/env/domain/storage/task mechanisms.
  No separate service platform or per-template backend model.
- Cover supported database engines and common app recipes with versioned tests;
  track incompatible recipes explicitly rather than claiming catalog-wide parity.
- Complete notification/log-drain destination adapters and opt-in traffic analytics
  with retention and anonymization controls.
- Add Cloudflare tunnels, cloud-provider provisioning/cloud-init and optional host
  patch administration. Provider purchasing actions require a concrete reviewed
  cost/action plan; ordinary app actions never imply buying or patching a server.
- Add opt-in GitHub Actions runner integration on dedicated capacity, without
  promoting it into a mandatory CI server inside Shipper.
- Add Caddy/custom proxy handoff where needed, plus optional relationship/canvas
  views rendered from existing data. Keep the compact list as the default.

**Exit checks:** recipes import through the same validation as authored Compose;
unsupported metadata is surfaced before execution; enabling one integration does
not load others; disabling it removes its background workload; importing a catalog
entry does not execute host-side provisioning scripts unexpectedly.

### P14. Exact compatibility and remaining specialist controls

**Outcome:** close explicitly requested compatibility gaps without burdening the
default product.

- Provide an optional pinned Nixpacks backend for projects whose configuration
  cannot be migrated faithfully to Railpack/Dockerfile.
- Keep a tracked legacy Swarm adapter for the supported v4 outcomes: existing
  destinations, registry images, replicas and placement. Document its deprecated
  reference status and prefer export/migration for new systems.
- Complete specialized trusted runtime/device/network/storage options and proxy
  backends against concrete workloads, with target capability checks.
- Maintain a field-by-field import report for remaining Coolify settings. An
  unsupported exact option is an open gap even when an alternative recipe exists.
- Keep cloud-hosted billing/subscriptions outside the operator capability matrix.

**Exit checks:** compatibility fixtures work with their declared backend versions;
unsupported legacy combinations are diagnosed before mutation; ordinary users see
no additional prerequisites, daemons or configuration fields when these modes are off.

## 6. QoL register, including the earlier discussion

Completed items remain regression requirements. A future overhaul must not re-add
the complexity already removed from `.env`, logs and static-site views.

| ID | Item | Current state / phase | UX rule |
| --- | --- | --- | --- |
| Q01 | Deployment duration, including a live elapsed counter | Present; preserve in P5+ | Put it on the existing small deployment card. |
| Q02 | Datetime before deployment log lines | Present; preserve | Keep filtering and copying useful without hiding chronology. |
| Q03 | Line numbers restart at 1 for each deployment | Present; preserve | Pagination cursors and display numbers remain different concepts. |
| Q04 | `.env` tab naming | Present; preserve | Do not introduce another “Variables” surface. |
| Q05 | Static “Serving” state and meaningful runtime view | Present; preserve | A healthy static site must not appear Missing. |
| Q06 | Hide container selector when zero/one option exists | Present; preserve | Select a service only when there is an actual choice. |
| Q07 | Readable `shp-...` runtime names | Present; preserve | Stable service identity is more important than exposing IDs. |
| Q08 | Search logs next to Copy in the lighter toolbar | Present; preserve | Keep the redundant dark toolbar removed. |
| Q09 | Minimal `.env`, paste support and secondary row actions | Present; extend P5 | Keep ordinary editing to name/value and one apply action. |
| Q10 | Canonical white icon/favicons and Inter wordmark asset | Present as assets; preserve | Follow DESIGN.md; avoid a new branding redesign as parity work. |
| Q11 | Multi-domain rows, www, per-domain HTTPS and verification | Present; extend P7 | A new hostname must not require a second application. |
| Q12 | Private repository and authenticated branch picker | P6 | Connect once, then select; advanced URLs/keys remain available. |
| Q13 | Push-to-deploy and branch/path filters | P6 | One obvious Auto deploy setting with useful trigger history. |
| Q14 | Automatic HTTPS on a Linux VPS | Partial; P7/P10 | Show DNS/issuance state and resolve setup prerequisites in one flow. |
| Q15 | Manual Traefik settings per application/service | P7 | Common fields first; scoped raw config under advanced controls. |
| Q16 | Per-service domains and other configuration | Present subset; P5/P7/P8 extend | Select the service once, then operate in its context. |
| Q17 | Detected build/start/task command suggestions | Build/start suggestions present; task inference remains explicit | Read repository scripts; let the user review rather than guess a migration. |
| Q18 | Saved task Run now and execution history | Present | Reuse a task instead of repeatedly opening a terminal. |
| Q19 | Missing `.env` values and template import | Template-name discovery present; richer provider/source scans P6 | Offer missing names without importing template values or inventing secrets. |
| Q20 | Secret/password generation | Present in .env row options; database adapters P8 | Put it in the existing value/row action, not a global tool page. |
| Q21 | Pending changes, masked diff, relevant Apply action | Present | Make rebuild versus recreate versus routing visible before execution. |
| Q22 | Jump to failure and deployment phase timeline | Failed-step filtering present; richer timeline P9 | Open the failed stage with its logs, exit status and duration. |
| Q23 | Release comparison and clear redeploy/rollback choices | P7 | Distinguish latest source, same revision and a recorded previous release. |
| Q24 | Copy service address, domain, port or database connection | P7/P8 | Keep secret-bearing copy actions deliberate and permission checked. |
| Q25 | Duplicate an application | P8/P11 | New storage/domains by default; review copied credentials and dependencies. |
| Q26 | Favorites, optional tags and saved views | P9/P11 | Organize without mandatory project creation. |
| Q27 | Keyboard command palette and direct actions | P9 | Search the same apps/actions available in the UI. |
| Q28 | Preserve tab/service/filter/log position | P5/P9 | Avoid reloads and layout shifts during navigation. |
| Q29 | Pause/resume log following and bounded export | P9 | Keep the viewer fast and its memory limit explicit in implementation. |
| Q30 | Actionable failure/recovery notifications | P9 | Quiet while unchanged; combine repeated alerts. |
| Q31 | Cancel, retry, supersede stale queued deployments | P5/P6 | Report partial state instead of pretending cancellation reversed work. |
| Q32 | Basic resource and disk diagnostics | P9 | Surface the cause near the failing app, not an unrelated analytics dashboard. |
| Q33 | Bulk restart/redeploy/image update | P9/P12 | Preview effects, limit concurrency and show each application's result. |
| Q34 | Small maintained examples and recipes | P6/P8/P10/P13 | A handful of reliable examples before a giant catalog. |
| Q35 | Performance and perceived speed | Every phase | Cached content, bounded background work, event updates and measured regressions. |
| Q36 | Review effective Compose/source configuration | P5/P7 | Show inheritance and reset overrides without making users manage two primary copies. |
| Q37 | Backups and a tested restore action | P8 | A green upload result is not proof that application data can be recovered. |
| Q38 | Maintenance mode without stopping workers/databases | P7 | Change routing separately from workload/data lifecycle. |

## 7. Dependency and release checkpoints

**1.0 targets Phase 10's single-server production gate.** Phases 5–9 contain both
required core outcomes and later parity breadth; optional breadth must not delay
a dependable release. Phase 5 is required in full. Before 1.0, require GitHub
private source selection and signed push deployments; common per-service routing
and working VPS/dashboard HTTPS; explicit previous-release recovery; tested
Shipper and supported application-data backup/restore; health/resource diagnostics,
actionable notifications and a scoped automation API; and the full Phase 10
installation, security, upgrade, recovery and performance checks.

PR previews, additional source providers, specialist Docker/Compose/proxy options,
every database adapter, full CLI/MCP, file-browser/log-drain/channel/traffic breadth,
and Phases 11–14 are later parity. Preserve their milestone IDs instead of
pretending they are required for the first stable release. The focused release
mapping is also in [the roadmap](docs/roadmap.md).


| Checkpoint | Required result | Claims that must remain limited |
| --- | --- | --- |
| P5 complete | Migrations/tasks and daily editing are dependable locally. | Private Git, full rollback, managed backups and remote-host parity are still open. |
| P6 complete | Private source/image workflows and automatic deployments work. | Public-fork isolation and remote build capacity need their own target work. |
| P7 complete | Advanced single-server runtime/routing/Compose controls and operator rollback work. | Stateful updates do not imply automatic DB reversal or zero downtime. |
| P8 complete | Application data and control-plane state have tested recovery paths. | Each database engine/backup format needs its own support declaration. |
| P9 complete | Operators and agents can diagnose and act through shared interfaces. | A few metrics/channels do not establish every Sentinel/notification/provider feature. |
| P10 complete | A supportable single-server release can be installed, updated and recovered on tested hosts. | This is a production-readiness checkpoint, not full Coolify breadth parity. |
| P11–P12 complete | Collaboration, configuration reuse, remote builds/deployments and scoped transfers work. | Replicas require application/state/load-balancer readiness; controller HA remains separate. |
| P13–P14 complete | Catalog, optional integrations and specialist compatibility have declared tests. | Parity is still snapshot-specific and any untested recipe/field remains an open entry. |

Priority within a phase is: protect a running application and data; make the
ordinary task simple; expose the necessary expert control; add convenience;
then broaden integrations. Templates and provider count must not delay recovery,
correct lifecycle ordering, or safe credentials.

## 8. How to keep this audit honest

- Each F-ID needs an implementation issue, a declared support boundary and an
  acceptance scenario before it can become Present.
- Track native workflow completion separately from “can run this in Compose.”
- Update this file and `docs/roadmap.md` together; completed status requires code
  and verification, not just a new setting or adapter interface.
- Re-audit a new pinned Coolify release before claiming current parity. Compare
  docs, route/schema changes and behavior; a new marketing list alone is insufficient.
- Record deliberate implementation differences. Deprecated/optional capabilities
  stay visible until supported or explicitly removed from the agreed parity target.
- Do not turn research source files into Shipper instructions or copy implementation
  code into the project. This audit uses public facts and proposed Shipper designs.

## 9. Evidence and references

### Shipper implementation evidence

- [L01]: [Single-container operations](docs/full-applications.md),
  [configuration](internal/deploy/config.go), [builder](internal/deploy/build.go),
  [runtime](internal/deploy/docker.go), [runtime actions](internal/deploy/runtime.go).
- [L02]: [Compose operations](docs/compose.md),
  [reference-loader/CLI boundary](internal/deploy/compose.go),
  [Compose worker](internal/deploy/compose_worker.go),
  [service UI](web/src/components/Services.tsx).
- [L03]: [Durable worker](internal/deploy/worker.go),
  [deployment store](internal/store/deployments.go),
  [Compose recovery tests](internal/deploy/compose_worker_test.go),
  [real Compose tests](internal/deploy/compose_integration_test.go),
  [browser checks](tests/e2e/foundation.spec.ts).
- [L04]: [API routes](internal/httpapi/server.go),
  [models](internal/store/models.go), [API documentation](docs/api.md),
  [packaging](Dockerfile), [security boundary](SECURITY.md),
  [roadmap](docs/roadmap.md).

The Shipper status assessment is a static review of these files and the recorded
tests, not a new execution of the application test suites during this audit.

### Pinned Coolify source evidence

- **C01:** [Deployment job and pre/post command implementation][C01].
  In particular, `run_pre_deployment_command` and `run_post_deployment_command`
  around lines 6211–6300 establish the current-container behavior.
- **C02:** [Webhook routes][C02] and [API/OpenAPI contract][C15] show source and
  automation endpoints; the docs specify user-facing provider behavior.
- **C03:** [Scheduled task execution][C03] and
  [scheduled task model](https://github.com/coollabsio/coolify/blob/0becfba9113063336b09860579627fd218cb3fc4/app/Models/ScheduledTask.php).
- **C04:** [Integration token provider registry][C04] and
  [resource secret manager](https://github.com/coollabsio/coolify/blob/0becfba9113063336b09860579627fd218cb3fc4/app/Services/SecretManagers/ResourceSecretManager.php)
  verify Cloudflare/Doppler/Infisical/Vault capabilities. General DNS docs focus
  on manually created records; managed Cloudflare behavior is also implemented
  in the [DNS API][C05].
- **C05:** [DNS API][C05] and
  [proxy API](https://github.com/coollabsio/coolify/blob/0becfba9113063336b09860579627fd218cb3fc4/app/Http/Controllers/Api/ServerProxyController.php).
- **C06:** [Database backup job][C06] and
  [database API](https://github.com/coollabsio/coolify/blob/0becfba9113063336b09860579627fd218cb3fc4/app/Http/Controllers/Api/DatabasesController.php).
- **C07:** [Storage backup job][C07] and
  [backup API](https://github.com/coollabsio/coolify/blob/0becfba9113063336b09860579627fd218cb3fc4/app/Http/Controllers/Api/VolumeBackupsController.php).
- **C08:** [Pinned catalog metadata][C08] and
  [Compose recipes](https://github.com/coollabsio/coolify/tree/0becfba9113063336b09860579627fd218cb3fc4/templates/compose).
- **C09:** [Server transfer API][C09] verifies a scoped cross-instance transfer
  workflow rather than physical movement of a host.
- **C10:** [Resource terminal][C10] and
  [file browser](https://github.com/coollabsio/coolify/blob/0becfba9113063336b09860579627fd218cb3fc4/app/Livewire/Project/Shared/FileBrowser.php).
- **C11:** [Server metrics/settings][C11] and
  [log-drain API](https://github.com/coollabsio/coolify/blob/0becfba9113063336b09860579627fd218cb3fc4/app/Http/Controllers/Api/ServerLogDrainsController.php).
- **C12:** [Notification API][C12] and the associated notification setting models.
- **C13:** [Server API][C13],
  [runner provisioner](https://github.com/coollabsio/coolify/blob/0becfba9113063336b09860579627fd218cb3fc4/app/Jobs/ProvisionGithubRunnerJob.php),
  and cloud-provider controllers named in the [OpenAPI contract][C15].
- **C14:** [Authorization policies][C14],
  [account authentication](https://github.com/coollabsio/coolify/tree/0becfba9113063336b09860579627fd218cb3fc4/app/Auth),
  and [audit events](https://github.com/coollabsio/coolify/blob/0becfba9113063336b09860579627fd218cb3fc4/app/Models/AuditEvent.php).
- **C15:** [OpenAPI schema][C15],
  [API routes](https://github.com/coollabsio/coolify/blob/0becfba9113063336b09860579627fd218cb3fc4/routes/api.php),
  [MCP endpoint middleware](https://github.com/coollabsio/coolify/blob/0becfba9113063336b09860579627fd218cb3fc4/routes/ai.php),
  and [MCP implementation](https://github.com/coollabsio/coolify/tree/0becfba9113063336b09860579627fd218cb3fc4/app/Mcp).

### Documentation references

The linked site is convenient for reading; the audited source is available at the
[pinned production docs revision](https://github.com/coollabsio/coolify-docs/tree/b4f93fae4a2417817d4159c0b2377f24a265e5b8/content/docs).

| Ref | Documentation family and material checked |
| --- | --- |
| D01 | [Deployment model][D01], deployment overview and rolling updates |
| D02 | [Application/build methods][D02], static and Dockerfile builds |
| D03 | [Compose][D03], service routing, storage, networking and raw mode |
| D04 | [Image rollback][D04] and image retention |
| D05 | [Nixpacks support and maintenance status][D05] |
| D06 | [Railpack and its Beta qualification][D06] |
| D07 | [Deprecated Swarm support][D07] |
| D08 | [Multi-server requirements and limitations][D08] |
| D09 | [Git sources][D09], deploy keys, GitLab, Bitbucket and Gitea |
| D10 | [Automatic deployments][D10] and manual webhooks |
| D11 | [Previews][D11], trust, variables and cleanup |
| D12 | [Advanced settings][D12] and custom Docker options |
| D13 | [Repository scan][D13] |
| D14 | [Image-only deployments][D14] |
| D15 | [Registries][D15] and build/push behavior |
| D16 | [Scheduled tasks][D16], execution history and deploy-by-tag guides |
| D17 | [Application operations][D17], clone/move and configuration guides |
| D18 | [Container file operations][D18]; also checked in pinned docs/source |
| D19 | [Networking/proxies][D19], domains, DNS, Traefik dynamic config, Caddy, TLS, maintenance/error pages |
| D20 | [Shared variables][D20] and scope behavior |
| D21 | [Database capabilities][D21], engines, SSL and operations |
| D22 | [Database backups][D22] and [restore](https://coolify.io/docs/databases/restore) |
| D23 | [Storage backups][D23], volume/directory/file mount guides |
| D24 | [Instance backup][D24] and restore |
| D25 | [Server transfer][D25] and application migration/resource operations |
| D26 | [Cloud provisioning][D26] |
| D27 | [Server patching][D27] |
| D28 | [GitHub Actions runners][D28] |
| D29 | [Monitoring/Sentinel][D29], metrics and traffic analytics |
| D30 | [Log drains][D30] and destination behavior |
| D31 | [Notifications][D31] and channel guides |
| D32 | [Servers][D32], build roles, SSH, terminals, cleanup and destinations |
| D33 | [Security][D33], roles, 2FA, passkeys, OAuth, OIDC and audit |
| D34 | [REST API][D34], scopes, sensitive permissions, expiry and IP restrictions |
| D35 | [CLI][D35] and command reference |
| D36 | [MCP][D36], capabilities and security |
| D37 | [Optional canvas views][D37] |
| D38 | [Instance administration][D38], installation, update, downgrade and uninstall guides |

[L01]: docs/full-applications.md
[L02]: docs/compose.md
[L03]: internal/deploy/worker.go
[L04]: docs/api.md
[C01]: https://github.com/coollabsio/coolify/blob/0becfba9113063336b09860579627fd218cb3fc4/app/Jobs/ApplicationDeploymentJob.php
[C02]: https://github.com/coollabsio/coolify/blob/0becfba9113063336b09860579627fd218cb3fc4/routes/webhooks.php
[C03]: https://github.com/coollabsio/coolify/blob/0becfba9113063336b09860579627fd218cb3fc4/app/Jobs/ScheduledTaskJob.php
[C04]: https://github.com/coollabsio/coolify/blob/0becfba9113063336b09860579627fd218cb3fc4/app/Models/IntegrationToken.php
[C05]: https://github.com/coollabsio/coolify/blob/0becfba9113063336b09860579627fd218cb3fc4/app/Http/Controllers/Api/DnsController.php
[C06]: https://github.com/coollabsio/coolify/blob/0becfba9113063336b09860579627fd218cb3fc4/app/Jobs/DatabaseBackupJob.php
[C07]: https://github.com/coollabsio/coolify/blob/0becfba9113063336b09860579627fd218cb3fc4/app/Jobs/VolumeBackupJob.php
[C08]: https://github.com/coollabsio/coolify/blob/0becfba9113063336b09860579627fd218cb3fc4/templates/service-templates.json
[C09]: https://github.com/coollabsio/coolify/blob/0becfba9113063336b09860579627fd218cb3fc4/app/Http/Controllers/Api/ServerTransferController.php
[C10]: https://github.com/coollabsio/coolify/blob/0becfba9113063336b09860579627fd218cb3fc4/app/Livewire/Project/Shared/Terminal.php
[C11]: https://github.com/coollabsio/coolify/blob/0becfba9113063336b09860579627fd218cb3fc4/app/Models/ServerSetting.php
[C12]: https://github.com/coollabsio/coolify/blob/0becfba9113063336b09860579627fd218cb3fc4/app/Http/Controllers/Api/NotificationsController.php
[C13]: https://github.com/coollabsio/coolify/blob/0becfba9113063336b09860579627fd218cb3fc4/app/Http/Controllers/Api/ServersController.php
[C14]: https://github.com/coollabsio/coolify/tree/0becfba9113063336b09860579627fd218cb3fc4/app/Policies
[C15]: https://github.com/coollabsio/coolify/blob/0becfba9113063336b09860579627fd218cb3fc4/openapi.json
[D01]: https://coolify.io/docs/core/build-deployment-model
[D02]: https://coolify.io/docs/applications/builds/overview
[D03]: https://coolify.io/docs/applications/builds/docker-compose
[D04]: https://coolify.io/docs/applications/deployments/rollbacks
[D05]: https://coolify.io/docs/applications/builds/nixpacks/overview
[D06]: https://coolify.io/docs/applications/builds/railpack
[D07]: https://coolify.io/docs/core/infrastructure/scaling/docker-swarm
[D08]: https://coolify.io/docs/core/infrastructure/scaling/multi-server-deployments
[D09]: https://coolify.io/docs/applications/sources/overview
[D10]: https://coolify.io/docs/applications/deployments/automatic-deployments
[D11]: https://coolify.io/docs/applications/deployments/preview-deployments
[D12]: https://coolify.io/docs/applications/configuration/advanced
[D13]: https://coolify.io/docs/applications/builds/smart-scan
[D14]: https://coolify.io/docs/applications/deployments/docker-image
[D15]: https://coolify.io/docs/applications/builds/docker-registries
[D16]: https://coolify.io/docs/core/automation/scheduled-tasks/overview
[D17]: https://coolify.io/docs/applications/operations/overview
[D18]: https://coolify.io/docs/core/container-file-browser
[D19]: https://coolify.io/docs/core/networking/proxy/overview
[D20]: https://coolify.io/docs/core/team/shared-variables
[D21]: https://coolify.io/docs/databases
[D22]: https://coolify.io/docs/databases/backups
[D23]: https://coolify.io/docs/core/persistent-storage/storage-mounts/backups
[D24]: https://coolify.io/docs/core/backup-and-recovery/instance-backup
[D25]: https://coolify.io/docs/core/infrastructure/servers/server-transfer
[D26]: https://coolify.io/docs/core/infrastructure/servers/provision-cloud-server
[D27]: https://coolify.io/docs/core/infrastructure/servers/server-patching
[D28]: https://coolify.io/docs/core/infrastructure/servers/github-actions-runners
[D29]: https://coolify.io/docs/core/observability/monitoring/overview
[D30]: https://coolify.io/docs/core/observability/log-drains/overview
[D31]: https://coolify.io/docs/core/notifications/overview
[D32]: https://coolify.io/docs/core/infrastructure/servers/overview
[D33]: https://coolify.io/docs/core/security-model
[D34]: https://coolify.io/docs/api/overview
[D35]: https://coolify.io/docs/cli/what-is-the-coolify-cli
[D36]: https://coolify.io/docs/mcp/what-is-mcp
[D37]: https://coolify.io/docs/core/canvas-views
[D38]: https://coolify.io/docs/core/instance-management/instance-settings

[L05]: docs/commands.md
