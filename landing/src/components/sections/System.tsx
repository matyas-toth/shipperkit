import { HugeiconsIcon } from "@hugeicons/react";
import {
  ArrowUpRight01Icon,
  CheckmarkCircle02Icon,
  Clock01Icon,
  CloudServerIcon,
  DatabaseBackupIcon,
  GlobeIcon,
} from "@hugeicons/core-free-icons";
import {
  ActionPrimary,
  ActionSecondary,
  Check,
  Container,
  Screenshot,
  Section,
  SectionIntro,
  Sparkline,
  Status,
} from "@/components/primitives";
import { cn } from "@/lib/utils";

function System1() {
  return (
    <Section id="system" className="section-rule">
      <Container>
        <div className="grid items-center gap-16 lg:grid-cols-[0.94fr_1.06fr]">
          <div>
            <SectionIntro
              title={<>A failed deploy is not an outage.</>}
              body="The new release has to pass its checks before traffic moves. If it fails, the version your users already have keeps serving."
            />
            <div className="mt-9 space-y-4">
              <Check>The old release stays online while you fix the new one.</Check>
              <Check>Every failed stage keeps its log and its place.</Check>
              <Check>You can restore a recorded release when you need it.</Check>
            </div>
            <div className="mt-9 flex flex-wrap gap-3">
              <ActionPrimary href="#get-shipper" />
              <ActionSecondary href="#faq">Read the FAQ</ActionSecondary>
            </div>
          </div>
          <Screenshot
            src="/shots/failed.png"
            alt="A failed deployment with logs and the previous release still serving traffic"
          />
        </div>
      </Container>
    </Section>
  );
}

function System2() {
  return (
    <Section id="system" className="section-rule">
      <Container>
        <SectionIntro
          title={<>You can always go back.</>}
          body="Every release is recorded with its source, its build, and its result. Restoring the previous one is a reviewed action, not a rebuild from memory."
        />
        <div className="mt-16 grid gap-4 lg:grid-cols-[0.9fr_1.1fr]">
          <div className="rounded-2xl border border-white/[0.1] bg-[#080909] p-7">
            <div className="flex items-center justify-between">
              <span className="text-[18px] font-[520] text-[#e7e7e2]">Release history</span>
              <Status label="Current live" pulse />
            </div>
            <div className="mt-7 space-y-3">
              {[
                ["4f8ac21d", "Live now", "Today, 14:56", "success"],
                ["de4af792", "Previous", "Yesterday, 14:59", "muted"],
                ["7b2d1c9a", "Restored", "Nov 4, 14:59", "info"],
                ["9c0e2a4b", "Failed", "Nov 3, 14:59", "danger"],
              ].map(([hash, state, date, tone]) => (
                <div key={hash} className="rounded-xl border border-white/[0.08] bg-[#0a0b0b] p-5">
                  <div className="flex items-center gap-3">
                    <span
                      className={cn(
                        "size-2.5 rounded-full",
                        tone === "success" && "bg-[#79e3a6]",
                        tone === "danger" && "bg-[#f08c90]",
                        tone === "info" && "bg-[#9dc4f0]",
                        tone === "muted" && "bg-[#797a75]",
                      )}
                    />
                    <span className="text-[16px] text-[#dcdcd6]">{state}</span>
                    <span className="mono ml-auto text-[14px] text-[#8d8e88]">{date}</span>
                  </div>
                  <div className="mono mt-3 text-[16px] text-[#ecece7]">{hash}</div>
                </div>
              ))}
            </div>
          </div>
          <div className="flex flex-col justify-center">
            <Screenshot
              src="/shots/deployments.png"
              alt="Shipper deployments view with release history and deployment logs"
            />
            <div className="mt-4 rounded-xl border border-white/[0.1] bg-[#080909] px-6 py-5">
              <div className="flex items-center justify-between">
                <span className="text-[15px] text-[#c3c4be]">Restore a recorded release</span>
                <HugeiconsIcon icon={ArrowUpRight01Icon} size={18} strokeWidth={1.6} className="text-[#8d8e88]" />
              </div>
              <p className="mt-2 text-[14px] leading-[1.6] text-[#8d8e88]">
                The action is reviewed before it runs. Your persistent data stays put.
              </p>
            </div>
          </div>
        </div>
      </Container>
    </Section>
  );
}

