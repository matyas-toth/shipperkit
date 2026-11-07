#!/usr/bin/env bash
set -Eeuo pipefail
umask 077
cfg=/etc/shipper
state=/var/lib/shipper-installer
data=/var/lib/shipper
unit=shipper.service
release=https://github.com/matyas-toth/shipperkit/releases/download
export DOCKER_HOST=unix:///var/run/docker.sock
unset DOCKER_CONTEXT
domain= version= image= archive= checksum=
apply=true
check=false
explicit_apply=false
uninstall=false
die() { printf 'Shipper: %s\n' "$*" >&2; exit 1; }
stage() { printf '%s %s\n' "$(date -u +%FT%TZ)" "$1"; }
compose() { docker compose --project-name shipper-host --env-file "$cfg/instance.env" -f "$cfg/compose.yaml" "$@"; }
while (($#)); do
  case "$1" in
    --domain|--version|--image|--archive|--sha256)
      [[ $# -ge 2 && -n $2 ]] || die "Missing value for $1"
      case "$1" in
        --domain) domain=$2;; --version) version=$2;; --image) image=$2;;
        --archive) archive=$2;; --sha256) checksum=$2;;
      esac
      shift;;
    --apply) apply=true; explicit_apply=true;;
    --check) check=true;;
    --uninstall) uninstall=true;;
    -h|--help)
      printf '%s\n' 'install.sh [--version X.Y.Z] [--check]' \
        'install.sh --image IMAGE --archive FILE --sha256 HASH [--check]' \
        'install.sh --uninstall [--apply]' 'Use --check to review the host checks without applying them.'
      exit 0;;
    *) die "Unknown option $1";;
  esac
  shift
done
if $check; then
  apply=false
elif $uninstall && ! $explicit_apply; then
  apply=false
fi
[[ $(uname -s) == Linux && -r /etc/os-release ]] || die 'This installer requires Linux'
. /etc/os-release
[[ ${ID:-} == ubuntu && ${VERSION_ID:-} == 24.04 && $(uname -m) == x86_64 ]] || die 'Use Ubuntu 24.04 on x86-64'
[[ $EUID == 0 && -d /run/systemd/system ]] || die 'Run as root on a systemd host'
command -v flock >/dev/null && command -v ss >/dev/null || die 'Install util-linux and iproute2 first'
installed=false
if [[ -e $cfg || -L $cfg || -e $data || -L $data ]]; then
  [[ ! -L $cfg && ! -L $data && -f $cfg/managed-install ]] || die 'Existing Shipper paths are not owned by this installer'
  installed=true
fi
begin() {
  [[ ! -L $state ]] || die 'Invalid installer state directory'
  install -d -m 0700 "$state"
  exec 9>"$state/install.lock"
  flock -n 9 || die 'Another installation is in progress'
  exec > >(tee -a "$state/install.log") 2>&1
  trap 'stage "failed at line $LINENO; correct the error and rerun"' ERR
}
if $uninstall; then
  [[ -f $cfg/instance.env ]] || die 'No installed control plane was found'
  printf '%s\n' 'Disable and remove the Shipper control plane. Preserve applications, certificates, data and Docker.'
  $apply || exit 0
  begin
  systemctl disable --now "$unit"
  systemctl disable --now shipper-host-status.timer 2>/dev/null || true
  compose rm --force shipper
  stage 'uninstall: complete'
  exit 0
fi
if [[ -f $cfg/instance.env ]]; then
  old_domain=$(sed -n 's/^SHIPPER_DOMAIN=//p' "$cfg/instance.env")
  [[ -n $domain ]] || domain=$old_domain
elif ! $installed; then
  [[ ! -e /etc/systemd/system/$unit && ! -e /usr/local/bin/shipperctl ]] || die 'The service or CLI path is already in use'
  for port in 80 443 8080; do
    [[ -z $(ss -ltnH "sport = :$port") ]] || die "TCP port $port is already in use"
  done
