import { HugeiconsIcon } from "@hugeicons/react";
import {
  ArrowUpRight01Icon,
  Clock01Icon,
  ShieldCheckIcon,
} from "@hugeicons/core-free-icons";
import {
  ActionSecondary,
  Bars,
  Check,
  Container,
  Screenshot,
  Section,
  SectionIntro,
  Status,
} from "@/components/primitives";
import { cn } from "@/lib/utils";

function Product1() {
  return (
    <Section id="product" className="section-rule">
      <Container>
        <div className="flex flex-col justify-between gap-8 lg:flex-row lg:items-end">
          <SectionIntro
            title={<>The whole app, one screen.</>}
            body="Apps, deploys, logs, domains, environment, tasks, backups, and settings. The useful state is already there when you open it."
          />
        </div>

        <div className="mt-14 grid gap-3 lg:grid-cols-12">
          <Screenshot
            src="/shots/applications.png"
            alt="Shipper applications list with status, domain, and deployment history"
            frameClassName="lg:col-span-8"
          />
          <div className="grid gap-3 lg:col-span-4">
            <div className="rounded-2xl border border-white/[0.1] bg-[#080909] p-6">
              <div className="flex items-center gap-3">
                <HugeiconsIcon icon={Clock01Icon} size={20} strokeWidth={1.5} className="text-[#8d8e88]" />
                <span className="text-[16px] font-[500] text-[#e7e7e2]">A deploy you can follow</span>
              </div>
              <div className="mt-6 flex items-end justify-between">
                <span className="mono text-[36px] leading-none text-[#ecece7]">14s</span>
                <span className="text-[14px] text-[#8d8e88]">from push to live</span>
              </div>
              <div className="mt-5">
                <Bars values={[34, 44, 39, 56, 51, 68, 63, 78, 72, 88]} />
              </div>
            </div>
            <div className="rounded-2xl border border-white/[0.1] bg-[#080909] p-6">
              <div className="flex items-center justify-between">
                <span className="text-[16px] font-[500] text-[#e7e7e2]">Logs that stay readable</span>
                <Status label="Live" pulse />
              </div>
              <div className="mt-5 space-y-2 text-[14px] leading-[1.7] text-[#9d9e98]">
                <div>Build completed</div>
                <div>Candidate check passed</div>
                <div className="text-[#79e3a6]">Release is serving traffic</div>
              </div>
            </div>
            <div className="rounded-2xl border border-white/[0.1] bg-[#080909] p-6">
              <div className="flex items-center justify-between">
                <span className="text-[16px] font-[500] text-[#e7e7e2]">Recovery is part of the app</span>
                <HugeiconsIcon icon={ShieldCheckIcon} size={20} strokeWidth={1.5} className="text-[#79e3a6]" />
              </div>
              <p className="mt-3 text-[15px] leading-[1.65] text-[#8d8e88]">
                Restore a previous release without rebuilding your confidence first.
              </p>
            </div>
          </div>
        </div>
      </Container>
    </Section>
  );
}

function Product2() {
  return (
    <Section id="product" className="section-rule">
      <Container>
        <div className="grid gap-16 lg:grid-cols-[0.8fr_1.2fr]">
          <div className="lg:sticky lg:top-28 lg:self-start">
            <SectionIntro
              title={<>See exactly what broke.</>}
              body="The failed stage, the line that caused it, and the release that is still serving stay together. You fix the problem without losing the context."
            />
            <div className="mt-9 space-y-4">
              <Check>The previous release keeps taking traffic.</Check>
              <Check>Failed builds never become the live version.</Check>
              <Check>Retry with the full log in front of you.</Check>
            </div>
            <div className="mt-9">
              <ActionSecondary href="#pricing">See the free self-hosted plan</ActionSecondary>
            </div>
          </div>
          <div className="space-y-4">
            <Screenshot
              src="/shots/failed.png"
              alt="Shipper failed deployment view with the failed stage, logs, and serving release"
            />
            <div className="grid gap-4 sm:grid-cols-2">
              <Screenshot src="/shots/diagnostics.png" alt="Shipper diagnostics with checks and service resources" />
              <div className="rounded-2xl border border-white/[0.1] bg-[#080909] p-6">
                <div className="flex items-center justify-between">
                  <span className="text-[16px] text-[#dcdcd6]">Serving release</span>
                  <span className="mono text-[14px] text-[#79e3a6]">4f8ac21d</span>
                </div>
                <div className="mt-5 space-y-3 text-[14px] text-[#8d8e88]">
                  <div className="flex items-center justify-between border-b border-white/[0.07] pb-3">
                    <span>Failed build</span>
                    <span className="mono text-[#f0a3a6]">9c0e2a4b</span>
                  </div>
                  <div className="flex items-center justify-between border-b border-white/[0.07] pb-3">
                    <span>Public traffic</span>
                    <span className="text-[#79e3a6]">Unaffected</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span>Routes</span>
                    <span className="text-[#79e3a6]">Still serving</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </Container>
    </Section>
  );
}