function System3() {
  return (
    <Section id="system" className="section-rule">
      <Container>
        <SectionIntro
          title={<>The unglamorous parts, handled.</>}
          body="HTTPS, backups, logs, and health checks are part of the deploy, not a second project you have to finish later."
        />
        <div className="mt-11 grid gap-4 md:grid-cols-3">
          {([
            ["HTTPS", "Domains and certificates renew without a side project.", GlobeIcon],
            ["Backups", "A verified copy of your data, on storage you choose.", DatabaseBackupIcon],
            ["Health", "See the app and the machine while they are running.", CloudServerIcon],
          ] as const).map(([title, body, Icon]) => (
            <div key={title} className="surface rounded-2xl p-7">
              <span className="flex size-11 items-center justify-center rounded-xl border border-white/[0.13] bg-white/[0.05]">
                <HugeiconsIcon icon={Icon} size={22} strokeWidth={1.6} className="text-[#dcdcd6]" />
              </span>
              <h3 className="mt-7 text-[24px] font-[520] tracking-[-0.035em] text-[#ecece7]">{title}</h3>
              <p className="mt-2.5 text-[15px] leading-[1.7] text-[#a2a39d]">{body}</p>
            </div>
          ))}
        </div>
        <div className="mt-4 grid gap-4 xl:grid-cols-2">
          <Screenshot
            src="/shots/diagnostics.png"
            alt="Shipper diagnostics checks with service health"
            className="h-[280px] object-cover object-top max-sm:w-[170%] max-sm:max-w-none"
          />
          <Screenshot
            src="/shots/backups.png"
            alt="Shipper backup plans and verified backup history"
            className="h-[280px] object-cover object-top max-sm:w-[170%] max-sm:max-w-none"
          />
        </div>
        <div className="mt-10 flex justify-center">
          <ActionPrimary href="#get-shipper" />
        </div>
      </Container>
    </Section>
  );
}