fi
memory=$(awk '/^MemTotal:/ {print $2}' /proc/meminfo)
disk=$(df -Pk /var/lib | awk 'NR==2 {print $4}')
(( memory >= 1000000 && disk >= 8*1024*1024 )) || die 'Shipper needs 1 GiB RAM and 8 GiB available disk'
latest=${SHIPPER_RELEASE_BASE:-https://raw.githubusercontent.com/matyas-toth/shipperkit/latest}
pin='^ghcr\.io/matyas-toth/shipperkit@sha256:[0-9a-f]{64}$'
semver='^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?$'
if [[ -n $archive ]]; then
  [[ -z $version && -f $archive && $checksum =~ ^[a-fA-F0-9]{64}$ &&
      $image =~ ^[a-zA-Z0-9][a-zA-Z0-9._/:@-]{0,250}$ ]] || die 'A local archive needs --image and its SHA-256'
  sum=$(sha256sum -- "$archive")
  [[ ${sum%% *} == "${checksum,,}" ]] || die 'The image archive checksum did not match'
elif [[ -n $version ]]; then
  [[ $version =~ $semver ]] || die 'Provide a release version or a pinned image'
elif [[ -z $image ]]; then
  if [[ -f image.txt ]]; then
    image=$(<image.txt)
  else
    image=$(curl -fsSL --proto '=https' --retry 3 "$latest/image.txt") || die 'No Shipper release image is available'
  fi
  [[ -n $image ]] || die 'No Shipper release image is available'
fi
engine=install
if command -v docker >/dev/null; then
  docker info >/dev/null 2>&1 || die 'Existing Docker is unavailable'
  [[ $(docker info --format '{{.OSType}}') == linux ]] || die 'Use a Linux Docker engine'
  docker compose version >/dev/null 2>&1 && docker buildx version >/dev/null 2>&1 || die 'Existing Docker needs Compose and Buildx'
  api=$(docker version --format '{{.Server.APIVersion}}')
  [[ $api =~ ^1\.([0-9]+)$ ]] && (( BASH_REMATCH[1] >= 48 )) || die 'Existing Docker must support API 1.48 or newer'
  engine=reuse
elif command -v dockerd >/dev/null || command -v containerd >/dev/null || command -v podman >/dev/null; then
  die 'Found an existing container runtime without a compatible Docker CLI'
fi
public_ip=
if command -v curl >/dev/null 2>&1; then
  public_ip=$(curl -fsS --max-time 5 https://api.ipify.org 2>/dev/null || true)
fi
if [[ ! $public_ip =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  public_ip=$(hostname -I | awk '{print $1}')
fi
[[ -n $public_ip ]] || public_ip='<server-ip>'
printf 'Host: Ubuntu 24.04 x86-64\nDocker: %s\nDashboard: http://%s:8080\n' "$engine" "$public_ip"
printf '%s\n' 'Bind app traffic on TCP 80/443 and the dashboard on TCP 8080. Keep firewall and SSH settings.'
$apply || exit 0
begin
stage 'preflight: passed'
export DEBIAN_FRONTEND=noninteractive
if [[ $engine == install ]] || { [[ -z $archive ]] && { ! command -v curl >/dev/null || [[ ! -s /etc/ssl/certs/ca-certificates.crt ]]; }; }; then
  stage 'dependencies: ensuring curl and trusted CA certificates'
  apt-get -q update
  apt-get -o DPkg::Lock::Timeout=60 -qy install ca-certificates curl
fi
if [[ $engine == install ]]; then
  stage 'docker: installing packages from the signed Docker repository'
  install -d -m 0755 /etc/apt/keyrings
  curl -fsSL --proto '=https' --retry 3 https://download.docker.com/linux/ubuntu/gpg -o "$state/docker.asc"
  if [[ -e /etc/apt/keyrings/shipper-docker.asc ]]; then
    cmp -s "$state/docker.asc" /etc/apt/keyrings/shipper-docker.asc || die 'Existing repository key differs'
  else
    install -m 0644 "$state/docker.asc" /etc/apt/keyrings/shipper-docker.asc
  fi
  cat >"$state/docker.sources" <<'APT'
Types: deb
URIs: https://download.docker.com/linux/ubuntu
Suites: noble
Components: stable
Architectures: amd64
Signed-By: /etc/apt/keyrings/shipper-docker.asc
APT
  if [[ -e /etc/apt/sources.list.d/shipper-docker.sources ]]; then
    cmp -s "$state/docker.sources" /etc/apt/sources.list.d/shipper-docker.sources || die 'Existing repository settings differ'
  else
    install -m 0644 "$state/docker.sources" /etc/apt/sources.list.d/shipper-docker.sources
  fi
  apt-get -q update
  apt-get -o DPkg::Lock::Timeout=60 -qy install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  systemctl enable --now docker
fi
[[ -S /var/run/docker.sock ]] || die 'The local Docker socket is unavailable'
gid=$(stat -c %g /var/run/docker.sock)
if [[ -n $version ]]; then
  image=$(curl -fsSL --proto '=https' --retry 3 "$release/v$version/image.txt")
  [[ $image =~ $pin ]] || die 'The release did not provide a valid image digest'
fi
if [[ -z $archive && ! $image =~ $pin ]] && ! docker image inspect "$image" >/dev/null 2>&1; then
  die 'Use a pinned Shipper image digest or a locally loaded image'
fi
stage 'artifact: loading and checking the image'
if [[ -n $archive ]]; then docker load -i "$archive"; elif ! docker image inspect "$image" >/dev/null 2>&1; then docker pull "$image"; fi
image_id=$(docker image inspect --format '{{.Id}}' "$image")
[[ $image_id =~ ^sha256:[0-9a-f]{64}$ ]] || die 'Invalid local image identity'
reported=$(docker run --rm --network none --pull never "$image_id" version)
[[ $reported =~ $semver && ( -z $version || $reported == "$version" ) ]] || die 'Artifact version did not match'
if ! $installed; then
  install -d -m 0700 "$cfg"
  printf '%s\n' 'shipper-managed-v1' >"$cfg/managed-install"
fi
[[ -d $data ]] || install -d -o 65532 -g 65532 -m 0700 "$data"
[[ $(stat -c %u "$data") == 65532 ]] || die 'Unexpected data directory ownership'
if [[ -n $domain ]]; then
  printf '%s\n' "$domain" >"$data/dashboard-domain"
  chown 65532:65532 "$data/dashboard-domain"
  chmod 0600 "$data/dashboard-domain"
fi
install -d -m 0755 "$state/status"
previous=
checkpoint=
recover_candidate() {
  local id=$1
  [[ $id =~ ^[a-zA-Z0-9-]{1,64}$ && -f $state/previous.env ]] || die 'Invalid interrupted update record'
  systemctl stop "$unit" || true
  compose stop shipper
  cp "$state/previous.env" "$cfg/instance.env"
  compose run --rm --no-deps -T shipper restore-checkpoint --id "$id" || die "Restore checkpoint $id before starting the previous image"
  systemctl reset-failed "$unit" || true
  systemctl start "$unit"
  : >"$state/update-pending"
  rm -f "$data/update.lock"
  stage 'update: previous image and database restored'
}
if [[ -s $state/update-pending ]]; then
  stage 'update: recovering an interrupted candidate'
  recover_candidate "$(cat "$state/update-pending")"
elif [[ -e $data/update.lock ]]; then
  rm -f "$data/update.lock"
fi
if [[ -f $cfg/instance.env ]]; then previous=$(sed -n 's/^SHIPPER_IMAGE=//p' "$cfg/instance.env"); fi
if [[ -n $previous && $previous != "$image_id" ]]; then
  stage 'update: checking operations and taking a checkpoint'
  compose exec -T shipper shipper update-ready
  systemctl stop "$unit"
  checkpoint=$(date -u +%Y%m%dT%H%M%SZ)-$$
  if ! compose run --rm --no-deps -T shipper checkpoint --id "$checkpoint"; then
    systemctl start "$unit"
    die 'Checkpoint failed; kept the previous version'
  fi
  cp "$cfg/instance.env" "$state/previous.env"
  install -m 0644 /dev/null "$data/update.lock"
  printf '%s\n' "$checkpoint" >"$state/update-pending"
fi
cat >"$cfg/compose.yaml" <<'COMPOSE'
services:
  shipper:
    image: "${SHIPPER_IMAGE}"
    pull_policy: never
    ports: ["0.0.0.0:8080:8080"]
    environment:
      SHIPPER_DASHBOARD_UPSTREAM: auto
      SHIPPER_PROXY_BIND: 0.0.0.0
      SHIPPER_HTTP_PORT: "80"
      SHIPPER_HTTPS_PORT: "443"
      SHIPPER_AUTO_HTTPS: "1"
      SHIPPER_MANAGED_INSTALL: "1"
    volumes:
      - /var/lib/shipper:/data
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - /var/lib/shipper-installer/status:/run/shipper-host:ro
    group_add: ["${SHIPPER_DOCKER_GID}"]
    read_only: true
    tmpfs: ["/tmp:size=32m,mode=1777"]
    cap_drop: [ALL]
    security_opt: [no-new-privileges:true]
    labels:
      io.shipper.control-plane: "true"
    restart: unless-stopped
    logging:
      driver: json-file
      options: {max-size: "5m", max-file: "3"}
COMPOSE
cat >/etc/systemd/system/shipper.service <<'UNIT'
[Unit]
Description=Shipper control plane
Wants=network-online.target
Requires=docker.service
After=network-online.target docker.service
PartOf=docker.service

[Service]
Type=oneshot
RemainAfterExit=yes
Environment=DOCKER_HOST=unix:///var/run/docker.sock
ExecStart=/usr/bin/env docker compose --project-name shipper-host --env-file /etc/shipper/instance.env -f /etc/shipper/compose.yaml up --detach --wait --wait-timeout 120
ExecStop=/usr/bin/env docker compose --project-name shipper-host --env-file /etc/shipper/instance.env -f /etc/shipper/compose.yaml stop --timeout 30
TimeoutStartSec=150
TimeoutStopSec=50

[Install]
WantedBy=multi-user.target
UNIT
chmod 0644 /etc/systemd/system/shipper.service
cat >/usr/local/bin/shipperctl <<'CONTROL'
#!/usr/bin/env bash
set -euo pipefail
export DOCKER_HOST=unix:///var/run/docker.sock
unset DOCKER_CONTEXT
compose() { docker compose --project-name shipper-host --env-file /etc/shipper/instance.env -f /etc/shipper/compose.yaml "$@"; }
case "${1:-}" in
  claim|rotate-claim)
    if [[ -t 0 ]]; then compose exec shipper shipper "$@"; else compose exec -T shipper shipper "$@"; fi;;
  reset-password|reset-factor)
    [[ -t 0 ]] || { printf '%s\n' 'Recovery needs a terminal'; exit 2; }
    systemctl stop shipper.service
    trap 'systemctl start shipper.service' EXIT
    compose run --rm --no-deps shipper "$@";;
  status) systemctl status shipper.service --no-pager;;
  logs) compose logs --tail 100 shipper;;
  *) printf '%s\n' 'Usage: shipperctl claim|status|logs|reset-password|reset-factor|rotate-claim'; exit 2;;
