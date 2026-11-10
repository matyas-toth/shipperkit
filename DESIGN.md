---
version: 1.0.0-rc.23
name: Shipper console
description: Dark operational console for self-hosted deployments, collaboration, provider sign-in and instance settings.
colors:
  background: "#0f1010"
  deep: "#090a0a"
  shell: "#070808"
  surface: "#161717"
  layer-02: "#1f2020"
  layer-03: "#292a2a"
  border: "#2a2b2b"
  border-muted: "#212222"
  text: "#f1f1ee"
  secondary: "#b6b6b1"
  muted: "#9f9f9a"
  accent: "#f2f2f2"
  accent-fill: "#232323"
  accent-border: "#3c3c3c"
  success: "#7fdaa6"
  warning: "#e8c07d"
  danger: "#f08c90"
typography:
  sans: Inter Variable
  mono: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace
  body:
    fontSize: 14px
    fontWeight: 450
    lineHeight: 1.45
  page-title:
    fontSize: 30px
    fontWeight: 620
    letterSpacing: -0.8px
  section-title:
    fontSize: 18px
    fontWeight: 600
    letterSpacing: -0.24px
  subsection:
    fontSize: 15px
    fontWeight: 590
  caption:
    fontSize: 12px
  auth-title:
    fontSize: 27px
    fontWeight: 540
    lineHeight: 32px
    letterSpacing: -0.7px
rounded:
  control: 7px
  card: 10px
  panel: 12px
  auth-card: 16px
spacing:
  base: 4px
  gutter: 32px
  mobile-gutter: 16px
  control-height: 40px
  workspace: 1160px
  editor-measure: 960px
  settings-rail: 210px
  settings-content: 800px
components:
  shell:
    topbar-height: 54px
    frame-padding: 0 6px 6px
    page-surface: "#0f0f0f"
    page-border: "#222423"
    page-radius: 12px 12px 10px 10px
  settings:
    layout: 210px rail plus 800px content
    group-gap: 22px
    group-heading-size: 11px
    link-height: 38px
  controls:
    height: 40px
    small-height: 30px
    radius: 7px
  tables:
    header-height: 38px
    row-height: 48px
    row-separator: "#262626"
  release-history:
    rail-max-height: 500px
    card-min-height: 64px
    card-shrink: 0
  auth:
    desktop-split: 1.08fr 0.92fr
    card-width: 400px
    card-padding: 30px
    provider-height: 44px
    video-min-width: 64rem
  users:
    table-min-width: 700px
    row-height: 60px
    role-width: 112px
    action-width: 56px
  providers:
    card-radius: 6px
    control-height: 22px
    field-max-width: 100%
---

# Shipper design system

Shipper is an operations console, not a marketing surface. Every screen should
answer one practical question quickly: what is running, what changed, what needs
attention, or what will happen if I continue. The interface is deliberately
dense, quiet and predictable. `web/src/styles.css` is the implementation source
of truth; this document explains the intent behind it.

## Principles

- Lead with the object and its state. Applications, releases, services, volumes
  and commands are the primary content.
- Use one clear action per surface. Destructive actions stay visually separate
  and require a confirmation that names the target.
- Keep operational facts visible without opening another page. Avoid decorative
  summaries that repeat the same value in a larger font.
- Prefer stable geometry. Live polling and background reconciliation must not
  resize controls, reorder rows or move the user's focus.
- Use color for state, not decoration. Neutral surfaces carry the layout; green,
  amber and red appear on status dots, icons, labels and focused confirmations.
- Cut taglines, eyebrows, helper text and secondary labels. If a label needs an
  explanation to make sense, fix the hierarchy instead.
- No staccato copy, no em-dashes and no filler. Labels are short verbs or nouns.
- Perceived speed is a requirement. Keep cached content visible during refresh,
  use local optimistic states only where they can be truthful, and reserve
  disabled controls for actions that are truly unavailable.