function Product3() {
  return (
    <Section id="product" className="section-rule overflow-hidden">
      <div className="absolute inset-x-0 top-0 -z-10 h-[520px] bg-[radial-gradient(ellipse_52%_100%_at_50%_0%,rgba(255,255,255,0.06),transparent_72%)]" />
      <Container>
        <SectionIntro
          title={<>A failed deploy is not an outage.</>}
          body="Shipper builds and verifies a new release before traffic moves. If anything fails, the app you already had keeps serving."
          align="center"
        />
        <div className="mx-auto mt-12 max-w-[1080px]">
          <Screenshot
            src="/shots/failed.png"
            alt="A failed deployment with logs and the previous release still serving"
            frameClassName="max-sm:hidden"
          />
        </div>
        <div className="mx-auto mt-4 grid max-w-[1080px] gap-3 md:grid-cols-3">
          <div className="surface rounded-xl p-5">
            <div className="text-[14px] text-[#a2a39d]">Failed stage</div>
            <div className="mt-2 text-[17px] font-[500] text-[#f0a3a6]">Build</div>
            <p className="mt-2 text-[14px] leading-[1.6] text-[#a2a39d]">
              The error line is preserved with the failed release.
            </p>
          </div>
          <div className="surface rounded-xl p-5">
            <div className="text-[14px] text-[#a2a39d]">Still serving</div>
            <div className="mono mt-2 text-[18px] text-[#ecece7]">4f8ac21d</div>
            <div className="mt-2 flex items-center gap-2 text-[14px] text-[#79e3a6]">
              <span className="size-2 rounded-full bg-[#79e3a6]" />
              store.example.com
            </div>
          </div>
          <div className="surface rounded-xl p-5">
            <div className="text-[14px] text-[#a2a39d]">Operation</div>
            <div className="mt-2 flex items-center justify-between">
              <span className="text-[17px] font-[500] text-[#ecece7]">28s</span>
              <span className="text-[14px] text-[#a2a39d]">4 checks</span>
            </div>
            <div className="mt-3 grid grid-cols-4 gap-1.5">
              {["daemon", "runtime", "build", "route"].map((label, index) => (
                <div key={label} className="rounded border border-white/[0.08] bg-white/[0.025] p-1.5">
                  <div className="text-[14px] text-[#a2a39d]">{label}</div>
                  <span className={cn("mt-1 block size-1.5 rounded-full", index === 2 ? "bg-[#f08c90]" : "bg-[#79e3a6]")} />
                </div>
              ))}
            </div>
          </div>
        </div>
      </Container>
    </Section>
  );
}

function Product4() {
  return (
    <Section id="product" className="section-rule">
      <Container>
        <div className="flex flex-col justify-between gap-8 lg:flex-row lg:items-end">
          <SectionIntro
            title={<>Every release has a receipt.</>}
            body="A readable release rail, the exact commit, the duration, and the log. No guessing which version is live."
          />
        </div>
        <div className="mt-14 grid gap-4 lg:grid-cols-[280px_1fr]">
          <div className="rounded-2xl border border-white/[0.1] bg-[#080909] p-3">
            <div className="px-3 py-3 text-[16px] font-[500] text-[#dcdcd6]">Release history</div>
            <div className="space-y-2">
              {[
                ["4f8ac21d", "Deployed", "Nov 6, 14:56", "14s", "success", true],
                ["de4af792", "Deployed", "Nov 5, 14:59", "41s", "success", false],
                ["7b2d1c9a", "Restored", "Nov 4, 14:59", "12s", "info", false],
                ["9c0e2a4b", "Failed", "Nov 3, 14:59", "21s", "danger", false],
              ].map(([hash, state, date, duration, tone, active], index) => (
                <div
                  key={index}
                  className={cn(
                    "rounded-xl border p-4",
                    active ? "border-white/[0.16] bg-white/[0.06]" : "border-transparent",
                  )}
                >
                  <div className="flex items-center gap-2.5">
                    <span
                      className={cn(
                        "size-2 rounded-full",
                        tone === "success" && "bg-[#79e3a6]",
                        tone === "danger" && "bg-[#f08c90]",
                        tone === "info" && "bg-[#9dc4f0]",
                      )}
                    />
                    <span className="text-[15px] text-[#c3c4be]">{state}</span>
                    {active && <span className="ml-auto text-[14px] text-[#79e3a6]">Current</span>}
                  </div>
                  <div className="mono mt-3 text-[15px] text-[#ecece7]">{hash}</div>
                  <div className="mt-2 flex items-center justify-between text-[14px] text-[#797a75]">
                    <span>{date}</span>
                    <span className="mono">{duration}</span>
                  </div>
                </div>
              ))}
            </div>
          </div>
          <Screenshot
            src="/shots/deployments.png"
            alt="Shipper deployment view with release history and deployment logs"
          />
        </div>
      </Container>
    </Section>
  );
}

