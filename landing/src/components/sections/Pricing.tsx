import { HugeiconsIcon } from "@hugeicons/react";
import {
  CheckmarkCircle02Icon,
  CreditCardIcon,
  GlobeIcon,
  ServerStack01Icon,
  ShieldCheckIcon,
  Tick02Icon,
} from "@hugeicons/core-free-icons";
import {
  ActionPrimary,
  ActionSecondary,
  Check,
  Container,
  Section,
  SectionIntro,
  Status,
} from "@/components/primitives";
import { cn } from "@/lib/utils";

function PriceCard() {
  return (
    <div className="relative overflow-hidden rounded-[24px] border border-white/[0.14] bg-[#080909]">
      <div className="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-white/40 to-transparent" />
      <div className="grid gap-10 p-8 md:grid-cols-[1fr_auto] md:items-end md:p-12">
        <div>
          <div className="text-[18px] font-[500] text-[#c3c4be]">Shipper, self-hosted</div>
          <div className="mt-6 flex items-baseline gap-3">
            <span className="display text-[96px] leading-none text-[#f4f4f0]">$0</span>
            <span className="text-[17px] text-[#8d8e88]">for the software</span>
          </div>
          <p className="mt-5 max-w-[54ch] text-[17px] leading-[1.7] text-[#969791]">
            Install it on the server you already pay for. No platform fee, no seat
            count, and no build minutes to watch.
          </p>
        </div>
        <div className="flex flex-col items-start gap-3">
          <ActionPrimary href="#get-shipper" />
          <span className="text-[14px] text-[#797a75]">MIT licensed. Yours to keep.</span>
        </div>
      </div>
      <div className="grid gap-px border-t border-white/[0.08] bg-white/[0.07] sm:grid-cols-3">
        {[
          ["Unlimited apps", "Deploy as many as your server can run."],
          ["Unlimited deploys", "No build-minute meter."],
          ["No seat fee", "There is no per-user invoice."],
        ].map(([title, body]) => (
          <div key={title} className="bg-[#080909] px-8 py-7">
            <div className="text-[17px] font-[500] text-[#e7e7e2]">{title}</div>
            <div className="mt-2 text-[14px] leading-[1.6] text-[#8d8e88]">{body}</div>
          </div>
        ))}
      </div>
    </div>
  );
}

function Pricing1() {
  return (
    <Section id="pricing" className="section-rule">
      <Container>
        <SectionIntro
          title={<>Everything you need, nothing you rent.</>}
          body="Shipper is free software. You bring the server, the domain, and the apps. We do not take a cut of any of them."
          align="center"
        />
        <div className="mx-auto mt-16 max-w-[1040px]">
          <PriceCard />
        </div>
      </Container>
    </Section>
  );
}

function Pricing2() {
  return (
    <Section id="pricing" className="section-rule">
      <Container>
        <div className="grid items-start gap-16 lg:grid-cols-[1fr_0.9fr]">
          <div>
            <SectionIntro
              title={<>One price: whatever your server costs.</>}
              body="There is no per-seat invoice hiding behind the dashboard. The software is free, and the running costs are the ones you already know."
            />
            <div className="mt-9 space-y-4">
              <Check>No credit card to install.</Check>
              <Check>No trial clock or feature paywall.</Check>
              <Check>No build minutes or egress markup.</Check>
            </div>
            <div className="mt-9">
              <ActionPrimary href="#get-shipper" />
            </div>
          </div>
          <div className="overflow-hidden rounded-2xl border border-white/[0.1] bg-[#080909]">
            <div className="flex items-center justify-between border-b border-white/[0.08] px-7 py-5">
              <span className="text-[16px] font-[500] text-[#e7e7e2]">Monthly bill</span>
              <Status label="You control it" />
            </div>
            {[
              ["Shipper", "$0", "software"],
              ["HTTPS", "$0", "included"],
              ["Backups", "$0", "your storage"],
              ["VPS", "your bill", "your provider"],
              ["Domain", "your bill", "your registrar"],
            ].map(([item, price, note]) => (
              <div key={item} className="flex items-center justify-between border-b border-white/[0.07] px-7 py-5 last:border-0">
                <div>
                  <div className="text-[16px] text-[#dcdcd6]">{item}</div>
                  <div className="mt-0.5 text-[14px] text-[#797a75]">{note}</div>
                </div>
                <span className={cn("mono text-[16px]", price === "$0" ? "text-[#79e3a6]" : "text-[#c3c4be]")}>{price}</span>
              </div>
            ))}
            <div className="flex items-center justify-between bg-white/[0.03] px-7 py-5">
              <span className="text-[16px] text-[#e7e7e2]">Software cost</span>
              <span className="mono text-[20px] text-[#79e3a6]">$0</span>
            </div>
          </div>
        </div>
      </Container>
    </Section>
  );
}

