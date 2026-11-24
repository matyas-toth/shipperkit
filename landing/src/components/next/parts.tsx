import type { ReactNode } from "react";

/* Static building blocks shared by every variant. Nothing here hydrates:
   copy buttons use the page-level data-copy handler and menus use <details>. */

export const LINKS = {
  docs: "/docs/",
  install: "/docs/install/",
  github: "https://github.com/matyas-toth/shipperkit",
  discord: "https://discord.shipperkit.dev",
  releases: "https://github.com/matyas-toth/shipperkit/releases",
  security: "https://github.com/matyas-toth/shipperkit/blob/master/SECURITY.md",
  license: "https://github.com/matyas-toth/shipperkit/blob/master/LICENSE",
};
export const INSTALL = "curl -fsSL https://shipperkit.dev/install.sh | sudo bash";

export const FEATURES = [
  {
    key: "deploy",
    title: "Deploy from anywhere",
    body: "Paste a Git URL, pick a private GitHub repository, upload a folder, or run an image, Dockerfile or Compose file. Shipper works out the build, and a push to your branch deploys again.",
  },
  {
    key: "releases",
    title: "Releases that fail safely",
    body: "A failed build never goes live. The previous release keeps serving while you read the line that broke, and any earlier release can be restored.",
  },
  {
    key: "https",
    title: "HTTPS on every domain",
    body: "Point a domain at the server and Shipper issues and renews the certificate from Let's Encrypt, with aliases and redirects on the same screen.",
  },
  {
    key: "databases",
    title: "Managed databases",
    body: "PostgreSQL, MySQL, MariaDB, MongoDB, Redis, ClickHouse and SQLite with a ready connection string, password rotation and scheduled backups.",
  },
  {
    key: "library",
    title: "Service Library",
    body: "Deploy n8n, Immich, Supabase, Grafana, Uptime Kuma and 360 more services from a searchable library, with their secrets generated for you.",
  },
  {
    key: "backups",
    title: "Backups and recovery",
    body: "Back up databases and volumes on a schedule to the server or S3-compatible storage, restore a backup as a separate copy, and recover the whole instance from an encrypted archive.",
  },
  {
    key: "agents",
    title: "Built for agents",
    body: "The shipper CLI and scoped tokens let you or your coding agent deploy, read logs and diagnose failures without SSH access. Every command has a --json form.",
  },
  {
    key: "ops",
    title: "Logs, terminal and tasks",
    body: "Live logs, a browser terminal, environment variables and scheduled tasks for every application, in the same place you deploy it.",
  },
  {
    key: "teams",
    title: "Teams and sign-in",
    body: "Admin, Developer and Viewer roles, sign-in with Google, GitHub or Discord, and two-factor authentication for everyone on the instance.",
  },
  {
    key: "updates",
    title: "Updates without downtime",
    body: "Update Shipper from the dashboard. Your applications keep running while the control plane restarts.",
  },
] as const;
export type FeatureKey = (typeof FEATURES)[number]["key"];
export const feature = (key: FeatureKey) => FEATURES.find((f) => f.key === key)!;

export const FAQ = [
  ["What does it cost?", "Nothing. Shipper is MIT licensed with no paid tier, seat count or build-minute meter. You pay your server and domain providers directly."],
  ["Which servers does it run on?", "Ubuntu 22.04 or 24.04 and Debian 12 or 13, on x86-64 or ARM64, with at least 1 GiB of RAM and ports 80, 443 and 8080 open. The installer sets up Docker and the proxy."],
  ["Does it work with my stack?", "Static sites, Dockerfiles, Docker Compose, prebuilt images and most language projects. If it runs in Docker, it runs on Shipper."],
  ["Do I need a GitHub account?", "No. Deploy a local folder, a public Git URL, an image, a Compose file or an inline Dockerfile. Private GitHub repositories are optional."],
  ["What happens when a deploy fails?", "The previous release keeps serving traffic. The failed stage and its logs stay next to the release that is still live, so you can fix it and deploy again."],
  ["Can my coding agent use it?", "Yes. The CLI and API expose the same deploy, logs, diagnose and restart actions with scoped tokens, so an agent never needs your SSH key or browser session."],
  ["Am I locked in?", "No. Your apps run as ordinary Docker containers, your data stays on your server, and stopping Shipper does not delete your workloads."],
] as const;

