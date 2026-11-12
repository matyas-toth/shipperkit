# @shipperkit/cli

Control a Shipper instance from your terminal. The CLI runs on your computer and
talks to the authenticated v1 API. It is built for people and agents: every
command has a stable `--json` form, and `shipper api` exposes any application route.

## Install

```sh
npm install -g @shipperkit/cli
```

Node 20 or newer.

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
shipper deploy Website
shipper deploy Website --no-cache
shipper logs Website --lines 50
shipper logs Website --follow
shipper doctor Website
shipper restart Website
shipper open Website
```

`shipper deploy .` packages the current folder and creates an application. It skips
`.git`, `node_modules`, `.env` files, private keys and other secrets; extend the
exclusions with a `.shipperignore` file. Local uploads are limited to 64 MiB and
10000 files. Local Compose uploads are not included yet; connect the repository
instead.

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
