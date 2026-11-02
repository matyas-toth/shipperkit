# Full application operations

Single-container applications support public HTTPS Git repositories. Source
authentication and signed push webhooks are planned separately. For Compose and
service-specific domains/settings, see [Compose operations](compose.md).

## Build and run

Automatic mode selects a root `Dockerfile`, otherwise a language manifest for
Railpack, otherwise a plain `index.html`. Set a method explicitly to override it.
The application directory is relative to the repository; Dockerfile and publish
paths are relative to that directory. Paths cannot escape the checkout.

Dockerfile builds honor `.dockerignore` after Shipper removes `.git`, `.ssh`,
`.env*`, private-key files, and dependency directories from the build context.
Symlinks and non-regular files are rejected. Contexts are bounded to 512 MiB and
30,000 entries. Keep public build configuration in ordinary tracked files. Use
variables for environment files and secret material.

Dockerfile install/build commands belong in the file. A start-command override
uses `/bin/sh -c`, so the image needs a shell. Leave it empty to preserve the
image entrypoint and command. Static custom builds use Node 24 and Apache; set
the publish directory to the generated output. Without a build command, static
sites package existing files directly. Railpack prepares a plan and uses the
version-matched custom BuildKit frontend. Supported toolchains follow Railpack.

Apps must listen on `0.0.0.0`. The port is the explicit setting, runtime `PORT`, a
single image EXPOSE declaration, or 3000. Static Apache uses 8080. No application
port is published on the host. Traefik connects over Shipper's private network.
Candidates get up to 45 seconds to return a 2xx/3xx HTTP response from the health
path. Docker image health marked unhealthy also prevents promotion. Long startup
times or non-HTTP worker-only applications need future health-policy support.

The default memory limit is 512 MiB for full apps and 128 MiB for static sites.
The UI permits 64–8192 MiB. Containers get one CPU and a 256-process limit, drop
Linux capabilities, and cannot gain privileges. Static roots are read-only;
other runtimes can write their container layer. Mount a named volume for durable
data. Names are stable across deployments and are scoped to instance/application.
Shipper never deletes application data volumes, including during app deletion.
Local files and SQLite databases in a shared volume may require stopping writes
or an application-specific migration strategy during overlapping deployments.

## Variables and logs

Variables have runtime, build, or both scope. All values are AES-GCM encrypted in
SQLite. Marking a variable Secret additionally masks it in ordinary API responses
and redacts exact occurrences in build and runtime logs. Reveal is an explicit
authenticated action. Log redaction cannot undo a program intentionally encoding
or transforming a secret. Docker administrators can inspect runtime environment.

Dockerfile build secrets use `RUN --mount=type=secret,id=NAME` or the Dockerfile
frontend's secret environment mount. A non-secret build variable is a build ARG;
declare it in the Dockerfile. Railpack receives build variables as secret mounts.
Secret values are passed through an isolated client environment, not process
command arguments on the host. Build cache invalidation accounts for Railpack
secret changes; Dockerfile secret mounts follow normal BuildKit cache semantics,
so use **Rebuild without cache** when a secret changes generated output.

Paste .env imports `NAME=value`, optional `export`, comments, and surrounding
single/double quotes. It does not execute shell expansion or interpolate values.
Duplicate names are rejected. New secret values stay in memory and are excluded
from browser drafts. Non-secret fields survive reloads within the browser session.
Successful saves clear their draft. Editing settings or variables applies them
with a new deployment, preserving the old runtime until the candidate passes.

Deployment logs persist in SQLite. Runtime logs read Docker's bounded log driver,
stream a recent tail, follow new output, and offer local search/copy. The viewer
keeps at most 5,000 lines and renders a 64-row window. Streams are authenticated,
session-checked, bounded to eight concurrent readers, and disconnect slow readers.

## Domains and lifecycle

