import type { Page } from "@playwright/test";

type Json = Record<string, any>;

const now = () => Math.floor(Date.now() / 1000);

export const referenceApps = {
  t3code: "t3code.png",
  codex: "codex.png",
  claudeWeb: "claude-web.png",
};

export async function installFixtures(page: Page) {
  const webResponse = await page.request.get("/api/applications/application-web");
  const web: Json = await webResponse.json();
  const apiInventory = await (await page.request.get("/api/applications")).json();
  const system = await (await page.request.get("/api/system")).json();
  const proxy = await (await page.request.get("/api/proxy")).json();
  const baseDiagnostics = await (await page.request.get("/api/applications/application-web/diagnostics")).json();
  const baseConfig = web.application.config;
  const t = now();
  const session = await (await page.request.get("/api/auth/session")).json();
  const schedules = ["0 */6 * * *", "0 3 * * 0", "0 2 * * *", "0 4 * * *"];
  const nextRuns = await Promise.all(schedules.map(async schedule => {
    const response = await page.request.post("/api/protection/schedule-preview", {
      headers: { "X-CSRF-Token": session.csrfToken }, data: { schedule, timezone: "Europe/Budapest" },
    });
    if (!response.ok()) throw new Error("Could not preview gallery schedule: " + schedule);
    return ((await response.json()).items as number[])[0];
  }));
  const databasePreview = await page.request.post("/api/protection/schedule-preview", {
    headers: { "X-CSRF-Token": session.csrfToken }, data: { schedule: "0 3 * * *", timezone: "UTC" },
  });
  if (!databasePreview.ok()) throw new Error("Could not preview gallery database schedule");
  const databaseNext = ((await databasePreview.json()).items as number[])[0];

  const variables = [
    { name: "DATABASE_URL", value: "", secret: true, scope: "both", keep: true, hasValue: true },
    { name: "SESSION_SECRET", value: "", secret: true, scope: "runtime", keep: true, hasValue: true },
    { name: "PUBLIC_ORIGIN", value: "https://website.localhost", secret: false, scope: "both", hasValue: true },
    { name: "NODE_ENV", value: "production", secret: false, scope: "build", hasValue: true },
    { name: "SMTP_URL", value: "", secret: true, scope: "runtime", keep: true, hasValue: true },
    { name: "STRIPE_WEBHOOK_SECRET", value: "", secret: true, scope: "runtime", keep: true, hasValue: true },
  ];

  const config: Json = {
    ...baseConfig,
    method: "static",
    repository: "https://github.com/mdn/beginner-html-site.git",
    branch: "main",
    publishDirectory: ".",
    routing: "files",
    htaccess: true,
    domain: "website.localhost",
    port: 8080,
    healthPath: "/",
    memoryMB: 512,
    variables,
    volumes: [
      { name: "uploads", path: "/app/uploads" },
      { name: "cache", path: "/app/cache" },
    ],
  };

  const domain = { id: "primary", hostname: "website.localhost", https: false, primary: true, redirect: false };
  const deployments = [
    ["deployment-web-6", "deploy", "succeeded", t - 190, 14, "4f8ac21d9e7712ba3c5d9086f4a2b1e3d5c7a9f0"],
    ["deployment-web-5", "deploy", "succeeded", t - 86_400, 17, "d6a4f792a0ce4d2cf5197aa9e54972be35f24a10"],
    ["deployment-web-4", "rollback", "succeeded", t - 172_800, 8, "7b2d1c9a44e8f0b3a1c5d7e9f2b4c6d8a0e1f3b5"],
    ["deployment-web-3", "deploy", "failed", t - 259_200, 21, "9c0e2a4b6d8f1c3e5a7b9d0f2c4e6a8b1d3f5c7e"],
    ["deployment-web-2", "deploy", "succeeded", t - 345_600, 15, "d6a4f792a0ce4d2cf5197aa9e54972be35f24a10"],
    ["deployment-web-1", "deploy", "succeeded", t - 864_000, 12, "2e4c6a8b1d3f5c7e9a0b2d4f6c8e1a3b5d7f9c0e"],
  ].map(([id, operation, state, createdAt, duration, commit], index) => ({
    id, applicationName: "Website", applicationId: "application-web", operation, state: state as string,
    config, commit, imageId: state === "failed" ? "" : "sha256:apache-fixture", containerId: state === "failed" ? "" : "b".repeat(64),
    previousId: index === 0 ? "" : "deployment-web-" + (6 - index), error: state === "failed" ? "Build exited with status 1" : "",
    createdAt: createdAt as number, startedAt: createdAt as number, finishedAt: (createdAt as number) + (duration as number),
    noCache: false, resolvedMethod: "static", runtimePort: 80, services: [],
  }));

  const containers = [
    { id: "b".repeat(64), name: "shipper-4f8ac21d-site-deployment-web-6", image: "httpd:2.4-alpine", state: "running", status: "Up 3 minutes (healthy)", health: "healthy", createdAt: t - 200, ports: [{ ip: "127.0.0.1", private: 80, public: 8088, protocol: "tcp" }], managed: true, applicationId: "application-web", deploymentId: "deployment-web-6" },
  ];

  const websiteDetail = {
    application: {
      ...web.application, name: "Website", state: "running", method: "static", sourceKind: "git",
      sourceRef: "https://github.com/mdn/beginner-html-site.git", activeMethod: "static",
      config, domains: [domain], containers, activeDeploymentId: "deployment-web-6",
      latestDeployment: deployments[0],
    },
    deployments,
    activeDeployment: deployments[0],
  };

  const atlasServices = [
    { name: "api", image: "ghcr.io/acme/atlas-api:2026.11.1", port: 3000, healthPath: "/health", volumes: [], variables: [] },
    { name: "worker", image: "ghcr.io/acme/atlas-worker:2026.11.1", port: 0, healthPath: "", volumes: [], variables: [] },
    { name: "postgres", image: "postgres:17-alpine", port: 5432, healthPath: "", volumes: [{ name: "postgres", path: "/var/lib/postgresql/data" }], variables: [] },
    { name: "redis", image: "redis:7-alpine", port: 6379, healthPath: "", volumes: [], variables: [] },
  ];
  const atlasConfig: Json = {
    ...config, method: "compose", composeSource: "git",
    repository: "https://github.com/acme/atlas.git", branch: "main",
    composeFile: "compose.yaml", domain: "atlas.localhost", port: 3000, healthPath: "/health",
    variables: variables.slice(0, 4), volumes: [{ name: "postgres", path: "/var/lib/postgresql/data" }],
    services: atlasServices.map(s => ({ ...s, memoryMB: 512 })),
  };
  const atlasContainers = atlasServices.map((s, index) => ({
    id: String(index + 4).repeat(64), name: "shipper-atlas-" + s.name + "-1", image: s.image,
    state: "running", status: index === 3 ? "Up 2 hours" : "Up 2 hours (healthy)", health: index === 3 ? "" : "healthy",
    createdAt: t - 7200, ports: s.port ? [{ ip: "127.0.0.1", private: s.port, public: 9100 + index, protocol: "tcp" }] : [],
    managed: true, applicationId: "application-atlas", project: "shipper-atlas", service: s.name,
  }));
  const atlasDeployments = deployments.slice(0, 4).map((d, index) => ({
    ...d, id: "deployment-atlas-" + (4 - index), applicationId: "application-atlas", applicationName: "Atlas API",
    commit: ["8e2c4a6b8d0f2a4c6e8b0d2f4a6c8e0b2d4f6a8c", "2c4e6a8b0d2f4a6c8e0b2d4f6a8c0e2b4d6f8a0c", "6a8c0e2b4d6f8a0c2e4g6i8k0m2o4q6s8u0w2y4a", "0e2b4d6f8a0c2e4g6i8k0m2o4q6s8u0w2y4a6c8e"][index],
    createdAt: t - 900 - index * 86_400, resolvedMethod: "compose", services: atlasServices,
    config: atlasConfig,
  }));
  const atlasDetail = {
    application: {
      ...web.application, id: "application-atlas", name: "Atlas API", state: "running", method: "compose",
      sourceKind: "compose", sourceRef: "https://github.com/acme/atlas.git", activeMethod: "compose",
      config: atlasConfig, domains: [{ ...domain, hostname: "atlas.localhost" }], containers: atlasContainers,
      activeDeploymentId: "deployment-atlas-4", latestDeployment: atlasDeployments[0],
    },
    deployments: atlasDeployments,
    activeDeployment: atlasDeployments[0],
  };

  const apiDetail = (() => {
    const app = apiInventory.items.find((item: Json) => item.id === "application-api");
    const base = apiDetailFrom(web, app);
    return base;
  })();

  function apiDetailFrom(webDetail: Json, app: Json) {
    const apiConfig = {
      ...webDetail.application.config,
      method: "dockerfile", repository: "https://github.com/acme/api.git", branch: "main",
      dockerfile: "Dockerfile", port: 3000, healthPath: "/healthz", memoryMB: 1024, domain: "api.localhost",
      variables: variables.slice(0, 3), volumes: [],
    };
    const apiDeployments = [
      ["deployment-api-4", "deploy", "succeeded", t - 3_600, "7f1d3b5a9c0e2g4i6k8m0o2q4s6u8w0y2a4c6e8g"],
      ["deployment-api-3", "restart", "succeeded", t - 86_400, "3b5d7f9a1c3e5g7i9k1m3o5q7s9u1w3y5a7c9e1g"],
      ["deployment-api-2", "deploy", "failed", t - 172_800, "5d7f9a1c3e5g7i9k1m3o5q7s9u1w3y5a7c9e1g3i"],
    ].map(([id, operation, state, createdAt, commit], index) => ({
      ...deployments[0], id, applicationId: "application-api", applicationName: "API", operation, state,
      commit, createdAt, startedAt: createdAt, finishedAt: (createdAt as number) + 9, previousId: "",
      config: apiConfig, resolvedMethod: "dockerfile", runtimePort: 3000, services: [],
      imageId: state === "failed" ? "" : "sha256:api-fixture", containerId: state === "failed" ? "" : "a".repeat(64),
      error: state === "failed" ? "Candidate HTTP check failed after 30s" : "",
    }));
    const containers = (app?.containers ?? []).map((c: Json) => ({ ...c, applicationId: "application-api" }));
    return {
      application: {
        ...webDetail.application, ...app, id: "application-api", name: "API", method: "dockerfile", sourceKind: "git",
        sourceRef: apiConfig.repository, activeMethod: "dockerfile", state: "degraded", desiredState: "running",
        config: apiConfig, domains: [{ ...domain, hostname: "api.localhost" }], containers,
        activeDeploymentId: "deployment-api-4", latestDeployment: apiDeployments[0],
      },
      deployments: apiDeployments,
      activeDeployment: apiDeployments[0],
    };
  }

  const checkoutConfig: Json = {
    ...config, repository: "https://github.com/acme/checkout.git", branch: "main",
    domain: "checkout.localhost", method: "dockerfile", port: 8080, healthPath: "/health",
    variables: variables.slice(0, 2), volumes: [],
  };
  const checkoutFailure = {
    ...deployments[3], id: "deployment-checkout-3", applicationId: "application-checkout", applicationName: "Checkout",
    state: "failed", operation: "deploy", createdAt: t - 420, startedAt: t - 420, finishedAt: t - 392,
    error: "Build failed at step 7: RUN npm run build", commit: "1a3c5e7g9i1k3m5o7q9s1u3w5y7a9c1e3g5i7k9m",
    config: checkoutConfig, notFound: false,
  };
  const checkoutDetail = {
    application: {
      ...web.application, id: "application-checkout", name: "Checkout", method: "dockerfile", sourceKind: "git",
      sourceRef: "https://github.com/acme/checkout.git", state: "failed", desiredState: "running",
      activeMethod: "dockerfile", config: checkoutConfig, domains: [{ ...domain, hostname: "checkout.localhost" }],
      containers: [], activeDeploymentId: "", latestDeployment: checkoutFailure,
    },
    deployments: [checkoutFailure, { ...deployments[1], id: "deployment-checkout-2", applicationId: "application-checkout", applicationName: "Checkout", config: checkoutConfig, createdAt: t - 86_400 }],
    activeDeployment: null,
  };

  const inventoryApps = [
    websiteDetail.application,
    atlasDetail.application,
    apiDetail.application,
    {
      ...websiteDetail.application, id: "application-docs", name: "Docs", domains: [{ ...domain, hostname: "docs.localhost" }],
      latestDeployment: { ...deployments[1], id: "deployment-docs-1", applicationId: "application-docs" }, createdAt: t - 86_400 * 8,
    },
    {
      ...atlasDetail.application, id: "application-worker", name: "Worker", method: "image", sourceKind: "image",
      sourceRef: "", config: { ...atlasConfig, method: "image", image: "ghcr.io/acme/worker:2.4" },
      domains: [], containers: [], state: "exited", activeDeploymentId: "",
      latestDeployment: { ...deployments[2], id: "deployment-worker-2", applicationId: "application-worker", createdAt: t - 3600 * 5, operation: "stop" },
    },
    checkoutDetail.application,
    {
      ...websiteDetail.application, id: "application-analytics", name: "Analytics",
      domains: [{ ...domain, hostname: "analytics.localhost" }],
      latestDeployment: { ...deployments[0], id: "deployment-analytics-4", applicationId: "application-analytics", createdAt: t - 60 * 26 },
    },
    {
      ...websiteDetail.application, id: "application-blog", name: "Blog",
      domains: [{ ...domain, hostname: "blog.localhost" }],
      latestDeployment: { ...deployments[1], id: "deployment-blog-2", applicationId: "application-blog", createdAt: t - 86_400 * 2 },
    },
    {
      ...websiteDetail.application, id: "application-status", name: "Status",
      domains: [{ ...domain, hostname: "status.localhost" }],
      latestDeployment: { ...deployments[0], id: "deployment-status-9", applicationId: "application-status", createdAt: t - 3600 * 8 },
    },
    {
      ...atlasDetail.application, id: "application-mailer", name: "Mailer", method: "image", sourceKind: "image",
      sourceRef: "", config: { ...atlasConfig, method: "image", image: "ghcr.io/acme/mailer:1.8" }, domains: [], containers: [],
      latestDeployment: { ...deployments[1], id: "deployment-mailer-3", applicationId: "application-mailer", createdAt: t - 86_400 * 4 },
    },
  ];

  const inventory = { ...apiInventory, items: inventoryApps };

  const tasks = {
    items: [
      { id: "task-warm", applicationId: "application-web", name: "Warm cache", command: { shell: "bash", argv: ["-lc", "curl -fsS http://127.0.0.1:8080/ >/dev/null"], service: "", directory: "", user: "", timeout: 60, mode: "exec" }, enabled: true, schedule: "0 */6 * * *", timezone: "Europe/Budapest", overlap: "skip", missed: "skip", nextAt: nextRuns[0] },
      { id: "task-prune", applicationId: "application-web", name: "Prune sessions", command: { shell: "bash", argv: ["-lc", "bin/prune-sessions --older-than 30d"], service: "", directory: "", user: "", timeout: 300, mode: "job" }, enabled: true, schedule: "0 3 * * 0", timezone: "Europe/Budapest", overlap: "queue", missed: "once", nextAt: nextRuns[1] },
      { id: "task-report", applicationId: "application-web", name: "Nightly report", command: { shell: "bash", argv: ["-lc", "bin/report >> /var/log/report.log"], service: "", directory: "", user: "", timeout: 120, mode: "job" }, enabled: false, schedule: "0 1 * * *", timezone: "Europe/Budapest", overlap: "skip", missed: "skip", nextAt: 0 },
    ],
    runs: [
      { id: "run-3", deploymentId: "deployment-web-5", stage: "Warm cache", state: "succeeded", command: {}, containerId: "b".repeat(64), execId: "exec-3", exitCode: 0, error: "", startedAt: t - 3600, finishedAt: t - 3592 },
      { id: "run-2", deploymentId: "deployment-web-4", stage: "Prune sessions", state: "failed", command: {}, containerId: "b".repeat(64), execId: "exec-2", exitCode: 1, error: "relation \"sessions\" does not exist", startedAt: t - 86_400, finishedAt: t - 86_370 },
      { id: "run-1", deploymentId: "deployment-web-2", stage: "Warm cache", state: "succeeded", command: {}, containerId: "b".repeat(64), execId: "exec-1", exitCode: 0, error: "", startedAt: t - 172_800, finishedAt: t - 172_793 },
    ],
  };

  const protection = {
    plans: [
      { id: "plan-postgres", applicationId: "application-web", name: "Nightly backup", config: { kind: "database", service: "", volume: "uploads", engine: "postgres", database: "atlas", user: "atlas", password: "", host: "127.0.0.1", port: 5432, tls: "prefer", destination: "s3-eu", keep: 14 }, enabled: true, schedule: "0 3 * * *", timezone: "UTC", nextAt: databaseNext },
      { id: "plan-uploads", applicationId: "application-web", name: "Uploads volume", config: { kind: "volume", service: "", volume: "uploads", engine: "", database: "", user: "", password: "", host: "", port: 0, tls: "", destination: "s3-eu", keep: 7 }, enabled: true, schedule: "0 4 * * *", timezone: "Europe/Budapest", nextAt: nextRuns[3] },
    ],
    runs: [
      { id: "run-4", planId: "plan-postgres", operation: "backup", state: "succeeded", createdAt: t - 3600 * 2, startedAt: t - 3600 * 2, finishedAt: t - 3600 * 2 + 74, error: "", bytes: 184_320_000, checksum: "sha256:2b6f8a", objectKey: "backups/website/2026-11-06.dump.age", result: "", sourceId: "", verifiedAt: t - 3600 * 2 + 92, kind: "database" },
      { id: "run-3", planId: "plan-uploads", operation: "backup", state: "succeeded", createdAt: t - 86400, startedAt: t - 86400, finishedAt: t - 86400 + 41, error: "", bytes: 52_428_800, checksum: "sha256:7c1e4d", objectKey: "backups/website/uploads.tar.age", result: "", sourceId: "", verifiedAt: t - 86400 + 55, kind: "volume" },
      { id: "run-2", planId: "plan-postgres", operation: "backup", state: "failed", createdAt: t - 172800, startedAt: t - 172800, finishedAt: t - 172800 + 12, error: "Database connection refused", bytes: 0, checksum: "", objectKey: "", result: "", sourceId: "", verifiedAt: 0, kind: "database" },
      { id: "run-1", planId: "plan-postgres", operation: "backup", state: "succeeded", createdAt: t - 259200, startedAt: t - 259200, finishedAt: t - 259200 + 68, error: "", bytes: 179_570_000, checksum: "sha256:8a2f9c", objectKey: "backups/website/2026-11-03.dump.age", result: "", sourceId: "", verifiedAt: t - 259200 + 88, kind: "database" },
    ],
    destinations: [
      { id: "s3-eu", name: "Hetzner Frankfurt", endpoint: "https://s3.eu-central.hetzner.com", region: "eu-central", bucket: "shipper-recovery", prefix: "website", private: true },
    ],
    recipient: "age1qy7xl2k9a3m5p8r0t2v4w6x8z0c3e5g7i9k1m3o5q7s9u1w3y5",
    volumes: [
      { applicationId: "application-web", applicationName: "Website", name: "uploads", logical: "uploads", attached: true, running: true, restored: false, services: [] },
      { applicationId: "application-web", applicationName: "Website", name: "cache", logical: "cache", attached: true, running: false, restored: false, services: [] },
      { applicationId: "", applicationName: "", name: "web-uploads-2026-10", logical: "uploads", attached: false, running: false, restored: true, services: [] },
    ],
    volumeError: "",
    revision: "browser-gallery-revision",
  };

  const operations = {
    ...(await (await page.request.get("/api/system/operations")).json()),
    bootReadiness: "ready",
    host: { observedAt: t - 20, stale: false, dockerAtBoot: true, shipperAtBoot: true, rebootRequired: false, securityServiceEnabled: true, clockSynchronized: true },
    items: [
      { stale: false, ageSeconds: 12, id: "daemon", applicationId: "", kind: "daemon", state: "healthy", summary: "Docker 29.8.2 connected", reference: "", observedAt: t - 12, details: {} },
      { stale: false, ageSeconds: 12, id: "proxy", applicationId: "", kind: "proxy", state: "healthy", summary: "Traefik routes reconciled", reference: "", observedAt: t - 12, details: {} },
      { stale: false, ageSeconds: 12, id: "storage", applicationId: "", kind: "storage", state: "healthy", summary: "Data filesystem has available space", reference: "", observedAt: t - 12, details: { scope: "Shipper data filesystem", free: 245_055_930_368, total: 999_243_640_832 } },
      { stale: false, ageSeconds: 18, id: "certificate", applicationId: "application-web", kind: "certificate", state: "warning", summary: "Certificate renews in 21 days", reference: "website.localhost", observedAt: t - 18, details: {} },
      { stale: false, ageSeconds: 8, id: "runtime-atlas", applicationId: "application-atlas", kind: "runtime", state: "healthy", summary: "4 services running", reference: "atlas", observedAt: t - 8, details: { cpuPercent: 4.2, memoryBytes: 348_127_232, memoryLimit: 2_147_483_648, restarts: 0, service: "api" } },
    ],
    notifications: { pending: 0, failed: 1 },
  };

  const connections = {
    items: [
      { id: "github-acme", name: "acme", kind: "github_app", host: "github.com", username: "acme", allowPrivate: true, updatedAt: t - 86400 * 9 },
      { id: "registry-ghcr", name: "GitHub registry", kind: "registry", host: "ghcr.io", username: "acme", allowPrivate: false, updatedAt: t - 86400 * 3 },
    ],
  };

  const tokens = {
    items: [
      { id: "token-ci", name: "GitHub Actions", scopes: ["read", "logs", "deploy"], applications: ["application-web", "application-atlas"], expiresAt: t + 86400 * 86 },
      { id: "token-ops", name: "Ops workstation", scopes: ["read", "logs", "execute", "secrets"], applications: [], expiresAt: t + 86400 * 24 },
      { id: "token-backup", name: "Backup verifier", scopes: ["read"], applications: [], expiresAt: t + 86400 * 12 },
    ],
  };

  const notifications = { enabled: true, kind: "webhook", events: ["deployment", "service", "backup", "certificate", "daemon", "proxy"], configured: true, webhook: "https://hooks.example.com/shipper", private: false, lastDelivery: t - 3600 };

  const retention = { plan: { logLines: 18420, revision: "retention-gallery", images: [{ id: "sha256:9f2c7e1a", bytes: 84_934_656 }, { id: "sha256:3b8d5f0c", bytes: 62_914_560 }], sources: [{ id: "upload-7c1e4d", bytes: 12_582_912 }], protected: "Serving releases and pinned images" }, automatic: true, runs: [{ id: "retention-1", state: "succeeded", createdAt: t - 86400, error: "" }] };

  const logLines = (deploymentId: string) => {
    const checkout = deploymentId.includes("checkout");
    const failed = checkout || deploymentId === "deployment-web-3";
    const steps = failed
      ? [
          ["cloning", checkout ? "Cloning https://github.com/acme/checkout.git" : "Cloning https://github.com/mdn/beginner-html-site.git"],
          ["cloning", checkout ? "Checked out 1a3c5e7g on main" : "Checked out 9c0e2a4b on main"],
          ["building", "Using BuildKit"],
          ["building", "Installing dependencies"],
          ["building", "RUN npm ci"],
          ["building", "added 1482 packages in 24s"],
          ["building", "RUN npm run build"],
          ["building", checkout ? "src/checkout.tsx:84:18 Type error" : "Sass compilation failed"],
          ["building", checkout ? "Property 'currency' does not exist on type 'Cart'" : "Undefined variable: $accent"],
          ["building", "Build failed with exit code 1"],
        ]
      : [
          ["cloning", "Cloning https://github.com/mdn/beginner-html-site.git"],
          ["cloning", "Checked out " + (deployments.find(d => d.id === deploymentId)?.commit.slice(0, 8) ?? "4f8ac21d") + " on main"],
          ["packaging", "Packaging Apache image"],
          ["packaging", "Copying 24 files into /usr/local/apache2/htdocs"],
          ["packaging", "Setting correct permissions"],
          ["starting", "Starting candidate container"],
          ["checking", "Candidate HTTP check passed"],
          ["promoting", "Waiting for Traefik routes to converge"],
          ["promoting", "Traefik acknowledged the routing update"],
          ["promoting", "Releasing to website.localhost"],
          ["promoting", "Release is serving traffic"],
        ];
    const started = deploymentId.includes("checkout") ? t - 420 : deployments.find(d => d.id === deploymentId)?.createdAt ?? t - 190;
    const items = steps.map(([step, line], index) => ({ number: index + 1, sequence: index + 1, step, line, createdAt: started + index * 2 }));
    return { items, state: failed ? "failed" : "succeeded", next: 0 };
  };

  const route = (pattern: string, handler: (route: any) => Promise<void>) => page.route(pattern, handler);
  const json = (body: unknown) => async (r: any) => r.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(body) });

  await route("**/api/applications", json(inventory));
  await route("**/api/applications/application-web", json(websiteDetail));
  await route("**/api/applications/application-atlas", json(atlasDetail));
  await route("**/api/applications/application-api", json(apiDetail));
  await route("**/api/applications/application-checkout", json(checkoutDetail));
  await route("**/api/applications/*/tasks", async r => {
    if (r.request().method() === "GET") return r.fulfill({ json: tasks });
    return r.fulfill({ json: { status: "queued" } });
  });
  await route("**/api/applications/*/protection", json(protection));
  await route("**/api/protection", json({ ...protection, plans: [], runs: protection.runs, destinations: protection.destinations, volumes: protection.volumes }));
  await route("**/api/system", async r => {
    if (r.request().url().endsWith("/api/system")) return r.fulfill({ json: { ...system, database: { ...system.database, counts: { applications: inventoryApps.length, containers: 9, sessions: 1 } } } });
    return r.continue();
  });
  await route("**/api/system/operations", json(operations));
  await route("**/api/system/retention", json(retention));
  await route("**/api/notifications", json(notifications));
  await route("**/api/notifications/deliveries", json({ items: [{ sequence: 42, kind: "deployment", summary: "Checkout deployment failed", createdAt: t - 3600, attempts: 5, error: "Webhook returned HTTP 503" }] }));
  await route("**/api/operation-tokens", json(tokens));
  await route("**/api/connections", json(connections));
  await route("**/api/proxy", json(proxy));
  await route("**/api/repositories/inspect", json({ method: "railpack", commit: "4e8b2d6f0a1c3e5g7i9k1m3o5q7s9u1w3y5a7c9e1", variables: ["DATABASE_URL", "PUBLIC_ORIGIN", "NODE_ENV"], installCommand: "npm ci", buildCommand: "npm run build", startCommand: "npm start" }));
  await route("**/api/plan", json({ applicationId: "application-portfolio", revision: "creation-gallery", name: "Portfolio", commit: "4e8b2d6f0a1c3e5g7i9k1m3o5q7s9u1w3y5a7c9e1", config: { ...atlasConfig, method: "railpack", repository: "https://github.com/acme/portfolio.git", domain: "portfolio.localhost" } }));
  await route("**/api/applications/*/changes", json({ operation: "recreate", changes: ["Environment variables"], revision: "browser-gallery-change", downtime: "About 20 seconds" }));

  await route("**/api/applications/*/diagnostics", async r => {
    const url = r.request().url();
    const id = url.match(/applications\/([^/]+)\/diagnostics/)?.[1];
    const checkHost = id === "application-atlas" ? "atlas.localhost" : id === "application-api" ? "api.localhost" : id === "application-checkout" ? "checkout.localhost" : "website.localhost";
    const detail = id === "application-atlas" ? atlasDetail : id === "application-api" ? apiDetail : websiteDetail;
    const serviceObservations = id === "application-atlas"
      ? [
          { service: "api", cpuPercent: 2.4, memoryBytes: 184_320_000, memoryLimit: 536_870_912, restarts: 0 },
          { service: "worker", cpuPercent: 0.6, memoryBytes: 92_274_688, memoryLimit: 536_870_912, restarts: 0 },
          { service: "postgres", cpuPercent: 1.1, memoryBytes: 268_435_456, memoryLimit: 1_073_741_824, restarts: 0 },
          { service: "redis", cpuPercent: 0.2, memoryBytes: 31_457_280, memoryLimit: 268_435_456, restarts: 0 },
        ].map((details, index) => ({ stale: false, id: "service-" + index, kind: "service", state: "healthy", summary: "Service resources", reference: details.service, observedAt: t - 10 - index, details }))
      : [{ stale: false, id: "runtime", kind: "service", state: "healthy", summary: "Service resources", reference: detail.application.containers[0]?.id ?? "apache", observedAt: t - 12, details: { service: "apache", cpuPercent: 1.8, memoryBytes: 184_320_000, memoryLimit: 536_870_912, restarts: 0 } }];
    const observations = [
      { stale: false, id: "route", kind: "route", state: "healthy", summary: "HTTP 200 from " + checkHost, reference: checkHost, observedAt: t - 18, details: {} },
      ...serviceObservations,
      { stale: false, id: "artifact", kind: "artifact", state: "healthy", summary: "Serving release artifact is present", reference: "sha256:apache-fixture", observedAt: t - 900, details: {} },
    ];
    const base = { ...baseDiagnostics, applicationId: id, name: id === "application-atlas" ? "Atlas API" : id === "application-api" ? "API" : id === "application-checkout" ? "Checkout" : "Website", observations, checks: baseDiagnostics.checks.map((check: Json) => check.layer === "route" ? { ...check, summary: checkHost, state: "healthy", observedAt: t - 10 } : check.layer === "build" ? { ...check, observedAt: t - 900, state: "healthy" } : check), servingRelease: detail.activeDeployment, latestOperation: detail.activeDeployment, durationSeconds: 14 };
    if (id === "application-checkout") {
      return r.fulfill({ json: { ...base, name: "Checkout", checks: [
        { layer: "daemon", observedAt: t - 10, state: "healthy", summary: "Docker connection" },
        { layer: "runtime", observedAt: t - 400, state: "failed", summary: "No running container" },
        { layer: "build", observedAt: t - 392, state: "failed", summary: "Build exited with status 1" },
        { layer: "route", observedAt: 0, state: "unknown", summary: "checkout.localhost" },
      ], latestOperation: checkoutFailure, failedStage: "building", durationSeconds: 28, servingRelease: undefined, changesSinceServing: [] } });
    }
    if (id === "application-api") {
      return r.fulfill({ json: { ...base, name: "API", checks: [
        { layer: "daemon", observedAt: t - 10, state: "healthy", summary: "Docker connection" },
        { layer: "runtime", observedAt: t - 10, state: "warning", summary: "Health check is failing" },
        { layer: "route", observedAt: t - 10, state: "healthy", summary: "api.localhost" },
        { layer: "build", observedAt: t - 3600, state: "healthy", summary: "Latest release artifact" },
      ], observations: [{ id: "cpu", kind: "runtime", state: "warning", summary: "CPU at 92%", reference: "api", observedAt: t - 10, details: { cpuPercent: 92.4, memoryBytes: 734_003_200, memoryLimit: 1_073_741_824, restarts: 2 }, stale: false }] } });
    }
    return r.fulfill({ json: base });
  });
  await route("**/api/applications/*/incidents", json({ items: [] }));
  await route("**/api/applications/*/deployments/*/logs*", async r => {
    const id = r.request().url().match(/deployments\/([^/]+)\/logs/)?.[1] ?? "";
    return r.fulfill({ json: logLines(id) });
  });
  await route("**/api/applications/*/deployments/*", async r => {
    const match = r.request().url().match(/applications\/([^/]+)\/deployments\/([^/?]+)/);
    const all = [websiteDetail, atlasDetail, apiDetail, checkoutDetail];
    const detail = all.find(d => d.application.id === match?.[1]);
    const deployment = detail?.deployments.find(d => d.id === match?.[2]) ?? deployments[0];
    return r.fulfill({ json: deployment });
  });
  await route("**/api/applications/*/logs*", async r => {
    if (r.request().headers()["accept"]?.includes("text/event-stream")) return r.continue();
    return r.fulfill({ json: { items: [] } });
  });
}

export const fixtureApps = ["website", "atlas", "api", "checkout"];
