# Next-gen observability

Status: proposal, 2026-11-16
Scope: Shipper 1.1 and the stable 1.0 follow-up
Audience: product, design, engineering

## The problem

The Garage deployment failed in ten seconds. Shipper showed this:

> Compose operation failed; check the deployment output

The logs showed a container becoming unhealthy, then a red `failed` line. Nothing said which service failed, why it failed, or what to do next. The only path forward was the troubleshooting page and a terminal:

```sh
docker ps -a --filter label=io.shipper.application.id=<application-id>
docker logs --tail 200 <container-name>
docker inspect <container-name> --format '{{json .Config.Entrypoint}} {{json .Config.Cmd}}'
```

That is the gap. Shipper knows the container state, the health check, the exit code, the compose service, and the last log lines. It shows them as raw output and asks the user to do the synthesis.

The same gap exists at the host level. CPU, memory, disk pressure, Docker health, proxy health, and the Shipper service are spread across `df`, `docker system df`, `systemctl`, `journalctl`, and `shipperctl`. The System page shows some of it, but the dashboard does not, and the failure path does not connect to it.

This plan closes both gaps without turning Shipper into a monitoring product.

## The vision

Shipper should answer three questions without a terminal:

1. **Is the host healthy?** At a glance, on the dashboard, in one quiet strip.
2. **What happened to this deployment?** In one click, with a cause, evidence, and the failed service.
3. **What should I do next?** One relevant action, or a diagnosis bundle to paste into an agent.

The same answers must exist as structured JSON. The UI is a view of the diagnosis, not a separate source of truth. An agent should be able to answer "why did this fail" from `shipper doctor <app> --json` without reading a dashboard.

The product should also speak when nobody is looking. A failed deployment, a disk crossing 80%, a certificate nearing expiry, and an application unhealthy for five minutes should reach the user through the existing notification path. The panel is where the detail lives, not the only place the product talks.

## Principles

**One surface, three depths.** Glance on the dashboard, inspect in a drawer, deep dive on the System page. No new tabs, no new navigation items.

**Evidence, not dashboards.** Every number and chart must lead to a check, a cause, or an action. If it does not help someone decide, it does not ship.

**Quiet by default.** Normal is neutral. Color appears only for a real threshold or a failure. No animated gauges, no wall of green dots, no mission control.

**Bounded by design.** Twenty-four hours of history, one-minute samples, fixed retention, bounded log tails, bounded diagnosis output. No Prometheus, no log index, no APM.

**Agent parity.** Every diagnosis the UI renders is available as JSON with the same schema, the same evidence, and the same unknowns. The copy button and the CLI return the same artifact.

**No new app tabs.** The application already has Overview, Deployments, Logs, Domains, Networking, Terminal, .env, Tasks, Backups, Garage, and Settings. The diagnosis belongs in the existing failure and health surfaces, opened on demand.

## The three surfaces

### 1. System pulse on the dashboard

A single strip above the application list. Three segments, one card, no title.

```text
┌──────────────────────────────────────────────────────────────────────┐
│  CPU         Memory        Disk                                      │
│  24%         39%           41%                                       │
│  ▁▂▃▅▂      ▁▂▃▅▂        ▁▂▃▅▂                                      │
└──────────────────────────────────────────────────────────────────────┘
```

Desktop: label and value on one line, sparkline under the value. Mobile: three equal columns, label above value, sparkline below. Height stays under 64px on desktop and 76px on mobile.

Each segment is a button. It opens the System drawer focused on that metric. The whole strip has one hairline border and the same `10px` radius as other cards.

Behavior:

- Samples refresh every 60 seconds. The strip updates in place, no layout shift, tabular numbers.
- The sparkline shows the last 24 hours at a downsampled resolution.
- Normal is muted. A warning dot appears only when a threshold is sustained: five minutes for CPU and memory, immediately for disk.
- If the last sample is older than three minutes, the strip says `Stale` and the age. It never shows fresh-looking numbers from stale data.
- If the metrics endpoint is unavailable, the strip shows `Unavailable` with a link to System. It does not block the application list.

Thresholds:

| Metric | Warning | Critical |
| --- | --- | --- |
| CPU | over 85% for 5 minutes | over 95% for 5 minutes |
| Memory | over 85% | over 95% |
| Disk | over 80% | over 90% |

The strip is visible to any signed-in user with read access. It contains aggregate host usage, no application names, no secrets.

### 2. Diagnosis drawer

The drawer opens from four places, all on existing surfaces:

- The failed deployment card on the Deployments tab.
- The application issue alert on the Overview tab.
- The failed release entry in the deployment history menu.
- A `Diagnose` action in the application actions menu when the latest release failed.

It uses the existing right-side drawer pattern from `ContainerDrawer`, with focus trapping, Escape to close, and the same scrim.

```text
┌ Why this failed ────────────────────────────────────────────────┐
│ Garage never became healthy                                     │
│ The container is running, but its health check did not pass.    │
│                                                                  │
│ Cause       Health check failed                     High          │
│                                                                  │
│ Checks                                                           │
│ ● Docker            healthy                                      │
│ ● Image pull        healthy                                      │
│ ● Container         unhealthy     garage                         │
│ ● Readiness         failed                                       │
│                                                                  │
│ garage                                                           │
│ Running · unhealthy · exit 0 · 0 restarts                        │
│                                                                  │
│ Last 20 lines                                          View all  │
│ 2026-11-16 12:06:43  container garage starting                   │
│ 2026-11-16 12:06:44  container garage is unhealthy               │
│                                                                  │
│ [Open Compose source]  [Retry release]                           │
│ [Copy for agent]                                                 │
└──────────────────────────────────────────────────────────────────┘
```

Sections, in order:

1. **Cause.** One sentence, one code, one confidence. The sentence is the answer. No jargon.
2. **Checks.** The layers that matter for this failure, each with a state and the service name when relevant. Healthy layers stay collapsed and quiet. Failed and unknown layers are visible.
3. **Failed service.** Container state, health, exit code, restart count, OOM flag, and the runtime command. The command is redacted.
4. **Logs.** The last 20 lines for the failed service, with `View all` opening the existing Logs tab filtered to that service. For an instant exit, the previous container's logs are included when available.
5. **Actions.** The smallest relevant actions, never a generic "fix it" button. Examples: open Compose source, open environment, open domains, retry release, copy for agent.
6. **For agents.** `Copy for agent` copies the full JSON bundle. A small `Download` is available for support.

The drawer does not replace the deployment log. It sits above it as the explanation, and the log remains the raw evidence. The existing `Show failed step` button stays, but it becomes a secondary action inside the drawer.

The drawer footer links to System for the full checks, the 24-hour charts, and the resource table. That is the only path to the deep dive, so the dashboard stays a dashboard and the System page stays the place for the full picture.

No new route. The drawer state is URL-addressable as `?diagnose=<deployment-id>` so a refresh or a shared link reopens the same diagnosis.

### 3. System page

The existing System page is the deep dive. It keeps its sections and gains one:

**Capacity.** The 24-hour charts for CPU, memory, and disk, and the disk composition donut. Below the charts, a compact resource table for images, containers, volumes, and build cache. The table is calculated on demand from `docker system df` with a 30-second cache. No auto-refresh while the drawer is closed.

The existing **Health** table stays. Each check gets an `Inspect` action that opens the same drawer in host mode. For checks that require host recovery, the drawer links to the relevant docs section instead of pretending the UI can restart Docker.

**Runtime** gains the Shipper service uptime and the last successful health probe. **Maintenance** stays as it is.

The System page is allowed to be denser than the dashboard. It is still not a Grafana board. Three charts, one donut, one resource table, the existing checks.

## The diagnosis model

The diagnosis is a deterministic object, not a model output and not a log dump. It is built from bounded facts and a small set of rules. It is stored with the deployment so the explanation survives cleanup and later restarts.

```ts
type Diagnosis = {
  schemaVersion: 1;
  generatedAt: number;
  vantage: "control-plane";
  application: { id: string; name: string; method: string };
  deployment: {
    id: string;
    operation: string;
    state: "failed" | "interrupted" | "cancelled";
    stage: string;
    durationSeconds: number;
    createdAt: number;
  };
  cause: {
    code: CauseCode;
    summary: string;
    detail: string;
    confidence: "high" | "medium" | "low";
    evidence: string[];
  };
  checks: Array<{
    layer: string;
    state: "healthy" | "warning" | "failed" | "unknown";
    summary: string;
    observedAt: number;
  }>;
  services: Array<{
    name: string;
    state: string;
    health: string;
    exitCode: number;
    restarts: number;
    oomKilled: boolean;
    command: string;
  }>;
  logs: Array<{ service: string; lines: string[] }>;
  changesSinceServing: string[];
  unknowns: string[];
  nextActions: Array<{
    id: string;
    label: string;
    kind: "navigate" | "retry" | "copy" | "docs";
    target: string;
  }>;
};
```

