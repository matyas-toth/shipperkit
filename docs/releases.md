# Releases, CI and staging

Shipper uses three channels:

- Pull request checks build and test a candidate without publishing.
- `master` builds an immutable edge image and deploys it to staging.
- A versioned release publishes the public image, installer assets and the latest installer channel.

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

Do not bump the version in ordinary feature branches. The release PR owns the bump. A release tag is `vX.Y.Z` or `vX.Y.Z-rc.N`.

## Release workflow

`.github/workflows/release.yml` runs for a pushed `v*` tag and is also called directly by the release-please workflow after a release PR merges.

It:

1. Verifies the tag matches `package.json`.
2. Builds `linux/amd64` with the version in the binary.
3. Pushes `ghcr.io/matyas-toth/shipperkit:<version>`.
4. Reads the manifest digest into `image.txt`.
5. Creates or updates the GitHub release and uploads `install.sh`, `image.txt` and `SHA256SUMS`.
6. Force-updates the `latest` branch with the same three installer assets.

The `latest` branch is a generated installer channel, not development history. The public one-liner reads it through `shipperkit.dev`.

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
sudo bash install.sh --version 1.0.0
```

That path fetches the immutable release assets from GitHub.

## Staging

`.github/workflows/staging.yml` runs after a successful CI run on `master`. It builds an edge image with a SemVer-compatible version, pushes it to GHCR, uploads the current installer to the staging host and runs:

```sh
bash /root/shipper-install.sh --image ghcr.io/matyas-toth/shipperkit@sha256:<digest>
```

The installer takes a checkpoint, updates the control plane and preserves the dashboard domain and data. A failed candidate is restored from the previous checkpoint.

Required repository secrets:

| Secret | Purpose |
| --- | --- |
| `STAGING_HOST` | Staging server address |
| `STAGING_USER` | SSH user, normally `root` for the disposable host |
| `STAGING_SSH_KEY` | Private key for the staging host |

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
