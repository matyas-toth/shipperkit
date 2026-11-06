---
version: 1.0.0-rc.6
name: Shipper console
description: Dark operational console for self-hosted deployments, Compose services, release history and instance settings.
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
rounded:
  control: 7px
  card: 10px
  panel: 12px
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

## Settings

Settings has two sidebar groups. The grouping is part of the information model,
not a visual decoration.

### Account settings

- **Account**: email, sign out, personal two-factor authentication and recovery
  codes. Personal controls never include instance-wide policy.

### Server settings

- **Instance**: Docker connection, engine, host, storage and Shipper runtime.
- **Dashboard**: the canonical Shipper domain, HTTPS status, certificate
  renewal and dashboard verification.
- **Connections**: GitHub and registry connections managed for the instance.
- **Recovery**: instance recovery plans, destinations and retained volumes.
- **Access policy**: instance-wide two-factor requirement.

Group headings are 11 px, muted and prefixed with a small icon. Keep 22 px
between the two groups and 3 px between links inside a group. Do not add helper
text below the headings. On narrow screens, hide the group headings and render
the links as one horizontal scroll strip.

The dashboard domain is an instance setting. Saving it posts the new hostname,
adds the dashboard route to the global proxy, and requests a trusted certificate.
The initial installer exposes the control plane at `http://<public-ip>:8080` so
the administrator can claim the instance before DNS or HTTPS exists. After the
domain is saved, the canonical HTTPS origin becomes the normal management path.

## Application workspace

- The application header carries name, state and the primary deploy action.
  Release identity, repository facts and domains stay close to the header.
- Overview uses a compact fact strip, service or container summary and separate
  diagnostics. Runtime health and route reachability are distinct.
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
- Empty states are short factual statements, not illustrations or marketing copy.

## Forms and actions

- Labels sit above fields. Placeholder text is not a label.
- Standard controls are 40 px high. Small controls are 30 px. Keep the same
  radius across buttons, inputs, selects and textareas.
- Use one primary action in a form. Put secondary and destructive actions in the
  same action row, aligned to the trailing edge.
- Disabled controls describe a real unavailable state. Never disable a control
  because a background poll is in flight. Track the explicit action's own busy
  state instead.
- Destructive actions use the danger treatment and name the target in the
  confirmation. Do not use a generic "Are you sure".
- Configuration edits use Review changes and one review for the entire draft.
  Reviews list field names and effects without repeating secret values.

## Diagnostics and failure surfaces

- Diagnostics use neutral cards with one uniform border. A healthy card may
  carry a green check icon and success text; a failed card may carry a red icon,
  muted red border and a clear action. The left edge is never thicker.
- Show the failed stage, the serving release and the next useful action together.
- Keep evidence and activity available in disclosures, not stacked on the
  primary summary.
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
  way that implies an exact completion time.

## Responsive behavior

- At widths below 48 rem, use 16 px gutters, 40 px controls and one content
  column. Application tabs and settings links become scroll strips.
- Release history keeps its card height and scrolls; do not compress cards into
  a fixed-height stack.
- Tables either keep their dense structure and scroll horizontally, or regroup
  cells while preserving the row's identity and action. Never hide the state.
- Settings group headings disappear on mobile, but the Account and Server order
  remains.

## Implementation contract

- Use the existing page, card, table, toolbar, settings, status and action
  classes before adding new ones.
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