Cause codes, stable for agents:

| Code | Meaning |
| --- | --- |
| `docker_unavailable` | The Docker socket or daemon is not answering. |
| `image_pull_failed` | The image could not be pulled or authenticated. |
| `build_failed` | The source build failed before an image existed. |
| `container_exited` | The container started and exited before readiness. |
| `healthcheck_failed` | The container is running but Docker marks it unhealthy. |
| `readiness_failed` | Shipper's readiness probe did not pass in time. |
| `port_mismatch` | The route targets a port the container is not listening on. |
| `compose_dependency_unhealthy` | A Compose dependency is unhealthy or missing. |
| `volume_mount_failed` | A bind or volume could not be mounted. |
| `route_unreachable` | The proxy has no healthy route for the hostname. |
| `certificate_failed` | TLS issuance or renewal failed. |
| `disk_pressure` | The data filesystem is low on space. |
| `oom_killed` | The kernel killed the container for memory. |
| `unknown` | The evidence is insufficient. Say so and list what is missing. |

Confidence rules:

- **High** when there is direct evidence: an exit code with logs, an OOM flag, a failed health check, a full disk, a refused Docker socket.
- **Medium** when the cause is correlated: a Compose service unhealthy, a 502 with a port mismatch, a build log signature.
- **Low** when it is a heuristic: a suspicious log line without a direct failure, a certificate warning without an expiry date.

The UI says `Cause` for high confidence and `Likely cause` for medium and low. It never claims certainty it does not have.

### Example: the Garage failure

```json
{
  "schemaVersion": 1,
  "application": { "id": "3f6a1c9e8b7d4a2f5c0e1d9b", "name": "Garage", "method": "compose" },
  "deployment": {
    "id": "d916797b2a5b5e856014ad37",
    "operation": "deploy",
    "state": "failed",
    "stage": "starting",
    "durationSeconds": 10,
    "createdAt": 1794476825
  },
  "cause": {
    "code": "healthcheck_failed",
    "summary": "Garage started, but its health check never passed.",
    "detail": "Docker reports the garage service as unhealthy after 10 seconds. The container did not exit, so the process is running but not ready.",
    "confidence": "high",
    "evidence": [
      "Container shp-...-5a997b2b4d86 is running with health unhealthy.",
      "The deployment stopped at the starting stage after 10 seconds.",
      "The last log lines show the container starting, then failing its health check."
    ]
  },
  "services": [
    {
      "name": "garage",
      "state": "running",
      "health": "unhealthy",
      "exitCode": 0,
      "restarts": 0,
      "oomKilled": false,
      "command": "garage server"
    }
  ],
  "logs": [
    {
      "service": "garage",
      "lines": [
        "2026-11-16 12:06:43 ERROR failed to read /etc/garage.toml",
        "2026-11-16 12:06:44 ERROR health endpoint did not start"
      ]
    }
  ],
  "checks": [
    { "layer": "daemon", "state": "healthy", "summary": "Docker connection", "observedAt": 1794476825 },
    { "layer": "build", "state": "healthy", "summary": "Image pulled", "observedAt": 1794476825 },
    { "layer": "runtime", "state": "failed", "summary": "garage is unhealthy", "observedAt": 1794476825 },
    { "layer": "readiness", "state": "failed", "summary": "Health check did not pass", "observedAt": 1794476825 }
  ],
  "changesSinceServing": [],
  "unknowns": [
    "The application protocol was not tested from outside the host.",
    "The Compose file's health check is the only readiness signal observed."
  ],
  "nextActions": [
    { "id": "open-compose", "label": "Open Compose source", "kind": "navigate", "target": "/applications/.../settings#source" },
    { "id": "retry", "label": "Retry release", "kind": "retry", "target": "d916797b2a5b5e856014ad37" },
    { "id": "copy", "label": "Copy for agent", "kind": "copy", "target": "diagnosis" }
  ]
}
```

## What we collect

| Signal | Source | Interval | Retention | Cost |
| --- | --- | --- | --- | --- |
| Host CPU | `/proc/stat` delta | 60s | 7 days at 1m | Negligible |
| Host memory | `/proc/meminfo` | 60s | 7 days at 1m | Negligible |
| Disk usage | `statfs` on the data filesystem | 60s | 7 days at 1m | Negligible |
| Load | `/proc/loadavg` | 60s | 7 days at 1m | Negligible |
| Container count | existing inventory | 60s | same row | Negligible |
| Container CPU and memory | existing observations | 15s | latest only | Already collected |
| Docker resource breakdown | `docker system df` | on demand | 30s cache | Seconds, drawer only |
| Proxy and Shipper logs | bounded Docker and journal tail | on demand | none | Bounded to 200 lines |
| Deployment failure snapshot | Docker inspect and logs at failure | once per failure | 30 days | Bounded to 256KB |

