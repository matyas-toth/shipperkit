# Planned real-system acceptance lab

Status: offered resources and future test plan. Access has not been supplied in
this discussion and these tests are not claimed complete. Credentials never
belong in this document. See the [roadmap](roadmap.md).

## Resources and scope

The user will provide a disposable Contabo VPS with 8 GB RAM, permission to
reinstall it with multiple Linux distributions, and 3–4 domains for deployment
and DNS/address-resolution testing. Record its actual CPU architecture, disk,
network, provider image and distro versions when access arrives. The 8 GB host
is not a measured minimum requirement or proof of other architectures.

Record the exact authorized VPS and test domains/records before execution.
Reinstallation and destructive tests apply only to that host. Keep redacted
evidence and recovery artifacts off the machine so reinstalling it cannot erase
the results or only backup. Preserve SSH/provider-console recovery. DNS changes
stay within authorized test names; record previous values and clean up afterward.
Production accounts, hosts, names and unrelated workloads are outside scope.

## Matrix and sequence

Use a small declared Linux matrix, with exact versions. Reinstall the allowed
host per image; cover Docker absent, compatible Docker present and incompatible
software producing a safe supported outcome.

1. Run the installer and exercise preflight, interruption/rerun, dependencies,
   secure first-admin creation and boot services.
2. Prove the first admin email initializes ACME without an extra email field or
   environment edit. Check partial setup recovery and preservation of deliberate
   legacy contact/account configuration.
3. Connect real GitHub/private repositories, pick revisions, receive public signed
   pushes, verify the deployed revision and exercise revoked access.
4. Deploy web, static, worker and multi-service database apps. Test .env, commands,
   migrations, scheduling, health and runtime controls.
5. Use the test names for dashboard, app and per-service routes, multiple domains,
   www, A/AAAA behavior and DNS propagation/address changes. Distinguish an
   unreachable AAAA record from a platform issuance failure.
6. Check trusted automatic HTTPS issuance, redirects, hostname coverage, renewal
   and persisted CA/account state. Use staging for repeated failure/reinstall
   exercises and a trusted production-CA issuance case. Record genuine renewal
   evidence; restarting a proxy or issuing a first certificate is not renewal.
   Keep TLS verification enabled, preserve accurate time and respect CA limits.
7. Break/restore DNS and reachability; test transient issuance failure, expiry-risk
   alerts, bounded retry, automatic recovery and quiet successful operation.
8. Restart proxy/host with an intentionally stopped app and verify desired states,
   routes, certificates, volumes and schedules recover.
9. Back up metadata/key/certificates and supported app data off-server, reinstall,
   restore and verify actual application reads/writes and credentials.
10. Apply a pinned update; inject interruption, low disk, daemon and port failures;
    verify diagnosis, safe retention and recovery without erasing live/retained data.
11. Test P10 TOTP, enforcement against existing admin sessions and offline recovery.
    Extend this in P11 to provisioned users, all OAuth methods, verified-email
    linking, mandatory password/factor gates and role isolation.
12. Exercise data-preserving uninstall and authorized cleanup; report manual
    SSH/provider intervention needed during any normal or exceptional operation.

## Complementary manual and agent journeys

Keep the entire matrix above. In addition, exercise the following P9/P10 workflow
with an external coding agent and an ordinary repository. This is an acceptance
plan, not a claim that CLI/bootstrap/local-upload support already exists.

- Give only the authorized test VPS/SSH access, repository and test-domain/DNS
  authority needed for the task. Record required human authorization separately
  from avoidable setup questions; never grant access to unrelated infrastructure.
- Bootstrap one fresh supported VPS through the same installer, verify host
  identity, securely claim/pair management access, inspect the local project,
  review effects, upload an immutable snapshot, deploy and watch the operation.
- Verify the active release and declared readiness/routing/HTTPS/endpoint results
  without dashboard navigation, GitHub connection or hand-configured Docker/proxy.
  Record the vantage and limits of checks, including any external probe.
- Repeat with wrong ports, missing variables, dependency readiness delays, failed
  updates, low disk and interrupted uploads/watches. Exercise idempotent retry,
  stale-plan rejection, unknown task outcomes and recovery while preserving data
  and the previous healthy release where supported.
- Run the equivalent installer/dashboard journey by hand without an agent or CLI.
  Local-source apps remain inspectable and operable through the normal dashboard.
- Record task completion, elapsed time, human interventions, operation/evidence
  references, tool calls/tokens when available and resource/data preservation.
  For comparative runs against direct SSH/Docker and Coolify, hold workloads,
  server resources, permissions and agent/model versions equivalent; disclose
  configuration differences and failure cases instead of claiming assumed wins.

## Evidence

Record commit/artifact identity, OS/architecture/engine/Compose versions, setup,
domain/DNS changes, steps, operation IDs/results, certificate issuer/expiry and
renewal evidence, restore proof, timings/resources and remaining gaps. Redact all
passwords, private keys, recovery codes, OAuth tokens and provider credentials.

Use Passed, Failed or Blocked with a concrete reason. Provider/DNS/rate-limit
blocks are not passed tests. Mocks and generated trust anchors remain useful
regressions but do not replace live acceptance. Publish support only for tested
host combinations.
