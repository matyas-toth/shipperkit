# Developer documentation

These documents describe how Shipper works inside. User guides live at [shipperkit.dev/docs](https://shipperkit.dev/docs/), built from `landing/docs`.

## Start here

| Document | Covers |
| --- | --- |
| [Development](development.md) | Running from source, configuration and the test suites |
| [Architecture](architecture.md) | The control plane, persistence, operations, proxy and realtime updates |
| [HTTP API](api.md) | The v1 API used by the dashboard, the CLI and agents |
| [Releases, CI and staging](releases.md) | Required checks, release-please, staging and installer channels |
| [Security model](security.md) | Isolation, credentials, sign-in and recovery boundaries |

## Features

| Document | Covers |
| --- | --- |
| [Sources and deployment triggers](sources.md) | Git, GitHub, images, inline Dockerfiles and push deploys |
| [Full application operations](full-applications.md) | Dockerfile and Railpack builds, variables and lifecycle |
| [Static site operations](static-sites.md) | Apache packaging, domains and publish isolation |
| [Docker Compose applications](compose.md) | Supported Compose options and rollback limits |
| [Commands and tasks](commands.md) | Startup hooks and scheduled tasks |
| [Routing and recorded releases](routing-and-releases.md) | Domains, path routing, maintenance and rollback |
| [Database connections](database-connections.md) | The private network between applications and databases |
| [Service Library](service-library.md) | The catalog, its generator and how templates deploy |
| [Backups and recovery](backups.md) | Backup plans, archives, S3 and restores |
| [Authentication](authentication.md) | Accounts, roles, sign-in providers and two-factor |
| [Operations and local deployments](operations.md) | Diagnostics, tokens and the CLI |
| [Diagnostic playbook](agent-diagnostics.md) | How an operator or agent diagnoses a failing application |
| [Managed installation](installation.md) | The installer, host layout and recovery |
| [Updates](updates.md) | Channels and the host-side updater |

## Planning and history

| Document | Covers |
| --- | --- |
| [Roadmap](roadmap.md) | Phases and milestones |
| [Feature parity](feature-parity.md) | The Coolify capability audit |
| [Observability proposal](observability-proposal.md) | The diagnosis and capacity design for 1.1 |
| [1.0 release acceptance](release-acceptance.md) | What was verified before 1.0 |
| [Real-system acceptance lab](vps-test-plan.md) | The VPS test plan |
| [Phase 3 decisions](phase-3-decisions.md) | Early implementation decisions |
