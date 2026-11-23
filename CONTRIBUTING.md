# Contributing

Thanks for helping with ShipperKit. Bug fixes, docs improvements and Service Library fixes are welcome as pull requests. For a new feature or a larger change, open an issue or ask on [Discord](https://discord.shipperkit.dev) first so we can agree on the approach before you write it.

By taking part you agree to follow the [code of conduct](CODE_OF_CONDUCT.md).

## Set up

You need Go 1.27.4+, Node.js 22.12+, Git 2.36+ and Docker with Buildx and Compose.

```sh
npm ci
npm ci --prefix web
npm run dev
```

[Development](docs/development.md) covers configuration, the Docker package and the integration suites. [Architecture](docs/architecture.md) explains how the pieces fit together.

## Make a change

1. Create a branch from `master`.
2. Keep the change focused on one behavior.
3. Add or update tests for what you changed, especially for security, ownership, persistence and failure handling.
4. Update the docs when behavior changes. Developer docs live in `docs/`, user docs in `landing/docs`.
5. Run the checks below and open a pull request.

Pull requests are squash merged. Use a [Conventional Commits](https://www.conventionalcommits.org/) title such as `feat: …`, `fix: …`, `perf: …`, `refactor: …`, `docs: …`, `test: …` or `chore: …`, because release-please builds the changelog and the next version from it. Mark breaking changes with `!`.

## Checks

Every pull request runs these:

```sh
npm run check
npm test
node scripts/go.mjs vet ./...
npm run build
```

Run the suites that match what you touched. The Docker suites need a disposable Docker host, and on Docker Desktop they should run one at a time.

| Change | Run |
| --- | --- |
| Dashboard UI | `npm run test:e2e`, and check desktop and mobile widths |
| Docker inventory or events | `npm run test:docker` |
| Builds, routing, Apache or Traefik | `npm run test:deploy` and `npm run test:full` |
| Compose | `npm run test:compose` |
| Startup hooks or scheduled tasks | `npm run test:tasks` |
| Sources, images or inline Dockerfiles | `npm run test:sources` |
| Backups and recovery | `npm run test:backups` |
| The production image | `docker build -t shipper:local .` then `npm run test:package` |
| The CLI | `npm run check --prefix cli` and `npm test --prefix cli` |

In the pull request, describe the behavior change, the checks you ran and anything you could not verify. Don't claim a guarantee without evidence someone else can reproduce.

## Guidelines

- Format Go with `gofmt`, keep exported APIs narrow and pass contexts through operations.
- Add a new SQL migration instead of editing one that has shipped.
- Never commit secrets, local databases, build output or screenshots from your own runs.
- `internal/catalog/catalog.json` is generated. Change `cmd/cataloggen` and regenerate it as its [README](cmd/cataloggen/README.md) describes.
- Perceived speed is not negotiable. Keep cached data on screen while it refreshes, and keep the shell and navigation instant.
- Every change should leave the experience better for users, for developers and for agents that drive Shipper.

### Writing

These rules apply to every piece of UI copy, and to docs as well:

- Cut taglines, eyebrows, supplementary text, helper text and secondary labels. If a screen needs them to make sense, fix the layout.
- Don't write staccato sentences, and don't use em-dashes.
- Follow the [design system](DESIGN.md) for layout, type and color.

## Releases

Merging to `master` updates a release-please pull request. Merging that pull request tags the release and publishes the image, the installer and the CLI. See [releases](docs/releases.md) for the pipeline and staging.

## License

Contributions are released under the [MIT license](LICENSE).
