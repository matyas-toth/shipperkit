# 1.0 release acceptance

Candidate: 1.0.0-rc.6. Scope: one managed Ubuntu 24.04 x86-64 VPS, one Docker
host, and the existing dashboard on desktop and mobile. The stable release and
public installer assets have not been published.

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
| Automated regression | Go and frontend suites, type checking and 13 desktop/mobile Playwright journeys. The two-factor tests verify API and stream gating, recovery code consumption, and rejection of a reused authenticator code. |

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

## Remaining stable-release gates

- Run with a real GitHub App and private repository; exercise signed public push
  delivery, revoked access and repository/ref selection.
- Prove certificate renewal using a controlled short-lived CA exercise, plus
  public www, DNS changes and representative failure/recovery cases. A proxy or
  host restart can prove persistence but cannot prove renewal.
- Restore a real off-server encrypted backup onto a fresh supported host and
  verify application data and deployment configuration.
- Finish the independent agent and human runs and equivalent platform baselines
  described in the [roadmap](roadmap.md), including timed failure scenarios and
  resource measurements. The browser and automated fixture runs are separate
  from that acceptance matrix.
- Complete the independent security review identified in
  [the security policy](../SECURITY.md).
- Publish versioned install artifacts only after the release gate passes.

See [managed installation](installation.md) for the supported platform,
preflight, maintenance and recovery commands.
