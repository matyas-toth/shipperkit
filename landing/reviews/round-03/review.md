# Round 03 independent review

Reviewer: DeepSeek V4.1 Flash max, fresh context.

Screenshots: `ours-fold.png`, `ours-full.png`, `ours-1024-*.png`, `ours-390-*.png`, plus desktop captures of `coolify`, `vercel`, and `t3code`.

## Raw scores

| Page | Typography | Hierarchy | Spacing and density | Color and contrast | Iconography | Consistency and polish | Clarity | Conversion | Interesting-ness | Overall |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| Shipper (ours) | 8.3 | 7.8 | 7.4 | 7.8 | 7.7 | 7.9 | 8.4 | 7.8 | 7.3 | **7.8** |
| Coolify | 7.8 | 7.4 | 7.0 | 7.3 | 7.5 | 7.2 | 8.0 | 8.0 | 7.4 | **7.4** |
| Vercel | 9.0 | 8.1 | 8.2 | 9.1 | 7.6 | 8.8 | 6.7 | 7.7 | 9.1 | **8.2** |
| T3 Code | 8.4 | 8.5 | 8.4 | 8.5 | 8.6 | 8.6 | 8.6 | 8.3 | 8.7 | **8.5** |

## Shipper strengths

- Three-line display at 1440 lands the promise before the first scroll.
- Narrative sequence is logical; "A failed deploy is not an outage" is the peak.
- Product-semantic status colors are used consistently.
- Card radius, border, button, code block, and icon tile repeat without drift.
- 1024 grids hold without overflow; the hero right column aligns with the display.

## Shipper weaknesses

- DevOps section is the weakest composition: heading and paragraph left, checks and button floating top-right, wide void.
- Hero install block is 420x140 with an unlabeled 32px copy icon detached in the lower-right; the primary action is the least legible control.
- Page, cards, and screenshot frames sit within a narrow band of near-blacks; screenshots are darker than their frames, reversing elevation.
- Mobile is 10,964px tall and all four console screenshots are unreadable at 342px.
- Most post-hero sections repeat the same heading, paragraph, and gap rhythm.
- No adoption proof; MIT and GitHub are the only trust signals.
- FAQ reads as metadata at 14px; final CTA card fills only its left half.
- "Revoke a token" note looks tappable.

## Five highest-impact changes

1. Recompose mobile product proof: crop to one meaningful slice at a legible scale, remove the second operations screenshot, and cut roughly 900 to 1,300px.
2. Rebuild the hero install action with a visible Copy control and a 44px target.
3. Turn the DevOps section into a two-column proof module with a compact checks panel and the CTA in its footer.
4. Introduce a real surface ladder: raise cards two or three tonal steps, strengthen borders, add inset highlights, lift the screenshots above the page.
5. Consolidate the closing surfaces: focal Shipper column, 15-16px FAQ answers, fill or narrow the final CTA card.

## Comparison

- T3 Code has a readable three-level surface stack; ours has two levels at most.
- T3 reserves blue strictly for action; our green doubles as affirmative mark and status color.
- T3 varies composition per section; ours repeats the heading/paragraph pattern.
- Vercel is the most comfortable; ours requires effort to parse dense 9-11px UI text in near-black frames.

## Responsive findings

- 1024: nav at capacity; DevOps right column starts above the heading line; table quiets below the cards; FAQ answers wrap 5-6 lines.
- 390: header at its limit; hero display 40px with an awkward four-line rag; 32px copy icon; all console screenshots unreadable; source stack ~800px; comparison cards ~1,100px; FAQ accordion works; final code URL fits; footer three columns readable.

## Verdict

Shipper is coherent and well-built with a strong hero and failure narrative, but safer and more generic than T3 Code and Vercel. Desktop craft is close; mobile proof and the shallow surface system are where the gap shows.
