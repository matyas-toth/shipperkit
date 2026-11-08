import { HugeiconsIcon } from "@hugeicons/react";
import {
  ArrowDown01Icon,
  CheckmarkCircle02Icon,
  GithubIcon,
  TerminalIcon,
} from "@hugeicons/core-free-icons";
import {
  ActionPrimary,
  ActionSecondary,
  AsciiField,
  CommandBar,
  Container,
  Screenshot,
  Sparkline,
  Status,
  TerminalInstall,
} from "@/components/primitives";
import { cn } from "@/lib/utils";

const docs = "https://github.com/matyas-toth/shipperkit/tree/main/docs";

function Actions({ secondary = "See how it works" }: { secondary?: string }) {
  return (
    <div className="flex flex-wrap items-center gap-3">
      <ActionPrimary href="#get-shipper" />
      <ActionSecondary href={secondary === "Read the docs" ? docs : "#how-it-works"} external={secondary === "Read the docs"}>
        {secondary}
      </ActionSecondary>
    </div>
  );
}

function LiveCard({ className }: { className?: string }) {
  return (
    <div
      className={cn(
        "animate-float rounded-xl border border-white/[0.14] bg-[#0b0c0c]/92 p-5 shadow-[0_30px_80px_-24px_rgba(0,0,0,0.95)] backdrop-blur-xl",
        className,
      )}
    >
      <div className="flex items-center justify-between gap-8">
        <Status label="Deployed" pulse />
        <span className="mono text-[14px] text-[#85867f]">14s</span>
      </div>
      <div className="mt-4 text-[16px] font-[500] text-[#ecece7]">store.example.com</div>
      <div className="mt-1.5 flex items-center gap-2 text-[14px] text-[#8d8e88]">
        <span className="size-1.5 rounded-full bg-[#79e3a6]" />
        HTTPS active
        <span className="mx-1 h-3 w-px bg-white/[0.12]" />
        Release 4f8ac21d
      </div>
      <div className="mt-4 h-10">
        <Sparkline height={40} points="0,30 20,26 40,28 60,18 80,22 100,13 120,16 132,7" />
      </div>
    </div>
  );
}

function Hero1() {
  return (
    <section className="relative overflow-hidden pt-32 md:pt-36">
      <div className="absolute inset-0 -z-10 bg-[radial-gradient(ellipse_65%_55%_at_75%_0%,rgba(255,255,255,0.1),transparent_62%)]" />
      <div className="grid-lines absolute inset-x-0 top-0 -z-10 h-[760px] opacity-60" />
      <AsciiField className="right-[-8%] top-24 -z-10 hidden lg:block" cols={64} rows={30} seed={12} opacity={0.09} />
      <Container>
        <div className="grid items-center gap-14 lg:grid-cols-[0.95fr_1.05fr] lg:gap-10">
          <div className="relative z-10 animate-rise">
            <h1 className="display max-w-[13ch] text-[clamp(3.2rem,5.2vw,5.2rem)] text-[#f4f4f0]">
              Deploy your app to your own server, in minutes.
            </h1>
            <p className="mt-7 max-w-[52ch] text-[18px] leading-[1.7] text-[#a9aaa4]">
              The self-hosted deploy console for repos, folders, and Docker images.
              You keep the server, the data, and the release history.
            </p>
            <div id="install" className="mt-9 scroll-mt-28">
              <Actions />
            </div>
            <div className="mt-8 max-w-[590px]">
              <CommandBar />
            </div>
            <div className="mt-7 flex flex-wrap items-center gap-x-6 gap-y-3 text-[14px] text-[#8d8e88]">
              <span className="inline-flex items-center gap-2">
                <HugeiconsIcon icon={CheckmarkCircle02Icon} size={16} strokeWidth={1.6} className="text-[#79e3a6]" />
                Free to self-host
              </span>
              <span className="inline-flex items-center gap-2">
                <HugeiconsIcon icon={CheckmarkCircle02Icon} size={16} strokeWidth={1.6} className="text-[#79e3a6]" />
                No platform fee
              </span>
              <span className="inline-flex items-center gap-2">
                <HugeiconsIcon icon={CheckmarkCircle02Icon} size={16} strokeWidth={1.6} className="text-[#79e3a6]" />
                Your data stays yours
              </span>
            </div>
          </div>

          <div className="relative lg:pl-4 animate-scale-in [animation-delay:120ms]">
            <div className="absolute -inset-8 -z-10 rounded-[50%] bg-[radial-gradient(ellipse_at_center,rgba(255,255,255,0.09),transparent_66%)] blur-2xl" />
            <Screenshot
              src="/shots/applications.png"
              alt="Shipper applications dashboard with live apps, domains, and deployment status"
              priority
              frameClassName="[transform:perspective(1800px)_rotateY(-4deg)_rotateX(1deg)] origin-left"
            />
            <LiveCard className="absolute -bottom-10 left-2 w-[300px] sm:left-6" />
          </div>
        </div>
      </Container>
    </section>
  );
}

