# Catalog generator

Builds `internal/catalog/catalog.json` from Coolify's generated
`service-templates.json` plus the curated templates in `curated.go`.

```sh
go run ./cmd/cataloggen -source service-templates.json -out internal/catalog/catalog.json
```

The current manifest was generated from Coolify commit
`5c42313f2d9de23924abbd9df148ca2b5cfaa1ec`:

```sh
curl -fLO https://raw.githubusercontent.com/coollabsio/coolify/5c42313f2d9de23924abbd9df148ca2b5cfaa1ec/templates/service-templates.json
```

The generator converts Coolify service variables into Shipper fields, turns files
a template writes inline into Compose configs, replaces other host bind mounts
with named volumes, removes unsupported host options, escapes container-side
shell interpolation, derives a main service and HTTP port, and marks templates
that still need host resources as advanced.

See [Service Library](../../docs/service-library.md) and
[attribution](../../internal/catalog/ATTRIBUTION.md).

Service logos are local assets. After regenerating the manifest, refresh them
with the same Coolify bundle:

```sh
node scripts/fetch-service-logos.mjs --source service-templates.json
```

The script writes `web/public/service-logos` and falls back to a monogram in the
UI when an upstream logo is unavailable. PNG assets are committed downscaled to
at most 160 px; use ffmpeg or an image optimizer before committing refreshed
assets.