export const FACTS = [
  ["MIT", "licensed, with no paid tier"],
  ["365", "services in the library"],
  ["7", "database engines"],
  ["ARM64", "and x86-64 servers"],
] as const;

export function Wordmark({ className = "h-[22px]" }: { className?: string }) {
  return <img src="/wordmark.svg" alt="Shipper" width={269} height={54} className={`w-auto ${className}`} />;
}

export function GitHubIcon({ className = "size-4" }: { className?: string }) {
  return (
    <svg viewBox="0 0 16 16" className={className} fill="currentColor" aria-hidden>
      <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" />
    </svg>
  );
}

const navLinks = [
  ["Features", "#features"],
  ["Docs", LINKS.docs],
  ["Changelog", LINKS.releases],
  ["Discord", LINKS.discord],
] as const;

export function Nav({ tone = "solid", wide = false }: { tone?: "solid" | "clear"; wide?: boolean }) {
  return (
    <header className={`sticky top-0 z-50 border-b ${tone === "solid" ? "border-line bg-[#080808]/80 backdrop-blur-xl" : "border-transparent bg-gradient-to-b from-black/60 to-transparent"}`}>
      <div className={`${wide ? "max-w-[1440px]" : ""} wrap flex h-16 items-center justify-between gap-6`}>
        <a href="/" aria-label="Shipper home" className="flex items-center">
          <Wordmark />
        </a>
        <nav className="hidden items-center gap-7 md:flex" aria-label="Primary">
          {navLinks.map(([label, href]) => (
            <a key={label} href={href} className="text-[14px] text-ink-2 transition-colors hover:text-ink">{label}</a>
          ))}
        </nav>
        <div className="flex items-center gap-2">
          <a href={LINKS.github} className="btn btn-ghost btn-sm hidden sm:inline-flex" aria-label="ShipperKit on GitHub">
            <GitHubIcon />
            GitHub
          </a>
          <a href={LINKS.install} className="btn btn-primary btn-sm">Install</a>
          <details className="relative md:hidden">
            <summary aria-label="Open menu" className="grid size-9 place-items-center rounded-lg border border-line-2 text-ink-2">
              <svg viewBox="0 0 16 16" className="size-4" fill="none" stroke="currentColor" strokeWidth="1.5"><path d="M2.5 5h11M2.5 11h11" /></svg>
            </summary>
            <div className="absolute right-0 top-11 w-56 rounded-xl border border-line-2 bg-[#0e0f0f] p-2 shadow-2xl">
              {[...navLinks, ["GitHub", LINKS.github] as const].map(([label, href]) => (
                <a key={label} href={href} className="block rounded-lg px-3 py-2.5 text-[15px] text-ink-2 hover:bg-white/[0.06] hover:text-ink">{label}</a>
              ))}
            </div>
          </details>
        </div>
      </div>
    </header>
  );
}

export function InstallBar({ className = "", size = "md" }: { className?: string; size?: "md" | "lg" }) {
  return (
    <div className={`flex w-full max-w-[620px] items-center gap-3 rounded-[10px] border border-line-2 bg-[#0c0d0d]/90 pl-4 pr-1.5 backdrop-blur ${size === "lg" ? "h-14" : "h-12"} ${className}`}>
      <span className="t-mono select-none text-mute">$</span>
      <code className={`t-mono min-w-0 flex-1 overflow-x-auto whitespace-nowrap text-ink [scrollbar-width:none] ${size === "lg" ? "text-[14.5px]" : "text-[13.5px]"}`}>{INSTALL}</code>
      <button type="button" data-copy={INSTALL} className="btn btn-ghost btn-sm !h-9 shrink-0 cursor-pointer data-[copied=true]:text-green">
        <span data-copy-label>Copy</span>
      </button>
    </div>
  );
}

export function Ctas({ className = "", center = false }: { className?: string; center?: boolean }) {
  return (
    <div className={`flex flex-wrap items-center gap-3 ${center ? "justify-center" : ""} ${className}`}>
      <a href={LINKS.install} className="btn btn-primary">Install on your server</a>
      <a href={LINKS.docs} className="btn btn-ghost">Read the docs</a>
    </div>
  );
}

export function Shot({ name, alt, className = "", eager = false }: { name: string; alt: string; className?: string; eager?: boolean }) {
  return (
    <div className={`shot ${className}`}>
      <img src={`/media/shots/${name}.webp`} alt={alt} width={1440} height={900} loading={eager ? "eager" : "lazy"} decoding="async" />
    </div>
  );
}

