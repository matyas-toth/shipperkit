# Shipper landing

The homepage and documentation site for Shipper. Astro renders the page, a few
React islands carry the interactive parts, and Tailwind carries the styles.

The page has one job: get someone to install Shipper on a server they own. It
follows the console's design language and the writing rules in `AGENTS.md`.

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

From the repository root, `npm run landing:dev`, `npm run landing:build`, and `npm run landing:check` are available.

## Documentation site

The Starlight documentation site lives in `docs/` and is served at `/docs`.
The landing build runs the docs build first, then merges the static output into
`dist/docs`.

```sh
npm run dev:docs      # http://127.0.0.1:4322/docs/
npm run build:docs
npm run check:docs
```

- `docs/src/content/docs/` holds the MDX pages, grouped as Get started, Deploy, Operate, Access, and Reference.
- `src/data/docs-navigation.mjs` is the shared sidebar and sitemap source.
- `docs/src/components/` holds the SEO head and site title.
- `docs/src/styles/docs.css` holds the documentation design tokens.
- `docs/public/shots/` holds the live product screenshots.

`npm run build` from this directory builds both surfaces. The landing sitemap
imports the same navigation source, so a new page is added to the sitemap when
it is added to the sidebar.

## Deployment

The site is static Astro output and assumes the canonical production origin
`https://shipperkit.dev`. That origin powers canonical tags, Open Graph URLs and
the sitemap. Use `npm run build` as the build command and `dist` as the output
directory so the documentation site is included at `/docs`.

`/install.sh` must resolve to the generated installer channel. The project ships:

- `public/_redirects` for Netlify and Cloudflare Pages. The trailing `!` forces
  the redirect even when the static fallback file exists.
- `vercel.json` for Vercel.
- `public/install.sh` as a fallback for static hosts that do not process redirect
  configuration. It downloads and executes the same `latest` installer.

Keep `/install.sh` on a `302`, not a long-cached permanent redirect, because the
target branch file changes with every release. `latest` is the Main update
channel, updated by the release workflow after each versioned release, so the
one-liner always resolves to the newest Main installer. The landing itself uses
`https://shipperkit.dev` in every install command.

## Structure

```text
landing/
  src/
    pages/index.astro        the homepage
    components/Home.astro    the page composition
    components/Closing.astro questions, final install section and footer
    components/parts.tsx     static parts: nav, install bar, features, FAQ, footer, copy
    components/widgets.tsx   interactive islands: the screenshot tour and the CLI
    components/Dither.tsx    WebGL dither for the brand footage
    layouts/Base.astro       head, SEO, scroll reveals and copy buttons
    styles/global.css        tokens, type scale, buttons and motion
  public/media/
    aurora.mp4, aurora.webp  dither source and still frame
    shots/                   product screenshots
```

## Imagery

Footage follows the brand imagery rules in `brand/README.md`: calm nature from
Pixabay, shown through an 8 by 8 Bayer dither where each dot keeps its color.
`Dither.tsx` does this live in WebGL from a 256 by 144 clip, so the page never
loads full-size video. The pointer lights up nearby dots and a click sends a
ripple. Clips play only while visible, and reduced motion shows the still frame.

To add a clip, crop it to 16:9, shrink it to 256 by 144, loop it forward and back,
and save the first frame as WebP:

```sh
ffmpeg -ss 0.3 -t 8 -i source.mp4 -filter_complex "[0:v]fps=24,scale=256:144:force_original_aspect_ratio=increase,crop=256:144,split[a][b];[b]reverse[r];[a][r]concat=n=2:v=1[v]" -map "[v]" -an -c:v libx264 -crf 24 -pix_fmt yuv420p -movflags +faststart public/media/name.mp4
ffmpeg -i public/media/name.mp4 -frames:v 1 -c:v libwebp -quality 85 public/media/name.webp
```

Credit the source in `THIRD_PARTY_NOTICES.md`.

## Motion

- Elements with `data-reveal` ease in once as they scroll into view. Without
  JavaScript, or with reduced motion, they are visible from the start.
- The screenshot tour rotates until someone picks a tab.
- There is no intro animation. The page paints on the first frame.

## SEO

- `src/layouts/Base.astro` owns the canonical URL, robots policy, Open Graph,
  Twitter card and SoftwareApplication JSON-LD.
- `public/og.png` is the 1200 x 630 social preview.
- `public/robots.txt` and `src/pages/sitemap.xml.ts` expose the production site.
- The homepage and the documentation pages are the URLs in the sitemap.
- The favicon is the same rounded-square Shipper mark used by the dashboard.

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
- Body copy stays at 14px or larger. Only the console mock-ups use console sizes.
- No eyebrows above section headlines, no helper text under CTAs, no decorative
  labels. If a section needs explanation, the headline or layout changes.

## Screenshots

`public/media/shots` holds WebP copies of the README screenshots in
`.github/assets`. Refresh them from the same capture, at 1440 by 900. The
documentation site keeps its own copies in `docs/public/shots`.
