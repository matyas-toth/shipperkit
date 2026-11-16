# 1.0 release acceptance

Candidate: **Shipper 1.0.0 Beta 2**, technical version `1.0.0-rc.23`. Scope: one
managed Ubuntu 24.04 x86-64 VPS, one Docker host, and the existing dashboard on
desktop and mobile. Public prerelease image and installer assets are published.
The stable `1.0.0` release remains gated by the checks below.

## Nightly release readiness, 2026-11-16

PR #71 merged as `870b3e831dd5c7552abb8b17bc39e76085b9625c`. Nightly
`1.0.0-rc.27.nightly.56` was published from that revision with image digest
`sha256:b7d2e41c5d5a902925c9083c6dcc044d04b6ea5e6d8f0301195e7a6aa4b2591b`.
The feed checksums, image revision, embedded version and Go 1.27.4 compiler were
verified before installation. The host runs Ubuntu 24.04.5 x86-64, Docker
29.9.3, Compose 5.7.0 and Buildx 0.38.1.

| Journey | Observed result |
| --- | --- |
| Automated acceptance | All GitHub CI gates passed. The complete real Docker deployment/API suite also passed on the VPS. Network inventory was 13 before and after, with no leaked test networks. |
| Managed update | The normal Nightly updater completed in 30 seconds. All 27 public application probes succeeded; existing workload/proxy container identities and public certificate identities were preserved. |
| CLI package and source update | The bundled tarball installed offline outside the repository. A local static project deployed, uploaded a different immutable snapshot through explicit app binding, and rolled back to the first snapshot. Both stored archives excluded literal registry credentials and project-ignored data. |
| Logs and credential binding | Empty history returned zero lines and exited; latest-five returned 206–210 from a 210-line fixture. A 211-line full snapshot redacted its canary secret, following cancelled, and an alternate-origin credential test made zero HTTP requests. |
| Current-role enforcement | Admin→Viewer, Admin→Developer and Developer→Viewer revoked existing sessions and terminated real Docker shells. Powerful tokens minted by the Admin subjects were revoked; fresh logins received the new role. Legacy Developer token records remain covered by the permanent store/API regressions. |
| Maintenance authority | Instance-scoped host-admin tokens performed recheck, reviewed repair and retention settings. Account/update administration, app-bound host maintenance and access to another application were denied. |
| Native protection | PostgreSQL 18 passed writes and reads with certificate verification. The encrypted native archive was downloaded off-server, its checksum verified, and a reviewed copy restore retained the original row. |
| Fresh installation | Data-preserving uninstall left the original application serving. After the saved installation was moved aside, the managed Nightly installer created a separate instance. First-owner setup supplied the ACME contact and the new dashboard served trusted HTTPS. |
| Authenticated instance recovery | The downloaded instance archive restored the original encryption key, database CA identity, accounts/roles/identity metadata and both source snapshots. Sessions/tokens were revoked and applications/schedules were stopped. A tampered copy of the real archive was rejected before installing a database. |
| Fresh database storage | The recovered PostgreSQL application started on a new empty volume. The original off-server client CA trusted its newly issued server certificate. Importing the downloaded encrypted native backup recovered the saved row, and subsequent writes/reads succeeded. |
| Workload recovery and reboot | The three original applications restarted from their preserved volumes, and the recovered local-source snapshot redeployed. A changed boot ID confirmed a real reboot: six intended workloads returned, the intentionally stopped worker stayed stopped, and trusted certificates/database trust persisted. |
| Failed update | A local candidate passed artifact-version preflight but exited instead of serving. The managed installer restored its prior image and SQLite checkpoint automatically; Nightly and the original application were healthy afterward. |

Live testing also found two presentation defects: the CLI assumed HTTPS even
when a public domain explicitly used HTTP, and installer preflight printed a
literal `%s` instead of the dashboard IP. The follow-up honors domain protocol
metadata, including legacy configuration, and expands the installer address.
The CLI correction passed real HTTP and HTTPS route checks on the installed
Nightly, plus behavioral tests through the executable CLI.
The patched installer passed syntax validation and `--check` on the VPS with
the expanded numeric address and no installation changes.

These results cover one supported VPS configuration. Live Google/GitHub/Discord
sign-in, a genuine ACME renewal, a broader OS/Docker installation matrix and an
independent security review remain unproven by this run.