function Pricing3() {
  const rows = [
    ["Software cost", "Free", "Monthly fee", "Free"],
    ["Per-seat pricing", "No", "Usually", "No"],
    ["Build minutes", "No", "Usually", "No"],
    ["Your server", "Yes", "No", "Yes"],
    ["Your data", "Yes", "Vendor-held", "Yes"],
    ["Open source", "Yes", "No", "Yes"],
  ] as const;
  return (
    <Section id="pricing" className="section-rule">
      <Container>
        <SectionIntro
          title={<>The platform tax, removed.</>}
          body="A managed platform charges for convenience. Running it yourself should not mean rebuilding the convenience from scratch."
          align="center"
        />
        <div className="mt-10 overflow-hidden rounded-xl border border-white/[0.13] bg-[#101111] sm:hidden">
          <div className="grid grid-cols-[1.05fr_repeat(3,1fr)] border-b border-white/[0.1] bg-white/[0.035]">
            <div className="px-2 py-3 text-[12px] leading-tight text-[#8d8e88]">What you care about</div>
            <div className="border-l border-white/[0.09] bg-[#79e3a6]/[0.045] px-1.5 py-3 text-center text-[12px] font-[520] leading-tight text-[#f4f4f0]">Shipper</div>
            <div className="border-l border-white/[0.09] px-1.5 py-3 text-center text-[12px] leading-tight text-[#969791]">Managed PaaS</div>
            <div className="border-l border-white/[0.09] px-1.5 py-3 text-center text-[12px] leading-tight text-[#969791]">Hand-rolled</div>
          </div>
          {rows.map(([label, shipper, paas, ssh], index) => (
            <div key={label} className={cn("grid grid-cols-[1.05fr_repeat(3,1fr)]", index !== rows.length - 1 && "border-b border-white/[0.09]")}>
              <div className="px-2 py-3 text-[13px] leading-tight text-[#dcdcd6]">{label}</div>
              <div className="border-l border-white/[0.09] bg-[#79e3a6]/[0.045] px-1.5 py-3 text-center text-[13px]">
                {shipper === "Yes" ? (
                  <HugeiconsIcon icon={Tick02Icon} size={16} strokeWidth={1.9} className="mx-auto text-[#79e3a6]" />
                ) : (
                  <span className="text-[#dcdcd6]">{shipper}</span>
                )}
              </div>
              <div className="border-l border-white/[0.09] px-1.5 py-3 text-center text-[13px] leading-tight text-[#8d8e88]">{paas}</div>
              <div className="border-l border-white/[0.09] px-1.5 py-3 text-center text-[13px] leading-tight text-[#8d8e88]">{ssh}</div>
            </div>
          ))}
        </div>
        <div className="surface mx-auto mt-12 hidden max-w-[920px] overflow-hidden rounded-2xl sm:block">
          <div className="grid grid-cols-[1.25fr_repeat(3,1fr)] border-b border-white/[0.1] bg-white/[0.035] sm:grid-cols-[1.5fr_repeat(3,1fr)]">
            <div className="px-4 py-4 text-[14px] text-[#8d8e88] sm:px-6 sm:py-5 sm:text-[15px]">What you care about</div>
            <div className="border-l border-white/[0.09] bg-[#79e3a6]/[0.045] px-2 py-4 text-center text-[14px] font-[520] text-[#f4f4f0] sm:px-4 sm:py-5 sm:text-[15px]">Shipper</div>
            <div className="border-l border-white/[0.09] px-2 py-4 text-center text-[14px] text-[#969791] sm:px-4 sm:py-5 sm:text-[15px]">Managed PaaS</div>
            <div className="border-l border-white/[0.09] px-2 py-4 text-center text-[14px] text-[#969791] sm:px-4 sm:py-5 sm:text-[15px]">Hand-rolled</div>
          </div>
          {rows.map(([label, shipper, paas, ssh], index) => (
            <div key={label} className={cn("grid grid-cols-[1.25fr_repeat(3,1fr)] sm:grid-cols-[1.5fr_repeat(3,1fr)]", index !== rows.length - 1 && "border-b border-white/[0.09]")}>
              <div className="px-4 py-4 text-[14px] text-[#c3c4be] sm:px-6 sm:text-[15px]">{label}</div>
              <div className="border-l border-white/[0.09] bg-[#79e3a6]/[0.045] px-2 py-4 text-center text-[14px] sm:px-4 sm:text-[15px]">
                {shipper === "Yes" ? (
                  <HugeiconsIcon icon={Tick02Icon} size={18} strokeWidth={1.9} className="mx-auto text-[#79e3a6]" />
                ) : (
                  <span className="text-[#dcdcd6]">{shipper}</span>
                )}
              </div>
              <div className="border-l border-white/[0.09] px-2 py-4 text-center text-[14px] text-[#8d8e88] sm:px-4 sm:text-[15px]">{paas}</div>
              <div className="border-l border-white/[0.09] px-2 py-4 text-center text-[14px] text-[#8d8e88] sm:px-4 sm:text-[15px]">{ssh}</div>
            </div>
          ))}
        </div>
        <div className="mt-10 flex justify-center">
          <ActionPrimary href="#get-shipper" />
        </div>
      </Container>
    </Section>
  );
}