export function Reveal({ children, d = 0, kind, className = "", as: Tag = "div" }: { children: ReactNode; d?: number; kind?: "blur" | "scale"; className?: string; as?: "div" | "li" | "section" | "article" }) {
  return (
    <Tag data-reveal={kind ?? ""} className={className} style={{ ["--d" as string]: d }}>
      {children}
    </Tag>
  );
}

export function Heading({ title, body, center = false, className = "" }: { title: string; body?: string; center?: boolean; className?: string }) {
  return (
    <div className={`${center ? "mx-auto text-center" : ""} max-w-[720px] ${className}`}>
      <Reveal><h2 className="t-h2">{title}</h2></Reveal>
      {body && <Reveal d={1}><p className={`t-lead mt-4 ${center ? "mx-auto" : ""} max-w-[620px]`}>{body}</p></Reveal>}
    </div>
  );
}

const icons: Record<FeatureKey, ReactNode> = {
  deploy: <path d="M12 3v12m0 0 4.5-4.5M12 15l-4.5-4.5M4 17v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-2" transform="rotate(180 12 12)" />,
  releases: <path d="M12 7v5l3 2M21 12a9 9 0 1 1-3-6.7M21 4v4h-4" />,
  https: <><rect x="5" y="10" width="14" height="10" rx="2" /><path d="M8 10V7a4 4 0 0 1 8 0v3" /></>,
  databases: <><ellipse cx="12" cy="6" rx="7" ry="3" /><path d="M5 6v12c0 1.7 3.1 3 7 3s7-1.3 7-3V6M5 12c0 1.7 3.1 3 7 3s7-1.3 7-3" /></>,
  library: <><rect x="4" y="4" width="7" height="7" rx="1.5" /><rect x="13" y="4" width="7" height="7" rx="1.5" /><rect x="4" y="13" width="7" height="7" rx="1.5" /><rect x="13" y="13" width="7" height="7" rx="1.5" /></>,
  backups: <path d="M4 7h16M4 7v11a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7M4 7l2-3h12l2 3M10 12h4" />,
  agents: <path d="M5 8l4 4-4 4M12 17h7" />,
  ops: <><rect x="3" y="4" width="18" height="16" rx="2" /><path d="M7 9h6M7 13h10M7 17h4" /></>,
  teams: <><circle cx="9" cy="8" r="3.2" /><path d="M3 20c.5-3.3 3-5.2 6-5.2s5.5 1.9 6 5.2M16 5.2a3 3 0 0 1 0 5.6M18 14.8c1.7.6 2.8 2.4 3 5.2" /></>,
  updates: <path d="M20 12a8 8 0 0 1-14.3 4.9M4 12a8 8 0 0 1 14.3-4.9M18.5 3v4.2h-4.2M5.5 21v-4.2h4.2" />,
};

export function FeatureIcon({ k, className = "size-5" }: { k: FeatureKey; className?: string }) {
  return (
    <svg viewBox="0 0 24 24" className={className} fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      {icons[k]}
    </svg>
  );
}

export function FeatureGrid({ keys, cols = 3, boxed = true }: { keys: FeatureKey[]; cols?: 2 | 3 | 4; boxed?: boolean }) {
  const grid = { 2: "md:grid-cols-2", 3: "md:grid-cols-2 lg:grid-cols-3", 4: "md:grid-cols-2 lg:grid-cols-4" }[cols];
  return (
    <ul className={`grid gap-px overflow-hidden ${boxed ? "rounded-[14px] border border-line bg-line" : "gap-x-10 gap-y-10"} ${grid}`}>
      {keys.map((k, i) => {
        const f = feature(k);
        return (
          <Reveal as="li" key={k} d={i % cols} className={boxed ? "bg-bg p-7" : ""}>
            <FeatureIcon k={k} className="size-5 text-ink-2" />
            <h3 className="t-h3 mt-4 !text-[17px]">{f.title}</h3>
            <p className="t-body mt-2 !text-[15px]">{f.body}</p>
          </Reveal>
        );
      })}
    </ul>
  );
}

