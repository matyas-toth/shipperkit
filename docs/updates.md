# Updates

Shipper can keep the control plane current without taking applications down.
The managed installer adds a small host-side updater. The dashboard only
requests an update; the host helper performs the privileged work.

## Channels

Every installation starts on **Main** with automatic updates off.

| Channel | Source | Intended use |
| --- | --- | --- |
| Main | Tagged releases published from `master` | Normal installations |
| Nightly | Every successful `master` build | Testing upcoming changes |

Nightly builds are published automatically and can contain bugs or breaking
changes. They are useful for validating a release candidate, not for an
installation that needs predictable behavior.

The feeds are generated branches:

- Main reads `version.txt`, `image.txt` and `release-url.txt` from
  `https://raw.githubusercontent.com/matyas-toth/shipperkit/latest`.
- Nightly reads the same files from
  `https://raw.githubusercontent.com/matyas-toth/shipperkit/nightly`.

The release workflow updates the `latest` branch. The nightly workflow updates
the `nightly` branch after every successful `master` CI run.

The checker resolves the current branch commit through the GitHub API, then
reads the immutable files at that commit. This avoids stale raw-content caches.
If the API is unavailable, it falls back to the generated branch path.

## Checks and notifications

Shipper checks the selected feed every five minutes. A successful check with a
newer version stores the available release and shows an **Update** badge on the
Settings navigation item. The check is read-only and uses small plain-text
files, so it works without a GitHub token or API rate limit.

Open **Settings > Server settings > Updates** to:

- Run a forced check.
- Switch between Main and Nightly.
- Enable automatic updates.
- Review the available version, release link, current version and last check.
- Start an update or open its progress dialog.

When automatic updates are off, an available release shows a green banner with
an Update button and a link to the release or commit. When automatic updates are
on, a newer version is installed on the next check without another click.

## Update process

Pressing Update, or an automatic update, writes a small request atomically to
`/var/lib/shipper/update-request.json`. A systemd path unit starts
`/usr/local/libexec/shipper-update`, which:

1. Validates the request against the Shipper image and version patterns.
2. Downloads the pinned image digest.
3. Reads the matching installer from the image itself.
4. Runs the installer with the pinned image.
5. Writes progress, errors and a bounded log to
   `/var/lib/shipper/update-status.json`.

The installer checks for queued operations, takes a database checkpoint, stops
only the Shipper control-plane service, switches the pinned image, starts the
new control plane and verifies its health. Application containers, volumes,
routes and the proxy continue running. If the candidate is unhealthy, the
installer restores the previous image and checkpoint.

The dashboard keeps a progress dialog open across the control-plane restart and
reloads when the new version is ready. Closing the dialog does not cancel the
update; the update continues and the Settings page shows its state. A failed
update keeps the previous version serving; the failure and log are available
under Updates.

## Security boundary

- The update image must match
  `ghcr.io/matyas-toth/shipperkit@sha256:<digest>`.
- The version embedded in the image must match the request.
- The installer is read from the verified image, not from the dashboard request.
- The request file is owned by the control plane; the host helper validates it
  before running anything.
- The helper runs as root because it uses systemd and the Docker CLI, but the
  dashboard never receives a shell or an arbitrary command channel.
- Only the control plane is restarted. Application workloads are not part of
  the control-plane compose project.

The helper is installed only by the managed Ubuntu installer. Local development
installations keep the update controls disabled.

## Limits

- The host must reach GHCR and the selected raw GitHub feed.
- A release candidate is a normal update target on Main during the 1.0 preview.
- Switching from Nightly to Main does not downgrade an installation; it only
  changes which releases are considered next.
- Manual recovery remains available through `shipperctl` and the installer if
  the host updater itself is unavailable.