function Product5() {
  const cards = [
    ["Environment", "Change variables without rebuilding the world.", "/shots/env.png"],
    ["Domains", "HTTPS, aliases, and redirects in one place.", "/shots/domains.png"],
    ["New app", "Pick a source and ship the first release.", "/shots/create.png"],
    ["System", "Know what your server is doing at a glance.", "/shots/system.png"],
  ];
  return (
    <Section id="product" className="section-rule">
      <Container>
        <SectionIntro
          title={<>The everyday work, in the same place.</>}
          body="The small changes should not require a second tool or a late-night SSH session."
        />
        <div className="mt-14 grid gap-4 md:grid-cols-2">
          {cards.map(([title, body, src]) => (
            <div
              key={title}
              className={cn(
                "group overflow-hidden rounded-2xl border border-white/[0.1] bg-[#080909]",
              )}
            >
              <div className="flex items-end justify-between gap-6 p-7">
                <div>
                  <h3 className="text-[22px] font-[520] tracking-[-0.03em] text-[#e7e7e2]">{title}</h3>
                  <p className="mt-2 text-[15px] leading-[1.65] text-[#8d8e88]">{body}</p>
                </div>
                <HugeiconsIcon
                  icon={ArrowUpRight01Icon}
                  size={20}
                  strokeWidth={1.6}
                  className="shrink-0 text-[#85867f] transition-transform duration-300 group-hover:-translate-y-1 group-hover:translate-x-1 group-hover:text-white"
                />
              </div>
              <div className="px-5 pb-5">
                <div className="overflow-hidden rounded-lg border border-white/[0.08]">
                  <img
                    src={src}
                    alt={`Shipper ${title.toLowerCase()} screen`}
                    width={1440}
                    height={900}
                    loading="lazy"
                    decoding="async"
                    className="h-[230px] w-full object-cover object-top opacity-85 transition-opacity duration-300 group-hover:opacity-100"
                  />
                </div>
              </div>
            </div>
          ))}
        </div>
      </Container>
    </Section>
  );
}

function Product6() {
  const shots = [
    ["/shots/applications.png", "Apps", "Everything you run, in one list."],
    ["/shots/overview.png", "Overview", "The state of the app and the machine."],
    ["/shots/deployments.png", "Deploys", "Every release, with its logs."],
    ["/shots/failed.png", "Failure", "The broken line and the safe release."],
    ["/shots/env.png", "Environment", "Variables without a rebuild."],
    ["/shots/backups.png", "Backups", "A verified copy you can restore."],
  ] as const;
  return (
    <Section id="product" className="section-rule">
      <Container>
        <SectionIntro
          title={<>Take a closer look.</>}
          body="Real screens from the console. No concept renders."
        />
        <div className="mt-14 grid gap-4 md:grid-cols-2">
          {shots.map(([src, title, body], index) => (
            <a
              key={title}
              href="#how-it-works"
              className={cn(
                "group overflow-hidden rounded-2xl border border-white/[0.1] bg-[#080909] transition-colors duration-300 hover:border-white/[0.2]",
              )}
            >
              <div className="flex items-center justify-between gap-5 px-6 py-5">
                <div className="flex items-baseline gap-4">
                  <span className="text-[18px] font-[520] text-[#e7e7e2]">{title}</span>
                  <span className="text-[15px] text-[#8d8e88]">{body}</span>
                </div>
                <HugeiconsIcon
                  icon={ArrowUpRight01Icon}
                  size={18}
                  strokeWidth={1.6}
                  className="shrink-0 text-[#85867f] transition-transform duration-300 group-hover:-translate-y-1 group-hover:translate-x-1 group-hover:text-white"
                />
              </div>
              <div className="px-4 pb-4">
                <div className="overflow-hidden rounded-lg border border-white/[0.08]">
                  <img
                    src={src}
                    alt={`Shipper ${title.toLowerCase()} screen`}
                    width={1440}
                    height={900}
                    loading="lazy"
                    decoding="async"
                    className={cn(
                      "h-auto w-full object-cover object-top opacity-85 transition-all duration-500 group-hover:scale-[1.01] group-hover:opacity-100",
                      index === 0 ? "aspect-[2/1]" : "aspect-[16/9]",
                    )}
                  />
                </div>
              </div>
            </a>
          ))}
        </div>
      </Container>
    </Section>
  );
}

export function Product({ variant = 1 }: { variant?: number }) {
  switch (variant) {
    case 2:
      return <Product2 />;
    case 3:
      return <Product3 />;
    case 4:
      return <Product4 />;
    case 5:
      return <Product5 />;
    case 6:
      return <Product6 />;
    default:
      return <Product1 />;
  }
}
