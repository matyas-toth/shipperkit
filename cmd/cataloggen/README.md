# Catalog generator

Builds `internal/catalog/catalog.json` from Coolify's generated
`service-templates.json` plus the curated templates in `curated.go`.

```sh
go run ./cmd/cataloggen -source service-templates.json -out internal/catalog/catalog.json
```

The generator converts Coolify service variables into Shipper fields, replaces
host bind mounts with named volumes where possible, removes unsupported host
options, escapes container-side shell interpolation, derives a main service and
HTTP port, and marks templates that still need host resources as advanced.

See [Service Library](../../docs/service-library.md) and
[attribution](../../internal/catalog/ATTRIBUTION.md).