export function Facts({ className = "", divided = true }: { className?: string; divided?: boolean }) {
  return (
    <dl className={`grid grid-cols-2 lg:grid-cols-4 ${divided ? "divide-line lg:divide-x" : ""} ${className}`}>
      {FACTS.map(([value, label], i) => (
        <Reveal key={value} d={i} className={`py-6 ${divided ? "lg:px-8 lg:first:pl-0" : ""}`}>
          <dt className="font-display text-[34px] font-semibold tracking-[-0.04em] text-ink">{value}</dt>
          <dd className="mt-1 text-[14px] text-mute">{label}</dd>
        </Reveal>
      ))}
    </dl>
  );
}

export function LogoMarquee({ className = "" }: { className?: string }) {
  const logos = [
    ["n8n", "n8n.png"], ["Immich", "immich.svg"], ["Supabase", "supabase.svg"], ["Grafana", "grafana.svg"], ["Uptime Kuma", "uptime-kuma.svg"],
    ["Ghost", "ghost.svg"], ["Nextcloud", "nextcloud.svg"], ["Gitea", "gitea.svg"], ["MinIO", "minio.svg"], ["PocketBase", "pocketbase.svg"],
    ["Umami", "umami.svg"], ["Vaultwarden", "vaultwarden.svg"], ["Jellyfin", "jellyfin.svg"], ["Outline", "outline.svg"], ["Metabase", "metabase.svg"],
    ["Directus", "directus.svg"], ["WordPress", "wordpress.svg"], ["Mattermost", "mattermost.svg"], ["Open WebUI", "open-webui.svg"], ["Plane", "plane.svg"],
  ];
  const row = logos.map(([name, file]) => (
    <span key={name} className="flex shrink-0 items-center gap-2.5 px-6 text-[14px] text-ink-2">
      <img src={`/media/logos/${file}`} alt="" className="size-6 object-contain" loading="lazy" />
      {name}
    </span>
  ));
  return (
    <div className={`relative overflow-hidden [mask-image:linear-gradient(90deg,transparent,#000_12%,#000_88%,transparent)] ${className}`}>
      <div className="marquee">
        {row}
        <span aria-hidden className="flex">{row}</span>
      </div>
    </div>
  );
}

export function Faq({ columns = false }: { columns?: boolean }) {
  return (
    <div className={columns ? "grid gap-x-12 md:grid-cols-2" : ""}>
      {FAQ.map(([q, a], i) => (
        <Reveal key={q} d={i % 3}>
          <details className="group border-b border-line">
            <summary className="flex items-center justify-between gap-6 py-5 text-[16px] font-medium text-ink">
              {q}
              <svg viewBox="0 0 16 16" className="faq-icon size-4 shrink-0 text-mute" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden><path d="M8 3v10M3 8h10" /></svg>
            </summary>
            <p className="t-body -mt-1 pb-5 pr-8 !text-[15px]">{a}</p>
          </details>
        </Reveal>
      ))}
    </div>
  );
}

export function Footer() {
  const cols: [string, [string, string][]][] = [
    ["Product", [["Features", "#features"], ["Documentation", LINKS.docs], ["Install guide", LINKS.install], ["Changelog", LINKS.releases]]],
    ["Community", [["Discord", LINKS.discord], ["GitHub", LINKS.github], ["Report an issue", `${LINKS.github}/issues`]]],
    ["Project", [["MIT license", LINKS.license], ["Security policy", LINKS.security]]],
  ];
  return (
    <footer className="border-t border-line">
      <div className="wrap grid gap-12 py-16 md:grid-cols-[1.4fr_repeat(3,1fr)]">
        <div>
          <Wordmark />
          <p className="mt-4 max-w-[280px] text-[14px] text-mute">Deploy apps, databases and self-hosted services to your own server.</p>
        </div>
        {cols.map(([title, links]) => (
          <div key={title}>
            <div className="text-[13px] font-medium text-ink">{title}</div>
            <ul className="mt-4 space-y-2.5">
              {links.map(([label, href]) => (
                <li key={label}><a href={href} className="text-[14px] text-mute transition-colors hover:text-ink">{label}</a></li>
              ))}
            </ul>
          </div>
        ))}
      </div>
      <div className="wrap flex flex-wrap items-center justify-between gap-4 border-t border-line py-6 text-[13px] text-mute">
        <span>© 2026 ShipperKit</span>
        <a href={LINKS.license} className="hover:text-ink">MIT licensed</a>
      </div>
    </footer>
  );
}
