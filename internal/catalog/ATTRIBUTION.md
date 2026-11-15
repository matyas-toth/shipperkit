# Service library attribution

The service templates in `catalog.json` under `"source": "coolify"` are derived
from the Coolify service template library at
<https://github.com/coollabsio/coolify/tree/main/templates/compose>.

Coolify is licensed under the Apache License, Version 2.0. The full license text
is in `licenses/Apache-2.0.txt`.

Shipper modifications:

- Decoded the generated template bundle and kept one manifest per service.
- Converted Coolify service variables and domains into Shipper fields.
- Replaced host bind mounts with named volumes where the definition was
  portable, and marked templates that still need host resources as advanced.
- Removed Coolify proxy labels, build-only services, and unsupported host
  options.
- Added Shipper metadata: category, logomark, roles, support state, and field
  labels.
- Added a small curated set of Shipper-authored templates with
  `"source": "shipper"`.

The manifests are snapshots. A deployed application keeps its own Compose file
and never reads the catalog at runtime.
