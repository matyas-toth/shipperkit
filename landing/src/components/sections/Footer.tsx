import { HugeiconsIcon } from "@hugeicons/react";
import { GithubIcon } from "@hugeicons/core-free-icons";
import { Container, Logo, Status } from "@/components/primitives";

const repo = "https://github.com/matyas-toth/shipperkit";
const docs = "/docs/";
const doc = (path: string) => `${docs}${path}`;

const groups = [
  {
    title: "Product",
    links: [
      ["Deploy", "#how-it-works"],
      ["Console", "#product"],
      ["Agents", "#agents"],
      ["Pricing", "#pricing"],
    ],
  },
  {
    title: "Docs",
    links: [
      ["Installation", doc("install/")],
      ["Recovery", doc("recovery/")],
      ["Troubleshooting", doc("diagnostics/")],
      ["API", doc("api-tokens/")],
    ],
  },
  {
    title: "Project",
    links: [
      ["GitHub", repo],
      ["Releases", `${repo}/releases`],
      ["Security", `${repo}/blob/master/SECURITY.md`],
      ["Contributing", `${repo}/blob/master/CONTRIBUTING.md`],
    ],
  },
];

function NavColumns() {
  return (
    <div className="grid grid-cols-3 gap-6 sm:gap-10 md:gap-14">
      {groups.map((group) => (
        <div key={group.title}>
          <div className="text-[14px] font-[500] text-[#797a75]">{group.title}</div>
          <div className="mt-5 space-y-3.5">
            {group.links.map(([label, href]) => (
              <a key={label} href={href} className="block text-[15px] text-[#a2a39d] transition-colors hover:text-white">
                {label}
              </a>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}

function Footer1() {
  return (
    <footer className="border-t border-white/[0.08] bg-[#060707]">
      <Container>
        <div className="grid gap-14 py-16 md:grid-cols-[1fr_1.5fr] md:py-20">
          <div>
            <Logo />
            <p className="mt-5 max-w-[36ch] text-[15px] leading-[1.7] text-[#797a75]">
              Self-hosted deployment for the server you already own.
            </p>
            <div className="mt-7 flex items-center gap-4">
              <Status label="Open source" />
              <span className="text-[14px] text-[#797a75]">MIT</span>
            </div>
          </div>
          <NavColumns />
        </div>
        <div className="flex flex-col gap-4 border-t border-white/[0.08] py-7 sm:flex-row sm:items-center sm:justify-between">
          <span className="text-[14px] text-[#797a75]">Shipper contributors</span>
          <div className="flex items-center gap-6">
            <a href={repo} className="flex items-center gap-2 text-[14px] text-[#85867f] hover:text-white">
              <HugeiconsIcon icon={GithubIcon} size={15} strokeWidth={1.6} />
              Source
            </a>
            <a href="/variants" className="text-[14px] text-[#85867f] hover:text-white">Landing lab</a>
          </div>
        </div>
      </Container>
    </footer>
  );
}

function Footer2() {
  return (
    <footer className="border-t border-white/[0.08] bg-[#060707]">
      <Container>
        <div className="flex flex-col gap-6 py-8 md:flex-row md:items-center md:justify-between">
          <Logo size={20} />
          <div className="flex flex-wrap items-center gap-x-7 gap-y-3">
            {[
              ["Docs", docs],
              ["GitHub", repo],
              ["License", `${repo}/blob/master/LICENSE`],
              ["Landing lab", "/variants"],
            ].map(([label, href]) => (
              <a key={label} href={href} className="text-[14px] text-[#8d8e88] transition-colors hover:text-white">
                {label}
              </a>
            ))}
          </div>
        </div>
      </Container>
    </footer>
  );
}

function Footer3() {
  return (
    <footer className="border-t border-white/[0.08] bg-[#060707]">
      <Container>
        <div className="grid gap-10 py-14 md:grid-cols-[1fr_auto]">
          <div className="max-w-[440px]">
            <Logo />
            <p className="mt-5 text-[16px] leading-[1.7] text-[#8d8e88]">
              Deploy apps, keep the release history, and recover the data. All on
              infrastructure you already control.
            </p>
          </div>
          <NavColumns />
        </div>
        <div className="flex flex-col gap-3 border-t border-white/[0.08] py-6 sm:flex-row sm:items-center sm:justify-between">
          <span className="text-[14px] text-[#8d8e88]">Shipper contributors · MIT</span>
          <div className="flex items-center gap-6">
            <a href={repo} className="flex items-center gap-2 text-[14px] text-[#8d8e88] transition-colors hover:text-white">
              <HugeiconsIcon icon={GithubIcon} size={15} strokeWidth={1.6} />
              Source
            </a>
            <a href={docs} className="text-[14px] text-[#8d8e88] transition-colors hover:text-white">Docs</a>
          </div>
        </div>
      </Container>
    </footer>
  );
}

function Footer4() {
  return (
    <footer className="border-t border-white/[0.08] bg-[#060707]">
      <Container>
        <div className="flex flex-col gap-5 py-6 lg:flex-row lg:items-center lg:justify-between">
          <div className="flex flex-wrap items-center gap-5">
            <Logo size={18} />
            <span className="text-[14px] text-[#797a75]">Open source · MIT</span>
          </div>
          <div className="flex flex-wrap items-center gap-x-6 gap-y-3">
            {[
              ["Docs", docs],
              ["GitHub", repo],
              ["API", doc("api-tokens/")],
              ["Landing lab", "/variants"],
            ].map(([label, href]) => (
              <a key={label} href={href} className="text-[14px] text-[#85867f] transition-colors hover:text-white">
                {label}
              </a>
            ))}
          </div>
        </div>
      </Container>
    </footer>
  );
}

function Footer5() {
  return (
    <footer className="border-t border-white/[0.08] bg-[#060707]">
      <Container>
        <div className="grid gap-12 py-14 md:grid-cols-[1fr_1.4fr]">
          <div>
            <Logo />
            <div className="mono mt-8 space-y-2 text-[14px] text-[#797a75]">
              <div className="text-[#a2a39d]">shipper/</div>
              <div>├── <a href={docs} className="hover:text-white">docs/</a></div>
              <div>├── <a href={repo} className="hover:text-white">internal/</a></div>
              <div>├── <a href={repo} className="hover:text-white">web/</a></div>
              <div>├── <a href="/variants" className="hover:text-white">landing/</a></div>
              <div>└── <a href={`${repo}/blob/master/LICENSE`} className="hover:text-white">LICENSE</a></div>
            </div>
          </div>
          <div className="grid grid-cols-2 gap-10 sm:grid-cols-3">
            {[
              ["Read", [["Installation", doc("install/")], ["First deploy", doc("first-deploy/")], ["Recovery", doc("recovery/")], ["API", doc("api-tokens/")]]],
              ["Operate", [["Compose", doc("move-an-app/")], ["Backups", doc("backups/")], ["Updates", doc("updates/")], ["Routing", doc("domains-tls/")]]],
              ["Project", [["GitHub", repo], ["Releases", `${repo}/releases`], ["Security", `${repo}/blob/master/SECURITY.md`], ["Landing lab", "/variants"]]],
            ].map(([title, links]) => (
              <div key={title as string}>
                <div className="text-[14px] font-[500] text-[#797a75]">{title as string}</div>
                <div className="mt-5 space-y-3.5">
                  {(links as string[][]).map(([label, href]) => (
                    <a key={label} href={href} className="block text-[15px] text-[#a2a39d] hover:text-white">{label}</a>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>
        <div className="flex flex-col gap-3 border-t border-white/[0.08] py-6 sm:flex-row sm:items-center sm:justify-between">
          <span className="text-[14px] text-[#797a75]">Shipper contributors · MIT</span>
          <span className="text-[14px] text-[#797a75]">Built for servers you own</span>
        </div>
      </Container>
    </footer>
  );
}

function Footer6() {
  return (
    <footer className="border-t border-white/[0.08] bg-[#060707]">
      <Container>
        <div className="grid gap-14 py-16 lg:grid-cols-[1.05fr_0.95fr]">
          <div>
            <Logo />
            <p className="mt-7 max-w-[26ch] text-[28px] font-[520] leading-[1.2] tracking-[-0.04em] text-[#dcdcd6] md:text-[36px]">
              Applications, releases, backups, and recovery stay on your server.
            </p>
            <div className="mt-8 flex flex-wrap items-center gap-3">
              <a href={repo} className="inline-flex h-11 items-center gap-2.5 rounded-lg border border-white/[0.12] bg-white/[0.04] px-4 text-[14px] text-[#dcdcd6] transition-colors hover:bg-white/[0.08]">
                <HugeiconsIcon icon={GithubIcon} size={16} strokeWidth={1.6} />
                GitHub
              </a>
              <a href={docs} className="inline-flex h-11 items-center gap-2.5 rounded-lg border border-white/[0.09] px-4 text-[14px] text-[#a2a39d] transition-colors hover:text-white">
                Docs
              </a>
              <a href="/variants" className="text-[14px] text-[#85867f] hover:text-white">Landing lab</a>
            </div>
          </div>
          <NavColumns />
        </div>
        <div className="flex flex-col gap-3 border-t border-white/[0.08] py-6 sm:flex-row sm:items-center sm:justify-between">
          <span className="text-[14px] text-[#797a75]">Shipper contributors · MIT</span>
          <span className="text-[14px] text-[#797a75]">Built for servers you own</span>
        </div>
      </Container>
    </footer>
  );
}

export function Footer({ variant = 1 }: { variant?: number }) {
  switch (variant) {
    case 2:
      return <Footer2 />;
    case 3:
      return <Footer3 />;
    case 4:
      return <Footer4 />;
    case 5:
      return <Footer5 />;
    case 6:
      return <Footer6 />;
    default:
      return <Footer1 />;
  }
}
