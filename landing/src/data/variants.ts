export type SectionKey =
  | "nav"
  | "hero"
  | "proof"
  | "product"
  | "deploy"
  | "agents"
  | "system"
  | "pricing"
  | "faq"
  | "cta"
  | "footer";

export type PageCombo = {
  id: string;
  name: string;
  note: string;
  nav: number;
  hero: number;
  proof: number;
  product: number;
  deploy: number;
  agents: number;
  system: number;
  pricing: number;
  faq: number;
  cta: number;
  footer: number;
};

export const landingPages: PageCombo[] = [
  {
    id: "01",
    name: "Launch",
    note: "Flagship conversion page with a split product hero, outcomes, console, three-step deploy, agents, reliability, and the free self-hosted offer.",
    nav: 1,
    hero: 1,
    proof: 1,
    product: 1,
    deploy: 1,
    agents: 1,
    system: 1,
    pricing: 1,
    faq: 1,
    cta: 1,
    footer: 1,
  },
  {
    id: "02",
    name: "Split",
    note: "Dual-column hero, included-features strip, sticky failure proof, one-command install, agent workflow, and a cost breakdown.",
    nav: 2,
    hero: 2,
    proof: 2,
    product: 2,
    deploy: 2,
    agents: 2,
    system: 2,
    pricing: 2,
    faq: 2,
    cta: 2,
    footer: 2,
  },
  {
    id: "03",
    name: "Cinematic",
    note: "Full-bleed product hero, no-DevOps objection handling, no-outage proof, sources map, and the framed ownership close.",
    nav: 3,
    hero: 3,
    proof: 3,
    product: 3,
    deploy: 3,
    agents: 3,
    system: 3,
    pricing: 3,
    faq: 3,
    cta: 3,
    footer: 3,
  },
  {
    id: "04",
    name: "Command",
    note: "Command-first hero, source strip, release receipts, fast-deploy proof, plain-English agents, and a single-line close.",
    nav: 4,
    hero: 4,
    proof: 4,
    product: 4,
    deploy: 4,
    agents: 4,
    system: 4,
    pricing: 4,
    faq: 4,
    cta: 4,
    footer: 4,
  },
  {
    id: "05",
    name: "Owned",
    note: "Ownership-led hero, what-you-keep proof, everyday console, no-stack deploy, scoped agent keys, and a $0 price card.",
    nav: 5,
    hero: 5,
    proof: 5,
    product: 5,
    deploy: 5,
    agents: 5,
    system: 5,
    pricing: 5,
    faq: 5,
    cta: 5,
    footer: 5,
  },
  {
    id: "06",
    name: "Agent",
    note: "Agent-first hero, launch checklist, screen gallery, review-before-deploy, no-SSH agent path, and a terminal close.",
    nav: 6,
    hero: 6,
    proof: 6,
    product: 6,
    deploy: 6,
    agents: 6,
    system: 6,
    pricing: 6,
    faq: 6,
    cta: 6,
    footer: 6,
  },
  {
    id: "07",
    name: "Signal",
    note: "Agent hero with source proof, everyday console, source matrix, plain-English agents, and the ownership receipt.",
    nav: 1,
    hero: 6,
    proof: 4,
    product: 5,
    deploy: 3,
    agents: 4,
    system: 3,
    pricing: 4,
    faq: 2,
    cta: 5,
    footer: 4,
  },
  {
    id: "08",
    name: "Receipts",
    note: "Ownership hero with public receipts, failure proof, release receipts, no-stack deploy, and the agent guardrail section.",
    nav: 3,
    hero: 5,
    proof: 3,
    product: 2,
    deploy: 5,
    agents: 3,
    system: 6,
    pricing: 5,
    faq: 1,
    cta: 6,
    footer: 5,
  },
  {
    id: "09",
    name: "Night Shift",
    note: "Split hero, launch checklist, no-outage proof, review-before-deploy, scoped keys, and a cost breakdown.",
    nav: 5,
    hero: 2,
    proof: 6,
    product: 3,
    deploy: 6,
    agents: 5,
    system: 4,
    pricing: 2,
    faq: 4,
    cta: 2,
    footer: 6,
  },
  {
    id: "10",
    name: "Open Exit",
    note: "Command hero, included-features proof, everyday console, one-command install, no-SSH agents, and a quiet ownership close.",
    nav: 4,
    hero: 4,
    proof: 2,
    product: 5,
    deploy: 2,
    agents: 6,
    system: 5,
    pricing: 6,
    faq: 5,
    cta: 4,
    footer: 2,
  },
];

export const sectionOrder: SectionKey[] = [
  "nav",
  "hero",
  "proof",
  "product",
  "deploy",
  "agents",
  "system",
  "pricing",
  "faq",
  "cta",
  "footer",
];

export const sectionLabels: Record<SectionKey, string> = {
  nav: "Navigation",
  hero: "Hero",
  proof: "Outcomes",
  product: "Console",
  deploy: "How it works",
  agents: "Agents",
  system: "Reliability",
  pricing: "Pricing",
  faq: "FAQ",
  cta: "Closing",
  footer: "Footer",
};

