import { HugeiconsIcon } from "@hugeicons/react";
import {
  CheckmarkCircle02Icon,
  ContainerIcon,
  FileCodeIcon,
  FolderGitIcon,
  GitBranchIcon,
  GithubIcon,
  PackageIcon,
} from "@hugeicons/core-free-icons";
import {
  ActionPrimary,
  ActionSecondary,
  Check,
  CodeWindow,
  Container,
  Screenshot,
  Section,
  SectionIntro,
  Status,
  TerminalInstall,
} from "@/components/primitives";
import { cn } from "@/lib/utils";

function Deploy1() {
  const steps = [
    ["01", "Install", "Run one command on a fresh Ubuntu server and open your dashboard."],
    ["02", "Connect", "Pick a local folder, a repo, an image, or a Compose file."],
    ["03", "Deploy", "Get a live HTTPS URL, readable logs, and a release you can restore."],
  ] as const;
  return (
    <Section id="how-it-works" className="section-rule">
      <Container>
        <SectionIntro
          title={<>Three steps between your code and a live URL.</>}
          body="No build server to provision. No reverse proxy to learn. No platform account to create."
        />
        <div className="mt-16 grid gap-px overflow-hidden rounded-2xl border border-white/[0.1] bg-white/[0.08] md:grid-cols-3">
          {steps.map(([index, title, body]) => (
            <div key={index} className="bg-[#080909] p-8 md:p-10">
              <div className="mono text-[15px] text-[#85867f]">{index}</div>
              <h3 className="mt-10 text-[30px] font-[520] tracking-[-0.04em] text-[#ecece7]">{title}</h3>
              <p className="mt-3 text-[16px] leading-[1.7] text-[#969791]">{body}</p>
            </div>
          ))}
        </div>
        <div className="mt-12 flex justify-end">
          <ActionPrimary href="#get-shipper" />
        </div>
      </Container>
    </Section>
  );
}

function Deploy2() {
  return (
    <Section id="how-it-works" className="section-rule overflow-hidden">
      <Container>
        <div className="grid items-start gap-16 lg:grid-cols-[0.9fr_1.1fr]">
          <div>
            <SectionIntro
              title={<>One command to your first deploy.</>}
              body="Install Shipper on a fresh Ubuntu host, claim the dashboard, and point it at your code. The installer handles the proxy and the plumbing."
            />
            <div className="mt-9">
              <ActionPrimary href="#get-shipper" />
            </div>
            <div className="mt-8 space-y-4">
              <Check>Fresh Ubuntu 24.04 host.</Check>
              <Check>A domain when you are ready for public HTTPS.</Check>
              <Check>Five minutes before the first release.</Check>
            </div>
            <div className="mt-9">
              <ActionSecondary href="https://github.com/matyas-toth/shipperkit/tree/main/docs/installation.md" external>
                Read the installation guide
              </ActionSecondary>
            </div>
          </div>
          <div className="overflow-hidden rounded-2xl border border-white/[0.12] bg-[#080909] shadow-[0_40px_100px_-40px_rgba(0,0,0,1)]">
            <div className="screen-bar">
              <div className="flex items-center gap-2">
                <span className="screen-dot" />
                <span className="screen-dot" />
                <span className="screen-dot" />
              </div>
              <span className="mono ml-2 text-[14px] text-[#898a85]">install</span>
            </div>
            <div className="p-6 md:p-8">
              <div className="mono space-y-4 text-[14px] leading-[1.8]">
                <div className="flex items-start gap-2">
                  <span className="text-[#85867f]">$ </span>
                  <TerminalInstall className="text-[#ecece7]" />
                </div>
                <div className="text-[#79e3a6]">Installed. Dashboard is ready on port 8080.</div>
                <div className="text-[#a8a9a3]">
                  Open the dashboard, claim your administrator account, and keep the
                  setup token private.
                </div>
                <div className="border-t border-white/[0.08] pt-4">
                  <span className="text-[#85867f]">$ </span>
                  <span className="text-[#ecece7]">shipper plan . --name my-app</span>
                </div>
                <div className="text-[#a8a9a3]">Source detected. Review the plan, then deploy.</div>
                <div className="text-[#79e3a6]">Live at https://my-app.example.com</div>
              </div>
              <div className="mt-8 flex items-center justify-between border-t border-white/[0.08] pt-5">
                <Status label="Dashboard ready" />
              </div>
            </div>
          </div>
        </div>
      </Container>
    </Section>
  );
}