Host CPU and memory use `/proc` because those values are not namespaced per container in the managed installation, so they describe the host. If the process is constrained by a cgroup, the collector detects it and labels the sample as `container` instead of `host`. The UI shows the vantage in the drawer, not on the pulse.

The metric sample table is intentionally small:

```sql
create table metric_samples (
  sampled_at integer primary key,
  cpu_percent real not null,
  memory_used integer not null,
  memory_total integer not null,
  disk_used integer not null,
  disk_total integer not null,
  load1 real not null,
  containers_running integer not null,
  containers_total integer not null
);
```

Seven days at one-minute resolution is about 10,000 rows. The query for the dashboard groups into 10-minute buckets, returns 144 points, and stays under 50ms.

The failure snapshot is stored separately:

```sql
create table deployment_diagnoses (
  deployment_id text primary key,
  captured_at integer not null,
  schema_version integer not null,
  cause_code text not null,
  cause_summary text not null,
  confidence text not null,
  snapshot_json text not null
);
```

The snapshot is captured when the deployment transitions to failed, before cleanup. The worker writes the snapshot before it records the terminal failed state, so a failure is never visible without its explanation. The capture has a ten-second budget, a 50-container cap, 100 log lines per service, and a 256KB total cap. If the capture times out, the state still moves to failed with partial evidence and `complete: false`. It is never silently discarded.

## API and agent contract

Existing:

- `GET /api/v1/applications/{id}/diagnostics` returns checks, observations, and optional logs.
- `GET /api/v1/applications/{id}/observations` returns the latest per-container facts.
- `GET /api/v1/system/operations` returns host and notification status.
- `shipper doctor <app> --json` reads the diagnostics resource.

New:

- `GET /api/v1/system/overview` returns the current CPU, memory, and disk, plus the downsampled 24-hour series and the threshold state.
- `GET /api/v1/system/diagnosis` returns the host bundle: checks, Docker facts, storage breakdown, service logs when permitted, unknowns, and next actions.
- `GET /api/v1/applications/{id}/diagnostics` gains `cause`, `services`, `nextActions`, and `snapshot`. The existing `schemaVersion` stays, and existing fields remain for compatibility.
- `shipper doctor --host --json` returns the host bundle. `shipper doctor <app> --json` returns the app bundle.
- `shipper doctor <app>` stays human-readable and prints the cause first, then the evidence, then the next actions.

Scopes:

- Pulse and aggregate capacity: `read`.
- App diagnosis and app logs: `read` for checks, `logs` for log tails.
- Host service logs and Docker resource detail: `host-admin`.
- The copy bundle follows the same rules. A user without `host-admin` gets the app bundle with host checks summarized, not the service logs.

The bundle is the support artifact. It contains schema version, instance version, application identity, deployment identity, checks, services, bounded logs, unknowns, and next actions. The copy variant trims logs to the last 20 lines per service and stays under 64KB. The stored snapshot may retain up to 256KB for support. It never contains environment values, tokens, registry credentials, or command output that the redactor cannot scrub. A `redacted: true` field makes the guarantee explicit.

## Replacing the troubleshooting commands

Each command in the current doc maps to a surface. The commands remain only for the case where the control plane itself is unavailable, and that case moves into a short appendix.

| Command | Replaced by |
| --- | --- |
| `docker ps -a --filter label=io.shipper.application.id=...` | Diagnose drawer, Services section, with state, health, exit code, restarts |
| `docker logs --tail 200 <container>` | Diagnose drawer, last 20 lines, with `View all` opening the existing logs |
| `docker inspect ... Entrypoint/Cmd` | Diagnose drawer, runtime command row, redacted |
| `docker logs ... proxy` | System, Checks, Proxy, recent logs (admin) |
| `systemctl status docker` | System, Checks, Docker, with version, API, storage driver, live state |
| `docker info` | System, Runtime, Docker engine facts |
| `sudo systemctl restart docker` | Host recovery appendix, not a UI action |
| `sudo systemctl restart shipper.service` | Host recovery appendix, not a UI action |
| `df -h /var/lib` | System pulse and System, Capacity, disk used and free |
| `docker system df` | System, Capacity, resource breakdown |
| `sudo shipperctl status` | System, Runtime, service state and uptime |
| `sudo shipperctl logs` | System, Checks, Shipper service, recent logs (admin) |
| `sudo journalctl -u shipper.service` | System, Checks, Shipper service, recent logs (admin) |
| `docker ps -a` | System, Capacity, resource summary with container counts and states |
| `docker volume ls` | System, Capacity, volumes list (admin) |
| `curl /api/health` | System, Checks, Health endpoint, with the same JSON |

