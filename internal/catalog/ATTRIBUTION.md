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

Service names and logos are trademarks of their respective projects. Most logo
assets are copied from Coolify's `public/svgs` directory under the same
Apache-2.0 license. Curated-service logos use [Simple Icons](https://simpleicons.org),
which is CC0. Logos are used only to identify the service in the library and are
not part of Shipper's own marks.
