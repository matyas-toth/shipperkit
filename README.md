# Shipper

[![CI](https://github.com/matyas-toth/shipperkit/actions/workflows/ci.yml/badge.svg)](https://github.com/matyas-toth/shipperkit/actions/workflows/ci.yml)
[![Release](https://github.com/matyas-toth/shipperkit/actions/workflows/release.yml/badge.svg)](https://github.com/matyas-toth/shipperkit/actions/workflows/release.yml)

A lightweight, self-hosted application platform built with Go, SolidJS, and SQLite.

Deploy local projects, public/private GitHub repositories, direct images,
inline Dockerfiles or Compose applications. Configure lifecycle commands and
scheduled tasks, automate signed push deployments, review configuration changes,
and restore recorded releases. Dockerfile, Railpack and static Apache builds
share the same application model.

New applications use a guided source, configuration and review flow. Automatic
build selection recommends a method after inspecting the source. Workers skip
HTTP settings, and Compose previews show services and their route targets.
Review with defaults skips the configuration step when no overrides are needed.

Service logs, editable environments, domain routing, health checks, persistent
volumes, and application lifecycle controls are available from the dashboard.
Compose builds finish before updates begin, and failed startup attempts restore
the previous pinned container configuration where possible. Persistent data is
retained. Encrypted instance recovery, local/S3 backup plans, cold-volume archives
and PostgreSQL/MySQL/MariaDB restore copies are available. Release rollback and
data recovery remain separate operations.
System diagnostics, quiet transition notifications, reviewed retention/proxy
repair, scoped operations tokens and a focused CLI share the same application
model. The dashboard remains a complete manual workflow. See
[operations and local deployments](docs/operations.md).
See [backups and recovery](docs/backups.md), [Compose operations](docs/compose.md) and [commands and tasks](docs/commands.md).
The [feature parity audit](feature_parity.md) compares Coolify capabilities and
sets the next [phases and milestones](docs/roadmap.md).
The 1.0 release candidate adds a [managed Ubuntu installer](docs/installation.md),
automatic HTTPS and [owner 2FA](docs/authentication.md). The installer runs
without arguments, opens the first-run dashboard on `http://<public-ip>:8080`,
and leaves the dashboard domain to **Settings → Dashboard**. See
[release acceptance](docs/release-acceptance.md) for what has been verified and
what remains before the stable release.

## Run locally

Requirements: Go 1.26+, Node.js 22.12+ (24 LTS recommended), and a running Docker
Engine or Docker Desktop using Linux containers. Native development also needs
Git 2.36+ and Docker CLI with Buildx and Compose; the production image includes them.

```sh
npm ci
npm ci --prefix web
npm run dev
```

Open [localhost:5173](http://127.0.0.1:5173).
Read the first-run setup token from `.shipper/setup-token`, then create your
administrator account. The token is a local capability and is never exposed by
the API. On Windows, use `Get-Content .shipper/setup-token`.

The Go executable is compiled before the dev runner starts. Frontend edits
hot-reload; restart `npm run dev` after Go changes. `SHIPPER_GO` can point to a Go
executable outside PATH. The runner also recognizes the local Shipper tool cache.

The SDK uses Docker's default socket or `DOCKER_HOST`, `DOCKER_TLS_VERIFY`, and
`DOCKER_CERT_PATH`. It does not automatically read CLI Docker contexts.
If Docker Desktop's default named pipe is unavailable on Windows:

```powershell
$env:DOCKER_HOST = 'npipe:////./pipe/dockerDesktopLinuxEngine'
npm run dev
```

## Run the embedded application

```sh
npm run build
./bin/shipper
```

On Windows run `./bin/shipper.exe`. Open
[localhost:8080](http://127.0.0.1:8080).
This serves the compiled SPA from the Go executable with no Node server.

## Docker development package

```sh
docker compose up --build -d
docker compose exec shipper cat /data/setup-token
```

Open [localhost:8080](http://127.0.0.1:8080) and use that setup token.
On Linux, set `DOCKER_GID=$(stat -c '%g' /var/run/docker.sock)` before starting
Compose so the non-root process can access the socket. On Docker Desktop the
default socket group generally works.

The image runs as UID/GID 65532, drops Linux capabilities, has a read-only root
filesystem, and stores metadata in the `shipper-data` volume.
A Docker socket still grants host-level control even when its mount uses `:ro`.
Keep the dashboard private. Run integration tests on a disposable Docker host.
Shipper's mutations check application and instance ownership before removing
containers. It never adopts unrelated workloads or removes application volumes.

`docker compose down` stops Shipper without removing its metadata volume or
application data. Avoid `down -v` unless you deliberately want to erase Shipper's
administrator and metadata.

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `SHIPPER_ADDR` | `127.0.0.1:8080` | HTTP listener |
| `SHIPPER_DATA_DIR` | `.shipper` | Private SQLite and setup-token directory |
| `SHIPPER_PUBLIC_URL` | Request origin | Optional canonical origin for manual installs, e.g. `https://shipper.example.com`, without a trailing slash. Managed installs store the dashboard domain in Settings. |
| `SHIPPER_DASHBOARD_UPSTREAM` | unset | `auto` for packaged dashboard HTTPS, or an explicit Docker-reachable HTTP origin |
| `SHIPPER_SETUP_TOKEN` | Generated local file | Optional first-run capability of at least 32 characters |
| `DOCKER_HOST` | SDK platform default | Unix socket, Windows named pipe, or TLS-protected TCP |
| `DOCKER_TLS_VERIFY` | unset | Must be `1` for TCP connections |
| `DOCKER_CERT_PATH` | SDK default | Docker TLS certificates |
| `SHIPPER_PORT` | `8080` | Compose host port |
| `DOCKER_GID` | `0` | Compose supplementary group for the Docker socket |
| `SHIPPER_PROXY_BIND` | `127.0.0.1` | Address on the Docker host for Traefik's published ports |
| `SHIPPER_HTTP_PORT` | `8780` | Traefik HTTP port on the Docker host |
| `SHIPPER_HTTPS_PORT` | `8743` | Traefik HTTPS port on the Docker host |
| `SHIPPER_ACME_EMAIL` | unset | Enable the Let's Encrypt resolver for custom domains |
| `SHIPPER_ACME_CA` | Let's Encrypt production | Optional ACME directory, including staging |

When putting Shipper behind an HTTPS reverse proxy, set `SHIPPER_PUBLIC_URL` to
its exact external origin. This fixes the canonical origin used by OAuth callbacks
and origin checks. Session cookies become Secure on HTTPS requests regardless.
Forwarded host headers are intentionally not trusted. The default loopback listener is
for local operation. Traefik is provisioned lazily for the first deployment or
dashboard domain in manual setups.

For a dedicated Ubuntu 24.04 x86-64 VPS, run:

```sh
curl -fsSL https://shipperkit.dev/install.sh | sudo bash
```

The installer prints the dashboard address and port, and stores the dashboard
domain in Settings. The same script can be run from a checked-out repository or a
verified local image archive. See [managed installation, updates, and recovery](docs/installation.md)
and [releases, CI and staging](docs/releases.md).

## What the dashboard shows

The application is the primary object. Its containers and deployment history
live in its detail screen. A Compose stack can have several containers without
becoming several applications. Legacy ownership discovery recognizes:

```yaml
labels:
  io.shipper.managed: "true"
  io.shipper.application.id: "a-stable-application-id"
```

New resources also carry `io.shipper.instance` and `io.shipper.deployment.id`.
The instance identity is persisted in SQLite. Another Shipper installation's
resources are excluded; legacy discovered records remain inspectable but cannot
be redeployed or deleted through the deployment pipeline.

SQLite retains a stable application record if its container disappears.
Container state, health, and application-level state are separate. A disconnected
daemon leaves the last complete inventory visible with an explicit warning.
Reconnects and periodic reconciliation repair missed Docker events.

The inventory window is virtualized, and search/status filters run locally.
Selecting an application opens Overview, Deployments, Logs, Domains, .env, Tasks, and Settings.
Container inspection uses a drawer. Navigation retains the query cache and shell.
Settings separates **Account settings** from **Server settings**. Account holds the
signed-in profile and personal 2FA. Server holds Instance, Dashboard, Connections,
Recovery and Access policy; the instance-wide 2FA requirement lives under Access
policy and the canonical dashboard hostname lives under Dashboard.

## Deploy an application

Connect GitHub in **Settings → Connections** to choose private repositories, or enter a public HTTPS
Git URL in **New application**. Direct images and inline Dockerfiles are also
available. See [sources and deployment triggers](docs/sources.md). Select a branch or use
the remote default, and choose **Deploy**. Automatic mode prefers a root Dockerfile,
then a supported language manifest through Railpack, then a plain `index.html`.
The deployment records the resolved method. Explicit method selection overrides
this choice. Configuration supports a repository subdirectory, commands, health
path, memory limit, and container port. Variables can be supplied before the first
deployment and later through the .env tab.

Static sites can deploy an existing publish directory directly or run an optional
Node build before Apache packaging. Dockerfile install/build steps belong in the
Dockerfile. Railpack detects its toolchain and accepts command overrides. A runtime
must listen on `0.0.0.0`; the port comes from the explicit setting, runtime `PORT`,
a sole image EXPOSE declaration, or 3000. Apache uses 8080 internally.

A local application gets `http://<application-id>.localhost:8780`. **Redeploy**
builds the current branch. The adjacent actions menu supports an uncached rebuild,
start, stop, and restart. **Deployments** records build and routing operations;
**Logs** streams runtime output separately. **Settings** saves a new name without
redeploying and uses **Review changes** then **Apply changes**. Runtime-only edits reuse the active
image where the saved model can represent them safely. **Redeploy** always fetches
and builds the source again.

**Domains** supports up to 32 hostnames with independent HTTPS and verification,
one primary hostname, and optional aliases redirected to it. Root domains can add
`www` automatically. **Save domains** only updates routing. HTTPS requires an ACME
email, public DNS, and reachable ports; local addresses use HTTP. See
[application operations](docs/full-applications.md) and
[static site operations](docs/static-sites.md).

See [routing, VPS HTTPS and recorded release recovery](docs/routing-and-releases.md)
for path routing, maintenance, certificate diagnostics, runtime limits and rollback.

## Password recovery

Run from the same data directory as the running instance, or set
`SHIPPER_DATA_DIR` to its data directory:

```sh
./bin/shipper reset-password
# Docker:
docker compose exec shipper shipper reset-password
```

Enter the administrator email and the new password in the interactive terminal.
The password is not echoed or accepted as a command-line argument. Recovery
revokes every existing session. Protect backups as credentials.

## Validation

```sh
npm run check
npm test
node scripts/go.mjs vet ./...
npm run build
npx playwright install chromium
npm run test:e2e
npm run test:docker
npm run test:deploy
npm run test:full
npm run test:compose
npm run test:tasks
# After building shipper:local:
npm run test:package
```

- Go tests cover setup races, session persistence/revocation, CSRF/origin checks,
  ownership, health aggregation, stale snapshots, event fan-out, and the official
  Docker SDK against an isolated API fixture.
- Browser tests start an isolated instance and a Docker API fixture over a local
  socket. They do not connect to your real Docker daemon.
- `test:docker` is an opt-in real-engine smoke test. It builds Shipper, creates a
  uniquely labeled BusyBox fixture, checks live state/SSE and restart persistence,
  and removes only that exact fixture after checking its test ownership label.
  It never removes volumes or unrelated containers.

Run the real-engine smoke test on a disposable Docker host. A second Shipper
instance watching that same host can discover its ownership-labeled fixture
and retain a missing-application record after the fixture is removed.

`test:deploy` exercises public Git, real Apache and Traefik, routing modes,
`.htaccess`, trusted local TLS, failed deployment preservation, proxy restart,
and scoped deletion. Local TLS uses a generated trust anchor; public ACME issuance
requires a reachable test domain and is a separate operator check.

`test:package` runs the built image with disposable data, tests the authenticated
API and public Git pipeline, verifies the domain from inside the container,
restarts the control plane, and checks failed-redeploy preservation and deletion.
It removes only this run's explicitly labeled test resources.

`test:full` runs real BuildKit Dockerfile and Railpack deployments, build secret
mounts, encrypted variable round trips, multi-domain routing without replacement,
failed-build preservation, runtime logs, and lifecycle actions. It removes only
resources carrying its random instance identity.

`test:tasks` verifies first-deploy PostgreSQL migrations, actual Docker startup
hooks, image reuse, service dependency controls, job recovery after reopening the
store, and argument-list execution in an image without a shell. Test resources
carry a unique instance identity.

Browser screenshots and test data go to the system temporary directory.
Production data and generated builds are ignored by Git.

## Project

See [architecture](docs/architecture.md), the [API contract](docs/api.md),
[releases and staging](docs/releases.md), [contributing](CONTRIBUTING.md),
[security](SECURITY.md), and [phase status](docs/roadmap.md). The interface is
based on the measured [design reference](DESIGN.md), with the concise-copy rules
in [AGENTS.md](AGENTS.md).

The project is MIT licensed. There is no hosted service or selected distribution
domain yet. Release assets, including the parameterless one-line installer, are
not published yet.

