# @shipperkit/cli

Control a Shipper instance from your terminal. The CLI runs on your computer and
talks to the authenticated v1 API. It is built for people and agents: every
command has a stable `--json` form, and `shipper api` exposes any application route.

## Install

```sh
git clone https://github.com/matyas-toth/shipperkit.git
cd shipperkit
npm ci --prefix cli
npm run build --prefix cli
npm run cli:package
npm install -g --offline ./shipper-cli.tgz
```

Node 20 or newer.

New application releases also attach `shipper-cli.tgz` and its checksum to the
GitHub release. Download both from the same tagged release, verify the archive
against `SHA256SUMS`, then run `npm install -g --offline ./shipper-cli.tgz`. Older releases
may not include this asset; the source installation above works without an npm
registry publication. Runtime dependencies are bundled from the locked build,
so installing the tarball does not need a registry connection.

## Sign in

```sh
shipper auth login
```

Enter the instance URL, for example `https://shipper.example.com`. The CLI opens
Shipper in the browser. Sign in, approve the terminal, and the CLI stores a
30-day full app access token. Only an administrator can approve a terminal.

```sh
shipper auth status
shipper auth logout
```

The token is stored in `%APPDATA%\shipper\config.json` on Windows and
`~/.config/shipper/config.json` elsewhere. Revoke it from **System > Access
tokens** at any time.

The token grants full app access: applications, environment, command bodies,
secret reveals, domains, releases, backups, tasks and deletion. Administration,
terminals and instance recovery remain interactive.

## Everyday commands

```sh
shipper apps
shipper app Website
shipper deploy .
shipper deploy . --name atlas --domain atlas.example.com
shipper deploy . --app atlas
shipper deploy Website
shipper deploy Website --no-cache
shipper logs Website --lines 50
shipper logs Website --follow
shipper doctor Website
shipper restart Website
shipper open Website
```

`shipper deploy .` packages the current folder and creates an application. It skips
`.git`, `node_modules`, `.env` files, private keys and credential files such as
`.npmrc` and `.netrc`. Directory-specific `.gitignore` and `.shipperignore` files
use Git ignore patterns; excluded directories are not traversed. Credential
exclusions always apply, including to explicitly unignored files. Local uploads are limited to 64 MiB and
10000 files. Local Compose uploads are not included yet; connect the repository
instead.

Use `shipper deploy . --app atlas` to update an existing local application by
exact name or ID. It uploads a new immutable snapshot, reviews the configuration
revision and applies that revision, preserving environment, storage, domains and
previous release snapshots. A name collision never overwrites an application
automatically. `shipper deploy atlas` redeploys its already stored source.

`shipper logs atlas --lines 50` returns the latest available lines and exits,
including when there are fewer than 50. Add `--follow` to wait for future output.
Finite `--json` logs return `{ "lines": [...] }`; following returns one JSON
object per log line. Both line limits are bounded to 500.

Saved tokens are bound to their instance origin. Changing `--url` or
`SHIPPER_URL` to a different host, scheme or port requires a new login or an
explicitly supplied token. Disabling or demoting an account revokes its saved
credentials; CLI logout removes only the local copy.

Create an application from a Git repository:

```sh
shipper apps create "Docs" \
  --repo https://github.com/acme/docs \
  --branch main \
  --domain docs.example.com
```

Remove one:

```sh
shipper delete Docs --yes
```

## Any app action

`shipper api` calls the same handlers as the dashboard with the CLI token:

```sh
shipper api GET /applications
shipper api GET /applications/application-web
shipper api POST /applications/application-web/lifecycle --body '{"action":"restart"}'
shipper api POST /applications/application-web/changes --body '{"config":{...}}'
```

Paths are relative to `/api/v1`. JSON is printed unchanged, so the command is
safe to pipe into `jq` or feed to an agent.

## Agents

Use environment credentials and JSON output:

```sh
export SHIPPER_URL="https://shipper.example.com"
export SHIPPER_TOKEN="<admin-created full token>"

shipper apps --json
shipper app Website --json
shipper doctor Website --json
shipper deploy Website --json
shipper api GET /applications --json
```

`--json` prints one JSON document on success and a JSON error object on stderr.
Exit code `1` means the command or the operation failed. `SHIPPER_CONFIG`
overrides the config path.

## Development

```sh
cd cli
npm install
npm run check
npm run build
node dist/index.js apps
```
