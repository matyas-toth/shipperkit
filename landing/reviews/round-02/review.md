# Round 02 independent review

Reviewer: DeepSeek V4.1 Flash max, fresh context.

Screenshots: `ours-fold.png`, `ours-full.png`, `ours-1024-*.png`, `ours-390-*.png`, plus desktop captures of `coolify`, `vercel`, and `t3code`.

## Raw scores

| Category | Shipper (ours) | Coolify | Vercel | T3 Code |
|---|---:|---:|---:|---:|
| Typography | 8.2 | 7.6 | 8.4 | 8.3 |
| Hierarchy | 8.0 | 7.3 | 7.9 | 8.2 |
| Spacing and density | 7.7 | 7.0 | 8.0 | 8.1 |
| Color and contrast | 7.9 | 7.4 | 8.7 | 8.2 |
| Iconography | 7.3 | 7.2 | 7.8 | 8.3 |
| Consistency and polish | 8.0 | 7.2 | 8.0 | 8.1 |
| Clarity | 8.6 | 8.2 | 7.4 | 8.3 |
| Conversion | 7.2 | 8.3 | 8.6 | 8.7 |
| Interesting-ness | 7.6 | 7.3 | 8.1 | 8.0 |
| **Overall** | **7.9** | **7.5** | **8.1** | **8.3** |

## Shipper strengths

- Fastest explanation of the product in the set: audience, mechanism, and outcome in the hero, followed by a real console.
- The failure story is the best differentiator: release history, failed stage, still-serving release, four checks.
- Disciplined type system and controlled three-line hero break.
- Comparison table and the closing gradient panel are memorable anchors.
- Grids hold at all breakpoints; no overflow.

## Shipper weaknesses

- The fold asks for trust before evidence and the install button is a link, not an install action.
- Hero mock clips its footer at 1440; console legibility collapses at 1024 and 390.
- The page is 9,215px tall versus T3 Code at 4,345px, with repeated section templates.
- "Scoped tokens" floats at the far right of the guardrails intro instead of belonging to a card.
- Mobile: curl URL clipped, stacked hero buttons misaligned, four-column comparison table squeezed, 13,029px scroll.
- Small polish: footer metadata near 4.5:1 contrast and redundant labels.

## Five highest-impact changes

1. Turn installation into the hero interaction (command surface or copy affordance), keep a mobile install action, and surface GitHub/MIT/open-source as a compact proof line.
2. Re-cut console imagery: hero ends cleanly, closer crops at 1024/390, one larger operational screen instead of two unreadable thumbnails.
3. Lift the surface and contrast floor: a third surface step, stronger hairlines, brighter 15px descriptions and metadata, off-white rather than pure-white display type.
4. Remove roughly 15 to 20 percent of vertical run and vary cadence; compact the caption cards, collapse the FAQ on mobile, alternate full-bleed evidence with text-only sections.
5. Repair mobile: stacked comparison rows, unclipped curl block, equal-width stacked buttons, slightly smaller h1 rag, tidy footer columns.

## Direct comparison

- Surface elevation: T3 Code has the most atmospheric depth; Shipper reads flat at full-page scale; Vercel intentionally has none.
- Monochrome: Shipper is the most disciplined and the least action-oriented; a command-chip primary action would add distinction without a new brand color.
- Hierarchy: T3 and Vercel put the primary action on the vertical path from headline to subhead; Shipper moves it to a side rail and repeats it six times.
- Eye comfort: Vercel calmest, T3 close, Shipper harshest due to a large white mass on black plus dense micro-UI.

## Responsive findings

- 1024: hero gutter tightens; the page barely shortens; FAQ rows start on different baselines; operational screenshots ~7px text.
- 390: h1 five lines with "From" alone; no persistent install anchor after the fold; hero buttons misaligned; console unreadable; source grid 1,700px; FAQ card 2,000px; comparison table squeezed four-up; curl URL clipped; footer orphan cell.

## Verdict

Shipper is the clearest explanatory page and the most coherent dark system, but not the strongest landing page. T3 Code wins the first-minute experience; Vercel wins calm and brand confidence. Shipper's gap is fixable: make installation an interaction, make the console evidence readable, lift the surface floor, and remove a fifth of the scroll.
