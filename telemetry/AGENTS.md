<!-- BEGIN:nextjs-agent-rules -->

## This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` (resolved from this file's directory; in monorepos the `next` package may not be visible from the repo root) before writing any code. Heed deprecation notices.

This block is written and re-added by `next dev` — verify at `node_modules/next/dist/server/lib/generate-agent-files.js`. Removing it from a diff only re-creates the uncommitted change; committing it with your work keeps the tree clean.

<!-- END:nextjs-agent-rules -->

## ShipperKit telemetry

- Follow the repository root `AGENTS.md` and `DESIGN.md`. Dark mode only, no helper text.
- Chart colors come from `src/lib/colors.ts`. Keep the slot order.
- Every data read calls `requireAdmin()` before a cached query. Proxy is only an optimistic gate.
- The ingest schema in `src/lib/report.ts` must match `internal/telemetry` in Go.
- Run `npm run check` before committing.