Add up to 32 hostnames. Each has independent HTTPS and verification. One hostname
is primary; aliases may serve normally or redirect to the primary while preserving
the request path/query. Root-domain www expansion uses the public suffix list,
including suffixes such as `co.uk`. Public HTTPS needs `SHIPPER_ACME_EMAIL`, correct
DNS, and reachable public HTTP/HTTPS ports. Existing proxy ACME configuration
remains supported; the public installer is not part of this phase.

**Save domains** queues a durable routing operation without rebuilding or replacing
containers. Shipper reserves hostnames across applications, publishes one complete
Traefik file, and waits for router/backend acknowledgment. A failed apply restores
the prior domains. Each saved hostname can be verified independently; a changed
hostname or HTTPS policy invalidates its previous verification. Empty domains
withdraw public routes while leaving the container running.

Start/stop/restart reuse the current release. Stop withdraws routes and persists
the stopped intent across control-plane restarts. Start/restart check HTTP before
restoring routes. Deploying starts the new release even if the previous one was
stopped. These operations share the per-application queue with builds and routing.
Rename changes metadata immediately and does not enqueue a deployment.

## Build infrastructure and backups

Native installs need Docker CLI with Buildx. The Docker package includes both.
The first full build creates an instance-owned privileged BuildKit daemon with a
dedicated cache volume. Its socket is reached through Docker's container transport;
no BuildKit port is exposed, and the operator's default builder is untouched.
The daemon is limited to two CPUs, 2 GiB, and a garbage-collected cache target of
4 GiB. Its container remains available to reuse cache. Railpack preparation uses
an ephemeral unprivileged helper with a read-only root, bounded tmpfs, no host
mounts, and no Docker socket. Builds need outbound access to upstream registries
and language package repositories. Treat Docker access as host-administrator access.

BuildKit image and Railpack frontend are pinned by digest in `internal/deploy/build.go`.
Railpack's Linux binary is pinned by version and release checksum, downloaded from
its official GitHub release, and cached in the private data directory. Its custom
frontend ignores Dockerfile label flags, so Shipper commits ownership metadata from
an unstarted isolated container while retaining the generated image configuration.

Back up SQLite, `encryption.key`, proxy certificate data, and application volumes
together. Keep the encryption key private and restore it unchanged; startup refuses
to replace a missing key when encrypted configuration exists. Replacing it makes
existing variables unreadable. Tool downloads and builder caches can be recreated.

## Runtime presentation

Static sites show **Serving** when their Apache container is running, and expose
that container in the Serving section. A missing serving runtime is **Unavailable**;
completed deployment history alone never implies that the site is still online.
Older static runtime images inherited a packaging label. Inventory recognizes only
their exact generated runtime names, while continuing to exclude actual helpers.
New runtime containers explicitly clear the helper label.

Generated runtime names use `shp-<application>-<release suffix>`. The naming helper
also accepts a service slug for multi-container runtimes. Names are normalized and
bounded; ownership remains in instance/application/deployment labels and routing
uses a stable release alias. At startup, old generated runtime names are renamed
without changing container IDs or restarting workloads.

The .env tab replaces the Variables label, retaining the old `tab=variables`
link as an alias. Runtime log selection is shown only for multiple containers.
Deployment cards display elapsed time, updating once per second while unfinished;
finished operations retain their recorded duration. Deployment logs show local
datetime and per-deployment line numbers independently of the pagination cursor.

## Simple environment editor

The .env editor shows name/value pairs with a blank row ready for the next entry.
Paste a `.env` file directly into a name field (or paste multiple assignments into
a value field) to add or replace entries locally. There is no separate import mode.
New entries are secret and available during both build and runtime. Pasting over
an existing name preserves its scope and secret setting while replacing its value.

Each row's options menu contains build/runtime scope, secret visibility, explicit
reveal, and removal. Existing settings and retained secrets remain unchanged when
editing another field. Revealing a saved value alone does not enable deployment.
Save and deploy ignores blank rows and is enabled only for configuration changes.
The same editor is available during creation, within the .env section.