function Pricing4() {
  return (
    <Section id="pricing" className="section-rule">
      <Container>
        <SectionIntro
          title={<>What free actually includes.</>}
          body="The whole self-hosted product, not a demo tier with the useful parts removed."
        />
        <div className="mt-16 grid gap-px overflow-hidden rounded-2xl border border-white/[0.1] bg-white/[0.08] sm:grid-cols-2 lg:grid-cols-3">
          {[
            ["Deploys", "Repos, folders, images, Compose, static, and Dockerfiles."],
            ["Domains", "Custom domains, aliases, redirects, and HTTPS."],
            ["Releases", "History, logs, rollback, and recovery."],
            ["Backups", "Database and volume protection to your storage."],
            ["Tasks", "Scheduled jobs and lifecycle hooks."],
            ["Agents", "CLI, API, and scoped tokens for your tools."],
          ].map(([title, body]) => (
            <div key={title} className="bg-[#080909] p-8">
              <HugeiconsIcon icon={CheckmarkCircle02Icon} size={22} strokeWidth={1.5} className="text-[#79e3a6]" />
              <h3 className="mt-8 text-[21px] font-[520] tracking-[-0.03em] text-[#e7e7e2]">{title}</h3>
              <p className="mt-2.5 text-[15px] leading-[1.7] text-[#8d8e88]">{body}</p>
            </div>
          ))}
        </div>
        <div className="mt-12 flex flex-col items-start justify-between gap-5 sm:flex-row sm:items-center">
          <p className="text-[16px] leading-[1.7] text-[#969791]">
            No credit card. No trial clock. Install it and keep it.
          </p>
          <ActionPrimary href="#get-shipper" />
        </div>
      </Container>
    </Section>
  );
}

