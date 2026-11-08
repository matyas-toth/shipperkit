# Sources and deployment triggers

## GitHub

Open Settings > Connections > Connect GitHub. Shipper creates a private GitHub App
through GitHub's manifest flow. Install it on the account and repositories you
intend to deploy. The repository list reflects the installation's permissions.
An existing App or a repository-scoped token can instead be added through Add
connection. Source connections are independent of dashboard login.

Paste a repository URL when creating an application, or choose an account,
container image or local files beneath the source field. Continue inspects the
source and proposes a deployment before showing manual controls. Compose files,
Dockerfiles and supported build systems are detected from the selected directory.
Configure manually opens the original Source, Configure and Review steps, with
the current source carried into the form. Without a GitHub connection, the first
source option is Connect GitHub and opens Settings > Connections. With a connected
account, Personal GitHub Account opens an account picker. Repository loading,
repository selection and branch selection appear beneath the source input.
The manual flow also accepts a full commit SHA.
Credentials are encrypted in SQLite and omitted from ordinary API responses.
Installation tokens are short-lived and restricted to the selected repository
for cloning. HTTPS Git credentials use process-local configuration, never URL
parameters or command arguments. Editing a connection retains blank credentials;
provide a replacement to rotate them. Disconnecting an in-use connection is blocked.

The GitHub App requests read access to contents and metadata and receives push
events. A public HTTPS dashboard origin is required for inbound GitHub webhooks.
Managed installs get it from **Settings → Dashboard**; manual installs can set
`SHIPPER_PUBLIC_URL`. Local onboarding works, but localhost cannot receive GitHub
deliveries. After moving to a public endpoint, update and activate the App's
webhook in GitHub.
The explicit Webhook action reveals the URL and signing secret for manual setup.
Manifest and installation callbacks have expiring, one-use state, and verify the
installation against the newly created App.

## Automatic deployments

Enable Deploy on push under Automatic deployments. The configured branch must
match the push. Optional watch paths support slash-separated globs, including
`apps/web/**`. Deleted branches, mismatched repositories and commits containing
`[skip shipper]` are ignored. When the provider truncates changed-file details,
Shipper deploys conservatively instead of silently missing a relevant change.

HMAC-SHA256 validation happens before an event is accepted. Delivery identifiers
are deduplicated for 90 days. Accepted revisions persist in SQLite, coalesce while
an application is busy, and run after the current operation. A current-head check
suppresses out-of-order stale pushes. Replace queued deployments supersedes only
queued builds, never migrations or application updates. Disabling automatic
deployments discards pending intents without touching the serving release.

Cancel is available while queued, cloning, pulling or building. Once application
updates or lifecycle commands begin, cancellation is rejected. A cancelled build
leaves the serving release intact. Retry uses the recorded commit or image digest;
if no reproducible revision exists, use Redeploy to resolve the source again.
Runtime-only Apply reuses the serving image, including for mutable image tags.

## Images and inline Dockerfiles

Choose Container image to deploy without Git. Docker Hub, GHCR and Quay are allowed
as public registries. Add a registry connection for authentication or another host;
credentials are sent only when the image's registry matches that connection.
Private registry addresses require explicit opt-in and Docker's normal TLS policy
still applies. Platform defaults to the server, with Linux amd64/arm64 overrides.
The image ID and available repository digest are recorded in deployment history.
Redeploy resolves the tag again. Inline Dockerfile uses an encrypted source editor
and the same isolated BuildKit build path, without a repository context.

These workflows retain ordinary runtime ports, process/HTTP readiness, variables,
volumes, lifecycle commands and domains. Images must support the configured host
architecture. Registry publishing, GitLab/SSH providers, submodules, Git LFS,
private Git hosts and isolated PR previews remain later parity work.

## Deploy tokens

Settings > Deploy token creates or rotates an application-scoped token, displayed
once. Only its hash is stored. Revoke invalidates it immediately. Use it as a
Bearer credential with `POST /api/v1/applications/{id}/deploy` and an empty JSON
body, or `{ "revision": "<full commit SHA>" }`. The token cannot read secrets,
change configuration, execute commands, or deploy another application.
An optional Idempotency-Key prevents duplicate requests. Keep tokens out of URLs.

## Verification boundary

Protocol tests cover GitHub authorization, JWT signatures, repository scoping,
credential rotation, signed deliveries, filtering, deduplication and deploy tokens.
Isolated Docker tests pull a pinned image, build an inline image, check readiness
and restart both. Browser tests exercise repository selection and queue controls.
Live GitHub App installation/private clone and public webhook delivery require a
real account and reachable endpoint; they are not claimed as locally verified.

## GitHub organizations

In **Settings > Connections > Connect GitHub**, choose **Organization** and enter
its GitHub organization name. Continue to GitHub to register the private app under
that organization, then install it for all repositories or only selected ones.
Both public and private repositories granted to the installation are available in
New application. Create a separate connection for each organization or personal
account. The default Personal account path remains available.

GitHub requires permission to register and install apps in the organization; its
owners may need to complete or approve setup. Shipper requests only Contents and
Metadata read access and push events, not organization administration permissions.
Existing GitHub App and token connections remain supported. For fine-grained
tokens, select the organization as resource owner and grant repository Contents
read access; organization policy and SSO authorization still apply.
