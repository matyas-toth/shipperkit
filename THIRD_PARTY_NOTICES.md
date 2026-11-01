# Third-party notices

Shipper's own code is MIT licensed. Dependency licenses remain with their authors.

The bundled Inter variable font is distributed under the SIL Open Font License
1.1. Its full license is included at `web/public/inter-license.txt` and embedded
in the application at `/inter-license.txt`.
See [Inter](https://github.com/rsms/inter).

Tabler Icons is MIT licensed, copyright Paweł Kuna and contributors. Its full
license is included at `web/public/tabler-license.txt` and served at
`/tabler-license.txt`. The frontend imports individual SolidJS outline components;
the complete icon catalog is not shipped to the browser.
See [Tabler Icons](https://github.com/tabler/tabler-icons).

Go dependencies include the official Moby/Docker client (Apache-2.0), chi (MIT),
modernc SQLite (BSD-3-Clause), SQLite (public domain), and Go cryptography packages
(BSD-3-Clause). JavaScript dependencies include SolidJS, Solid Router, TanStack
Query, Vite, and Tailwind CSS under their respective permissive licenses.

Coolify was studied as an architectural reference. No Coolify source is included
in this implementation.

The runtime image includes unmodified Git from Alpine Linux (GPLv2), along with
its system dependencies. Source and package metadata are available from
[Git](https://github.com/git/git) and [Alpine's package sources](https://gitlab.alpinelinux.org/alpine/aports).
Apache and Traefik run in their upstream images; their licenses and source remain
with the respective upstream distributions. The base-image digests are recorded
in `internal/deploy/docker.go`.

