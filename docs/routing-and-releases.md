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

## Dashboard and instance HTTPS

The managed Ubuntu installer now starts without a domain. It binds the control
plane on `0.0.0.0:8080`, prints `http://<public-ip>:8080`, and lets the first
administrator claim the instance or use the setup token in
`/var/lib/shipper/setup-token`. Allow TCP 80, 443 and 8080 through the provider
firewall for the first run.

After signing in, open **Settings → Dashboard** and enter the hostname Shipper
should answer on. Shipper validates it, reserves it from application assignment,
stores it as the instance dashboard domain, adds its own route to the global
proxy, and requests a trusted certificate through the owner's ACME contact.
Point DNS at the server first. The dashboard route uses HTTP-01, redirects HTTP
to HTTPS, and carries the instance identity header used by verification.

The direct `http://<public-ip>:8080` listener remains available as the initial
setup and recovery path. Once HTTPS is working, use the dashboard domain for
normal authenticated writes. Removing the domain from Settings removes only the
dashboard route; the control plane stays reachable on 8080.

For manual or reverse-proxied installations, `SHIPPER_PUBLIC_URL` can still set
the canonical external origin, and `SHIPPER_DASHBOARD_UPSTREAM=auto` can connect
the labelled control-plane container to Shipper's edge network. Managed installs
use the SQLite dashboard-domain setting instead of a Compose domain variable.

Traefik obtains and renews certificates through HTTP-01 and persists ACME state
in the instance-owned proxy-data volume. Leave ports 80/443 reachable for
renewals. A DNS AAAA record must point to a reachable IPv6 listener, or be removed.

Changing the dashboard hostname is a live settings action. Changing published
ports, bind address or static ACME settings still requires a maintenance window:
remove only the instance's labelled proxy container, retain its proxy-data
volume, and restart Shipper to recreate it. Do not delete `acme.json` or volumes
as part of an ordinary upgrade.

**Settings → Dashboard → Verify dashboard** checks DNS, a trusted certificate
and this instance's health route. It reports DNS/TLS/routing failures and
observed certificate expiry. An application that already uses the hostname must
move before the dashboard can claim it. No DNS provider account is needed.

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
Public ACME issuance has passed on the staging VPS, including the dashboard
hostname. Controlled renewal, DNS changes and failure recovery remain release
gates, not claims inferred from a local TLS fixture.

References: [Traefik HTTP-01 and renewal](https://doc.traefik.io/traefik/v3.2/https/acme/),
[Traefik empty-service maintenance responses](https://doc.traefik.io/traefik/getting-started/faq/).