- Authentication is a threshold, not a second product. Sign in, first-admin setup
  and onboarding use the same shell, panel and control grammar as the console.
- Security settings must show evidence before asking for action: what is
  configured, what is masked, what the next test proves and what will be revoked.
- Collaboration screens name the person, role and consequence. Do not hide
  privileged changes behind generic account labels.

## Visual language

The console uses a near-black shell around charcoal work surfaces. Borders are
thin and uniform. There is no decorative accent stripe on one side of a card,
row, log line or selection. State is communicated by the status component, icon
or text, never by a thicker left edge.

### Color roles

| Token | Use |
| --- | --- |
| `--deep` / shell `#070808` | Exposed application frame and topbar |
| `--background` | Page background inside the scrolling surface |
| `--surface` | Tables, cards, settings groups and selected rows |
| `--layer-02` | Inputs, nested trays and hover surfaces |
| `--border` / `--border-muted` | Dividers and card outlines |
| `--text` | Primary values, commands and page titles |
| `--secondary` | Navigation and ordinary supporting text |
| `--muted` | Timestamps, metadata and inactive labels |
| `--success` | Healthy state, completed operations and check icons |
| `--warning` | Degraded state, overdue work and attention without failure |
| `--danger` | Failed state, destructive actions and validation errors |

### Typography

- Use Inter Variable for all interface text. Use the system monospace stack for
  release IDs, hashes, commands, container names, paths and log excerpts.
- Page titles are 30 px and decisive. Section titles are 18 px. Subsections are
  15 px. Body text is 14 px with 1.45 line height. Metadata is 12 px.
- Keep weights between 450 and 620. Avoid bold labels and all-caps headings.
- Numbers that are compared in columns use tabular numerals.
- Do not use letter spacing as a substitute for hierarchy.

### Shape and elevation

- Controls use a 7 px radius. Cards, trays and settings groups use 10 px. The
  framed scrolling page uses a 12 px top radius and 10 px bottom radius.
- Ordinary cards use a single 1 px border and no floating shadow. Raised menus,
  drawers and dialogs use the stronger floating and overlay shadows.
- Inner surfaces may use a 0.5 px inset highlight, but avoid stacked borders and
  double rings.
- Inputs are darker than their surrounding card and keep the same radius as
  buttons.

## Shell and navigation

- The topbar is 54 px high with the brand at the left, primary navigation in the
  center-left, and connection, search and account controls at the right.
- Primary navigation uses compact pill links. The active application section is
  shown with a filled surface and the accent text token.
- The scrolling page sits in a bordered, rounded surface inset 6 px from the
  shell. Keep the topbar fixed.
- Content uses a 1160 px workspace, 32 px desktop gutters and 16 px mobile
  gutters. Long forms and editors cap at 960 px.
- Application tabs are horizontal, scroll when needed, and keep 40 px height.
  On narrow screens, keep Overview, Deployments and Logs visible and place the
  remaining sections in the labeled picker.
- Below 48 rem, the top tabs are replaced by a floating bottom navigation bar
  with Applications, System and Settings plus search. Non-admin roles remove
  System and use a two-column pill. The bar stays clear of form action trays.

## Authentication and onboarding

Authentication shares the main shell. The topbar keeps the Shipper brand, and
the content below it is the same rounded, shadowed application panel used by the
console. It should not look like a separate identity product.

### Split panel

- At 64 rem and wider, the panel uses two columns. The left column carries a
  muted looping cloud film; the right column centers the account card.
- The film is cropped to fill, desaturated and darkened toward the right edge so
  the card side stays quiet. It never carries text, badges or calls to action.
- Below 64 rem, the film is removed from the page and not downloaded. The card
  centers in the single column with a 24 px mobile padding.
- Reduced-motion visitors keep the poster frame without autoplay. Video is muted,
  looping, inline and non-interactive.

### Account card

- The card caps at 400 px and uses a 16 px radius, 30 px padding, a dark vertical
  gradient and a single subtle top highlight. It is the only raised surface on
  the auth page.
