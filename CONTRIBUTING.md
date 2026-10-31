# Contributing

Start with [README.md](README.md) and [docs/architecture.md](docs/architecture.md).
Keep changes scoped to a working phase. Preserve application-first workflows,
the measured design system, and the concise-copy rules in AGENTS.md.

## Development

Use Go 1.26+, Node 22.12+, and Docker for opt-in real-engine tests.
Install dependencies with `npm ci` and `npm ci --prefix web`.
Run `npm run dev` for the backend and frontend together.

Before opening a pull request, run `npm run check`, `npm test`,
`node scripts/go.mjs vet ./...`, and `npm run build`.
For UI changes, also run `npm run test:e2e` and inspect desktop/mobile rendering.
Run `npm run test:docker` when Docker integration changes.

Go code is formatted with gofmt. Keep exported APIs narrow, use contexts for
operations, and test security, ownership, persistence, and failure behavior.
Use ordinary SQL migrations rather than changing a previously shipped migration.
No secrets, local database files, generated frontend bundles, or screenshots
belong in a pull request.

Describe the concrete behavior change, the checks you ran, and any limitations.
Do not claim deployment or performance guarantees without reproducible evidence.