function Pricing5() {
  return (
    <Section id="pricing" className="section-rule">
      <Container>
        <div className="mx-auto max-w-[980px] text-center">
          <SectionIntro
            title={<>Free is the business model.</>}
            body="Shipper is MIT licensed and self-hosted by design. The project stays useful because you can leave with everything."
            align="center"
            size="lg"
          />
          <div className="mt-14 grid gap-4 text-left md:grid-cols-3">
            {([
              ["No card", "There is no checkout, trial, or upgrade button.", CreditCardIcon],
              ["No lock-in", "Your data, domains, and releases are yours.", ServerStack01Icon],
              ["No surprise", "The server bill is the server bill.", GlobeIcon],
            ] as const).map(([title, body, Icon]) => (
              <div key={title} className="rounded-2xl border border-white/[0.1] bg-[#080909] p-7">
                <HugeiconsIcon icon={Icon} size={24} strokeWidth={1.5} className="text-[#a8a9a3]" />
                <div className="mt-8 text-[20px] font-[520] text-[#e7e7e2]">{title}</div>
                <p className="mt-2 text-[15px] leading-[1.7] text-[#8d8e88]">{body}</p>
              </div>
            ))}
          </div>
          <div className="mt-12 flex flex-col items-center gap-5">
            <ActionPrimary href="#get-shipper" />
            <ActionSecondary href="https://github.com/matyas-toth/shipperkit" external>
              Star the repository
            </ActionSecondary>
          </div>
        </div>
      </Container>
    </Section>
  );
}

function Pricing6() {
  return (
    <Section id="pricing" className="section-rule">
      <Container>
        <div className="grid items-center gap-16 lg:grid-cols-[0.9fr_1.1fr]">
          <div>
            <SectionIntro
              title={<>Your server. Your bill. Your call.</>}
              body="No managed markup, no egress games, no surprise invoice when a deploy gets popular."
            />
            <div className="mt-9 space-y-4">
              <Check>You pay your VPS provider directly.</Check>
              <Check>You pay your registrar directly.</Check>
              <Check>Shipper stays free to self-host.</Check>
            </div>
            <div className="mt-9">
              <ActionPrimary href="#get-shipper" />
            </div>
          </div>
          <div className="rounded-2xl border border-white/[0.1] bg-[#080909] p-8">
            <div className="flex items-baseline justify-between border-b border-white/[0.08] pb-6">
              <span className="text-[16px] text-[#8d8e88]">Software</span>
              <span className="display text-[64px] leading-none text-[#f4f4f0]">$0</span>
            </div>
            <div className="mt-7 space-y-4">
              {[
                ["Your VPS", "The machine you already have."],
                ["Your domain", "The one you already own."],
                ["Your data", "No extraction fee."],
                ["Your access", "No per-user invoice."],
              ].map(([title, body]) => (
                <div key={title} className="flex items-start gap-3 border-b border-white/[0.07] pb-4 last:border-0 last:pb-0">
                  <HugeiconsIcon icon={ShieldCheckIcon} size={18} strokeWidth={1.6} className="mt-0.5 shrink-0 text-[#79e3a6]" />
                  <div>
                    <div className="text-[16px] text-[#dcdcd6]">{title}</div>
                    <div className="mt-0.5 text-[14px] text-[#797a75]">{body}</div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      </Container>
    </Section>
  );
}

export function Pricing({ variant = 1 }: { variant?: number }) {
  switch (variant) {
    case 2:
      return <Pricing2 />;
    case 3:
      return <Pricing3 />;
    case 4:
      return <Pricing4 />;
    case 5:
      return <Pricing5 />;
    case 6:
      return <Pricing6 />;
    default:
      return <Pricing1 />;
  }
}