- A 40 px icon tile leads the card. The title is 27 px, followed by one short
  lede sentence. The icon changes with the step: key for sign in, rocket for
  first-admin setup, lock for password replacement, shield for factor checks and
  key for recovery codes.
- Copy is specific to the moment: Welcome back aboard, Begin the journey, Make it
  yours, Add a second lock, One more check, Keep these somewhere safe.
- Inputs use a leading field icon, 42 px height and the same radius as buttons.
  Labels stay above fields. Password fields add a trailing reveal control.
- Provider buttons are full-width rows with a brand icon tile, the provider name
  and a trailing chevron. A divider with a single uppercase `OR` separates them
  from email and password.
- The primary action is full width with a trailing forward chevron. Secondary
  actions become quiet text buttons below the primary action.
- Sign in ends with a single line: `Need access? Ask your Shipper administrator.`
  First-admin setup has no footer.
- Errors stay inline above the form. Do not use toasts or dialogs for sign-in,
  provider, password or factor failures.

### Restricted onboarding

- Password replacement, factor enrollment, factor verification and recovery-code
  display reuse the same card and shell. They never inherit the application
  navigation.
- Recovery codes are monospace, listed in one column and copied as a single
  block. The primary action is Done after the codes are shown.
- Factor enrollment keeps the QR and setup key inside the card at 160 px.

## Settings

Settings has two sidebar groups. The grouping is part of the information model,
not a visual decoration.

### Account settings

- **Account**: email, sign out, personal two-factor authentication and recovery
  codes. Personal controls never include instance-wide policy.

### Server settings

- **Instance**: Docker connection, engine, host, storage and Shipper runtime.
- **Users**: people, roles, sign-in methods, temporary-password state, account
  actions and per-user activity.
- **Authentication**: Email and password toggle, provider credentials, exact
  callbacks, setup steps and provider tests.
- **Dashboard**: the canonical Shipper domain, HTTPS status, certificate
  renewal and dashboard verification.
- **Connections**: GitHub and registry connections managed for the instance.
- **Updates**: channel, automatic updates and release progress.
- **Recovery**: instance recovery plans, destinations and retained volumes.
- **Access policy**: instance-wide two-factor requirement.

Group headings are 11 px and muted. Links carry a small leading icon. Keep 22 px
between the two groups and 3 px between links inside a group. Do not add helper
text below the headings. On narrow screens, hide the group headings and render
the links as one horizontal scroll strip.

### Users

- The Users page is an operational table, not a profile gallery. Columns are
  User, Role, Sign-in, Last active and a 56 px action column.
- Identity is an avatar initial plus email. Disabled status and Set password
  status sit directly below the email, never in a separate summary card.
- Role is an inline select with Admin, Developer and Viewer. Changing a role is a
  direct server action, not a separate edit page.
- Sign-in is a row of compact chips: Password, Google, GitHub, Discord. A user
  with no credential shows a muted None chip.
- Add user opens a right drawer with email, role and a choice between Temporary
  password and Provider sign-in only. The generated password is monospace with
  generate and copy controls. The success state shows the credential once and
  never reveals it again.
- Reset password reuses the drawer with a generated temporary password. It warns
  by naming the account in the confirmation and revokes existing sessions.
- View activity opens a drawer with a chronological audit list. Action names are
  humanized; provider and role detail follows the action in muted text.
- Destructive account actions use a small confirmation dialog that names the
  account and states the revocation consequence.
- The table switches to fixed layout with 30/18/24/17 percent columns plus the
  action column. On narrow screens it keeps its columns and scrolls horizontally
  rather than dropping role or sign-in state.

### Authentication

- The sign-in methods group starts with a single Email and password row. The
  22 px switch and its Enabled or Disabled label sit together in a centered flex
  group at the trailing edge.
- Providers use full-width cards below a `Providers` subsection heading. The
  heading keeps 28 px above it so it never collides with the method row.