esac
CONTROL
chmod 0755 /usr/local/bin/shipperctl
install -d -m 0755 /usr/local/libexec
cat >/usr/local/libexec/shipper-host-status <<'HOST'
#!/usr/bin/env bash
set -euo pipefail
status=/var/lib/shipper-installer/status
docker=false shipper=false reboot=false security=false clock=false
if systemctl is-enabled --quiet docker.service && systemctl is-active --quiet docker.service; then docker=true; fi
if systemctl is-enabled --quiet shipper.service && systemctl is-active --quiet shipper.service; then shipper=true; fi
if systemctl is-enabled --quiet unattended-upgrades.service; then security=true; fi
if [[ -e /var/run/reboot-required ]]; then reboot=true; fi
if [[ $(timedatectl show -p NTPSynchronized --value 2>/dev/null || true) == yes ]]; then clock=true; fi
printf '{"observedAt":%s,"dockerAtBoot":%s,"shipperAtBoot":%s,"rebootRequired":%s,"securityServiceEnabled":%s,"clockSynchronized":%s}\n' \
  "$(date +%s)" "$docker" "$shipper" "$reboot" "$security" "$clock" >"$status/status.next"
chmod 0644 "$status/status.next"
mv -f "$status/status.next" "$status/status.json"
HOST
chmod 0755 /usr/local/libexec/shipper-host-status
cat >/etc/systemd/system/shipper-host-status.service <<'HOSTUNIT'
[Unit]
Description=Observe Shipper host boot and maintenance state

