export type NextVariant = { id: string; name: string; note: string; media: string };

export const NEXT_VARIANTS: NextVariant[] = [
  { id: "01", name: "Horizon", media: "clouds", note: "Full-bleed dithered sky behind a centered headline, with the dashboard overlapping the fold." },
  { id: "02", name: "Split", media: "hills", note: "The brand image layout as a hero, then alternating feature rows that each carry a live widget." },
  { id: "03", name: "Console", media: "aurora", note: "A tabbed product tour over real screenshots, sitting on a dithered aurora band." },
  { id: "04", name: "Platform", media: "lake", note: "The most corporate take: facts up front, a live status board, a bento of capabilities and a security section." },
  { id: "05", name: "Bento", media: "forest", note: "A compact hero followed by one large bento grid mixing dithered tiles and interactive widgets." },
  { id: "06", name: "Walkthrough", media: "valley", note: "A sticky scroll story from install to backups, with the widget for each step beside it." },
  { id: "07", name: "Terminal", media: "moon", note: "Leads with the installer itself, including a host and architecture picker, then the dashboard and the CLI." },
  { id: "08", name: "Landscape", media: "peak", note: "An editorial full-height video hero with the headline at the bottom left, then calm, wide sections." },
  { id: "09", name: "Ownership", media: "woods", note: "Frames Shipper against renting a platform, with an honest side by side of what changes." },
  { id: "10", name: "Triptych", media: "clouds", note: "Three dithered panels for apps, databases and services, then the server capacity playground." },
  { id: "11", name: "Night", media: "aurora-stars", note: "Round-dot aurora, centered and quiet, with the traffic router as the centerpiece." },
  { id: "12", name: "Quickstart", media: "lake", note: "Reads like a product docs home: three steps with code, then cards that lead into the docs." },
  { id: "13", name: "Showcase", media: "sunset", note: "Screenshot-led. The dashboard sits on a dithered frame like the brand images, then a tour of each screen." },
  { id: "14", name: "Reliability", media: "storm", note: "Built around safe releases: the breakable pipeline in the hero, then rollback, backups and diagnosis." },
  { id: "15", name: "Minimal", media: "hills", note: "Lots of space, one idea per screen, and thin dithered bands between sections." },
];
