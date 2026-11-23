# Development

This guide covers running Shipper from source, its configuration and the test suites. To install Shipper on a server, use the [installer](https://shipperkit.dev/docs/install/) instead.

## Requirements

- Go 1.27.4 or newer
- Node.js 22.12 or newer, 24 LTS recommended
- Docker Engine or Docker Desktop with Linux containers, Buildx and Compose
- Git 2.36 or newer

## Run the development server

```sh
npm ci
npm ci --prefix web
npm run dev
```

Open [localhost:5173](http://127.0.0.1:5173), read the setup token from `.shipper/setup-token` and create the administrator. On Windows, use `Get-Content .shipper/setup-token`.

The dev runner compiles the Go binary before it starts. Frontend edits hot reload, and Go changes need a restart of `npm run dev`. Set `SHIPPER_GO` when the Go executable is outside `PATH`. The runner also finds the local Shipper tool cache.

Shipper talks to Docker through the SDK defaults: the default socket, or `DOCKER_HOST`, `DOCKER_TLS_VERIFY` and `DOCKER_CERT_PATH`. It does not read Docker CLI contexts. If Docker Desktop's default named pipe is unavailable on Windows:

```powershell
$env:DOCKER_HOST = 'npipe:////./pipe/dockerDesktopLinuxEngine'
npm run dev
```

## Run the production build

```sh
npm run build
./bin/shipper
```

On Windows, run `./bin/shipper.exe`. Open [localhost:8080](http://127.0.0.1:8080). The Go binary serves the compiled dashboard without a Node server. A direct `go build` needs `go generate ./internal/startup` first so the Linux helpers are embedded.

## Run the Docker package

```sh
docker compose up --build -d
docker compose exec shipper cat /data/setup-token
```

Open [localhost:8080](http://127.0.0.1:8080) and use that setup token. On Linux, set `DOCKER_GID=$(stat -c '%g' /var/run/docker.sock)` before starting Compose so the non-root process can reach the socket.

The image runs as UID and GID 65532, drops Linux capabilities, has a read-only root filesystem and keeps its metadata in the `shipper-data` volume. A Docker socket grants control of the host even when it is mounted `:ro`, so keep the dashboard private and run integration tests on a disposable Docker host.

`docker compose down` keeps the metadata volume and application data. `down -v` erases the administrator and all metadata.

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `SHIPPER_ADDR` | `127.0.0.1:8080` | HTTP listener |
| `SHIPPER_DATA_DIR` | `.shipper` | SQLite database and setup token |
| `SHIPPER_PUBLIC_URL` | Request origin | Canonical origin for manual installs behind a reverse proxy, without a trailing slash. Managed installs store the dashboard domain in Settings. |
| `SHIPPER_DASHBOARD_UPSTREAM` | unset | `auto` for packaged dashboard HTTPS, or a Docker-reachable HTTP origin |
| `SHIPPER_SETUP_TOKEN` | Generated file | First-run token of at least 32 characters |
| `DOCKER_HOST` | SDK default | Unix socket, Windows named pipe or TLS-protected TCP |
| `DOCKER_TLS_VERIFY` | unset | Must be `1` for TCP connections |
| `DOCKER_CERT_PATH` | SDK default | Docker TLS certificates |
| `SHIPPER_PORT` | `8080` | Compose host port |
| `DOCKER_GID` | `0` | Compose supplementary group for the Docker socket |
| `SHIPPER_PROXY_BIND` | `127.0.0.1` | Host address for Traefik's published ports |
| `SHIPPER_HTTP_PORT` | `8780` | Traefik HTTP port on the host |
| `SHIPPER_HTTPS_PORT` | `8743` | Traefik HTTPS port on the host |
| `SHIPPER_ACME_EMAIL` | unset | Enables the Let's Encrypt resolver for custom domains |
| `SHIPPER_ACME_CA` | Let's Encrypt production | ACME directory, including staging |
| `SHIPPER_CATALOG_URL` | Repository manifest | Service Library manifest URL, or `off` to disable remote updates |

Behind an HTTPS reverse proxy, set `SHIPPER_PUBLIC_URL` to the exact external origin. OAuth callbacks and origin checks use it, and forwarded host headers are never trusted. Traefik starts lazily with the first deployment or dashboard domain.

A local application answers on `http://<application-id>.localhost:8780`.

## Ownership labels

Every resource Shipper creates carries `io.shipper.managed`, `io.shipper.application.id`, `io.shipper.instance` and `io.shipper.deployment.id`. Mutations check the instance and application labels before touching a container, so Shipper never adopts unrelated workloads, never removes application volumes and ignores resources that belong to another installation.

## Password recovery

Run this from the instance's data directory, or set `SHIPPER_DATA_DIR`:

```sh
./bin/shipper reset-password
docker compose exec shipper shipper reset-password
```

The password is read from the terminal and never accepted as an argument. Recovery revokes every session. Managed installs use `sudo shipperctl reset-password`.

## Tests

Run these before every pull request:

```sh
npm run check
npm test
node scripts/go.mjs vet ./...
npm run build
```

The browser suite starts an isolated instance against a Docker API fixture and never touches your real daemon:

```sh
npx playwright install chromium
npm run test:e2e
```

The real Docker suites need a disposable Docker host. Each one labels its resources with a random instance identity and removes only those. On Docker Desktop, run them one at a time.

| Suite | Covers |
| --- | --- |
| `test:docker` | Live inventory, events and restart persistence against a labelled BusyBox fixture |
| `test:deploy` | Public Git, Apache, Traefik, routing modes, local TLS, failed deploy preservation and scoped deletion |
| `test:full` | BuildKit Dockerfile and Railpack builds, build secrets, encrypted variables, multi-domain routing and lifecycle actions |
| `test:compose` | Compose orchestration, services, volumes and inline configs |
| `test:tasks` | Startup hooks, migrations, scheduled tasks and command recovery |
| `test:sources` | Direct images, inline Dockerfiles, source identity and recorded image recovery |
| `test:backups` | Archives, database dumps, volumes, S3, recovery and restore copies |
| `test:database` | Connections between applications and database images |
| `test:package` | The built `shipper:local` image end to end, including a control plane restart |

Public ACME issuance needs a reachable test domain and is checked on staging. See [releases, CI and staging](releases.md).

## Repository layout

| Path | Contents |
| --- | --- |
| `cmd/shipper` | Control plane entry point |
| `cmd/shipper-start` | Runs startup commands inside application containers before their entrypoint |
| `cmd/cataloggen` | Service Library generator |
| `internal/` | Go packages: API, deploy engine, store, auth, backups, updates and more |
| `web/` | Dashboard, SolidJS and Vite |
| `cli/` | The `shipper` CLI |
| `landing/` | shipperkit.dev and the public docs |
| `telemetry/` | Anonymous usage report ingest and admin dashboard |
| `install.sh` | Managed installer |
| `scripts/` | Dev runner and integration test drivers |
| `tests/e2e/` | Playwright suites |
| `docs/` | Developer documentation |
