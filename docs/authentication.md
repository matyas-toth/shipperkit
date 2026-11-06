# Authentication and account provisioning

The 1.0 release candidate supports a single owner, password sessions, personal
TOTP, instance-wide TOTP enforcement, one-use recovery codes and offline owner
recovery. Managed installations claim the first owner through `shipperctl claim`.
The multi-user provisioning and Google, GitHub and Discord sign-in below are the
planned P11 contract. GitHub repository access is a separate integration.
See [installation and recovery](installation.md), the [roadmap](roadmap.md) and
the [security boundary](../SECURITY.md).

## Accounts are provisioned by administrators

There is no public signup, self-invitation, domain-wide admission or OAuth
just-in-time account creation. The private first-admin setup capability is a
single-use installation bootstrap, not an ongoing registration endpoint.

Admins add people in Settings > Users, choosing email, role and an optional
temporary password. Provisioning does not require an invitation-email service.

| Provisioning choice | Result |
| --- | --- |
| Email and temporary password | Store a password hash and require a different password before full app access after initial authentication. The admin delivers the temporary password separately. |
| Email with no password | Store no password credential. Login requires an enabled supported provider proving the provisioned email. |
| Password added/reset by an admin later | Mark the replacement temporary and require a change again. Do not reveal an old password. |
| Account disabled/removed | Deny every login method and revoke sessions and personal access credentials. |

An outstanding temporary-password gate cannot be bypassed by choosing OAuth.
Replacing or deliberately removing that credential through a verified/admin
operation clears the gate. Blank never means a valid empty password, nor an
invitation to create one through an unauthenticated reset endpoint.

Email + Password, Google, GitHub and Discord are independent instance toggles.
A method must be both enabled/configured and usable by the account. An OAuth-only
account is not given a password when password login is enabled globally.
Disabling password login disables it for everyone through ordinary sign-in.

Prevent changes that leave the last active administrator with no working login
and recovery route. Bootstrap starts with password login available. Provider and
account changes are audited and never assign roles implicitly. GitHub repository
connections remain entirely separate from signing in to Shipper.

## One internal identity across providers

The experience is one account, role, preferences and 2FA setup regardless of the
enabled provider used. A verified provisioned email can link a new provider
without asking the admin to create duplicate users.

The primary key is an immutable Shipper user ID. Email is a controlled admission/
linking attribute, not the permanent provider identifier:

1. Validate the authorization flow and retrieve identity server-side.
2. Resolve an already linked provider's stable user/subject ID to the existing
   active account. Never move it to another account because its email changed.
3. For a new identity, require a verified matching email and exactly one active
   admin-provisioned account. Use consistent documented normalization; do not
   strip plus aliases/Gmail dots, guess from usernames or fuzzy-match.
4. Complete password-change, enrollment and 2FA gates before finalizing the
   identity link and issuing full application credentials.
5. Persist a unique provider/issuer + stable subject/user ID binding. Reject
   collisions and ambiguous mappings without a silent merge.

