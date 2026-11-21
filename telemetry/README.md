# ShipperKit Telemetry

Admin dashboard and ingest endpoint for anonymous ShipperKit usage reports. Next.js, Prisma and Postgres.

## Run locally

```sh
cd telemetry
cp .env.example .env        # then set ADMIN_KEY
npm install
npm run db:up               # Postgres 17 on 127.0.0.1:5434
npm run db:deploy           # apply migrations
npm run db:seed             # optional: 420 synthetic instances
npm run dev                 # http://localhost:3100
```

Sign in with `ADMIN_KEY`. It is the only credential. Sessions are HMAC-signed with it, so rotating the key signs everyone out.

## How reports arrive

ShipperKit's control plane (`internal/telemetry`) posts one report a day to `POST /api/v1/ingest`. The schema lives in `src/lib/report.ts` and mirrors `telemetry.Report` in Go. Bump `schema` on both sides for breaking changes.

- Reports are enabled by default on managed installs only. Source runs, tests and CI never send.
- `SHIPPER_TELEMETRY=0` or `DO_NOT_TRACK=1` turns them off. Admins can also switch them off in Settings → Usage data.
- `SHIPPER_TELEMETRY_URL` points an instance at a different endpoint, for example `http://host.docker.internal:3100/api/v1/ingest` while developing.
- The endpoint stores no IP addresses. It keeps a country code only when a CDN header (`x-vercel-ip-country` or `cf-ipcountry`) provides one.
- Reports from the same instance less than 30 minutes apart are acknowledged and dropped.

## Data model

- `instances` holds the latest report per anonymous id, plus first and last seen times.
- `reports` keeps every accepted report for time series.
- `upgrades` records each version change.

An instance is active while it has reported within `ACTIVE_DAYS` (default 3). Aggregates are cached for a minute with `"use cache"`.

## Deploy

The `Dockerfile` builds a standalone image that applies migrations on start. It needs `DATABASE_URL` and `ADMIN_KEY`, and serves on port 3000. The default endpoint compiled into ShipperKit is `https://telemetry.shipperkit.dev/api/v1/ingest`, so point that hostname at the deployment.