function Hero2() {
  return (
    <section className="relative overflow-hidden pt-36 md:pt-40">
      <div className="absolute inset-0 -z-10 bg-[radial-gradient(ellipse_58%_55%_at_92%_0%,rgba(255,255,255,0.11),transparent_60%)]" />
      <div className="absolute inset-y-0 right-0 -z-10 w-[52%] border-l border-white/[0.05]" />
      <Container>
        <div className="grid items-center gap-16 lg:grid-cols-[0.92fr_1.08fr]">
          <div className="animate-rise">
            <h1 className="display max-w-[12ch] text-[clamp(3.2rem,6vw,6rem)] text-[#f4f4f0]">
              A deploy button for your own server.
            </h1>
            <p className="mt-7 max-w-[50ch] text-[18px] leading-[1.7] text-[#a9aaa4]">
              Install Shipper on a VPS you already pay for. Connect a repo, a folder,
              or an image, then ship with HTTPS, logs, and rollbacks included.
            </p>
            <div id="install" className="mt-9 scroll-mt-28">
              <Actions />
            </div>
            <div className="mt-9">
              <CommandBar />
            </div>
          </div>
          <div className="relative animate-scale-in [animation-delay:120ms]">
            <Screenshot
              src="/shots/overview.png"
              alt="Shipper application overview with deployment status and diagnostics"
              priority
              frameClassName="[transform:perspective(1900px)_rotateY(-3deg)] origin-left"
            />
            <div className="absolute -bottom-12 -left-2 w-[310px] rounded-xl border border-white/[0.14] bg-[#0b0c0c]/94 p-5 shadow-2xl backdrop-blur-xl sm:left-0">
              <div className="flex items-center justify-between">
                <span className="text-[15px] font-[500] text-[#e7e7e2]">Nightly backup</span>
                <Status label="Verified" />
              </div>
              <p className="mt-2 text-[14px] leading-[1.6] text-[#8d8e88]">
                Your database is saved to storage you control.
              </p>
            </div>
            <div className="absolute -right-4 top-12 hidden w-[210px] rotate-3 rounded-xl border border-white/[0.14] bg-[#0b0c0c]/94 p-4 shadow-2xl backdrop-blur-xl xl:block">
              <div className="flex items-center gap-2 text-[14px] text-[#a8a9a3]">
                <HugeiconsIcon icon={CheckmarkCircle02Icon} size={16} strokeWidth={1.6} className="text-[#79e3a6]" />
                SSL active
              </div>
              <div className="mono mt-3 text-[15px] text-[#e7e7e2]">auto-renewed</div>
            </div>
          </div>
        </div>
      </Container>
    </section>
  );
}