function System4() {
  const events = [
    ["14:56", "New release deployed", "Live", "success"],
    ["14:58", "Health check passed", "Verified", "success"],
    ["15:12", "A later build failed", "Kept out of traffic", "danger"],
    ["15:13", "Previous release still serving", "No outage", "success"],
  ] as const;
  return (
    <Section id="system" className="section-rule overflow-hidden">
      <Container>
        <div className="grid items-center gap-16 lg:grid-cols-[0.9fr_1.1fr]">
          <div>
            <SectionIntro
              title={<>Your site stays up while you fix the release.</>}
              body="A deploy that fails at any stage is stopped before promotion. The previous release stays in front of users, with its routes and data intact."
            />
            <div className="mt-9">
              <ActionPrimary href="#get-shipper" />
            </div>
          </div>
          <div className="relative rounded-2xl border border-white/[0.1] bg-[#080909] p-8">
            <div className="absolute bottom-10 left-[38px] top-10 w-px bg-white/[0.09]" aria-hidden="true" />
            <div className="space-y-7">
              {events.map(([time, title, status, tone]) => (
                <div key={title} className="relative flex gap-5">
                  <span
                    className={cn(
                      "relative z-10 mt-1 size-3 rounded-full border-2 border-[#080909]",
                      tone === "success" ? "bg-[#79e3a6]" : "bg-[#f08c90]",
                    )}
                  />
                  <div className="flex-1">
                    <div className="flex items-center justify-between gap-4">
                      <span className="text-[17px] text-[#dcdcd6]">{title}</span>
                      <span className="mono text-[14px] text-[#797a75]">{time}</span>
                    </div>
                    <div className={cn("mt-1 text-[14px]", tone === "success" ? "text-[#79e3a6]" : "text-[#f0a3a6]")}>
                      {status}
                    </div>
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

function System5() {
  return (
    <Section id="system" className="section-rule">
      <Container>
        <div className="grid items-center gap-16 lg:grid-cols-[1.02fr_0.98fr]">
          <div>
            <SectionIntro
              title={<>Nobody can turn off your server.</>}
              body="Your apps run on infrastructure you control, with a dashboard that lives on it too. There is no platform account standing between you and production."
            />
            <div className="mt-9 grid gap-3 sm:grid-cols-2">
              {[
                ["Your account", "You own the server and the domain."],
                ["Your data", "Backups and volumes stay with you."],
                ["Your history", "Releases remain readable and restorable."],
                ["Your exit", "Open source, with no proprietary runtime."],
              ].map(([title, body]) => (
                <div key={title} className="rounded-xl border border-white/[0.09] bg-[#080909] p-5">
                  <div className="text-[17px] font-[500] text-[#e7e7e2]">{title}</div>
                  <div className="mt-2 text-[14px] leading-[1.6] text-[#8d8e88]">{body}</div>
                </div>
              ))}
            </div>
            <div className="mt-9">
              <ActionPrimary href="#get-shipper" />
            </div>
          </div>
          <div className="rounded-2xl border border-white/[0.1] bg-[#080909] p-8">
            <div className="flex items-center justify-between">
              <span className="text-[18px] font-[520] text-[#e7e7e2]">A quiet day</span>
              <Status label="All clear" pulse />
            </div>
            <div className="mt-8 space-y-5">
              {[
                ["App", "Responding normally", "success"],
                ["Certificate", "Valid and renewing", "success"],
                ["Backup", "Verified this morning", "success"],
                ["Disk", "Plenty of room", "success"],
              ].map(([label, value]) => (
                <div key={label} className="flex items-center gap-4 border-b border-white/[0.07] pb-5 last:border-0 last:pb-0">
                  <HugeiconsIcon icon={CheckmarkCircle02Icon} size={19} strokeWidth={1.6} className="text-[#79e3a6]" />
                  <div className="flex-1">
                    <div className="text-[16px] text-[#dcdcd6]">{label}</div>
                    <div className="mt-0.5 text-[14px] text-[#797a75]">{value}</div>
                  </div>
                </div>
              ))}
            </div>
            <div className="mt-8">
              <Sparkline height={56} points="0,36 24,31 48,34 72,22 96,26 120,15 144,19 168,10 180,12" />
            </div>
          </div>
        </div>
      </Container>
    </Section>
  );
}

function System6() {
  return (
    <Section id="system" className="section-rule">
      <Container>
        <SectionIntro
          title={<>The console tells you the truth.</>}
          body="No synthetic green checks. If something cannot be observed, it stays unknown until it can be. If a route is ready, you see it. If a certificate is close to expiring, you see that too."
          align="center"
        />
        <div className="mt-16 grid gap-4 lg:grid-cols-[1.1fr_0.9fr]">
          <Screenshot
            src="/shots/diagnostics.png"
            alt="Shipper diagnostics view with deployment checks and service details"
          />
          <div className="rounded-2xl border border-white/[0.1] bg-[#080909] p-8">
            <div className="flex items-center justify-between">
              <span className="text-[18px] font-[520] text-[#e7e7e2]">What you can see</span>
              <HugeiconsIcon icon={Clock01Icon} size={20} strokeWidth={1.5} className="text-[#8d8e88]" />
            </div>
            <div className="mt-8 space-y-5">
              {[
                "The release that is serving traffic",
                "The stage that failed, with its log",
                "Certificate and domain state",
                "CPU, memory, restarts, and disk",
                "The latest verified backup",
              ].map((item) => (
                <div key={item} className="flex items-start gap-3 border-b border-white/[0.07] pb-5 text-[16px] text-[#c3c4be] last:border-0 last:pb-0">
                  <HugeiconsIcon icon={CheckmarkCircle02Icon} size={18} strokeWidth={1.6} className="mt-0.5 shrink-0 text-[#79e3a6]" />
                  {item}
                </div>
              ))}
            </div>
          </div>
        </div>
      </Container>
    </Section>
  );
}

export function System({ variant = 1 }: { variant?: number }) {
  switch (variant) {
    case 2:
      return <System2 />;
    case 3:
      return <System3 />;
    case 4:
      return <System4 />;
    case 5:
      return <System5 />;
    case 6:
      return <System6 />;
    default:
      return <System1 />;
  }
}
