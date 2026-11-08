import { HugeiconsIcon } from "@hugeicons/react";
import {
  ArrowUpRight01Icon,
  CheckmarkCircle02Icon,
} from "@hugeicons/core-free-icons";
import {
  ActionSecondary,
  CommandBar,
  Container,
  CopyButton,
  Logo,
  Section,
  Status,
  AsciiField,
  TerminalInstall,
} from "@/components/primitives";

function Cta1() {
  return (
    <Section id="get-shipper" className="overflow-hidden border-t border-white/[0.08] text-center">
      <div className="absolute inset-0 -z-10 bg-[radial-gradient(ellipse_52%_62%_at_50%_100%,rgba(255,255,255,0.1),transparent_66%)]" />
      <AsciiField className="left-1/2 top-6 -z-10 hidden -translate-x-1/2 md:block" cols={86} rows={24} seed={41} opacity={0.075} />
      <div className="pointer-events-none absolute inset-x-0 bottom-0 -z-10 h-[420px] bg-[linear-gradient(180deg,transparent,rgba(255,255,255,0.02))]" />
      <Container>
        <div className="mx-auto flex max-w-[860px] flex-col items-center">
          <h2 className="display max-w-[10ch] text-[clamp(3.4rem,7.3vw,7rem)] text-[#f4f4f0]">
            Put it on your server.
          </h2>
          <p className="mt-7 max-w-[56ch] text-[18px] leading-[1.7] text-[#a5a6a0]">
            One installer, one administrator account, and a dashboard that stays on
            the machine you control. Your first release can come from a folder on
            your laptop.
          </p>
          <div className="mt-10 w-full max-w-[680px]">
            <CommandBar />
          </div>
          <div className="mt-8 flex flex-wrap items-center justify-center gap-3">
            <ActionSecondary href="https://github.com/matyas-toth/shipperkit/tree/main/docs" external>
              Read the docs
            </ActionSecondary>
          </div>
          <div className="mt-12 flex flex-wrap items-center justify-center gap-x-8 gap-y-3 text-[14px] text-[#8d8e88]">
            <span>Free to self-host</span>
            <span>MIT licensed</span>
            <span>No credit card</span>
          </div>
        </div>
      </Container>
    </Section>
  );
}

function Cta2() {
  return (
    <Section id="get-shipper" className="border-t border-white/[0.08]">
      <Container>
        <div className="grid items-center gap-16 lg:grid-cols-[1fr_0.9fr]">
          <div>
            <h2 className="display max-w-[11ch] text-[clamp(2.9rem,5.6vw,5.4rem)] text-[#f4f4f0]">
              Your first deploy is one command away.
            </h2>
            <p className="mt-6 max-w-[48ch] text-[17px] leading-[1.7] text-[#a5a6a0]">
              Install on a fresh Ubuntu server, claim the dashboard, and ship a
              project you already have. No platform account and no card.
            </p>
            <div className="mt-9 flex flex-wrap gap-3">
              <ActionSecondary href="https://github.com/matyas-toth/shipperkit" external>
                View on GitHub
              </ActionSecondary>
            </div>
            <div className="mt-9 flex items-center gap-4">
              <Status label="Ready when your server is" pulse />
            </div>
          </div>
          <div className="overflow-hidden rounded-2xl border border-white/[0.12] bg-[#080909] p-2">
            <div className="rounded-xl border border-white/[0.08] bg-[#0a0b0b] p-6 md:p-8">
              <div className="flex items-center gap-2">
                <span className="screen-dot" />
                <span className="screen-dot" />
                <span className="screen-dot" />
                <span className="mono ml-3 text-[14px] text-[#797a75]">install</span>
              </div>
              <div className="mono mt-8 space-y-4 text-[14px] leading-[1.8]">
                <div className="text-[#85867f]"># on your server</div>
                <div className="flex items-start gap-2">
                  <span className="text-[#85867f]">$ </span>
                  <TerminalInstall className="text-[#ecece7]" />
                </div>
                <div className="text-[#79e3a6]">Shipper is ready on port 8080</div>
                <div className="text-[#85867f]"># then</div>
                <div>
                  <span className="text-[#85867f]">$ </span>
                  <span className="text-[#ecece7]">open http://203.0.113.24:8080</span>
                  <span className="ml-1 inline-block h-4 w-1 animate-blink bg-[#e7e7e2]" />
                </div>
              </div>
              <div className="mt-9 flex items-center justify-between border-t border-white/[0.08] pt-6">
                <span className="text-[14px] text-[#797a75]">No card. No trial clock.</span>
                <CopyButton />
              </div>
            </div>
          </div>
        </div>
      </Container>
    </Section>
  );
}

