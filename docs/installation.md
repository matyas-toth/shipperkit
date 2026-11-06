# Managed Linux installation

The release candidate targets one dedicated Ubuntu 24.04 x86-64 server with
systemd. It checks for at least 1 GiB RAM and 8 GiB free in `/var/lib`. These are
bootstrap thresholds, not workload sizing guidance. Docker Engine API 1.48 or
newer, Compose and Buildx are required. The installer can install missing Docker
from the [signed Docker package repository](https://docs.docker.com/engine/install/ubuntu/).

Use the published installer. The default one-liner installs the latest release
without arguments, then prints the public IP and port to open:

```bash
curl -fsSL https://github.com/matyas-toth/shipperkit/releases/latest/download/install.sh | sudo bash
```

To pin a version or inspect the script first:

```bash
version=1.0.0
origin="https://github.com/matyas-toth/shipperkit/releases/download/v$version"
curl -fsSLO "$origin/install.sh"
curl -fsSLO "$origin/image.txt"
curl -fsSLO "$origin/SHA256SUMS"
sha256sum -c SHA256SUMS
sudo bash install.sh --version "$version" --check
sudo bash install.sh --version "$version"
```

The image is pulled by its release digest. An unpublished candidate can be tested
by supplying a local Docker archive, its image name and its SHA-256 using
`--archive`, `--image` and `--sha256`. A local `image.txt` beside the installer
selects an already loaded image for repeatable lab installs. Rerunning the
installer reuses compatible Docker. Foreign services, conflicting ports and
unmanaged Shipper data fail preflight. Installer stages are recorded in
`/var/lib/shipper-installer/install.log`.

## First administrator

The installer prints the public address, for example `http://203.0.113.10:8080`.
Open it. Create the first administrator with `sudo shipperctl claim` on the host,
or use the setup token from `/var/lib/shipper/setup-token` on the setup form.
Allow TCP 80, 443 and 8080 through the provider firewall.

The dashboard initially runs without a domain. After signing in, open
**Settings**, **Dashboard**, and enter the hostname Shipper should answer on.
Point the DNS record at this server first. Shipper then adds its own route to the
global proxy and requests a trusted certificate. Application domains use HTTPS by
default on a managed install; an application may run without a public route.
Explicit HTTP remains available. Traefik owns issuance and renewal using its
[certificate resolver](https://doc.traefik.io/traefik/v3.6/reference/install-configuration/tls/certificate-resolvers/overview/).
System and Domains show observed readiness, issuer and expiry where available.

For manually hosted instances the existing setup form uses a private expiring
token from the data directory. `shipper rotate-claim` replaces it when needed.
Existing deliberate ACME overrides remain in effect.

For an authorized fresh host, `shipper bootstrap --help` documents SSH install
using the same local installer script and its checksum. Bring a verified
known-hosts file and an OpenSSH identity or agent. Apply only after reviewing
the preflight. Bootstrap retains no SSH credentials. Use System access tokens
and `shipper connect` for ongoing scoped CLI pairing.

## Updates and recovery

Use the installer with a new pinned version, or rerun the latest one-liner.
Review the checks with `--check` before applying. The managed update waits for
queued work, stops the control plane, and verifies a SQLite checkpoint against
its key and checksum. The candidate can report liveness before serving requests
or running the worker. An unhealthy or interrupted candidate can restore the
prior image and checkpoint; the candidate database is preserved for diagnosis.
Keep an off-host encrypted instance backup independently of these local
checkpoints.

The host's OS and Docker patch policy remains operator managed. System reports
boot services, the distribution's security updates service, clock synchronization
and whether a reboot is required. Run disruptive maintenance in a planned window
after verifying backups. Shipper does not patch or reboot the host during a deploy.

Use `sudo shipperctl status` or `sudo shipperctl logs` for exceptional host
diagnosis. `sudo shipperctl reset-factor` is an offline owner recovery: it
revokes sessions and owner operations tokens, keeps the instance policy, and
requires enrollment again if 2FA is enforced. Keep the one-use recovery codes
for ordinary recovery. See [backup and recovery](backups.md) for restoring a new
installation.

`sudo bash install.sh --uninstall` previews the removal; append `--apply` to
stop and remove the control plane and disable its boot service. The owned proxy,
application containers, certificates, application data, Shipper metadata and
Docker are preserved. Rerun the installer to attach the preserved state.

See the [acceptance plan](vps-test-plan.md) for the production gate. A candidate
install on one host is not a claim of support for other distributions or architectures.
