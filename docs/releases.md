# Releases, CI and staging

Shipper uses four channels:

- Pull request checks build and test a candidate without publishing.
- `master` publishes an immutable nightly image and updates the nightly feed.
- The nightly image is deployed to staging for continuous real-host acceptance.
- A versioned release publishes the public image, installer assets and the latest installer feed.

## Branch and pull request flow

1. Create a branch from `master`.
2. Use a conventional commit title or squash title: `feat:`, `fix:`, `perf:`, `refactor:`, `docs:`, `test:` or `chore:`.
3. Open a pull request.
4. CI must pass. The required checks are `verify`, `browser` and `image`.
5. Merge the pull request. Do not push directly to `master`.

`release-please` reads conventional commits and opens a version PR with the changelog and version bump. Merging that version PR creates the release tag and starts the release workflow.

## Continuous integration

`.github/workflows/ci.yml` runs on every pull request and on `master`:

- TypeScript check and frontend unit tests.
- Go vet and race tests.
- Production build and installer syntax check.
- Playwright browser journeys.
- Docker image build and version check.

The real Docker deployment, routing, Compose, tasks, sources, backups and
packaged-image suites remain in the repository scripts for a disposable Docker
host. Run them locally or on a dedicated host. The staging workflow provides the
continuous real-host acceptance path.

## Versioning

Versions use SemVer. The source of truth is `package.json`, mirrored in `web/package.json` and the development default in `cmd/shipper/main.go` by `release-please`.

Do not bump the version in ordinary feature branches. The release PR owns the bump.

Shipper is on the `1.0.0-rc.N` line, so `release-please` uses the `rc` prerelease
strategy. Feature and fix commits advance the prerelease and mark the GitHub
release as a prerelease. The current public milestone is **Shipper 1.0.0 Beta 2**
at `1.0.0-rc.23`; the beta name lives in the GitHub release title while the `rc`
suffix preserves SemVer update ordering. To graduate to a stable release, add a
`Release-As: 1.0.0` footer to a commit and switch the versioning strategy back to
the default once the stable line begins.

## Release workflow

`.github/workflows/release.yml` runs for a pushed `v*` tag. The tag created when a release-please version PR merges starts it once, so a release is built once and the feed and release assets share one digest.

It:

1. Verifies the tag matches `package.json`.
2. Builds `linux/amd64` with the version in the binary.
3. Pushes `ghcr.io/matyas-toth/shipperkit:<version>`.
4. Reads the manifest digest into `image.txt`.
5. Writes `version.txt` and `release-url.txt` for the update checker.
6. Creates or updates the GitHub release and uploads `install.sh`, `image.txt`, `version.txt`, `release-url.txt` and `SHA256SUMS`.
7. Force-updates the `latest` branch with the same installer and update feed files.

The `latest` branch is a generated release feed, not development history. The public one-liner reads its installer through `shipperkit.dev`, and installed instances read its version and image metadata.

## Nightly feed

`.github/workflows/nightly.yml` runs after every successful `master` CI run. It builds and pushes a nightly image, tags it as `nightly`, and force-updates a generated `nightly` branch with:

- `version.txt`, using `<package-version>.nightly.<run-number>`.
- `image.txt`, the immutable image digest.
- `release-url.txt`, the source commit.
- `SHA256SUMS`.

The nightly version sorts after the release it is based on and before the next prerelease. This lets an instance on `1.0.0-rc.22` update to `1.0.0-rc.22.nightly.36`, and then to `1.0.0-rc.23` when that release is published.

See [updates](updates.md) for the in-app update flow and feed checks.

## Public installer

The desired public redirect is:

```text
https://shipperkit.dev/install.sh
  -> https://raw.githubusercontent.com/matyas-toth/shipperkit/latest/install.sh
```

Use a `302` response with no long cache. The installer then fetches `image.txt`
from the same `latest` branch and pulls the pinned digest.

The pinned release form remains available:

```sh
sudo bash install.sh --version 1.0.0-rc.23
```

That path fetches the immutable release assets from GitHub.

## Staging

`.github/workflows/staging.yml` runs after a successful Nightly workflow on `master`. It reads the nightly digest from the generated `nightly` branch, uploads the current installer to the staging host and runs:

```sh
bash /root/shipper-install.sh --image ghcr.io/matyas-toth/shipperkit@sha256:<digest>
```

The installer takes a checkpoint, updates the control plane and preserves the dashboard domain and data. A failed candidate is restored from the previous checkpoint. The staging workflow does not rebuild the image, so it always exercises the exact digest published to the nightly feed.

Set the repository variable `STAGING_AUTO_DEPLOY` to `false` to pause automatic deployments while keeping nightly publishing active. Manual `workflow_dispatch` runs still deploy.

Required repository secrets:

| Secret | Purpose |
| --- | --- |
| `RELEASE_PLEASE_TOKEN` | Repository-scoped token used by release-please so its version PR triggers CI |
| `STAGING_HOST` | Staging server address |
| `STAGING_USER` | SSH user, normally `root` for the disposable host |
| `STAGING_SSH_KEY` | Private key for the staging host |

`RELEASE_PLEASE_TOKEN` is required because pull requests created with the default
`GITHUB_TOKEN` do not start workflows, so the required checks would never appear.
Use a fine-grained token limited to this repository with contents and pull request
write access. Do not use a personal token with broader access.

Staging uses the `staging` GitHub environment. Do not use a personal SSH key.

## Preview deployments

A live PR preview should use a separate disposable host or a dedicated preview instance with its own ports and data. Running a second control plane against the same Docker socket on the production staging host is not supported.

Until a preview host exists, use the PR `image` check and the staging deployment after merge. A manual preview workflow can be added later without changing the release contract.

## Package visibility

The GHCR container package must be public. The public installer and staging host pull it without registry credentials. If a new package is created as private, set its visibility to public in the package settings or with the GitHub API before publishing the release.

## Emergency release

For an intentional tag outside release-please:

```sh
git tag vX.Y.Z
git push origin vX.Y.Z
```

The tag must match `package.json`. The release workflow still creates the release, uploads assets and updates the latest channel.
