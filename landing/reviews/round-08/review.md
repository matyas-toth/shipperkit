# Round 07 independent review

Reviewer: DeepSeek V4.1 Flash max, fresh context.

Screenshots: `ours-fold.png`, `ours-full.png`, `ours-1024-*.png`, `ours-390-*.png`, plus desktop captures of `coolify`, `vercel`, and `t3code`.

## Raw scores

| Category | Shipper (ours) | T3 Code | Vercel | Coolify |
|---|---:|---:|---:|---:|
| Typography | 8.5 | 8.5 | 8.8 | 7.7 |
| Hierarchy | 8.2 | 8.6 | 8.6 | 8.0 |
| Spacing and density | 8.2 | 8.3 | 9.0 | 7.2 |
| Color and contrast | 8.4 | 8.4 | 9.0 | 7.4 |
| Iconography | 8.3 | 8.7 | 7.8 | 7.6 |
| Consistency and polish | 7.8 | 8.4 | 8.7 | 7.3 |
| Clarity | 8.9 | 8.7 | 8.2 | 8.6 |
| Conversion | 7.5 | 8.5 | 8.4 | 8.4 |
| Interesting-ness | 7.5 | 8.9 | 9.0 | 8.0 |
| **Overall** | **8.1** | **8.6** | **8.6** | **7.8** |

## Shipper strengths

- 96px hero headline with controlled three-line breaks and warm off-white.
- Product proof is real: ten applications with mixed states, realistic domains/methods/timestamps, failed release with logs.
- Warm monochrome is calm, technical, expensive; status colors are semantic.
- Narrative is complete: sources, guardrails, HTTPS, backups, health, cost, FAQ.
- Responsive type holds 96 to 60 to 40; the mobile app preview is purpose-built.
- Mobile FAQ accordion is a strong adaptation.

## Shipper weaknesses

- Desktop hero unbalanced: 380px support column ends around y=300; left block runs to y=500; command spans 1,156px at y=520.
- Command bar clips the URL mid-token at 390 and the copy button appears to overlap the text.
- "Unglamorous" screenshots cut through rows at 1440; at 1024 one is clipped to ~210px with an empty right half.
- Surface separation too subtle (cards near #0F1010 against #050606).
- No stars/users/testimonials; only MIT and GitHub.
- FAQ cramped at 1024 (three 300px columns, uneven rows).
- Full-width mobile primary button keeps content at the left edge, leaving ~200px empty.
- Mobile page length equals desktop despite removed screenshots (8,720px).
- One section template repeats across five sections.

## Five highest-impact changes

1. Consolidate the fold into one primary action and one install module; move the support paragraph left; fill the right quadrant with the command and the three facts.
2. Repair the command bar across breakpoints (two-row or scroll with inset button and fade); add a visible Copied state.
3. Give dark surfaces a usable elevation scale and fix screenshot framing.
4. Rebuild the proof-heavy sections for 1024/390; don't drop one console and clip the other.
5. Vary section rhythm and tighten interaction language; correct the external-link arrow for an in-page jump; 44px mobile menu.

## Comparison

- Surface elevation: ours low/flat; T3 highest; Vercel minimal with scale and placement.
- Monochrome: Vercel most rigorous; ours most distinctive warm cream; T3 expressive with blue.
- Hierarchy: T3 action-first; Vercel focal-first; ours information-first but three actions compete.
- Eye comfort: ours calmest for long dark reading; faint surfaces and 10-11px console labels cause effort.

## Responsive findings

- 1024: hero lopsided; source grid holds; guardrails two-up; unglamorous section breaks (one clipped window, empty right half); table comfortable; FAQ densest block.
- 390: header fits with no headroom; button content left with empty surface; command bar clips/overlaps; app preview adapts well; failed-deploy screenshot removed; four-column tax table compressed to 11-12px; FAQ accordion best adaptation; page 8,720px.

## Verdict

Ours is the most complete and calm page, with the best single proof asset. It is not the most memorable: T3 has more personality, proof, and a clearer action; Vercel has a stronger focal gesture. The gap is execution, not concept. The page reads as a well-crafted 8.1.
