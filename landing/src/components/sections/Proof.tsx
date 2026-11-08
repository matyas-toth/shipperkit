import { HugeiconsIcon } from "@hugeicons/react";
import {
  CheckmarkCircle02Icon,
  CloudServerIcon,
  DatabaseBackupIcon,
  GlobeIcon,
  ShieldCheckIcon,
  SourceCodeIcon,
  TerminalIcon,
} from "@hugeicons/core-free-icons";
import {
  ActionPrimary,
  ActionSecondary,
  AsciiField,
  Check,
  Container,
  Screenshot,
  Section,
  SectionIntro,
  Status,
} from "@/components/primitives";

function Proof1() {
  return (
    <Section className="section-rule">
      <Container>
        <SectionIntro
          title={<>The boring parts are handled.</>}
          body="Builds, HTTPS, logs, rollbacks, and backups. You get the deploy button without spending a weekend assembling YAML."
        />
        <div className="mt-16 grid gap-12 md:grid-cols-3 md:gap-10">
          {([
            ["From code to URL", "Point Shipper at a repo, a folder, or an image. It builds the app and serves it on a real domain.", GlobeIcon],
            ["A way back", "When a release breaks, the last good version keeps serving while you inspect the failed stage.", ShieldCheckIcon],
            ["Data you can keep", "Your database, volumes, and backups stay on your server. Leaving is a decision, not a migration project.", DatabaseBackupIcon],
          ] as const).map(([title, body, Icon]) => (
            <div key={title} className="border-t border-white/[0.12] pt-7">
              <HugeiconsIcon icon={Icon} size={26} strokeWidth={1.4} className="text-[#a8a9a3]" />
              <h3 className="mt-7 text-[24px] font-[520] tracking-[-0.035em] text-[#ecece7]">{title}</h3>
              <p className="mt-3 text-[16px] leading-[1.7] text-[#969791]">{body}</p>
            </div>
          ))}
        </div>
      </Container>
    </Section>
  );
}

function Proof2() {
  return (
    <Section className="section-rule">
      <Container>
        <div className="grid items-end gap-12 lg:grid-cols-[0.9fr_1.1fr]">
          <SectionIntro
            title={<>Everything between git push and a live URL.</>}
            body="The features you would otherwise wire together yourself, already in one console."
          />
          <div className="grid grid-cols-2 gap-px overflow-hidden rounded-2xl border border-white/[0.1] bg-white/[0.08] sm:grid-cols-4">
            {[
              ["Deploy", "Repo, folder, image"],
              ["HTTPS", "Domains and certificates"],
              ["Observe", "Live logs and health"],
              ["Recover", "Rollbacks and backups"],
            ].map(([title, body]) => (
              <div key={title} className="bg-[#080909] p-6">
                <Checkmark />
                <div className="mt-5 text-[18px] font-[520] text-[#e7e7e2]">{title}</div>
                <div className="mt-2 text-[14px] leading-[1.6] text-[#8d8e88]">{body}</div>
              </div>
            ))}
          </div>
        </div>
      </Container>
    </Section>
  );
}

