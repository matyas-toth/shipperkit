# Shipper

A lightweight, self-hosted application platform built with Go, SolidJS, and SQLite.

**Phase 1: Foundation.** The local dashboard supports secure administrator setup,
login/logout, persistent sessions, Docker inventory, application ownership,
event-driven state reconciliation, and embedded frontend assets.

Deployment creation, builds, lifecycle actions, domains, and HTTPS provisioning
are planned for subsequent phases. Shipper currently observes Docker; it does
not start, stop, adopt, or delete your workloads.

## Run locally

Requirements: Go 1.26+, Node.js 22.12+ (24 LTS recommended), and a running Docker
Engine or Docker Desktop using Linux containers.

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
Keep the dashboard private. Docker Desktop users should use this read-only
foundation against personal workloads; deployment testing belongs on a
disposable Linux host.

`docker compose down` stops Shipper without removing its metadata volume or
application data. Avoid `down -v` unless you deliberately want to erase Shipper's
administrator and metadata.

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `SHIPPER_ADDR` | `127.0.0.1:8080` | HTTP listener |
| `SHIPPER_DATA_DIR` | `.shipper` | Private SQLite and setup-token directory |
| `SHIPPER_PUBLIC_URL` | Request origin | Canonical origin, e.g. `https://shipper.example.com`, without a trailing slash |
| `SHIPPER_SETUP_TOKEN` | Generated local file | Optional first-run capability of at least 32 characters |
| `DOCKER_HOST` | SDK platform default | Unix socket, Windows named pipe, or TLS-protected TCP |
| `DOCKER_TLS_VERIFY` | unset | Must be `1` for TCP connections |
| `DOCKER_CERT_PATH` | SDK default | Docker TLS certificates |
| `SHIPPER_PORT` | `8080` | Compose host port |
| `DOCKER_GID` | `0` | Compose supplementary group for the Docker socket |

When putting Shipper behind an HTTPS reverse proxy, set `SHIPPER_PUBLIC_URL` to
its exact external origin. This enables Secure cookies and canonical origin
checks. Forwarded headers are intentionally not trusted. The default loopback
listener is for local operation, and no installer or proxy is provisioned yet.

## What the dashboard shows

All containers are visible except Shipper's own control plane. The Shipper
filter selects only containers with both ownership labels:

```yaml
labels:
  io.shipper.managed: "true"
  io.shipper.application.id: "a-stable-application-id"
```

Use these only for workloads intentionally owned by Shipper. Discovery does not
change Docker labels. Ordinary existing containers remain external.

SQLite retains a stable application record if its container disappears.
Container state, health, and application-level state are separate. A disconnected
daemon leaves the last complete inventory visible with an explicit warning.
Reconnects and periodic reconciliation repair missed Docker events.

The inventory window is virtualized, and search/status filters run locally.
Selecting a row opens a read-only drawer. Navigation retains the query cache and
application shell.

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

Browser screenshots and test data go to the system temporary directory.
Production data and generated builds are ignored by Git.

## Project

See [architecture](docs/architecture.md), the [API contract](docs/api.md),
[contributing](CONTRIBUTING.md), [security](SECURITY.md), and
[phase status](docs/roadmap.md). The interface is based on the measured
[design reference](DESIGN.md), with the concise-copy rules in [AGENTS.md](AGENTS.md).

The project is MIT licensed. There is no hosted service, selected distribution
domain, or published one-line installer yet.

