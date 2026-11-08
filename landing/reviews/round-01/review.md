# Round 01 independent review

Reviewer: DeepSeek V4.1 Flash max, fresh context.

Screenshots: `ours-fold.png`, `ours-full.png`, `coolify-fold.png`, `coolify-full.png`, `vercel-fold.png`, `vercel-full.png`, `t3code-fold.png`, `t3code-full.png`.

## Raw scores

| Category | Shipper (ours) | T3 Code | Vercel | Coolify |
|---|---:|---:|---:|---:|
| Typography | 8.5 | 9.1 | 8.8 | 8.0 |
| Hierarchy | 7.4 | 8.9 | 7.6 | 6.8 |
| Spacing and density | 6.5 | 8.5 | 7.0 | 7.0 |
| Color and contrast | 7.6 | 9.0 | 9.3 | 7.7 |
| Iconography | 7.7 | 9.1 | 6.6 | 8.2 |
| Consistency and polish | 6.8 | 8.7 | 8.4 | 7.4 |
| Clarity | 8.3 | 9.0 | 6.5 | 7.8 |
| Conversion | 7.3 | 8.4 | 6.9 | 7.3 |
| Interesting-ness | 8.0 | 8.9 | 9.1 | 7.5 |
| **Overall** | **7.5** | **8.8** | **7.7** | **7.4** |

## Shipper strengths

- The hero is the best piece of landing composition here: a specific before/after promise, an economic reason to care about the VPS, and a clean primary/secondary CTA pair.
- The product console is bespoke and credible. Running, Degraded, Failed, and Stopped tell a reliability story without a diagram.
- The page covers the right objections: no YAML, no SSH, failed deploy safety, source options, agent permissions, HTTPS, backups, health, cost comparison, and a practical FAQ.
- The source grid, guardrails table, comparison table, and FAQ grid are clean systems with equal-height cells and consistent rules.
- The final copyable install command is the correct close for a self-hosted developer tool.

## Shipper weaknesses

- Two large browser frames in "The unglamorous parts, handled." render empty, adding roughly 400px of dead space and removing the visual proof the section promises.
- The floating cards cover product content. "One command / One deploy / One screen" hides table rows and the footer; "Failed stage" covers the release history; "Still serving" bleeds outside the frame.
- The page is 10,066px tall. It asks for a long scroll before the final action and uses several near-empty transitions.
- There is no social proof.
- The two-tone headings fade the meaningful half of the sentence, the icons are too dim, the guardrail cards are misaligned, and the nav and hero install buttons use different arrow treatments.
- "Landing lab" in the bottom-right footer reads as an internal build label.

## Five highest-impact changes

1. Eliminate the dead media frames.
2. Move the annotations out of the product UI.
3. Rebuild the vertical rhythm and remove dead space.
4. Resolve surface and monochrome contrast: raise card surface, strengthen borders, give icons more contrast or a small tile, stop fading the second half of every section heading. Reserve the two-tone treatment for one closing moment.
5. Make the CTA path a system. Use the same arrow treatment in nav and hero. Add a compact install affordance after the guardrails or unglamorous section where a skeptical reader is most likely to be convinced. Remove or align the detached CTA under the source grid.

## Comparison notes

- Ours has stronger hero copy than T3 and Vercel, but lower execution discipline.
- T3 earns its premium feel through layered surfaces and a single accent.
- Vercel earns it through absolute tonal clarity.
- Ours needs fewer places where gray, borders, and overlays do work that contrast and spacing should do.
- The most valuable direction is T3: keep the near-black canvas and warm off-white, but make the layers and hierarchy crisp.

## Responsive risks

- The two-column hero may squeeze at 1024.
- The three-column hero overlay will not fit at 768.
- Absolute annotations collide at narrower widths.
- The final install command will wrap at 768; keep Copy on its own row.
- Reference sites also have responsive risks, but no mobile captures were provided.

## Verdict

Shipper is the most authored dark page after Vercel and has the clearest hero copy, but it is not yet at T3's level as a landing page because its execution discipline is lower: dead media frames, annotations that cover the product, a 10,000px page, no third-party proof, and a monochrome system that reaches for gray where it needs contrast.
