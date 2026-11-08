# Shipper landing

The conversion surface for Shipper. Astro renders the page, React owns the section
components, coss UI provides the interactive primitives, Hugeicons supplies the icon
set, and Tailwind carries the design system.

The page is built around one job: get a solo developer or small team to install
Shipper on a server they already own. The visual language stays close to the
product console, but the composition is louder: oversized display type, a hero
product shot, hairlines, ASCII texture, and one strong idea per section.

## Run

```sh
cd landing
npm install
npm run dev
```

The dev server uses `http://127.0.0.1:4321`.

```sh
npm run check
npm run build
npm run preview
```

From the repository root, `npm run landing:dev`, `npm run landing:build`, and
`npm run landing:check` are available.

## Routes

| Route | Purpose |
| --- | --- |
| `/` | The final composition, same as `/v/03` |
| `/v/01` to `/v/10` | Ten complete landing pages |
| `/variants` | Page index and the variant matrix |
| `/variants/nav` | Six navigation variants |
| `/variants/hero` | Six hero variants |
| `/variants/proof` | Six outcome and proof variants |
| `/variants/product` | Six console showcase variants |
| `/variants/deploy` | Six how-it-works variants |
| `/variants/agents` | Six agent-access variants |
| `/variants/system` | Six reliability variants |
| `/variants/pricing` | Six ownership and pricing variants |
| `/variants/faq` | Six FAQ variants |
| `/variants/cta` | Six closing variants |
| `/variants/footer` | Six footer variants |

Every page combines one variant from each of the 11 groups. The combination table
lives in `src/data/variants.ts`.

## Page combinations

| Page | Name | Direction |
| --- | --- | --- |
| 01 | Launch | Flagship split hero, outcomes, console bento, three-step deploy, agent path, reliability, and the free self-hosted offer |
| 02 | Split | Dual-column hero, included features, sticky failure proof, one-command install, agent workflow, and a cost breakdown |
| 03 | Cinematic | Final route. Full-bleed product hero, no-DevOps objection handling, no-outage proof, source map, and the framed ownership close |
| 04 | Command | Command-first hero, source strip, release receipts, fast deploy, plain-English agents, and a quiet close |
| 05 | Owned | Ownership-led hero, what-you-keep proof, everyday console, no-stack deploy, scoped agent keys, and a $0 price card |
| 06 | Agent | Agent-first hero, launch checklist, screen gallery, review-before-deploy, no-SSH agent path, and a terminal close |
| 07 | Signal | Agent hero with source proof, everyday console, source matrix, plain-English agents, and an ownership receipt |
| 08 | Receipts | Ownership hero with public receipts, failure proof, release receipts, no-stack deploy, and agent guardrails |
| 09 | Night Shift | Split hero, launch checklist, no-outage proof, review-before-deploy, scoped keys, and a cost breakdown |
| 10 | Open Exit | Command hero, included features, everyday console, one-command install, no-SSH agents, and a quiet close |

## Structure

```text
landing/
  src/
    components/
      nav/Nav.tsx            6 navigation variants
      sections/Hero.tsx      6 hero variants
      sections/Proof.tsx     6 outcome variants
      sections/Product.tsx   6 console variants
      sections/Deploy.tsx    6 how-it-works variants
      sections/Agents.tsx    6 agent-access variants
      sections/System.tsx    6 reliability variants
      sections/Pricing.tsx   6 ownership and pricing variants
      sections/Faq.tsx       6 FAQ variants
      sections/Cta.tsx       6 closing variants
      sections/Footer.tsx    6 footer variants
      ui/                    coss UI primitives
      primitives.tsx         shared frames, commands, metrics, ASCII field
      PageBody.astro         page composition
    data/variants.ts         combinations, labels, variant index
    pages/v/[id].astro       ten landing pages
    pages/variants/*         variant gallery
    styles/global.css        tokens, type scale, texture, motion
  public/
    shots/                   product screenshots
```

## Copy and conversion rules

- The hero states the outcome and the next action. Supporting copy stays under a
  short paragraph.
- Every page repeats one primary action: install Shipper.
- Pricing is explicit: free software, your server, your domain, no card.
- The agent section answers the practical objection: scoped access, revocation, and
  the same checks as a human deploy.
- Reliability answers the failed-deploy objection before the FAQ does.
- FAQ ordering follows the questions that stop an install.
- No fake logos, quotes, customer counts, or benchmarks. Open source and MIT are the
  available trust signals.
- No stack internals in sales copy. Docker, Compose, and Dockerfiles appear only
  because the reader's own project uses them.
- No font below 14px. Display hierarchy does the work, not micro-labels.
- No eyebrows above section headlines, no helper text under CTAs, no decorative
  labels. If a section needs explanation, the headline or layout changes.

## Visual system

- Near-black surfaces, off-white type, signal green states.
- Geist for display, Inter for body, Geist Mono for commands and numbers.
- Hairline grids, browser frames, release rails, sparklines, and topology.
- ASCII fields and a fixed film grain provide texture without another asset request.
- CSS motion only, with reduced-motion fallbacks. Hero content enters once; nothing
  animates continuously except the deploy pulse, the floating deploy card, and the
  source marquee.

## Screenshots

The product shots come from the repository gallery at
`.shipper/ui-gallery/round-16`. To refresh them, run the app gallery capture and
copy the relevant PNGs into `landing/public/shots` under the same names.

Third-party licenses are listed in [THIRD_PARTY_NOTICES.md](./THIRD_PARTY_NOTICES.md).