The troubleshooting doc becomes symptom first, in-app action second. The host commands stay in a collapsed "If the dashboard is unavailable" section so the user still has a recovery path when Shipper cannot render the diagnosis.

## Failure playbooks

**Container exits immediately.** Capture the inspect result before cleanup: exit code, OOM flag, restart count, entrypoint, command, started and finished times, and the last 100 log lines. The cause card says `Exited with code 1 after 0.8s` and shows the last lines inline. `View all` opens the Logs tab filtered to that service. If the container restarted, the previous run's logs are included with a `Previous run` label.

**Compose service unhealthy.** After a Compose failure, list every service with state and health, identify the first unhealthy service, and show its health check command and last output. The cause is `healthcheck_failed` or `compose_dependency_unhealthy`. The drawer offers `Open Compose source` and `Retry release`. It does not restart the service automatically.

**Build failed.** Keep the failed stage, but surface the first error line, not the final exit code. The cause card links to the filtered log at that line. Common signatures are labeled as likely causes with medium confidence. The full build log stays below.

**Readiness failed.** Show the probe target, the timeout, the last response, and the port the container is listening on. If the app binds to `127.0.0.1`, say so. The cause card links to the port and environment settings.

**Route returns 502.** Show the route target, the container port, the proxy state, and the last proxy error. If DNS and TLS are fine, the cause is `port_mismatch` or `route_unreachable`. The action opens Domains with the hostname focused.

**Certificate warning.** Show issuer, expiry, DNS result, and the last ACME error. `Domains > Verify` stays the recovery action. The cause is `certificate_failed` with the exact reason.

**Docker unreachable.** This is the first check. Every application check is marked downstream. The drawer says `Fix Docker first`, links to the host recovery appendix, and does not suggest restarting applications.

**Disk pressure.** The pulse turns amber at 80% and red at 90%. The drawer shows the breakdown and the largest consumers. It links to retention and to the host recovery doc for pruning, but it never runs `docker system prune` itself.

## UI details

**Typography and color.** Use the existing console tokens. The pulse values use tabular numerals, 15px, 550 weight. Labels are 11px muted. The charts use one muted line, a soft area fill at 8% opacity, and the accent only for the current point. Warning uses the existing amber, critical uses the existing red. No new palette.

**Motion.** No animated counters. The sparkline path transitions only when a new sample arrives, over 200ms, and respects `prefers-reduced-motion`. The drawer enters with the existing drawer transition.

**Empty and stale states.** Before the first sample, the pulse says `Measuring` for up to 60 seconds. If the last sample is older than three minutes, it says `Stale` with the age. If Docker is down, the pulse still shows the last host sample and marks the Docker check separately. It never invents data.

**Mobile.** The pulse is three columns with 12px padding. The drawer becomes full-screen. The charts are 120px tall. The diagnosis card stacks its actions. The failure card keeps the cause sentence visible without opening the drawer.

**Accessibility.** Each pulse segment is a button with an accessible name that includes the metric and value. The charts have a text summary and a table fallback in the drawer. The cause card is a live region only when a deployment transitions to failed, not on every poll.

**No bloat checklist.** Before shipping any element:

- Does it answer a question the troubleshooting doc currently answers with a command?
- Does it lead to a check, a cause, or an action?
- Is it hidden until it is needed, or is it part of the one glance strip?
- Can it be removed without losing the answer?

If the answer is no, it does not ship.

## Performance and security

**Budgets.** The overview endpoint stays under 50ms p95 and 5KB per response at the dashboard resolution. The diagnosis endpoint stays under 300ms p95 when the snapshot is stored, and under 2s when it has to capture live facts. The dashboard renders the app list first and the pulse in parallel. The pulse never blocks the inventory.

**Caching.** The overview is cached for 10 seconds in memory. `docker system df` is cached for 30 seconds and only runs when the drawer is open. The diagnosis snapshot is immutable. Live checks are cached for 5 seconds.

