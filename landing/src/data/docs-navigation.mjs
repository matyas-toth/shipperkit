// Shared navigation and sitemap source for the documentation site.
// Keep this file plain JavaScript so both Astro configs can import it.

export const docGroups = [
  {
    label: "Get started",
    items: [
      { slug: "index", label: "Documentation", priority: "1.0", changefreq: "weekly" },
      { slug: "prerequisites", label: "Prerequisites", priority: "0.9", changefreq: "monthly" },
      { slug: "install", label: "Install", priority: "0.9", changefreq: "monthly" },
      { slug: "installer", label: "What the installer changes", priority: "0.6", changefreq: "monthly" },
      { slug: "first-deploy", label: "First deploy", priority: "0.9", changefreq: "monthly" },
      { slug: "move-an-app", label: "Move an existing app", priority: "0.8", changefreq: "monthly" },
      { slug: "uninstall", label: "Uninstall", priority: "0.6", changefreq: "yearly" },
    ],
  },
  {
    label: "Deploy",
    items: [
      { slug: "sources", label: "Sources", priority: "0.8", changefreq: "monthly" },
      { slug: "builds", label: "Builds and checks", priority: "0.8", changefreq: "monthly" },
      { slug: "domains", label: "Domains", priority: "0.8", changefreq: "monthly" },
      { slug: "tls", label: "HTTPS and certificates", priority: "0.8", changefreq: "monthly" },
      { slug: "environment", label: "Environment variables", priority: "0.8", changefreq: "monthly" },
      { slug: "secrets", label: "Secrets", priority: "0.8", changefreq: "monthly" },
      { slug: "releases", label: "Releases", priority: "0.8", changefreq: "monthly" },
      { slug: "rollbacks", label: "Rollbacks", priority: "0.8", changefreq: "monthly" },
      { slug: "updates", label: "Updates", priority: "0.8", changefreq: "monthly" },
    ],
  },
  {
    label: "Operate",
    items: [
      { slug: "backups", label: "Backups", priority: "0.9", changefreq: "monthly" },
      { slug: "off-server-storage", label: "Off-server storage", priority: "0.8", changefreq: "monthly" },
      { slug: "restore", label: "Restore", priority: "0.9", changefreq: "monthly" },
      { slug: "recovery", label: "Recovery runbook", priority: "1.0", changefreq: "monthly" },
      { slug: "lost-access", label: "Lost access", priority: "0.8", changefreq: "monthly" },
      { slug: "host-recovery", label: "Host recovery", priority: "0.9", changefreq: "monthly" },
      { slug: "diagnostics", label: "Diagnostics", priority: "0.9", changefreq: "monthly" },
      { slug: "troubleshooting", label: "Troubleshooting", priority: "0.9", changefreq: "monthly" },
      { slug: "maintenance", label: "Maintenance", priority: "0.7", changefreq: "monthly" },
    ],
  },
  {
    label: "Access",
    items: [
      { slug: "users", label: "Users and roles", priority: "0.8", changefreq: "monthly" },
      { slug: "oauth", label: "OAuth", priority: "0.7", changefreq: "monthly" },
      { slug: "two-factor", label: "Two-factor authentication", priority: "0.7", changefreq: "monthly" },
      { slug: "tokens", label: "API and agent tokens", priority: "0.9", changefreq: "monthly" },
      { slug: "deploy-tokens", label: "Deploy tokens", priority: "0.7", changefreq: "monthly" },
    ],
  },
  {
    label: "Reference",
    items: [
      { slug: "cli", label: "CLI", priority: "0.7", changefreq: "monthly" },
      { slug: "api", label: "API", priority: "0.8", changefreq: "monthly" },
      { slug: "ports", label: "Ports", priority: "0.6", changefreq: "yearly" },
      { slug: "instance-variables", label: "Instance variables", priority: "0.6", changefreq: "monthly" },
    ],
  },
];

export const docRoutes = docGroups.flatMap((group) => group.items);

export const sidebar = docGroups.map((group) => ({
  label: group.label,
  items: group.items.map(({ slug, label }) => ({ slug, label })),
}));