| Journey | Evidence |
| --- | --- |
| Fresh managed install | Ubuntu lab with 7.8 GiB RAM and 94 GiB free. Preflight detected missing Docker. The installer added Docker Engine 29.9.3, Buildx 0.38.1 and Compose 5.7.0 from the signed package repository. Subsequent candidate updates reused Docker and verified a pre-update SQLite checkpoint. |
| First owner and public HTTPS | Claimed the owner on the host. Shipper used that email for ACME without an extra setting. Verified ordinary TLS trust and an HTTP redirect at the dashboard, trusted HTTPS for a local-source static site and its second domain, and a Compose service route. The first install exposed a certificate-file permissions bug, which was repaired while preserving the ACME volume. |
| Deployment | Uploaded a local folder archive and deployed its static site, then added a second domain without rebuilding. Deployed a process-only worker and stopped it. Deployed a Compose web and PostgreSQL stack with dependency readiness and an after-deploy SQL migration. |
| Reboot and update recovery | Confirmed a new boot ID, matching serial numbers for four trusted certificates and an intentionally stopped worker still stopped. Injected an unhealthy image; Shipper restored the previous image and verified SQLite checkpoint. Trusted dashboard and application routes worked afterward. |
| Uninstall and reattach | Removed only the control plane; the HTTPS application routes stayed live and all workload container IDs and states were preserved. Reapplied the installer and the dashboard returned with the same instance data. |
| Backup and recovery | Downloaded the encrypted native PostgreSQL backup to another machine and verified its checksum. A reviewed restore to a new database preserved the original; both contained the migration row. The isolated Docker suite also passed native PostgreSQL, MySQL and MariaDB restore tests, encrypted off-server recovery and instance portability. |
| CLI | Used a verified host key and temporary SSH identity for installer preflight. Paired with a scoped token, reviewed and applied a local static project, waited for deployment and verified trusted HTTPS in 24 seconds. Confirmed the revoked token could no longer list applications. |
| Account | On the lab owner, verified 2FA enforcement for an existing session, QR enrollment, challenge on sign-in, one-use recovery and disabling under the optional policy. Restored the lab's original policy. |
| Dashboard | On the existing local instance, opened the Compose service from Overview, searched timestamped deployment logs, opened .env, domains, backups and System, previewed a weekly schedule and its cron expression, opened the 2FA reauthentication step, and reviewed a Worker deployment without applying it. Rechecked stopped service log history after removing its misleading live state. |
| Automated regression | Go and frontend suites, type checking and 18 desktop/mobile Playwright journeys. The two-factor tests verify API and stream gating, recovery code consumption, and rejection of a reused authenticator code. |

An observation after reboot: 12.88 MiB of control plane container memory,
excluding Docker, Traefik and workloads. Five trusted health requests from the
development machine took 127–276 ms (153 ms median), including network and TLS.
These are sample measurements, with no comparative platform claim.
The reboot, rollback and uninstall drills used RC4. RC5 adds the scheduling
presentation and release packaging fixes. The RC5 archive was installed on the
lab through the checkpointed update path. The dashboard and all four application
hostnames responded with trusted HTTPS afterward. The packaged Linux image
manifest is `sha256:71658df3e552bbdb23fac2a889174a8e29ad6ca4a12e4a105f7c0d06c31d6259`.

RC6 separates source and build method, adds a browsable branch picker, makes
Recovery and other disclosures visible, and scopes Settings and System to the
selected section. The new domain editor wraps on narrow screens. Verified the
candidate in the real local and staging browsers, including a public Git Compose
review, an internal service's link to port settings, and a proxy repair preview.
All 13 desktop and mobile browser journeys passed. The staging update preserved
the five previously checked HTTPS routes. Linux image manifest:
`sha256:455893524c631aa5b67aa69c4f2c88eeae811444b89011031de7745d8d73e051`.

A staging update on 2026-11-06 exercised the deployment command scopes, release
history layout, protected volume ordering and background refresh behavior. A
Compose fixture with two services and three volumes proved that an all-services
command runs before the service command for before deploy, before start, after
deploy and before stop, and that the volume API and backup UI return adata,
mdata, zdata in stable order. The release history rendered 34 entries without
clipping, the Recheck control stayed enabled through 18 background refresh
samples, and a reviewed all-services command edit applied from the dashboard.
The interface test also caught a revision bug where planning reordered the
caller's hooks; the fix clones the comparison configuration before sorting. Full
Go and frontend suites passed after the fix. Linux image manifest:
`sha256:799e45fe84c4947b2bdd3728f6a863a2a15e84a4a3f6caebb1b23a33f76ec06b`.

