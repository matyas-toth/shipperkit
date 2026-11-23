# Security policy

## Report a vulnerability

Please don't report vulnerabilities in public issues, pull requests or Discord channels.

Report them privately through [GitHub's vulnerability reporting](https://github.com/matyas-toth/shipperkit/security/advisories/new). If that isn't available to you, send a direct message to a maintainer on [Discord](https://discord.shipperkit.dev) and we'll set up a private channel.

Include the affected version, the steps to reproduce and the impact you expect. We'll acknowledge the report, keep you updated while we fix it and credit you in the advisory unless you'd rather stay anonymous.

## Supported versions

Security fixes go into the latest release on the Main channel. Update from **Settings → Updates** or by running the installer again.

## Scope

The control plane, its API, the dashboard, the installer, the `shipper` CLI and the Service Library templates are in scope. Vulnerabilities in the applications you deploy, or in the third-party images the library references, belong to their own projects.

Shipper can control everything on its host through Docker, so keep the dashboard behind HTTPS and two-factor authentication. The [security model](docs/security.md) describes the boundaries in detail.
