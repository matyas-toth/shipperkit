# Docker Compose applications

Choose **Docker Compose** when creating an application. Use a public Git
repository or paste a Compose file. A repository revision can be a branch, tag,
or full commit SHA. **Detect services** validates the model and
lists its services. For Git sources, it also finds Compose files for the file
picker. The project directory is relative to the checkout; paths in the selected
file resolve against that directory. Selecting Compose is explicit because a
repository's Compose file may only describe its development environment.

Deploy the application, then open a service from its overview. Each service has
logs, `.env`, domains, and settings. Settings provide its HTTP port, health path,
memory limit, and mounted volumes. Edit the source Compose file under application
settings for commands, dependencies, volume definitions, restart policies, and
other supported Compose settings. Git files are edited in the repository; pasted
YAML has an explicit **Edit Compose** action.

## Environment

Application `.env` supplies **Compose interpolation**. It takes precedence over
the repository's project-directory `.env` and is never populated from Shipper's
own process environment. Compose `environment` and `env_file` choose which
variables reach each service. Variables are not automatically injected into
every service. Use `${NAME:?required}` for required inputs.

Service `.env` shows effective Compose environment entries, with values masked
until explicitly revealed. Editing creates a service override; unchanged inherited
entries remain inherited. Removing an entry suppresses it, and renaming a retained
entry preserves its value without fetching it into the browser. Service overrides
apply at runtime. Build inputs belong in Compose `build.args`; do not pass build
secrets as ARGs. Compose build secret/SSH forwarding is not supported in this phase.

Pasted YAML, variable values, and resolved release plans use AES-GCM encryption
in SQLite. Back up `encryption.key` with the database. Ordinary responses do not
contain pasted source, resolved plans, or secret values. Source and value reveal
actions require the authenticated, CSRF-protected management API. Secret drafts
and pasted YAML are not saved in browser storage. Docker administrators can
inspect container environments and mounted files.

## Routing

Host `ports` bindings are removed. Use service domains for HTTP ingress; a
database's development port mapping does not publish it on the VPS. Compose
service names remain available for communication inside the project's networks.

Common HTTP ports declared by `ports` or `expose` are inferred, while database
and queue ports remain internal. Set another HTTP port in service settings when
needed, or clear it to make that service internal. Add domains after that service
has been deployed with its HTTP port. Each domain belongs to a service and has
independent HTTPS and verification. Primary domains and alias redirects are
scoped to their service. The application Visit action uses the first primary
domain. Routing edits do not rebuild or restart containers.

Only HTTP services join Shipper's proxy network, using application-specific
aliases. Traefik still uses Shipper's file provider without access to the Docker
socket. Repository-supplied Traefik and Shipper ownership labels are discarded;
raw Traefik configuration is not exposed by this phase.

## Updates and persistent data

Every application has a stable project identifier derived from its Shipper
instance and application ID. Renaming an application does not change its project
or volume names. Managed named volumes and networks are scoped to that project;
explicit managed resource names are normalized to that scope. Declare
`external: true` to deliberately mount an existing volume. Shipper requires it to
exist and never claims or deletes it. External networks are not supported.

Before an update, Shipper validates the model, builds all buildable services,
pulls image services, and records their resolved image IDs. A failed build or pull
leaves the active stack and routes untouched. Applying an update withdraws routes,
uses Compose `up --wait`, checks HTTP services, then restores traffic. Recovery preserves the previous running/stopped intent.
This is a
stateful update with possible downtime, not a blue/green deployment. Unchanged
services retain their containers. Old orphaned services are removed only within
the application's owned project.

If application startup or HTTP checks fail, Shipper reapplies the previous pinned
model and restores its routes after health checks. If recovery fails, routes stay
withdrawn and the failure is recorded. Restoring containers **does not reverse
database migrations or changes to volume contents**. Maintain database backups
and use backward-compatible migrations. A first deployment with no previous
release removes its failed containers while retaining volumes.

Control-plane restarts resume queued operations from the persisted release plan.
Completed plans contain image IDs and Docker volume references, so recovery does
not depend on an old checkout. Start, stop, and restart operate on the application
as a unit. Normal updates, lifecycle actions, and deletion never use `down -v` or
remove persistent volumes. Deletion retains images and Docker volumes for manual
recovery; it does not currently clean up retained project networks or file volumes.

## Repository files and supported boundary

Relative build contexts, Dockerfiles, `env_file`, configs, secrets, YAML anchors,
dependencies, health checks, internal networks, and named volumes are supported.
Repository bind mounts and Compose configs/secrets are copied into release-owned
Docker volumes with subpath mounts. This works with a remote daemon and inside
the packaged control plane without mounting its filesystem into application
containers. Repository directory mounts are release snapshots; use named volumes
for mutable persistent data. Updates replace those snapshots.

Referenced paths must be regular files or directories inside the Git checkout
without symbolic links. Project-directory paths may reference siblings inside that
checkout. Compose files and environment files are limited to 128 KiB;
referenced individual files to 16 MiB and directory copies to 32 MiB/10,000 files.
Pasted Compose can use images, inline Dockerfiles, inline configs, and
environment-backed secrets; local repository files require a Git source.

This phase supports up to 32 active services and one container per service.
Inactive profiles are omitted. `include`, `extends`, service providers, jobs,
models, external configs/secrets, additional build contexts, SSH/build secrets,
privileged builds, host namespaces, devices, capability additions, Docker socket
mounts, external links/networks, and lifecycle hooks are rejected. Application
containers receive bounded logs, process/memory/CPU defaults and
`no-new-privileges`. Source restart policies and stricter limits are respected.

Native operation requires the official Docker Compose plugin and Buildx.
Docker Engine 26+ is needed for file volume subpaths. Use a current Compose
plugin; the production image includes Alpine's Compose package. The integration
suite is verified with Docker Desktop's Compose 5.5.1 and the packaged Linux
plugin. Private Git and private-registry credential management remain future work.

## Validation

Run `npm run test:compose` against a disposable Docker engine. It deploys a Git
fixture with a relative build, file mount, and PostgreSQL; writes database data;
updates and restarts the application; verifies failed-build preservation,
inventory grouping and deletion retention; and deploys pasted YAML. Unit tests
cover environment isolation, source boundaries, encrypted plan/source persistence,
retained inherited values, and missing-key recovery. Browser coverage exercises
creation, service editing, domain assignment and mobile layouts.

Reference implementation and semantics:
[compose-go](https://github.com/compose-spec/compose-go),
[Compose project model](https://docs.docker.com/compose/intro/compose-application-model/),
[Compose up](https://docs.docker.com/reference/cli/docker/compose/up/),
[volume subpaths](https://docs.docker.com/engine/release-notes/26.0/).
