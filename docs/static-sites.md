# Static site operations

## Local development

Start Docker, then `npm run dev` or the embedded Go binary. Create an application
from a public HTTPS Git repository. The publish directory must contain
`index.html`. Without a build command, no dependency installation runs. Optional
Node builds and their settings are covered in [full application operations](full-applications.md).

The first deployment provisions one instance-scoped Traefik container, a bridge
network, and a proxy data volume on the Docker host. Port defaults are loopback
8780 (HTTP) and 8743 (HTTPS). Site containers publish no host ports and are reached
through the proxy. **Visit** opens the application's `.localhost` address.
Browsers resolve `.localhost` to loopback; CLI clients can use
`curl --resolve <application-id>.localhost:8780:127.0.0.1 http://<application-id>.localhost:8780/`.

The dashboard groups containers by application ID. Candidate and old releases
may briefly coexist while the serving release changes. New deployments carry
instance, application, and deployment ownership labels. Legacy labeled Docker
applications can be inspected but are not adopted by the deployment worker.

## Public domains and HTTPS

On a disposable Linux host with available ports 80/443, configure before the
first deployment:

```sh
export SHIPPER_PROXY_BIND=0.0.0.0
export SHIPPER_HTTP_PORT=80
export SHIPPER_HTTPS_PORT=443
export SHIPPER_ACME_EMAIL=owner@example.com
# Use staging while testing to avoid production certificate rate limits:
export SHIPPER_ACME_CA=https://acme-staging-v02.api.letsencrypt.org/directory
docker compose up --build -d
```

Point the application's A record to the server's public IPv4 address. Publish an
AAAA record only if that server is reachable over IPv6 as well. Allow inbound
TCP 80 and 443. Keep Shipper's management port private. Enter the domain without
a scheme/path/port, enable HTTPS, and deploy. Traefik uses HTTP-01 on port 80,
persists certificates in its own volume, and redirects the application's HTTP
traffic to HTTPS. Wildcards and DNS API automation are not supported.

Staging certificates are deliberately not publicly trusted. Use the production
CA directory (or unset `SHIPPER_ACME_CA`) for a trusted public certificate after
testing. **Verify domain** checks DNS, HTTP status, the serving deployment's
response header, and normal TLS certificate validation. A certificate failure
is shown as pending verification; deployment success itself means the candidate
and proxy route are ready, not that a public certificate has been issued.

Traefik's log command is `docker logs <instance-proxy-name>`. Find the exact
container with `docker ps --filter label=io.shipper.infrastructure=proxy` and
verify its `io.shipper.instance` label against `instanceId` from authenticated
`GET /api/system`. The proxy dashboard/API port is not published, and no
Docker socket is mounted into Traefik.

Proxy static configuration is fingerprinted. Changing ports, bind address, or
ACME settings requires stopping and removing only that installation's proxy
container, retaining its `shipper-<instance-prefix>-proxy-data` volume, and
redeploying. The worker then recreates it with the new configuration. Existing
site containers and certificates remain intact. This operator action causes a
brief routing interruption, so schedule it accordingly.

## Routing and publish isolation

- **File based**: existing files/directories resolve normally and `/about` can
  resolve to `about.html`.
- **SPA fallback**: missing paths serve `index.html`.
- **Strict**: missing paths return 404.

Opt-in `.htaccess` permits common rewrite, redirect, error-document, and header
directives through an explicit `AllowOverrideList`. It does not enable
`AllowOverride All`, arbitrary Apache modules, CGI, PHP, filesystem aliases, or
access outside the publish root. An invalid directive makes the candidate fail
its HTTP check, preserving the current release. Custom `.htaccess` rules can
override the selected routing mode.
`<IfModule>` wrappers are supported, and failed candidate checks include a bounded
Apache stderr tail in the persisted deployment error/log.

Publishing excludes Git metadata, hidden secrets, key files, Dockerfiles,
Compose files, and node_modules. `.well-known` and explicitly enabled `.htaccess`
are retained. Archive entries use normalized modes and reject links/devices and
publish-directory escapes. Sites are limited to 100 MiB and 10,000 files. Git
clone has a three-minute timeout, bounded text logs, and a periodically checked
512 MiB working-directory budget. It is not a hard filesystem quota, so deploy
only repositories you intend to serve and reserve sufficient disk space.

Git runs with system/global configuration, credential helpers, hooks, redirects,
submodules, non-HTTPS protocols, and inherited Git/proxy variables disabled.
DNS must resolve entirely to public addresses, which are pinned for the HTTPS
connection. Private Git providers, SSH URLs, Git LFS, and Git submodules are not
implemented in this phase.

Apache runs as UID/GID 65532 with a read-only root, writable bounded `/tmp`, no
Linux capabilities, no host mounts, no Docker socket, and memory/process/CPU
limits. The packaged image records the source commit, deployment ID, and pinned
base. Runtime image IDs and per-deployment tags are persisted; no mutable
application `latest` tag controls promotion.
Traefik also runs as UID/GID 65532 and keeps its writable files in its own volume.

## Recovery and deletion

The SQLite worker lease allows one deployment worker per metadata database and
renews every five seconds. After an abrupt exit a replacement waits for lease
expiry (at most about 30 seconds), then replays unfinished stages. Docker names
and image tags are deployment-specific and side effects are checked for
ownership before reuse. If a branch changed before interrupted packaging was
completed, recovery fails explicitly so a new redeploy can record a new commit.

Promotion persists its intent, writes the full routing snapshot to a temporary
file, atomically renames it into Traefik's watched directory, and waits for its
internal API to acknowledge the desired services. Only then is the previous
container retired. A failed routing update restores the previous active ID and
route; an existing healthy release is kept during clone/package/health failures.
This is not a guarantee of zero downtime or a general stateful rollback system.

Application deletion requires the application's name. The durable delete job
withdraws and acknowledges routing, removes only containers matching this
instance/application, then soft-deletes the record. A late Docker event cannot
resurrect it. No volumes are removed. Images and history are deliberately
retained; automatic image retention/garbage collection is not implemented yet.

Back up the SQLite database using a SQLite-aware backup (or stop Shipper before
copying all database state), retain `encryption.key` unchanged, and retain the proxy volume separately for ACME
account/certificate data. Do not delete either during a normal upgrade.

## Integration verification

`npm run test:deploy` runs on a real Linux Docker engine or Docker Desktop. It
checks the public Git transport, real Apache images, all routing modes,
`.htaccess`, failed-release preservation, trusted local TLS, proxy restart, and
application deletion. Tests use unique instance labels and loopback ports.
Use a disposable host; the local certificate test does not claim to verify public
DNS or Let's Encrypt issuance. Before a public release, repeat the deployment on
a disposable Linux VPS using a dedicated domain and ACME staging.

After `docker build -t shipper:local .`, run `npm run test:package` to verify the
actual non-root control-plane image, Git runtime dependencies, domain checks
inside Docker, session/release persistence, and ownership-scoped cleanup.