function Deploy3() {
  const sources = [
    ["Local folder", "No Git account. Drop in a project and ship it.", FolderGitIcon],
    ["Public repo", "Point at a Git URL and pick a branch.", GitBranchIcon],
    ["Private GitHub", "Connect once, then deploy the private repos.", GithubIcon],
    ["Docker image", "Bring a prebuilt image from any registry.", ContainerIcon],
    ["Compose file", "Keep your services, volumes, and network.", PackageIcon],
    ["Inline Dockerfile", "Paste a definition and use the same builder.", FileCodeIcon],
  ] as const;
  return (
    <Section id="how-it-works" className="section-rule">
      <Container>
        <SectionIntro
          title={<>Bring your repo, folder, or image.</>}
          body="Shipper fits the source you already have. There is no special project layout and no new config language."
        />
        <div className="surface mt-11 grid overflow-hidden rounded-2xl md:grid-cols-2 lg:grid-cols-3">
          {sources.map(([title, body, Icon]) => (
            <div key={title} className="group flex gap-4 border-b border-white/[0.07] p-5 transition-colors duration-300 last:border-b-0 hover:bg-white/[0.015] sm:p-6 md:block md:border-r md:[&:nth-child(2n)]:border-r-0 lg:[&:nth-child(2n)]:border-r lg:[&:nth-child(3n)]:border-r-0 lg:[&:nth-last-child(-n+3)]:border-b-0 md:[&:nth-last-child(-n+2)]:border-b-0">
              <span className="flex size-10 shrink-0 items-center justify-center rounded-xl border border-white/[0.13] bg-white/[0.05]">
                <HugeiconsIcon icon={Icon} size={20} strokeWidth={1.6} className="text-[#dcdcd6]" />
              </span>
              <div className="min-w-0 md:mt-5">
                <h3 className="text-[17px] font-[520] tracking-[-0.03em] text-[#ecece7] md:text-[20px]">{title}</h3>
                <p className="mt-1 text-[14px] leading-[1.6] text-[#a2a39d] md:mt-2 md:text-[15px] md:leading-[1.7]">{body}</p>
              </div>
            </div>
          ))}
        </div>
      </Container>
    </Section>
  );
}

function Deploy4() {
  return (
    <Section id="how-it-works" className="section-rule">
      <Container>
        <div className="grid items-center gap-16 lg:grid-cols-[0.88fr_1.12fr]">
          <div>
            <SectionIntro
              title={<>From code to live URL, without the waiting.</>}
              body="Shipper streams the deploy while it happens. A simple app can be live in under a minute, and the last good release stays online the whole time."
            />
            <div className="mt-9 space-y-4">
              <Check>Build and verify before traffic moves.</Check>
              <Check>Read the log while it is being written.</Check>
              <Check>Keep the old release if the new one fails.</Check>
            </div>
            <div className="mt-9">
              <ActionPrimary href="#get-shipper" />
            </div>
          </div>
          <div className="overflow-hidden rounded-2xl border border-white/[0.1] bg-[#080909] p-6 md:p-8">
            <div className="flex items-end justify-between border-b border-white/[0.08] pb-6">
              <div>
                <div className="text-[15px] text-[#8d8e88]">Release in progress</div>
                <div className="mt-2 text-[28px] font-[530] tracking-[-0.04em] text-[#ecece7]">store.example.com</div>
              </div>
              <Status label="Live" pulse />
            </div>
            <div className="mt-8 space-y-4">
              {[
                ["Build", "Completed", "success"],
                ["Candidate check", "HTTP 200", "success"],
                ["Routes", "HTTPS active", "success"],
                ["Traffic", "Switch when ready", "muted"],
              ].map(([label, value, tone], index) => (
                <div key={label} className="flex items-center gap-4">
                  <span className={cn("flex size-9 items-center justify-center rounded-full border", tone === "success" ? "border-[#79e3a6]/25 bg-[#79e3a6]/[0.06] text-[#79e3a6]" : "border-white/[0.12] text-[#797a75]")}>
                    {tone === "success" ? (
                      <HugeiconsIcon icon={CheckmarkCircle02Icon} size={17} strokeWidth={1.6} />
                    ) : (
                      <span className="mono text-[14px]">{index + 1}</span>
                    )}
                  </span>
                  <div className="flex-1 border-b border-white/[0.07] pb-3">
                    <div className="flex items-center justify-between gap-4">
                      <span className="text-[16px] text-[#dcdcd6]">{label}</span>
                      <span className={cn("text-[14px]", tone === "success" ? "text-[#79e3a6]" : "text-[#797a75]")}>{value}</span>
                    </div>
                  </div>
                </div>
              ))}
            </div>
            <div className="mt-8 flex items-center justify-between border-t border-white/[0.08] pt-5">
              <span className="text-[14px] text-[#8d8e88]">Previous release stays available</span>
              <span className="mono text-[14px] text-[#ecece7]">4f8ac21d</span>
            </div>
          </div>
        </div>
      </Container>
    </Section>
  );
}