- Provider card anatomy: header with icon tile, name and state; Client ID field;
  masked Client secret with Saved or Not set state; callback row with Copy;
  scopes; a Setup steps disclosure; and a footer with the enable switch, Test
  sign-in, Remove and Save.
- Never render a stored secret. Rotation is a deliberate empty input that only
  replaces the secret when filled.
- The callback is read-only and derived from the canonical HTTPS dashboard
  origin. It is always shown with a copy control, never reconstructed in copy.
- A successful provider test returns a green inline result on Settings. A failed
  test returns an inline error naming the account, callback or provider problem.
- Removing a provider uses the same small confirmation dialog and states that
  linked identities will be removed.

The dashboard domain is an instance setting. Saving it posts the new hostname,
adds the dashboard route to the global proxy, and requests a trusted certificate.
The initial installer exposes the control plane at `http://<public-ip>:8080` so
the administrator can claim the instance before DNS or HTTPS exists. After the
domain is saved, the canonical HTTPS origin becomes the normal management path.

## Application workspace

- Application header carries name, state and the primary deploy action. Release
  identity, repository facts and domains stay close to the header.
- Overview uses a compact fact strip, a container or service summary and one
  issue alert when something needs attention. Activity is the only collapsible
  history block. CPU and memory appear in the container drawer and on Compose
  service cards, not as a separate metrics section.
- Deployments use a left release rail and a right log/detail panel. The rail is a
  scroll region, not a compressed list. Each release card sizes to its content
  and keeps its status, hash, date and duration readable. Never let a flex list
  shrink cards below their text height; use `flex: none` and let the rail scroll.
- Logs stream in the right panel with their own toolbar. Keep raw copy, wrap on
  narrow screens and distinguish recognized warnings and errors.
- Domains, .env, Tasks, Backups and Settings each occupy one focused section.
  Service-specific pages use their own tabs and never duplicate the application
  header.

### Deployment commands

Compose applications use service tabs above the four-stage command list:

- **All services**
- one tab per service in the stack

The selected tab scopes the four stages: Before deploy, Before start, After
deploy and Before stop. All-services commands run first in each service
container, then that service's own command. The stage list and editor share one
workspace: choose a stage on the left, edit its command on the right. Keep the
execution options disclosure inside the editor. Non-Compose applications keep
the four-stage editor without service tabs.

## Tables and status

- Tables are operational inventories. Keep them dense on desktop and reorganize
  rows on narrow screens without losing identity, state, time or action.
- Table trays use a surface fill and a 1 px border. Rows use a single bottom
  divider. Selected rows use a filled background and keep their geometry.
- Do not add a colored bar to the first cell of failed, degraded or selected
  rows. Use the status component and the row background.
- Status combines a dot and a readable label. Never rely on color alone.
- Sign-in chips are compact outlined pills with a 12 px icon and 11 px label. Use
  them for Password, Google, GitHub and Discord. Do not tint a provider chip by
  brand.
- Toggle switches are 38 x 22 px. Pair the switch and its state label in one
  centered flex row with 8 to 10 px space. Never baseline-align a switch against
  text.
- Empty states are short factual statements, not illustrations or marketing copy.

## Forms and actions

- Labels sit above fields. Placeholder text is not a label.
- Standard controls are 40 px high. Small controls are 30 px. Keep the same
  radius across buttons, inputs, selects and textareas.
- Auth and account inputs use 42 px height, a 10 px radius and a leading field
  icon. The icon is decorative; the label remains the accessible name.
- Use one primary action in a form. Put secondary and destructive actions in the
  same action row. On auth cards the primary action is full width and secondary
  actions sit below it.
- Disabled controls describe a real unavailable state. Never disable a control
  because a background poll is in flight. Track the explicit action's own busy
  state instead.
- Account drawers keep the primary action at the bottom of the drawer, not
  immediately after the fields. The form owns the remaining height so the footer
  does not float in a short drawer.
