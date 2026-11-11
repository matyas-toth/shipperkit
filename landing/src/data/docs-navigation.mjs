// Shared navigation and sitemap source for the documentation site.
// Keep this file plain JavaScript so both Astro configs can import it.

export const docGroups = [
  {
    label: "Start here",
    items: [
      { slug: "index", label: "Overview", priority: "1.0", changefreq: "weekly" },
      {
        slug: "install",
        label: "Install and prerequisites",
        priority: "0.9",
        changefreq: "monthly",
      },
      {
        slug: "first-deploy",
        label: "First deploy in five minutes",
        priority: "0.9",
        changefreq: "monthly",
      },
      {
        slug: "move-an-app",
        label: "Move an existing app",
        priority: "0.8",
        changefreq: "monthly",
      },
      {
        slug: "uninstall",
        label: "Uninstall",
        priority: "0.6",
        changefreq: "yearly",
      },
    ],
  },
  {
    label: "Operate",
    items: [
      {
        slug: "recovery",
        label: "Recovery runbook",
        badge: { text: "Critical", variant: "danger" },
        priority: "1.0",
        changefreq: "monthly",
      },
      {
        slug: "diagnostics",
        label: "Diagnostics and troubleshooting",
        badge: { text: "Critical", variant: "caution" },
        priority: "1.0",
        changefreq: "monthly",
      },
      {
        slug: "backups",
        label: "Backups and off-server storage",
        priority: "0.9",
        changefreq: "monthly",
      },
      {
        slug: "domains-tls",
        label: "Domains and TLS",
        priority: "0.8",
        changefreq: "monthly",
      },
      {
        slug: "releases-rollbacks",
        label: "Releases and rollbacks",
        priority: "0.8",
        changefreq: "monthly",
      },
      {
        slug: "updates",
        label: "Updates and channels",
        priority: "0.8",
        changefreq: "monthly",
      },
      {
        slug: "environment",
        label: "Environment variables and secrets",
        priority: "0.8",
        changefreq: "monthly",
      },
      {
        slug: "access",
        label: "Users, roles, OAuth and TOTP",
        priority: "0.8",
        changefreq: "monthly",
      },
      {
        slug: "api-tokens",
        label: "API and agent tokens",
        priority: "0.9",
        changefreq: "monthly",
      },
    ],
  },
  {
    label: "Reference",
    items: [
      { slug: "cli", label: "CLI reference", priority: "0.7", changefreq: "monthly" },
      {
        slug: "ports",
        label: "Ports and firewall",
        priority: "0.6",
        changefreq: "yearly",
      },
      {
        slug: "instance-variables",
        label: "Instance environment variables",
        priority: "0.6",
        changefreq: "monthly",
      },
    ],
  },
];

export const docRoutes = docGroups.flatMap((group) => group.items);

export const sidebar = docGroups.map((group) => ({
  label: group.label,
  items: group.items.map(({ slug, label, badge }) => ({
    slug,
    label,
    ...(badge ? { badge } : {}),
  })),
}));
