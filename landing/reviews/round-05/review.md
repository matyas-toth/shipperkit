# Round 05 independent review

Reviewer: DeepSeek V4.1 Flash max, fresh context.

Screenshots: `ours-fold.png`, `ours-full.png`, `ours-1024-*.png`, `ours-390-*.png`, plus desktop captures of `coolify`, `vercel`, and `t3code`.

## Raw scores

| Page | Typography | Hierarchy | Spacing and density | Color and contrast | Iconography | Consistency and polish | Clarity | Conversion | Interesting-ness | Overall |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| **Shipper (ours)** | 7.6 | 7.1 | 7.2 | 7.4 | 7.2 | 7.4 | 7.6 | 7.0 | 7.1 | **7.3** |
| Coolify | 7.2 | 7.2 | 7.0 | 7.2 | 7.1 | 6.8 | 7.3 | 7.7 | 7.4 | **7.2** |
| Vercel | 9.1 | 8.9 | 8.8 | 9.0 | 8.4 | 9.0 | 7.9 | 8.3 | 8.6 | **8.7** |
| T3 Code | 8.4 | 8.5 | 8.3 | 8.6 | 8.7 | 8.7 | 8.4 | 8.8 | 8.8 | **8.6** |

## Shipper strengths

- The fold earns attention: 96px three-line display, a white split Install button, a copyable command, and a credible app window by the fold.
- Product UI is the best asset: realistic chrome, coherent statuses, release history, failure log.
- The objection card is well judged; the comparison table is honest with a highlighted Shipper column.
- The responsive repo list is the cleanest module on the page.
- The component system is disciplined: radius, borders, status palette.

## Shipper weaknesses

- Seven 56px section heads make the page read as co-equal chapters; the FAQ is as loud as the differentiator.
- No third-party proof; "Open source · MIT" is a license, not adoption.
- The close is quiet: command, dark Copy, ghost GitHub, MIT. No white install action.
- Action weight changes without a rule: 13px text links, outlined links, split buttons.
- Hero right rail is unresolved: the command sits 96px left of and 15px lower than the paragraph.
- Proof is illegible: 8-10px UI text in the failure window and 5-7px in the operations thumbnails.
- Repo cards are hollow: 150px of surface for one line of copy.
- Mobile: comparison table loses Hand-rolled with no scroll cue; command Copy becomes a full-width bar beneath three wrapped lines.
- Small craft: straight apostrophe, unexplained 90 days pill, two check treatments.

## Five highest-impact changes

1. Re-cut section scale and rhythm: keep the 96px hero and 64px close, take section heads to 40-44px, reserve >56px for the failure story and close.
2. Give the close the fold's conviction: white install action in the final panel, command secondary, GitHub as a quiet link.
3. Repair mobile controls: comparison table with a visible edge cue, inline command Copy.
4. Make proof readable: crop to the relevant region at a legible scale; keep one cropped proof per section on mobile.
5. Recompose the hero right rail: align the command to the paragraph and give See how it works a clear link row.

## Comparison

- Surface elevation: ours is the flattest of the three dark pages; T3 layers a grid, tiles, glows; Vercel uses shadow and overlap on light.
- Monochrome: ours and Vercel are purists; T3 spends one blue on action for wayfinding.
- Hierarchy: Vercel first, T3 second, ours third; ours' 96/64/56 scale collapses after the fold.
- Eye comfort: Vercel easiest; T3 varied; ours most monotonous due to same-value cards and hairlines.

## Responsive findings

- 1024: hero command 136px left of the paragraph; mockup text ~9px; repo cards hollow; table and FAQ hold; no overflow.
- 390: header crowded (logo, 114px pill, 36px hamburger, 12px gaps); hero CTA full width; comparison table cuts Hand-rolled with no cue; failure/operations proof hidden; repo list strong; FAQ accordion strong; page 9,509px.

## Verdict

Shipper has the bones of a strong product page but loses confidence after the hero. Every section speaks at the same volume, proof shrinks below legibility, and the page closes on a copy command instead of an install. The mobile table and command are outright breaks; the rest is a hierarchy and trust problem.