- One-time credentials are shown in monospace with an explicit Copy control.
  Never write them to storage, drafts or ordinary audit entries.
- Destructive actions use the danger treatment and name the target in the
  confirmation. Do not use a generic "Are you sure".
- Configuration edits use Review changes and one review for the entire draft.
  Reviews list field names and effects without repeating secret values.

## Failure surfaces

- Application issues use one alert above the overview facts. The alert names the
  failing layer and offers one next step: inspect the route, open the failed
  release or review deployments.
- Failed deployment logs keep the failed stage and the retry path in view. They
  do not send the user to a separate diagnostics table.
- Activity is a disclosure, not a card. It shows the latest incidents in reverse
  order with a status, summary and time.
- Show the failed stage, the serving release and the next useful action together.
- Container facts stay in the drawer. CPU and memory are measured values, not
  progress bars, and read Not measured when Docker has no sample.
- Recheck and verification actions stay enabled during background refresh. They
  disable only while their own request is running.

## Motion and feedback

- Use 90 ms ease transitions for background, color and outline changes. Avoid
  movement that competes with live data.
- Prefer state replacement over layout movement. Keep list identity stable while
  queries refetch.
- No shimmer placeholders behind already cached content. A loading line is for
  first-load or empty regions only.
- Keep reduced-motion behavior. Do not animate progress or status changes in a
  way that implies an exact completion time. The auth film pauses to its poster;
  provider and card interactions keep only short color and transform feedback.

## Responsive behavior

- At widths below 48 rem, use 16 px gutters, 40 px controls and one content
  column. Application tabs and settings links become scroll strips.
- Release history keeps its card height and scrolls; do not compress cards into
  a fixed-height stack.
- Tables either keep their dense structure and scroll horizontally, or regroup
  cells while preserving the row's identity and action. Never hide the state.
- Settings group headings disappear on mobile, but the Account and Server order
  remains.
- Auth removes the film below 64 rem and centers the card with 24 px padding.
  The card keeps its radius, border and shadow; it does not become a full-bleed
  sheet.
- The Users table keeps its columns and scrolls horizontally on narrow screens.
  The Add user drawer becomes the full viewport width minus 16 px.
- Dropdowns, drawers and confirmations stay inside the viewport and keep 16 px
  safe gutters.

## Reference captures

The current beta surfaces are captured in
[`docs/screenshots/phase-11`](docs/screenshots/phase-11):

- `sign-in.png`, `admin-setup.png`, `password-gate.png` and
  `password-gate-mobile.png` for the authentication shell.
- `users.png`, `add-user.png` and `activity.png` for collaboration settings.
- `authentication.png` for the provider configuration surface.

Treat these as composition references, not pixel-perfect specifications. The
live components and `web/src/styles.css` remain authoritative.

## Implementation contract

- Use the existing page, card, table, toolbar, settings, status and action
  classes before adding new ones.
- Auth uses the shell and panel primitives, then the dedicated auth classes:
  `auth-visual`, `auth-video`, `auth-stage`, `auth-card`, `auth-field`,
  `auth-provider`, `auth-divider`, `auth-badge` and `auth-submit`.
- Users and provider surfaces use the shared settings, table, drawer, status,
  switch and action-menu primitives. Do not build a second settings layout.
- Use the CSS variables for color and borders. Do not hand-pick near-duplicate
  grays in component rules.
- A border on one side must match the border on the other three sides unless it
  is a real divider between two adjacent surfaces.
- For scrollable flex lists, set `flex: none` on cards that must keep their
  content height. The container owns scrolling.
- Use monospace only for values people copy or compare. Do not set whole screens
  in monospace.
- Keep UI copy imperative and terse. If a control needs an explanatory sentence,
  first try a better label and layout.

This document replaces the earlier OpenCode Console reconstruction. The old
reference informed some proportions, but Shipper's product tokens and the rules
above are the current design specification.