function Cta3() {
  return (
    <Section id="get-shipper" className="border-t border-white/[0.08]">
      <Container>
        <div className="relative overflow-hidden rounded-[28px] border border-white/[0.14] bg-[#101111] px-7 py-14 md:px-14 md:py-20">
          <div className="absolute inset-0 -z-0 bg-[radial-gradient(ellipse_60%_100%_at_50%_0%,rgba(255,255,255,0.09),transparent_62%)]" />
          <div className="relative z-10 mx-auto flex max-w-[900px] flex-col items-center text-center">
            <h2 className="display max-w-[20ch] text-[clamp(2.375rem,6.4vw,6.4rem)] text-[#f4f4f0] [text-wrap:wrap]">
              Everything you
              <br className="sm:hidden" /> need,
              <br className="hidden sm:block" />{" "}
              <span className="text-[#ecece7]">nothing<br className="sm:hidden" /> you rent.</span>
            </h2>
            <p className="mt-7 max-w-[52ch] text-[17px] leading-[1.7] text-[#a5a6a0]">
              The deploy console, the release history, and the recovery plan live on
              your server. Keep the keys.
            </p>
            <div className="mt-9 w-full max-w-[640px]">
              <CommandBar />
            </div>
            <div className="mt-8 flex flex-wrap items-center justify-center gap-7">
              <a href="https://github.com/matyas-toth/shipperkit" className="inline-flex items-center gap-1.5 text-[14px] text-[#a2a39d] transition-colors hover:text-white">
                GitHub
                <HugeiconsIcon icon={ArrowUpRight01Icon} size={13} strokeWidth={1.8} />
              </a>
              <span className="text-[14px] text-[#8d8e88]">MIT licensed</span>
            </div>
          </div>
        </div>
      </Container>
    </Section>
  );
}

function Cta4() {
  return (
    <Section id="get-shipper" className="border-t border-white/[0.08] py-20 md:py-24">
      <Container>
        <div className="flex flex-col gap-7 border-y border-white/[0.1] py-10 md:flex-row md:items-center md:justify-between">
          <div className="flex items-center gap-5">
            <Logo size={22} />
            <span className="hidden h-6 w-px bg-white/[0.12] sm:block" />
            <span className="text-[18px] text-[#c3c4be]">Ship something today.</span>
          </div>
          <div className="flex flex-wrap items-center gap-3">
            <code className="mono hidden text-[14px] text-[#85867f] lg:block">curl -fsSL shipperkit.dev/install.sh | sudo bash</code>
            <CopyButton />
          </div>
        </div>
      </Container>
    </Section>
  );
}

function Cta5() {
  return (
    <Section id="get-shipper" className="border-t border-white/[0.08]">
      <Container>
        <div className="mx-auto max-w-[940px] overflow-hidden rounded-[24px] border border-white/[0.11] bg-[#080909]">
          <div className="grid md:grid-cols-[1.15fr_0.85fr]">
            <div className="p-8 md:p-12">
              <h2 className="display max-w-[11ch] text-[clamp(2.4rem,4.7vw,4.2rem)] text-[#f4f4f0]">
                From zero to live, without the setup tax.
              </h2>
              <p className="mt-5 max-w-[44ch] text-[17px] leading-[1.7] text-[#a5a6a0]">
                Install on a fresh Ubuntu host, connect a source, and review the
                first release before it takes traffic.
              </p>
            </div>
            <div className="flex flex-col justify-between border-t border-white/[0.08] bg-white/[0.02] p-8 md:border-l md:border-t-0 md:p-10">
              <div className="space-y-4">
                {["Add the server", "Claim the dashboard", "Deploy the first app", "Point a domain"].map((title) => (
                  <div key={title} className="flex items-center gap-3 text-[16px] text-[#dcdcd6]">
                    <HugeiconsIcon icon={CheckmarkCircle02Icon} size={18} strokeWidth={1.6} className="text-[#79e3a6]" />
                    {title}
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
        <div className="mx-auto mt-6 max-w-[680px]">
          <CommandBar />
        </div>
      </Container>
    </Section>
  );
}

function Cta6() {
  return (
    <Section id="get-shipper" className="border-t border-white/[0.08]">
      <Container className="max-w-[1060px]">
        <div className="grid items-center gap-14 lg:grid-cols-[0.9fr_1.1fr]">
          <div>
            <h2 className="display max-w-[9ch] text-[clamp(2.9rem,5.5vw,5.2rem)] text-[#f4f4f0]">
              Take the shortcut.
            </h2>
            <p className="mt-6 max-w-[40ch] text-[17px] leading-[1.7] text-[#a5a6a0]">
              Your server is already paid for. Give it something to do.
            </p>
            <div className="mt-8 flex items-center gap-3">
              <Status label="Free to self-host" />
              <span className="text-[14px] text-[#797a75]">MIT</span>
            </div>
          </div>
          <div className="rounded-2xl border border-white/[0.12] bg-[#080909] p-8 md:p-10">
            <div className="mono space-y-4 text-[14px] leading-[1.8]">
              <div className="text-[#85867f]"># one supported Ubuntu host</div>
              <div className="flex items-start gap-2">
                <span className="text-[#85867f]">$ </span>
                <TerminalInstall className="text-[#ecece7]" />
              </div>
              <div className="text-[#79e3a6]">Shipper is ready on port 8080</div>
              <div className="text-[#85867f]"># claim it, then keep it</div>
              <div>
                <span className="text-[#85867f]">$ </span>
                <span className="text-[#ecece7]">open http://203.0.113.24:8080</span>
                <span className="ml-1 inline-block h-4 w-1 animate-blink bg-[#e7e7e2]" />
              </div>
            </div>
            <div className="mt-9 flex items-center justify-between border-t border-white/[0.08] pt-6">
              <span className="text-[14px] text-[#797a75]">No platform account. No agents required.</span>
              <CopyButton />
            </div>
          </div>
        </div>
      </Container>
    </Section>
  );
}

export function Cta({ variant = 1 }: { variant?: number }) {
  switch (variant) {
    case 2:
      return <Cta2 />;
    case 3:
      return <Cta3 />;
    case 4:
      return <Cta4 />;
    case 5:
      return <Cta5 />;
    case 6:
      return <Cta6 />;
    default:
      return <Cta1 />;
  }
}