Google uses validated `sub` and `email_verified` claims. GitHub uses its stable
user ID and the authenticated Emails API's verified addresses, including private
email, rather than the public profile email alone. Discord uses its stable user
ID and verified email. Selection must be deterministic: do not choose whichever
of several GitHub addresses happens to match a more privileged Shipper account;
confirm the matching address and resolve ambiguity explicitly.
[Google claims](https://developers.google.com/identity/openid-connect/openid-connect),
[GitHub email API](https://docs.github.com/en/rest/users/emails),
[Discord identity](https://docs.discord.com/developers/resources/user).

Google documents that a third-party email can remain marked verified when Google
is not its current mailbox authority. In such cases, establish fresh mailbox
control or use an explicit authenticated administrator binding before initial
linking. A challenge confirms a pre-provisioned account; it never creates one.
If proof cannot be established, fail clearly.
[Google verification guidance](https://developers.google.com/identity/gsi/web/guides/verify-google-id-token).

Changing a Shipper email or identity link requires recent authentication,
applicable 2FA and audit. It must not silently rebind identities or reactivate a
removed account. Account revocation applies across every provider.

## TOTP without administrator configuration

A user selects Set up 2FA under **Settings → Account**. Shipper generates a
cryptographically random per-user secret and authenticator QR/manual setup value,
with issuer/service name `Shipper` and the email plus instance hostname in the
account label, so entries from different VPSs are distinguishable. No external
client registration or authenticator service is required.

Enrollment remains pending until a correct code is submitted. Only then activate
the factor and provide one-time recovery codes. Encrypt TOTP secrets, hash
recovery codes and reveal setup material only through protected enrollment.
Never put secrets/codes in ordinary responses, logs or browser storage. Use a
standard compatible TOTP profile, bounded clock tolerance, rate limits, expiring
challenges and replay prevention. Include clock checks in host provisioning.

After every interactive password or provider authentication, enrolled users
complete Shipper's own second-factor screen. Provider MFA claims do not skip it.
Recovery codes are a deliberate alternate factor and are consumed atomically.
Do not add a remembered-device bypass at this stage.

When enforcement is off, a user may disable personal 2FA after recent first- and
second-factor verification. Replacing factors or recovery codes requires
equivalent protection and invalidates stale challenges/credentials as appropriate.

## Require 2FA for the instance

The admin's **Settings → Access policy** Require 2FA setting includes the admin,
existing signed-in users without a factor, fresh password accounts and fresh
OAuth-only accounts.

| State | Allowed next action |
| --- | --- |
| Not authenticated | Perform an enabled first-factor flow. |
| Temporary password outstanding | Change it in a restricted onboarding session. |
| 2FA required without a confirmed factor | Set up/confirm 2FA, sign out or use defined recovery. |
| Confirmed factor needs verification | Submit TOTP or a recovery code. |
| All requirements satisfied | Receive full application access under the account's role. |

These are server-side assurance states. Incomplete sessions cannot access app,
configuration, secrets, terminal, administration APIs or ordinary SSE streams.
Re-evaluate existing sessions on enforcement changes so old cookies/open tabs
cannot bypass enrollment. Sessions already satisfying the policy need not enroll
again. Successful enrollment confirmation satisfies the factor for that login.

Users cannot disable their last required factor under enforcement. Turning off
enforcement makes factors optional without silently removing them. Admin factor
resets revoke affected credentials and require enrollment, never grant full access.

Personal API credentials cannot bypass required onboarding/enrollment.
Noninteractive application/service automation identities remain separately scoped,
audited machine capabilities. A push/deploy token never becomes a human session
or authentication-administration capability. Define their policy explicitly
without stopping running applications when login policy changes.

Keep an audited offline owner recovery path for lost factors/provider failure.
It must not become a public signup, blank-password or emailed-OTP backdoor.
Restoring Shipper retains identity links, factor/enforcement state and original
encryption material.

## Provider setup guidance in Settings

Admins enter client IDs/secrets, with encrypted storage, masking and deliberate
rotation. Show a copyable callback derived from the canonical HTTPS dashboard
origin, provider-specific instructions and a connection test. Origin changes
require updated provider callbacks. Never derive them from untrusted headers.

These paths are proposed, not existing endpoints. For a dashboard at
`https://shipper.example.com`, register the exact displayed URL:

| Provider | Proposed callback | Identity scopes |
| --- | --- | --- |
| Google | `https://shipper.example.com/auth/oauth/google/callback` | `openid email profile` |
| GitHub | `https://shipper.example.com/auth/oauth/github/callback` | `read:user user:email` |
| Discord | `https://shipper.example.com/auth/oauth/discord/callback` | `identify email` |

### Google

In Google Auth Platform/Cloud Console, select a project, configure branding and
audience, and create a Web application OAuth client. Add the displayed callback
to Authorized redirect URIs and copy the client ID/secret to Shipper. Allow the
intended test users while testing; choose the appropriate publication/audience
configuration for actual collaborators. Shipper validates signature, issuer,
audience, expiry, nonce and verified-email policy server-side.
[Google web OAuth setup](https://developers.google.com/identity/protocols/oauth2/web-server),
[OIDC configuration](https://developers.google.com/identity/openid-connect/openid-connect).

### GitHub

Create an OAuth App in Settings > Developer settings > OAuth apps. Use the
dashboard origin as Homepage URL and the displayed GitHub callback as Authorization
callback URL. Disable wildcard matching where offered. Copy the client ID and
generated secret into Shipper. Request identity/email access only; repository
permissions are unnecessary. Read verified/private email through the authenticated
email endpoint. Deployment callbacks under `/connect/github/...` are separate.
[GitHub OAuth App setup](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/creating-an-oauth-app),
[Email access](https://docs.github.com/en/rest/users/emails).

### Discord

In the Discord Developer Portal, create/select an application, add the displayed
callback under OAuth2 Redirects, and copy the client ID/secret to Shipper. Use
authorization code flow with `identify email` and validate identity/email
server-side. Sign-in needs no bot installation or guild/messaging permissions.
[Discord OAuth2](https://docs.discord.com/developers/topics/oauth2),
[User/email fields](https://docs.discord.com/developers/resources/user).

Recheck provider-console requirements at implementation. All flows use expiring
one-use state, PKCE where supported, exact registered redirects, server-side
exchange and identity retrieval. A provider success does not grant Shipper
admission, roles, completed onboarding or satisfaction of Shipper's 2FA policy.

## Acceptance

- Password-only and OAuth-only provisioned accounts, temporary-password changes,
  disabled methods/accounts and denial of unknown emails.
- All methods resolve to one account/role; test GitHub private email,
  missing/unverified email, collisions and changed provider addresses.
- Non-authoritative claims, state/code replay, callback mismatch, secret rotation,
  provider removal and failure produce safe, actionable outcomes.
- TOTP follows every enrolled password/provider login; test confirmation,
  invalid/expired/reused codes, recovery and factor replacement.
- Enabling enforcement gates existing admin/user tabs, streams and personal API
  credentials; fresh OAuth-only users enroll and enforced factors cannot be removed.
- Verify last-admin access protection, offline recovery and restored identity/
  factor/enforcement state.