function MobileConsole() {
  const rows = [
    ["Website", "website.local", "Running", "success"],
    ["Atlas API", "atlas.local", "Running", "success"],
    ["API", "api.local", "Degraded", "warning"],
    ["Checkout", "checkout.local", "Failed", "danger"],
  ] as const;
  return (
    <div className="overflow-hidden rounded-[18px] border border-white/[0.14] bg-[#0a0b0b] shadow-[0_34px_70px_-34px_rgba(0,0,0,1)]">
      <div className="flex items-center justify-between border-b border-white/[0.08] bg-gradient-to-b from-[#141515] to-[#0a0b0b] px-4 py-3">
        <span className="text-[13px] font-[560] tracking-[-0.02em] text-[#f4f4f0]">Shipper</span>
        <span className="flex items-center gap-1.5 text-[12px] text-[#8d8e88]">
          <span className="size-1.5 rounded-full bg-[#79e3a6]" />
          Connected
        </span>
      </div>
      <div className="px-4 pb-2 pt-4">
        <div className="text-[19px] font-[560] tracking-[-0.03em] text-[#f4f4f0]">Applications</div>
        <div className="mt-3 flex h-9 items-center rounded-lg border border-white/[0.08] bg-black/25 px-3 text-[13px] text-[#797a75]">
          Search applications
        </div>
        <div className="mt-2 divide-y divide-white/[0.07]">
          {rows.map(([name, domain, status, tone]) => (
            <div key={name} className="flex items-center justify-between gap-3 py-3">
              <div className="min-w-0">
                <div className="text-[14px] text-[#dcdcd6]">{name}</div>
                <div className="mono mt-0.5 truncate text-[12px] text-[#797a75]">{domain}</div>
              </div>
              <span
                className={cn(
                  "flex shrink-0 items-center gap-1.5 text-[12px]",
                  tone === "success" && "text-[#79e3a6]",
                  tone === "warning" && "text-[#e8c07d]",
                  tone === "danger" && "text-[#f0a3a6]",
                )}
              >
                <span
                  className={cn(
                    "size-1.5 rounded-full",
                    tone === "success" && "bg-[#79e3a6]",
                    tone === "warning" && "bg-[#e8c07d]",
                    tone === "danger" && "bg-[#f08c90]",
                  )}
                />
                {status}
              </span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

function Hero3() {
  return (
    <section className="relative overflow-hidden pt-32 md:pt-36">
      <div className="absolute inset-0 -z-10 bg-[radial-gradient(ellipse_66%_50%_at_50%_0%,rgba(255,255,255,0.12),transparent_64%)]" />
      <AsciiField className="left-1/2 top-24 -z-10 hidden -translate-x-1/2 md:block" cols={96} rows={26} seed={19} opacity={0.08} />
      <Container>
        <div className="grid gap-8 lg:grid-cols-[1fr_400px] lg:grid-rows-[auto_auto] lg:gap-x-10">
          <h1 className="display max-w-[13ch] animate-rise text-[clamp(2.35rem,6.4vw,6.8rem)] text-[#ecece7] lg:col-start-1 lg:row-start-1">
            From localhost to your own cloud in one deploy.
          </h1>
          <div className="w-full max-w-[420px] lg:col-start-2 lg:row-span-2 lg:row-start-1">
            <p className="text-[18px] leading-[1.7] text-[#b3b4ad]">
              Shipper runs on the VPS you already pay for. Ship any app and get a
              live URL, real logs, and a clean way back.
            </p>
            <a href="#how-it-works" className="mt-4 inline-flex items-center gap-1.5 text-[14px] text-[#a2a39d] underline decoration-white/20 underline-offset-4 transition-colors hover:text-white hover:decoration-white">
              See how it works
              <HugeiconsIcon icon={ArrowDown01Icon} size={13} strokeWidth={1.8} />
            </a>
            <div className="mt-6 hidden divide-y divide-white/[0.08] border-t border-white/[0.08] lg:block">
              {[
                ["One command", "Installs Shipper and the proxy"],
                ["One deploy", "Builds, verifies, and routes your app"],
                ["One screen", "Releases, logs, and rollbacks"],
              ].map(([title, body]) => (
                <div key={title} className="py-3">
                  <div className="text-[14px] font-[500] text-[#ecece7]">{title}</div>
                  <div className="mt-0.5 text-[13px] leading-[1.5] text-[#8d8e88]">{body}</div>
                </div>
              ))}
            </div>
          </div>
          <div id="install" className="scroll-mt-28 lg:col-start-1 lg:row-start-2">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
              <ActionPrimary href="#get-shipper" className="w-full justify-center lg:w-auto" />
              <div className="flex flex-wrap items-center gap-x-5 gap-y-2">
                <a href="https://github.com/matyas-toth/shipperkit" className="inline-flex items-center gap-2 text-[14px] text-[#a2a39d] transition-colors hover:text-white">
                  <HugeiconsIcon icon={GithubIcon} size={15} strokeWidth={1.7} />
                  GitHub
                </a>
                <span className="text-[14px] text-[#8d8e88]">Ubuntu 24.04</span>
                <span className="text-[14px] text-[#8d8e88]">5-minute install</span>
                <span className="text-[14px] text-[#8d8e88]">Open source · MIT</span>
              </div>
            </div>
            <CommandBar className="mt-4" />
            <div className="mt-3 flex flex-wrap items-center gap-x-2.5 gap-y-1 text-[13px] text-[#8d8e88] lg:hidden">
              {["One command", "One deploy", "One screen"].map((item, index) => (
                <span key={item} className="flex items-center gap-2.5">
                  {index > 0 && <span className="size-1 rounded-full bg-white/20" />}
                  {item}
                </span>
              ))}
            </div>
          </div>
        </div>
        <div className="relative mt-10">
          <div className="pointer-events-none absolute -inset-x-24 -top-10 bottom-0 -z-10 rounded-[50%] bg-[radial-gradient(ellipse_at_center,rgba(255,255,255,0.1),transparent_64%)] blur-3xl" />
          <div className="sm:hidden">
            <MobileConsole />
          </div>
          <div className="hidden sm:block">
            <Screenshot
              src="/shots/applications.png"
              alt="Shipper applications dashboard showing live apps and deployment status"
              priority
            />
          </div>
        </div>
        <div className="mt-5 flex flex-wrap items-center gap-x-6 gap-y-2 border-t border-white/[0.08] pt-4 text-[13px] text-[#8d8e88]">
          <span className="text-[#797a75]">Runs on</span>
          {["Docker", "Compose", "Git", "Any Git source"].map((item) => (
            <span key={item}>{item}</span>
          ))}
        </div>
      </Container>
    </section>
  );
}

function Hero4() {
  return (
    <section className="relative overflow-hidden pt-36 pb-20 md:pt-40">
      <div className="dot-grid absolute inset-0 -z-10 opacity-25 mask-fade-b" />
      <Container>
        <div className="grid items-center gap-14 lg:grid-cols-[1fr_0.95fr] lg:gap-12">
          <div className="animate-rise">
            <h1 className="display max-w-[11ch] text-[clamp(3.2rem,6.1vw,6rem)] text-[#f4f4f0]">
              One command puts your app on your server.
            </h1>
            <p className="mt-7 max-w-[50ch] text-[18px] leading-[1.7] text-[#a9aaa4]">
              Install on a fresh Ubuntu box, claim your dashboard, and deploy from a
              folder, a repo, or an image. Your first release can be live before your
              coffee cools.
            </p>
            <div id="install" className="mt-9 scroll-mt-28">
              <Actions secondary="Read the docs" />
            </div>
            <div className="mt-10 grid grid-cols-3 gap-6 border-t border-white/[0.08] pt-6">
              {[
                ["Install", "One shell command"],
                ["Connect", "Repo, folder, or image"],
                ["Deploy", "Live URL with HTTPS"],
              ].map(([title, body]) => (
                <div key={title}>
                  <div className="text-[16px] font-[500] text-[#e7e7e2]">{title}</div>
                  <div className="mt-1.5 text-[14px] leading-[1.6] text-[#8d8e88]">{body}</div>
                </div>
              ))}
            </div>
          </div>

          <div className="relative animate-scale-in [animation-delay:120ms]">
            <div className="overflow-hidden rounded-2xl border border-white/[0.13] bg-[#090a0a] shadow-[0_50px_120px_-50px_rgba(0,0,0,1)]">
              <div className="screen-bar">
                <div className="flex items-center gap-2">
                  <span className="screen-dot" />
                  <span className="screen-dot" />
                  <span className="screen-dot" />
                </div>
                <span className="mono ml-2 text-[14px] text-[#898a85]">operator@your-server</span>
              </div>
              <div className="p-6 md:p-8">
                <div className="mono space-y-3 text-[14px] leading-[1.85]">
                  <div className="text-[#85867f]"># one supported Ubuntu host</div>
                  <div className="flex items-start gap-2">
                    <span className="text-[#85867f]">$ </span>
                    <TerminalInstall className="text-[#ecece7]" />
                  </div>
                  <div className="text-[#79e3a6]">Shipper is ready on port 8080</div>
                  <div className="text-[#85867f]"># claim it, then ship</div>
                  <div>
                    <span className="text-[#85867f]">$ </span>
                    <span className="text-[#ecece7]">open http://203.0.113.24:8080</span>
                    <span className="ml-1 inline-block h-4 w-1 animate-blink bg-[#e7e7e2]" />
                  </div>
                </div>
                <div className="mt-8 border-t border-white/[0.08] pt-6">
                  <div className="text-[15px] text-[#c3c4be]">What you get</div>
                  <div className="mt-4 grid grid-cols-2 gap-3 text-[14px] text-[#a8a9a3]">
                    {["HTTPS domains", "Release history", "Live logs", "One-click backups"].map((item) => (
                      <div key={item} className="flex items-center gap-2">
                        <span className="size-1.5 rounded-full bg-[#79e3a6]" />
                        {item}
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </Container>
    </section>
  );
}

function Hero5() {
  return (
    <section className="relative overflow-hidden pt-36 md:pt-40">
      <div className="absolute inset-0 -z-10 bg-[radial-gradient(ellipse_52%_50%_at_22%_0%,rgba(255,255,255,0.1),transparent_60%)]" />
      <Container>
        <div className="grid items-start gap-16 lg:grid-cols-[1.1fr_0.9fr]">
          <div className="animate-rise">
            <h1 className="display max-w-[10ch] text-[clamp(3.4rem,6.9vw,6.9rem)] text-[#f4f4f0]">
              Deploy like you own the place.
            </h1>
            <p className="mt-7 max-w-[52ch] text-[18px] leading-[1.7] text-[#a9aaa4]">
              Because you do. Shipper gives your own server the deploy button, the
              release history, and the recovery plan that managed platforms keep
              behind their dashboard.
            </p>
            <div id="install" className="mt-9 scroll-mt-28">
              <Actions />
            </div>
            <div className="mt-9 max-w-[590px]">
              <CommandBar />
            </div>
          </div>

          <div className="relative lg:pt-6">
            <div className="rounded-2xl border border-white/[0.12] bg-[#090a0a] p-2">
              <div className="rounded-xl border border-white/[0.08] bg-[#0b0c0c] p-6">
                <div className="text-[15px] font-[500] text-[#e7e7e2]">What stays with you</div>
                <div className="mt-5 space-y-4">
                  {[
                    ["Your server", "The VPS you already pay for"],
                    ["Your data", "Databases, volumes, and backups"],
                    ["Your release history", "Every deploy, with a way back"],
                    ["Your domain", "HTTPS, aliases, and redirects"],
                  ].map(([title, body]) => (
                    <div key={title} className="flex items-start gap-3">
                      <span className="mt-[7px] size-1.5 shrink-0 rounded-full bg-[#79e3a6]" />
                      <div>
                        <div className="text-[16px] text-[#dcdcd6]">{title}</div>
                        <div className="mt-0.5 text-[14px] text-[#85867f]">{body}</div>
                      </div>
                    </div>
                  ))}
                </div>
                <div className="mt-6 flex items-center justify-between border-t border-white/[0.08] pt-5">
                  <Status label="Open source" />
                  <span className="mono text-[14px] text-[#85867f]">MIT</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div className="relative mt-16">
          <Screenshot
            src="/shots/deployments.png"
            alt="Shipper deployments view with release history and live logs"
            priority
            frameClassName="max-w-[1120px] mx-auto"
            className="max-h-[620px] object-cover object-top"
          />
        </div>
      </Container>
    </section>
  );
}

function Hero6() {
  return (
    <section className="relative overflow-hidden pt-36 md:pt-40">
      <div className="absolute inset-0 -z-10 bg-[radial-gradient(ellipse_60%_55%_at_78%_6%,rgba(255,255,255,0.11),transparent_62%)]" />
      <AsciiField className="right-0 top-10 -z-10 hidden md:block" cols={58} rows={30} seed={31} opacity={0.08} />
      <Container>
        <div className="grid items-center gap-14 lg:grid-cols-[0.92fr_1.08fr]">
          <div className="animate-rise">
            <h1 className="display max-w-[11ch] text-[clamp(3.2rem,6.2vw,6.1rem)] text-[#f4f4f0]">
              Give your agent a deploy button.
            </h1>
            <p className="mt-7 max-w-[50ch] text-[18px] leading-[1.7] text-[#a9aaa4]">
              Shipper gives you and your AI agents the same safe actions: plan a
              deploy, ship it, read the logs, and roll back. Scoped tokens keep the
              power on a leash.
            </p>
            <div id="install" className="mt-9 scroll-mt-28">
              <Actions secondary="Read the docs" />
            </div>
            <div className="mt-8 flex flex-wrap items-center gap-x-6 gap-y-3 text-[14px] text-[#8d8e88]">
              <span className="inline-flex items-center gap-2">
                <HugeiconsIcon icon={TerminalIcon} size={16} strokeWidth={1.6} />
                CLI and API
              </span>
              <span className="inline-flex items-center gap-2">
                <HugeiconsIcon icon={CheckmarkCircle02Icon} size={16} strokeWidth={1.6} className="text-[#79e3a6]" />
                Scoped permissions
              </span>
              <span className="inline-flex items-center gap-2">
                <HugeiconsIcon icon={CheckmarkCircle02Icon} size={16} strokeWidth={1.6} className="text-[#79e3a6]" />
                Same release checks
              </span>
            </div>
          </div>

          <div className="relative animate-scale-in [animation-delay:120ms]">
            <div className="overflow-hidden rounded-2xl border border-white/[0.13] bg-[#090a0a] shadow-[0_50px_120px_-50px_rgba(0,0,0,1)]">
              <div className="screen-bar">
                <div className="flex items-center gap-2">
                  <span className="screen-dot" />
                  <span className="screen-dot" />
                  <span className="screen-dot" />
                </div>
                <span className="mono ml-2 text-[14px] text-[#898a85]">agent session</span>
              </div>
              <div className="p-6 md:p-8">
                <div className="mono space-y-4 text-[15px] leading-[1.8]">
                  <div className="text-[#85867f]"># your agent plans the release</div>
                  <div>
                    <span className="text-[#85867f]">$ </span>
                    <span className="text-[#ecece7]">shipper plan . --name store</span>
                  </div>
                  <div className="text-[#a8a9a3]">source inspected</div>
                  <div className="text-[#a8a9a3]">plan ready for review</div>
                  <div className="pt-2">
                    <span className="text-[#85867f]">$ </span>
                    <span className="text-[#ecece7]">shipper deploy --plan PLAN_7c1e4d --wait</span>
                  </div>
                  <div className="text-[#79e3a6]">deployed in 14s</div>
                  <div className="text-[#ecece7]">https://store.example.com</div>
                  <div className="pt-2 text-[#85867f]"># scoped token · deploy + logs only</div>
                </div>
                <div className="mt-7 flex items-center justify-between border-t border-white/[0.08] pt-5">
                  <Status label="Agent access granted" />
                  <span className="text-[14px] text-[#8d8e88]">revoke anytime</span>
                </div>
              </div>
            </div>
            <div className="mt-3 rounded-xl border border-white/[0.12] bg-[#0b0c0c] p-5">
              <div className="text-[14px] text-[#8d8e88]">The same checks as you</div>
              <div className="mt-3 grid gap-3 sm:grid-cols-3">
                {["Build passed", "Health check passed", "Route verified"].map((item) => (
                  <div key={item} className="flex items-center gap-2.5 text-[14px] text-[#c3c4be]">
                    <HugeiconsIcon icon={CheckmarkCircle02Icon} size={16} strokeWidth={1.6} className="shrink-0 text-[#79e3a6]" />
                    {item}
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      </Container>
    </section>
  );
}

export function Hero({ variant = 1 }: { variant?: number }) {
  switch (variant) {
    case 2:
      return <Hero2 />;
    case 3:
      return <Hero3 />;
    case 4:
      return <Hero4 />;
    case 5:
      return <Hero5 />;
    case 6:
      return <Hero6 />;
    default:
      return <Hero1 />;
  }
}