A fresh staging reinstall on 2026-11-06 removed the previous control plane,
proxy, applications, volumes, images and data, then ran the installer with no
arguments against a locally loaded image. It printed
`http://13.140.218.79:8080`, started the control plane and proxy, and accepted the
web setup token. Settings now groups Account settings and Server settings, with
Access policy as a server page and the dashboard domain on the Dashboard page.
Saving `shipper-stage.askmyfridge.com` there created its own proxy route and a
trusted Let's Encrypt certificate while the public IP continued to serve the
control plane on 8080. Parameterless updates preserved the dashboard domain and
the certificate. Final Linux image manifest:
`sha256:bf14d2568a273b7d5188a1a2617192f793d1c6560c92e3f5cb681ac77bff76bb`.

## Beta 2 collaboration acceptance

Shipper 1.0.0 Beta 2 adds administrator-provisioned collaboration, Viewer,
Developer and Admin roles, Email and password plus Google, GitHub and Discord
sign-in, verified-email identity linking, temporary-password gates that survive
provider login, personal and enforced TOTP, per-user audit, and the split-video
authentication shell.

Automated evidence:

- Migration 014 upgrades a single-owner database and preserves its administrator,
  session, factor and recovery state.
- Provider secrets are sealed, excluded from API responses and rotate deliberately.
- Google state replay, PKCE verifier, nonce, audience, issuer and signature
  failures are rejected. GitHub and Discord verified and private-email paths are
  covered, including changed addresses and ambiguous matches.
- One provisioned identity signs in through password, Google, GitHub and Discord;
  unprovisioned, disabled, unverified and ambiguous identities fail safely.
- Temporary-password reuse, removal, later password additions and provider login
  are covered. Disabling an account revokes sessions, operation tokens and
  terminal sessions.
- Factor enrollment and verification gate existing sessions and personal tokens.
  Passwordless provider accounts enroll without a password.
- Role isolation covers Viewer reads and blocked mutations, Developer application
  actions, and Admin-only users and provider settings.
- Last-admin protection covers demotion, disable, delete, password removal,
  password policy disable and provider removal.
- Browser coverage includes user provisioning and provider save through the real
  Settings UI, including the CSRF regression that Beta 2 fixes.

Staging was updated to `1.0.0-rc.22.nightly.36`, which includes the settings CSRF
fix. Health reports that version after the install. Beta 2 is published as
`1.0.0-rc.23`; the Main channel and installer assets use the release digest
`sha256:7dcaf84aaf66f80058e9c88a2b752ce7ab7002f2867f6b497c4c7e9ff5c77bce`.

Not yet proven on a real installation:

- Live Google, GitHub and Discord consoles with real client credentials.
- An in-place Main-channel update from Beta 1 or rc.21 to Beta 2 with an existing
  multi-user database.
- Live provider sign-in and provider-console interoperability after recovery;
  the Nightly drill above verifies recovered account/identity metadata and keys.
- Full human role acceptance across every Viewer, Developer and Admin surface.

## Remaining stable-release gates

- Run live Google, GitHub and Discord sign-in with real OAuth clients, including
  provider-side callback registration, verified/private email, secret rotation,
  provider removal, an outage and a failed return-to-Settings test.
- Update a real Beta 1 or rc.21 installation in place to Beta 2 from the Main
  channel and verify accounts, roles, provider links, factors, sessions,
  applications and releases survive.
- Restore a real off-server encrypted backup from a multi-user installation onto
  a fresh supported host and verify application data, identities, factor state,
  enforcement and provider secrets.
- Exercise every Users and Authentication action as a human on staging for all
  three roles, including password reset, factor reset, account removal and the
  last-admin guard.
- Run with a real GitHub App and private repository; exercise signed public push
  delivery, revoked access and repository/ref selection.
- Prove certificate renewal using a controlled short-lived CA exercise, plus
  public www, DNS changes and representative failure/recovery cases. A proxy or
  host restart can prove persistence but cannot prove renewal.
- Finish the independent agent and human runs and equivalent platform baselines
  described in the [roadmap](roadmap.md), including timed failure scenarios and
  resource measurements. The browser and automated fixture runs are separate
  from that acceptance matrix.
- Complete the independent security review identified in
  [the security policy](../SECURITY.md).
- Publish the stable `1.0.0` install artifacts only after the release gate passes.

See [managed installation](installation.md) for the supported platform,
preflight, maintenance and recovery commands.