**Concurrency.** One diagnosis capture at a time per instance. A second request joins the first or returns the stored snapshot. No unbounded `docker logs` calls.

**Redaction.** Every log line and command passes through the existing deploy redactor and the application's variable masks before it is stored or returned. The snapshot is written after redaction, not before. The copy bundle is safe to paste into a public issue.

**Permissions.** The pulse is aggregate and read-only. The app diagnosis follows the existing read and logs scopes. Host service logs and Docker resource detail require `host-admin`. The System page remains admin-only. The UI never exposes a host shell action.

## Phases

### Phase 0: foundations

- Add `metric_samples` and `deployment_diagnoses` tables and migrations.
- Add the host sampler to the operations service, 60-second interval, bounded retention.
- Add the failure capture hook before cleanup.
- Add the diagnosis cause rules for the first five codes: `docker_unavailable`, `image_pull_failed`, `build_failed`, `container_exited`, `healthcheck_failed`.
- Add the API fields and the CLI JSON shape.

Acceptance: samples survive a restart, a failed compose deployment stores a snapshot, `shipper doctor <app> --json` returns the cause, and existing clients ignore the new fields.

### Phase 1: diagnosis drawer

- Add `DiagnosisDrawer`.
- Replace the generic failure card in `LogViewer` with the cause card and the drawer action.
- Add the `Diagnose` action to `ApplicationIssue` and the deployment history menu.
- Add `?diagnose=<deployment-id>` to the URL state.
- Add the Compose service list, the previous-run logs, and the build first-error line.
- Add `Copy for agent` and `Download`.

Acceptance: the Garage failure shows `Garage never became healthy` with the unhealthy service and last logs within two seconds of opening the Deployments tab. The troubleshooting doc's `docker ps` and `docker logs` steps are no longer needed for that case.

### Phase 2: pulse

- Add `SystemPulse` above the application list.
- Add the overview endpoint with the 24-hour downsampled series.
- Add the System drawer at the capacity view.
- Add the thresholds and stale states.

Acceptance: the pulse matches `top`, `free`, and `df` within 5%, never blocks the app list, and adds no navigation.

### Phase 3: host diagnosis and agent parity

- Add the host bundle endpoint and `shipper doctor --host`.
- Add the proxy, Shipper service, and Docker resource checks to the System drawer.
- Add the disk composition donut and the resource table.
- Add the docs appendix for panel-down recovery.
- Rewrite the troubleshooting page around the in-app actions.

Acceptance: an agent can answer "why did this fail" and "what is the host state" from JSON alone. The copy bundle is under 64KB, redacted, and contains the unknowns.

### Phase 4: speak when you are not looking

- Reuse the existing notification outbox for the new thresholds: deployment failed, app unhealthy for five minutes, disk over 80%, certificate under 14 days, backup failed, update failed.
- Add a once-per-day suppression for capacity warnings and a recovery notice.
- Show the same thresholds in the pulse and the notifications.
- Add a `Test notification` path that includes the diagnosis link.

Acceptance: a failed deployment notifies once, a disk crossing 80% notifies once per day, and the notification deep-links to the diagnosis drawer or the System check.

## Non-goals

- Prometheus, Grafana, or a metrics database.
- Distributed tracing or APM.
- Log search indexing or full-text search across history.
- Per-container 24-hour CPU and memory charts.
- A custom dashboard builder.
- A generic alert rule builder.
- A host terminal or a `docker system prune` button.
- A mobile app.
- Replacing host recovery when the control plane is down.

## Decisions needed

1. **Should viewers see the pulse?** Recommendation: yes, aggregate only. It answers "is the host healthy" without exposing application data.
2. **Is seven days of samples worth it?** Recommendation: yes, one-minute resolution is about 10,000 rows. Display 24 hours, keep a week for context.
3. **Should the UI restart Docker or the Shipper service?** Recommendation: no. Show the check and the diagnosis, link to host recovery, keep destructive host actions out of the panel.
4. **Should Compose dependencies auto-restart?** Recommendation: no. Surface the unhealthy service and the evidence first. Auto-restart hides the cause.
5. **Should the diagnosis bundle be available to `read` tokens?** Recommendation: yes for app diagnostics, with logs gated by the `logs` scope and host service logs gated by `host-admin`.

The product is not missing another dashboard. It is missing the sentence that explains the failure, the evidence behind it, and a clean path to the next action. Build that, keep the pulse quiet, and the panel will feel complete without growing into a monitoring product.