[Service]
Type=oneshot
ExecStart=/usr/local/libexec/shipper-host-status
NoNewPrivileges=yes
ProtectSystem=strict
ReadWritePaths=/var/lib/shipper-installer/status
HOSTUNIT
cat >/etc/systemd/system/shipper-host-status.timer <<'TIMER'
[Unit]
Description=Refresh Shipper host status

[Timer]
OnBootSec=20s
OnUnitActiveSec=1min
AccuracySec=5s

[Install]
WantedBy=timers.target
TIMER
chmod 0644 /etc/systemd/system/shipper-host-status.service /etc/systemd/system/shipper-host-status.timer
printf 'SHIPPER_IMAGE=%s\nSHIPPER_DOCKER_GID=%s\n' "$image_id" "$gid" >"$cfg/instance.next"
mv -f "$cfg/instance.next" "$cfg/instance.env"
systemctl daemon-reload
systemctl enable "$unit"
stage 'service: starting the control plane'
if ! systemctl start "$unit"; then
  if [[ -n $checkpoint ]]; then
    recover_candidate "$checkpoint"
  fi
  die 'Candidate did not become healthy; inspect journalctl -u shipper.service'
fi
if [[ -n $checkpoint ]]; then
  : >"$state/update-pending"
  rm -f "$data/update.lock"
fi
stage "service: Shipper $reported is ready"
systemctl enable --now shipper-host-status.timer
systemctl start shipper-host-status.service
printf 'Open Shipper: http://%s:8080\n' "$public_ip"
printf '%s\n' 'For a fresh installation, run sudo shipperctl claim, then add the dashboard domain in Settings.'