function Deploy5() {
  return (
    <Section id="how-it-works" className="section-rule">
      <Container>
        <div className="grid items-center gap-16 lg:grid-cols-[1.02fr_0.98fr]">
          <div>
            <SectionIntro
              title={<>You don't need a deployment stack.</>}
              body="No Kubernetes, no YAML maze, no separate proxy dashboard. Install one thing, then use the console or the CLI."
            />
            <div className="mt-9 grid gap-3 sm:grid-cols-2">
              {[
                "Deploy from a folder",
                "HTTPS with one click",
                "Readable release history",
                "Rollback without drama",
              ].map((item) => (
                <div key={item} className="flex items-center gap-3 rounded-xl border border-white/[0.09] bg-[#080909] px-5 py-4 text-[16px] text-[#c3c4be]">
                  <HugeiconsIcon icon={CheckmarkCircle02Icon} size={18} strokeWidth={1.6} className="shrink-0 text-[#79e3a6]" />
                  {item}
                </div>
              ))}
            </div>
            <div className="mt-9">
              <ActionPrimary href="#get-shipper" />
            </div>
          </div>
          <CodeWindow
            title="behind the scenes"
            lines={[
              <><span className="text-[#79e3a6]">source</span> resolved from your folder</>,
              <><span className="text-[#79e3a6]">build</span> image created and verified</>,
              <><span className="text-[#79e3a6]">candidate</span> passed the HTTP check</>,
              <><span className="text-[#79e3a6]">route</span> switched after the proxy acknowledged</>,
              <><span className="text-[#79e3a6]">release</span> stored with a way back</>,
              <><span className="text-[#797a75]">you only see the useful parts</span></>,
            ]}
          />
        </div>
      </Container>
    </Section>
  );
}

function Deploy6() {
  return (
    <Section id="how-it-works" className="section-rule">
      <Container>
        <div className="grid items-center gap-16 lg:grid-cols-[0.9fr_1.1fr]">
          <div>
            <SectionIntro
              title={<>You see what will happen before it happens.</>}
              body="Review the source, the build method, the domain, and the effects. Apply when it looks right, and keep the old release if it doesn't."
            />
            <div className="mt-9 space-y-4">
              <Check>A review step for the first deploy.</Check>
              <Check>Explicit effects before a change.</Check>
              <Check>One action when you are ready.</Check>
            </div>
            <div className="mt-9 flex flex-wrap gap-3">
              <ActionPrimary href="#get-shipper" />
              <ActionSecondary href="#faq">Read the FAQ</ActionSecondary>
            </div>
          </div>
          <Screenshot
            src="/shots/create.png"
            alt="Shipper new application screen with source, build method, and runtime setup"
          />
        </div>
      </Container>
    </Section>
  );
}

export function Deploy({ variant = 1 }: { variant?: number }) {
  switch (variant) {
    case 2:
      return <Deploy2 />;
    case 3:
      return <Deploy3 />;
    case 4:
      return <Deploy4 />;
    case 5:
      return <Deploy5 />;
    case 6:
      return <Deploy6 />;
    default:
      return <Deploy1 />;
  }
}