function Proof3() {
  return (
    <Section className="section-rule overflow-hidden">
      <AsciiField className="right-0 top-0 -z-10 hidden lg:block" cols={58} rows={24} seed={9} opacity={0.08} />
      <Container>
        <div className="grid items-center gap-16 lg:grid-cols-[1.02fr_0.98fr]">
          <div>
            <SectionIntro
              title={<>You don't have to become a DevOps person.</>}
              body="If you can push code, you can ship with Shipper. The console tells you what happened in plain language, and the next useful action is always one click away."
            />
            <div className="mt-9 space-y-4">
              <Check>No YAML required to get your first app live.</Check>
              <Check>No SSH session needed for the everyday work.</Check>
              <Check>No dashboard account between you and your server.</Check>
            </div>
            <div className="mt-9">
              <ActionPrimary href="#get-shipper" />
            </div>
          </div>
          <Screenshot
            src="/shots/overview.png"
            alt="Shipper application overview with a clear deployment status and next actions"
          />
        </div>
      </Container>
    </Section>
  );
}

function Proof4() {
  return (
    <Section className="section-rule">
      <Container>
        <SectionIntro
          title={<>Use what you already have.</>}
          body="Shipper fits the way your project already works. There is no special directory layout and no new config language."
          align="center"
        />
        <div className="mt-14 overflow-hidden border-y border-white/[0.09] bg-[#080909]">
          <div className="flex w-max animate-marquee">
            {[0, 1].map((copy) => (
              <div key={copy} className="flex shrink-0 items-center">
                {["Local folder", "Public Git", "Private GitHub", "Docker image", "Compose file", "Static site"].map((item) => (
                  <div key={`${copy}-${item}`} className="flex items-center gap-5 border-r border-white/[0.08] px-10 py-6">
                    <span className="size-2 rounded-full bg-[#79e3a6]" />
                    <span className="whitespace-nowrap text-[18px] font-[500] text-[#d8d8d2]">{item}</span>
                  </div>
                ))}
              </div>
            ))}
          </div>
        </div>
        <div className="mt-10 flex flex-col items-center gap-6 text-center">
          <p className="max-w-[52ch] text-[17px] leading-[1.7] text-[#969791]">
            Bring a repo, drag in a folder, or point at an image. Your first deploy
            does not need a Git provider.
          </p>
          <ActionSecondary href="#how-it-works">See how a deploy works</ActionSecondary>
        </div>
      </Container>
    </Section>
  );
}

function Proof5() {
  return (
    <Section className="section-rule">
      <Container>
        <div className="grid gap-16 lg:grid-cols-[0.9fr_1.1fr]">
          <SectionIntro
            title={<>What you keep when you host it yourself.</>}
            body="The point is not to move everything to another platform. The point is to stop needing one."
          />
          <div className="grid gap-px overflow-hidden rounded-2xl border border-white/[0.1] bg-white/[0.08] sm:grid-cols-2">
            {[
              ["Your server", "The VPS or machine you already pay for."],
              ["Your data", "Databases, volumes, and backups on your disk."],
              ["Your release history", "Every deploy, with a way back."],
              ["Your domain", "HTTPS, aliases, and redirects you control."],
              ["Your agent access", "Scoped tokens, not your SSH keys."],
              ["Your exit", "Open source, MIT, no proprietary runtime."],
            ].map(([title, body], index) => (
              <div key={title} className="bg-[#080909] p-7">
                <div className="mono text-[14px] text-[#8d8e88]">{String(index + 1).padStart(2, "0")}</div>
                <div className="mt-5 text-[19px] font-[520] text-[#e7e7e2]">{title}</div>
                <p className="mt-2 text-[15px] leading-[1.65] text-[#8d8e88]">{body}</p>
              </div>
            ))}
          </div>
        </div>
      </Container>
    </Section>
  );
}

function Proof6() {
  const features = [
    ["A live URL", "Domains and HTTPS without a separate proxy project.", GlobeIcon],
    ["A way back", "Roll back to a previous release when the new one is wrong.", ShieldCheckIcon],
    ["A real paper trail", "Every release, log, and failed stage stays readable.", TerminalIcon],
    ["Data that survives", "Backups for databases and volumes, on storage you choose.", DatabaseBackupIcon],
    ["Your own stack", "Dockerfile, Compose, static, or a prebuilt image.", SourceCodeIcon],
    ["Agent access", "The same deploy path for your CLI, API, and AI agents.", CloudServerIcon],
  ] as const;
  return (
    <Section className="section-rule">
      <Container>
        <SectionIntro
          title={<>A launch checklist that stays checked.</>}
          body="Everything you need to put an app in front of people, and keep it there after the first deploy."
          align="center"
        />
        <div className="mt-16 grid gap-px overflow-hidden rounded-2xl border border-white/[0.1] bg-white/[0.08] md:grid-cols-2 lg:grid-cols-3">
          {features.map(([title, body, Icon]) => (
            <div key={title} className="group bg-[#080909] p-8 transition-colors duration-300 hover:bg-[#0c0d0d]">
              <HugeiconsIcon icon={Icon} size={24} strokeWidth={1.4} className="text-[#8d8e88]" />
              <h3 className="mt-8 text-[21px] font-[520] tracking-[-0.03em] text-[#e7e7e2]">{title}</h3>
              <p className="mt-2.5 text-[15px] leading-[1.7] text-[#8d8e88]">{body}</p>
            </div>
          ))}
        </div>
        <div className="mt-10 flex flex-col items-center gap-6 text-center">
          <Status label="Free to self-host" />
          <ActionPrimary href="#get-shipper" />
        </div>
      </Container>
    </Section>
  );
}

function Checkmark() {
  return (
    <span className="flex size-10 items-center justify-center rounded-full border border-[#79e3a6]/20 bg-[#79e3a6]/[0.07]">
      <HugeiconsIcon icon={CheckmarkCircle02Icon} size={19} strokeWidth={1.6} className="text-[#79e3a6]" />
    </span>
  );
}

export function Proof({ variant = 1 }: { variant?: number }) {
  switch (variant) {
    case 2:
      return <Proof2 />;
    case 3:
      return <Proof3 />;
    case 4:
      return <Proof4 />;
    case 5:
      return <Proof5 />;
    case 6:
      return <Proof6 />;
    default:
      return <Proof1 />;
  }
}
