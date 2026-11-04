# Routing and recorded releases

## Domains and routing

An application keeps its domain list independently of its deployment source.
Compose services each have their own primary hostname, aliases, HTTPS and
verification. Root-domain www expansion remains available. The Domains tab's
Routing options are collapsed by default and apply only to the selected hostname:

- Path prefix adds a Traefik PathPrefix match. Strip prefix removes that path
  before sending the request to the service. A prefix is a literal path, with
  Traefik's prefix semantics; `/api` also matches `/apiv2`.
- Compression uses Traefik's HTTP compression middleware.
- Response headers accept one `Name: value` per line. HTTP framing, cookies,
  redirects and Shipper's release-identity header cannot be overridden.
- Maintenance mode responds with HTTP 503 while keeping containers running.
  It does not prevent internal traffic, scheduled tasks or background jobs.

Save domains creates a route-only queued operation. Shipper validates inputs,
writes the file-provider configuration atomically, and waits for Traefik to
acknowledge services, routers and revision-specific middleware names. A failed
route update attempts to restore the preceding configuration. No Docker labels,
Docker socket provider, raw router expressions or arbitrary backend URLs are
accepted from applications. Existing aliases and HTTP-to-HTTPS redirects remain.
Visit links include the selected path prefix. One hostname belongs to one
application/service; sharing a hostname between services by path is later work.

Verify checks DNS, reachability and serving-release identity, with normal TLS
validation. Successful HTTPS checks record certificate issuer and expiry in the
routing options. Results are on-demand observations, not a background expiry
monitor. A 503 is a valid verified response for deliberate maintenance mode.

## VPS and dashboard HTTPS

For a fresh packaged Linux installation, point a public hostname to the VPS and
allow inbound TCP 80/443. Keep the dashboard's direct port bound to loopback.
Set these values in Compose's environment file before starting Shipper:

```dotenv
SHIPPER_PUBLIC_URL=https://shipper.example.com
SHIPPER_DASHBOARD_UPSTREAM=auto
SHIPPER_PROXY_BIND=0.0.0.0
SHIPPER_HTTP_PORT=80
SHIPPER_HTTPS_PORT=443
SHIPPER_ACME_EMAIL=owner@example.com
```

`docker compose up -d --build` starts Shipper. The worker provisions its private
Traefik container even before the first application, connects the explicitly
labelled control-plane container to its own edge network, and adds the canonical
dashboard route. HTTP redirects to HTTPS; the dashboard uses Secure session
cookies. Its hostname is reserved from application assignment. The direct
loopback listener remains a recovery path through SSH port forwarding; once the
canonical HTTPS origin is set, use that origin for normal authenticated writes.
A fresh first-admin setup capability is still required.

Traefik obtains and renews certificates through HTTP-01 and persists its ACME
state in the instance-owned proxy-data volume. Leave ports 80/443 reachable for
renewals. A DNS AAAA record must point to a reachable IPv6 listener, or be removed.
An existing reverse proxy may instead terminate dashboard TLS; leave the managed
dashboard upstream unset and set the canonical public URL to its external origin.
Native deployments can explicitly configure a Docker-reachable HTTP upstream;
`auto` is supported only inside the labelled packaged control plane.

Changing published ports, bind address or ACME static settings on an existing
proxy is intentionally not a live UI edit. Shipper reports a configuration
mismatch and leaves the existing proxy intact. During a maintenance window,
remove only the instance's labelled proxy **container**, retain its proxy-data
volume, and restart Shipper to recreate it. Do not delete `acme.json` or volumes
as part of an ordinary upgrade. Broad proxy replacement with reviewed automatic
recovery remains later parity work.

Settings > Proxy > Verify dashboard checks the configured hostname, a trusted
certificate and this instance's health route. It reports DNS/TLS/routing failures
and observed certificate expiry. An existing application with the same hostname
must move before enabling the dashboard route. No DNS provider account is needed.

## Runtime limits

Runtime limits is collapsed in application settings and each Compose service's
settings. CPU, PID, restart policy, startup timeout and graceful stop are optional.
Single-container defaults remain one CPU, 256 PIDs, unless-stopped restart,
45-second readiness deadline and 10-second stop grace. Startup-command timeout
extends readiness as before. The startup timeout is a deadline, not an obligatory
delay; healthy applications promote immediately. CPU limits are hard limits,
not reservations. Docker's resource support still depends on the host.

An optional traffic-drain delay keeps the old single-container release alive
briefly after routing switches, then sends Docker's normal stop signal and waits
for its stop grace before removal. Existing connections can finish during this
window, but Shipper does not promise indefinite WebSocket draining. Compose uses
its service stop grace and stateful reconciliation; it has no blue-green drain.
Service defaults inherit Compose where represented. Clearing an explicit resource
or restart override conservatively rebuilds the source model to restore inheritance.
Ordinary runtime edits reuse pinned images and enter the existing review flow.

## Restore release

Select a previous successful deployment in Deployments and choose Restore release.
The review compares runtime configuration, confirms retained domains/data and
skipped deployment migrations, and rejects stale state before enqueueing.
The target must belong to this application and all recorded local image artifacts
must still exist. Compose additionally needs its encrypted resolved plan and named
volumes. Missing artifacts reject the operation rather than fetching a changed
branch/tag and calling that a rollback. There is no automatic image pruning yet;
external Docker cleanup can make an older release unavailable.

Restoration creates a new durable deployment with `rollbackId`, reuses the target's
recorded code and runtime settings, and retains current domains and push policy where the restored source supports
GitHub triggers. Image, inline and unconnected sources disable push deployment.
Before-deploy and after-deploy commands are skipped. Before-start still runs when
its foreground container actually starts. The serving release's before-stop hook
and graceful stop apply during replacement. Failed single-container readiness or
promotion preserves the previous serving release. Compose attempts to restore the
previous pinned stack and its stopped/running intent if reconciliation fails.

Persistent volume data and database schema are **not rolled back**. A historical
runtime can be incompatible with newer data; review migration compatibility before
restoring it. Database/data recovery is Phase 8. Applications, unrelated containers
and persistent volumes remain subject to existing ownership and deletion rules.

## Supported boundary and verification

The 1.0 Phase 7 core is routing controls, dashboard HTTPS setup/diagnostics,
per-service runtime controls and explicit recorded-release recovery. Wildcards,
DNS-01, managed DNS, uploaded certificates, general raw Traefik configuration,
TCP/UDP publishing, cross-application networks, Compose profiles/multiple files,
GPU/devices and trusted host mounts remain explicit later-parity milestones.

Local suites cover recorded rollback, missing artifacts, skipped migration hooks,
source/image serving, route-only path/header/maintenance changes, runtime limits,
Compose persistence, desktop/mobile review and trusted local certificate routing.
Public ACME issuance and renewal require a reachable VPS/domain and remain a
Phase 10 release-gate check, not a claim inferred from a local TLS fixture.

References: [Traefik HTTP-01 and renewal](https://doc.traefik.io/traefik/v3.2/https/acme/),
[Traefik empty-service maintenance responses](https://doc.traefik.io/traefik/getting-started/faq/).