export const variantNames: Record<SectionKey, string[]> = {
  nav: ["Glass bar", "Overlay", "Minimal", "Floating", "Quiet", "Editorial"],
  hero: ["Launch", "Split", "Cinematic", "Command", "Owned", "Agent"],
  proof: ["Outcomes", "Included", "No DevOps", "Sources", "What you keep", "Checklist"],
  product: ["Console", "Failure", "No outage", "Receipts", "Everyday", "Gallery"],
  deploy: ["Three steps", "One command", "Sources", "Fast", "No stack", "Review"],
  agents: ["Agent path", "Plan, ship, explain", "Guardrails", "Plain English", "Scoped keys", "No SSH"],
  system: ["Failure safe", "Rollback", "Handled", "Timeline", "Ownership", "Truth"],
  pricing: ["$0", "One bill", "Compare", "Included", "Free", "Your bill"],
  faq: ["Accordion", "Public story", "Practical", "Boundary", "Console", "Short answers"],
  cta: ["Orbit", "One command", "Everything", "Single line", "Checklist", "Terminal"],
  footer: ["Directory", "Minimal", "Wordmark", "Quiet bar", "Tree", "Split"],
};

export const sectionBlurbs: Record<SectionKey, string[]> = {
  nav: [
    "Pinned glass navigation with product links and a single install action.",
    "Overlay navigation for a clean, product-led first viewport.",
    "Minimal bar with docs and install, no visual noise.",
    "Floating navigation card that sits above the hero.",
    "Quiet full-width bar with the same focus as the console.",
    "Editorial bar with room for the brand and GitHub.",
  ],
  hero: [
    "Split hero with a clear promise, install command, and live console screenshot.",
    "Dual-column hero with a screenshot, backup card, and SSL proof.",
    "Cinematic full-bleed console shot with a three-outcome strip.",
    "Command-first install story with a terminal and three steps.",
    "Ownership-led hero with a what-you-keep card and product band.",
    "Agent-first hero with a real agent session and safety checks.",
  ],
  proof: [
    "Three outcomes: a URL, a way back, and data you keep.",
    "Four included features between git push and live traffic.",
    "The no-DevOps objection answered with checks and a screenshot.",
    "A source marquee for repos, folders, images, and Compose.",
    "Six things that stay yours after the install.",
    "A six-item launch checklist that stays checked.",
  ],
  product: [
    "A console bento with live app status, deploy time, logs, and recovery.",
    "Sticky failure proof beside the failed release and safe old release.",
    "A single failed deploy with floating callouts, no outage.",
    "Release receipts beside the full deployments surface.",
    "Everyday settings, domains, and backups in one asymmetric grid.",
    "A real screen gallery with the app list as the lead image.",
  ],
  deploy: [
    "Three steps: install, connect, deploy.",
    "One command on the left, a real install transcript on the right.",
    "Six supported sources with no special project layout.",
    "Deploy progress with the old release held safely behind it.",
    "The objection handled: you do not need a deployment stack.",
    "Review before you deploy, with the first-release screen.",
  ],
  agents: [
    "The full agent path with a real terminal and scoped access.",
    "Plan, ship, explain: the three jobs an agent can do.",
    "The same guardrails for humans, developers, and agents.",
    "A plain-English agent prompt answered with real checks.",
    "One key per agent, limited by app and action.",
    "No SSH key, just a narrow deploy path that can be revoked.",
  ],
  system: [
    "Failed deploys are stopped before traffic moves.",
    "Release history and a reviewed way back.",
    "HTTPS, backups, and health handled in the same place.",
    "A timeline that shows the old release staying online.",
    "Ownership as reliability: nobody can turn off your server.",
    "A console that shows the real state, including what is unknown.",
  ],
  pricing: [
    "Everything you need, nothing you rent, with a $0 software price.",
    "One monthly bill: your server, your domain, and free software.",
    "A direct comparison with managed platforms and hand-rolled setups.",
    "What free actually includes, with no feature paywall.",
    "Free as the business model, not a trial.",
    "Your server, your bill, your call.",
  ],
  faq: [
    "Sticky intro beside a native details accordion.",
    "Large public answers with room to read.",
    "Six practical answers in a dense grid.",
    "Five boundary questions beside the operations guide.",
    "A console-style FAQ with documentation references.",
    "Two columns of short answers for fast scanning.",
  ],
  cta: [
    "Centered orbit close with install command and one primary action.",
    "Split close with a real install transcript.",
    "The framed ownership close with the oversized wordmark.",
    "A single quiet strip with the install command.",
    "A compact checklist close with four steps.",
    "A terminal close with a blinking cursor.",
  ],
  footer: [
    "Full directory with product, docs, and project links.",
    "A minimal single row for quiet endings.",
    "Oversized wordmark above a compact link set.",
    "A quiet status bar with the essentials.",
    "A repository tree and three documentation columns.",
    "A split ownership statement with the full link set.",
  ],
};

export function pagesForSectionVariant(section: SectionKey, variant: number): string[] {
  return landingPages
    .filter((page) => page[section] === variant)
    .map((page) => page.id);
}
