# Contributing

Start with [README.md](README.md) and [docs/architecture.md](docs/architecture.md).
Keep changes scoped to a working phase. Preserve application-first workflows,
the measured design system, and the concise-copy rules in AGENTS.md.

## Development

Use Go 1.26+, Node 22.12+, Git 2.36+, and Docker CLI/Buildx/Compose for full builds
and opt-in real-engine tests.
Install dependencies with `npm ci` and `npm ci --prefix web`.
Run `npm run dev` for the backend and frontend together.

Before opening a pull request, run `npm run check`, `npm test`,
`node scripts/go.mjs vet ./...`, and `npm run build`.
For UI changes, also run `npm run test:e2e` and inspect desktop/mobile rendering.
Run `npm run test:docker` when Docker integration changes.
Run `npm run test:deploy` on a disposable Docker host when orchestration, Apache,
or Traefik behavior changes. Public ACME tests require dedicated reachable DNS;
the local suite verifies TLS against its own generated trust anchor.
Run `npm run test:full` for BuildKit/Railpack or runtime configuration changes,
`npm run test:compose` for Compose orchestration, and `npm run test:package`
after building the production image. Run heavy Docker suites sequentially on
Docker Desktop to avoid competing cold-build and database-startup deadlines. Run `npm run test:tasks` for lifecycle/task,
startup-hook, scheduler or command-recovery changes. Run `npm run test:sources`
for direct-image/inline workflows, source identity and recorded-image recovery.
Run `npm run test:backups` for archive, database, volume, S3, recovery or protection-queue
changes. It uses isolated database fixtures and verifies actual read/write behavior.
Direct Go builds need
`go generate ./internal/startup` before `go build` to include Linux helpers.

Go code is formatted with gofmt. Keep exported APIs narrow, use contexts for
operations, and test security, ownership, persistence, and failure behavior.
Use ordinary SQL migrations rather than changing a previously shipped migration.
No secrets, local database files, generated frontend bundles, or screenshots
belong in a pull request.

Describe the concrete behavior change, the checks you ran, and any limitations.
Do not claim deployment or performance guarantees without reproducible evidence.

## Pull requests and releases

Do not push feature work directly to `master`. Create a branch, open a pull
request and let CI run. Use a conventional squash title such as `feat:`,
`fix:`, `perf:`, `refactor:`, `docs:`, `test:` or `chore:`.

`release-please` uses those titles to open a version PR with the changelog and
version bump. Merging the version PR tags the release and publishes the image,
installer assets and latest installer channel. See
[releases, CI and staging](docs/releases.md) for the pipeline, required checks,
staging secrets and the `shipperkit.dev` redirect.

A live preview should use a separate host or preview instance. Do not run a
second control plane against the same Docker socket on the staging host.

