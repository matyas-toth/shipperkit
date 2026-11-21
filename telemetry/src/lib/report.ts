import { z } from "zod";

const key = z.string().regex(/^[a-z0-9][a-z0-9._-]{0,39}$/);
const count = z.number().int().min(0).max(1_000_000);
const breakdown = z.record(key, count).refine(value => Object.keys(value).length <= 32, "Too many keys").default({});
const label = (max: number) => z.string().trim().max(max).regex(/^[\w .+()\/-]*$/, "Unexpected characters");

/** Schema 1 of the ShipperKit usage report. Unknown fields are dropped. */
export const reportSchema = z.object({
  schema: z.literal(1),
  id: z.string().regex(/^[0-9a-f]{32}$/),
  version: z.string().regex(/^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$/).max(40),
  channel: z.enum(["main", "nightly"]).catch("main"),
  autoUpdate: z.boolean().default(false),
  install: z.enum(["managed", "manual"]).catch("manual"),
  uptimeHours: z.number().int().min(0).max(1_000_000).default(0),
  platform: z.object({
    os: key.catch("other"),
    arch: key.catch("other"),
    hostOs: label(80).catch(""),
    docker: label(40).catch(""),
    cpus: z.number().int().min(0).max(4096).default(0),
    memoryMb: z.number().int().min(0).max(64 * 1024 * 1024).default(0),
  }),
  usage: z.object({
    applications: count.default(0),
    services: count.default(0),
    domains: count.default(0),
    users: count.default(0),
    deployments: count.default(0),
    failedDeployments: count.default(0),
    rollbacks: count.default(0),
    deployMedianSeconds: count.default(0),
    backupPlans: count.default(0),
    tasks: count.default(0),
    accessTokens: count.default(0),
  }),
  methods: breakdown,
  sources: breakdown,
  databases: breakdown,
  catalog: breakdown,
  connections: breakdown,
  features: z.array(key).max(32).default([]),
});

export type Report = z.infer<typeof reportSchema>;
