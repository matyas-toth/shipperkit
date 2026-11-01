# Shipper

A lightweight, self-hosted application platform built with Go, SolidJS, and SQLite.

**Phase 2: Static sites.** Deploy a public Git repository as a versioned Apache
image, check the candidate, and route it through Traefik. Shipper persists the
deployment queue, history, and logs, preserves a serving release when a redeploy
fails, and groups multiple containers under one application.

The dashboard shows Shipper applications. Unrelated Docker containers are excluded.
Railpack, Dockerfile builds, build commands, and Compose deployments follow in
later phases.

## Run locally

Requirements: Go 1.26+, Node.js 22.12+ (24 LTS recommended), and a running Docker
Engine or Docker Desktop using Linux containers. Native development also needs
Git 2.36+; the production image includes Git.

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
| `SHIPPER_PUBLIC_URL` | Request origin | Canonical origin, e.g. `https://shipper.example.com`, without a trailing slash |
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
its exact external origin. This enables Secure cookies and canonical origin
checks. Forwarded headers are intentionally not trusted. The default loopback
listener is for local operation. Traefik is provisioned lazily for the first
deployment; no public installer is published yet.

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
be redeployed or deleted through this static-site pipeline.

SQLite retains a stable application record if its container disappears.
Container state, health, and application-level state are separate. A disconnected
daemon leaves the last complete inventory visible with an explicit warning.
Reconnects and periodic reconciliation repair missed Docker events.

The inventory window is virtualized, and search/status filters run locally.
Selecting an application opens Overview, Deployments, Domains, and Settings.
Container inspection uses a drawer. Navigation retains the query cache and shell.

## Deploy a static site

Choose **New application**, enter a public HTTPS Git URL, and choose **Deploy**.
The default publish directory is the repository root, which must contain
`index.html`. Configuration exposes branch, publish directory, file/SPA/strict
routing, and opt-in `.htaccess` rewrites. Repository code is never executed on the
control plane. Build commands and dependency installation are deferred to Phase 3.

A local application gets `http://<application-id>.localhost:8780`. Use **Visit**
to open it. Use **Redeploy** to ship the current branch again, and **Deployments**
to inspect the exact commit, operation state, and persisted logs.

For public DNS and automatic HTTPS, see [static site operations](docs/static-sites.md).

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

Browser screenshots and test data go to the system temporary directory.
Production data and generated builds are ignored by Git.

## Project

See [architecture](docs/architecture.md), the [API contract](docs/api.md),
[contributing](CONTRIBUTING.md), [security](SECURITY.md), and
[phase status](docs/roadmap.md). The interface is based on the measured
[design reference](DESIGN.md), with the concise-copy rules in [AGENTS.md](AGENTS.md).

The project is MIT licensed. There is no hosted service, selected distribution
domain, or published one-line installer yet.

