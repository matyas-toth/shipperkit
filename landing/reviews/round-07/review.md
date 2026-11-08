# Round 06 independent review

Reviewer: DeepSeek V4.1 Flash max, fresh context.

Screenshots: `ours-fold.png`, `ours-full.png`, `ours-1024-*.png`, `ours-390-*.png`, plus desktop captures of `coolify`, `vercel`, and `t3code`.

## Raw scores

| Page | Typography | Hierarchy | Spacing and density | Color and contrast | Iconography | Consistency and polish | Clarity | Conversion | Interesting-ness | Overall |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| **Shipper (ours)** | 8.2 | 8.4 | 7.8 | 8.5 | 8.0 | 7.5 | 8.7 | 8.0 | 7.8 | **8.1** |
| Coolify | 7.0 | 7.0 | 6.5 | 7.0 | 6.6 | 6.7 | 8.9 | 8.3 | 7.4 | **7.3** |
| Vercel | 9.4 | 9.0 | 8.7 | 9.5 | 8.6 | 9.0 | 8.6 | 8.4 | 9.2 | **8.9** |
| T3 Code | 9.0 | 8.8 | 8.4 | 8.7 | 9.0 | 8.8 | 9.0 | 8.9 | 9.0 | **8.8** |

## Shipper strengths

- Hero is complete: proposition, supporting sentence, section link, primary CTA, open-source proof, install command, and product screenshot.
- Product evidence is specific: applications, statuses, domains, deploy methods, release history, diagnostics, real build logs.
- The mobile console reflow is the best responsive decision on the page.
- Guardrails and comparison tables answer two different objections well.
- No eyebrows, helper-text stacks, or badge clutter.

## Shipper weaknesses

- Mobile comparison table runs off the right edge; only label, Shipper, and part of Managed PaaS visible; Hand-rolled off-canvas.
- Install command wraps into three visual lines at 390; Copy becomes a large 48px control; hero uses dark Copy while close uses white Copy.
- Section-level Install buttons place a dark rounded square inside a white pill; nested radii read as a split button.
- FAQ heading appears smaller; 3-column FAQ becomes dense at 1024.
- Unglamorous screenshots cut through rows at 1440, stack and clip at 1024, disappear at 390.
- Failure-status tiles rely on color alone.
- No traction signal near the first CTA.
- Product mockup microtext is faint at 10-11px.

## Five highest-impact changes

1. Make the mobile comparison table fully operable (all columns visible or clear scroll affordance).
2. Rebuild the install block as one consistent system (single-line desktop, scrollable mobile, Copy secondary and consistent between hero and close).
3. Resolve the nested chevron in the primary CTA (single target or matched radii).
4. Unify the section heading system and FAQ interaction.
5. Repair product screenshot crops and the mobile feature trio (boundary/fade, one screenshot at 1024, icon-left rows at 390).

## Comparison

- Surface elevation: ours has three nested levels and feels literal; T3 is more graphic; Vercel is completely flat with whitespace.
- Monochrome: ours strict warm monochrome with status colors; T3 adds blue action and brand tiles; Vercel the strictest.
- Hierarchy: strong first screen, then a flatter middle with many equal cards.
- Eye comfort: good body contrast; fatigue from the 8,651px scroll and 10-11px product microtext.

## Responsive findings

- 1024: hero holds; unglamorous section clips both stacked screenshots; table fits; FAQ 3-up becomes narrow.
- 390: header fits but buttons are 36px; hero command wraps 3 lines; comparison table overflows; operations proof removed; FAQ accordion is the best mobile pattern; footer holds.

## Verdict

Shipper is the strongest-designed page in this set after Vercel and T3 Code. The middle loses ground: hierarchy flattens, the FAQ becomes a dense wall, and the repeated CTA motif has unresolved nested radii. The mobile comparison table, wrapped install command, and vanished screenshot proof are the concrete execution gaps.
