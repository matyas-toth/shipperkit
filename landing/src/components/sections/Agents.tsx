import { HugeiconsIcon } from "@hugeicons/react";
import {
  ArtificialIntelligence01Icon,
  ComputerTerminal01Icon,
  Key01Icon,
  LockKeyIcon,
  ShieldCheckIcon,
  Tick02Icon,
  UserIcon,
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

function AgentTerminal() {
  return (
    <div className="overflow-hidden rounded-2xl border border-white/[0.12] bg-[#080909] shadow-[0_40px_100px_-45px_rgba(0,0,0,1)]">
      <div className="screen-bar">
        <div className="flex items-center gap-2">
          <span className="screen-dot" />
          <span className="screen-dot" />
          <span className="screen-dot" />
        </div>
        <span className="mono ml-2 text-[14px] text-[#898a85]">agent session</span>
      </div>
      <div className="p-6 md:p-8">
        <div className="mono space-y-4 text-[15px] leading-[1.85]">
          <div className="text-[#85867f]"># plan without touching production</div>
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
          <div className="text-[#79e3a6]">build passed · health passed · route verified</div>
          <div className="text-[#ecece7]">live at https://store.example.com</div>
          <div className="pt-2 text-[#85867f]"># token scope: deploy + logs only</div>
        </div>
        <div className="mt-7 flex items-center justify-between border-t border-white/[0.08] pt-5">
          <Status label="Same checks as a human deploy" />
          <span className="text-[14px] text-[#797a75]">revoke anytime</span>
        </div>
      </div>
    </div>
  );
}

function Agents1() {
  return (
    <Section id="agents" className="section-rule">
      <Container>
        <div className="grid items-center gap-16 lg:grid-cols-[0.9fr_1.1fr]">
          <div>
            <SectionIntro
              title={<>Your agents can ship too.</>}
              body="Give your AI agent a scoped deploy path, not your SSH keys. It plans, deploys, reads logs, and rolls back with the same checks you get."
            />
            <div className="mt-9 space-y-4">
              <Check>Scoped tokens for deploy, logs, or read-only access.</Check>
              <Check>The same ownership and safety checks as the dashboard.</Check>
              <Check>No browser session to scrape and no shell on the host.</Check>
            </div>
            <div className="mt-9 flex flex-wrap gap-3">
              <ActionPrimary href="#get-shipper" />
              <ActionSecondary href="https://github.com/matyas-toth/shipperkit/blob/main/docs/api.md" external>
                Read the API docs
              </ActionSecondary>
            </div>
          </div>
          <AgentTerminal />
        </div>
      </Container>
    </Section>
  );
}

function Agents2() {
  return (
    <Section id="agents" className="section-rule">
      <Container>
        <SectionIntro
          title={<>Let your agent do the boring deploy work.</>}
          body="Your agent can inspect a project, propose a deploy, watch it, and explain the result. You keep the final call on anything that changes production."
          align="center"
        />
        <div className="mt-16 grid gap-px overflow-hidden rounded-2xl border border-white/[0.1] bg-white/[0.08] md:grid-cols-3">
          {([
            ["Plan", "The agent inspects the project and proposes a release.", ArtificialIntelligence01Icon],
            ["Ship", "A scoped token lets it deploy the plan it reviewed.", ComputerTerminal01Icon],
            ["Explain", "It reads the same logs and checks you would read.", ShieldCheckIcon],
          ] as const).map(([title, body, Icon], index) => (
            <div key={title} className="bg-[#080909] p-8 md:p-10">
              <div className="flex items-center justify-between">
                <HugeiconsIcon icon={Icon} size={24} strokeWidth={1.5} className="text-[#a8a9a3]" />
                <span className="mono text-[15px] text-[#85867f]">{String(index + 1).padStart(2, "0")}</span>
              </div>
              <h3 className="mt-10 text-[26px] font-[520] tracking-[-0.035em] text-[#ecece7]">{title}</h3>
              <p className="mt-3 text-[16px] leading-[1.7] text-[#969791]">{body}</p>
            </div>
          ))}
        </div>
        <div className="mt-12 flex justify-center">
          <ActionPrimary href="#get-shipper" />
        </div>
      </Container>
    </Section>
  );
}

function Agents3() {
  return (
    <Section id="agents" className="section-rule">
      <Container>
        <SectionIntro
          title={<>The same guardrails for you and your agent.</>}
          body="A token does not get a shortcut through the checks. The dashboard, the CLI, and your agent all use the same release path."
        />
        <div className="mt-11 grid gap-4 lg:grid-cols-2">
          <div className="surface flex flex-col rounded-2xl p-7">
            <div className="text-[16px] font-[520] text-[#c3c4be]">Same release path</div>
            <div className="mt-6 space-y-4">
              {([
                ["Human", "Dashboard", UserIcon],
                ["Developer", "CLI", ComputerTerminal01Icon],
                ["Agent", "API token", ArtificialIntelligence01Icon],
              ] as const).map(([kind, tool, Icon]) => (
                <div key={kind} className="flex items-center gap-4 border-b border-white/[0.08] pb-4 last:border-0 last:pb-0">
                  <span className="flex size-11 items-center justify-center rounded-lg border border-white/[0.13] bg-white/[0.05]">
                    <HugeiconsIcon icon={Icon} size={21} strokeWidth={1.6} className="text-[#dcdcd6]" />
                  </span>
                  <div>
                    <div className="text-[16px] font-[500] text-[#e7e7e2]">{kind}</div>
                    <div className="mt-0.5 text-[14px] text-[#8d8e88]">{tool}</div>
                  </div>
                  <span className="ml-auto flex items-center gap-1.5 text-[14px] text-[#79e3a6]">
                    <HugeiconsIcon icon={Tick02Icon} size={15} strokeWidth={1.9} />
                    same checks
                  </span>
                </div>
              ))}
            </div>
          </div>
          <div className="surface flex flex-col rounded-2xl p-7">
            <div className="flex items-center justify-between">
              <span className="text-[16px] font-[520] text-[#c3c4be]">Agent access</span>
              <Status label="90-day tokens" />
            </div>
            <div className="mt-6 space-y-4">
              {[
                ["Can deploy", "Yes"],
                ["Can read logs", "Yes"],
                ["Can reveal secrets", "No"],
                ["Can run host commands", "No"],
              ].map(([permission, value]) => (
                <div key={permission} className="flex items-center justify-between border-b border-white/[0.07] pb-4 text-[16px] last:border-0 last:pb-0">
                  <span className="text-[#c3c4be]">{permission}</span>
                  <span className={cn("text-[14px]", value === "Yes" ? "text-[#79e3a6]" : "text-[#797a75]")}>{value}</span>
                </div>
              ))}
            </div>
            <div className="mt-auto flex items-center gap-3 border-t border-white/[0.08] pt-5 text-[14px] leading-[1.6] text-[#8d8e88]">
              <HugeiconsIcon icon={LockKeyIcon} size={18} strokeWidth={1.6} className="shrink-0 text-[#797a75]" />
              Revoke a token without touching the app.
            </div>
          </div>
        </div>
      </Container>
    </Section>
  );
}

function Agents4() {
  return (
    <Section id="agents" className="section-rule overflow-hidden">
      <Container>
        <div className="grid items-start gap-16 lg:grid-cols-[0.9fr_1.1fr]">
          <div>
            <SectionIntro
              title={<>Ask in plain English. Ship with real checks.</>}
              body="Your agent can plan the release in a chat window. Shipper still validates the source, builds the app, and verifies the route before traffic moves."
            />
            <div className="mt-9 space-y-4">
              <Check>Works with any agent that can call a CLI or an API.</Check>
              <Check>The plan and the operation are both durable.</Check>
              <Check>The agent can stop at review and wait for you.</Check>
            </div>
            <div className="mt-9">
              <ActionPrimary href="#get-shipper" />
            </div>
          </div>
          <div className="space-y-4">
            <div className="rounded-2xl border border-white/[0.1] bg-[#080909] p-7">
              <div className="flex items-center gap-3">
                <HugeiconsIcon icon={ArtificialIntelligence01Icon} size={22} strokeWidth={1.5} className="text-[#a8a9a3]" />
                <span className="text-[16px] font-[500] text-[#e7e7e2]">Your agent</span>
              </div>
              <p className="mt-4 text-[17px] leading-[1.7] text-[#c3c4be]">
                “Deploy this folder to my server, check the logs, and tell me if
                anything fails.”
              </p>
            </div>
            <div className="rounded-2xl border border-white/[0.1] bg-[#080909] p-7">
              <div className="flex items-center justify-between">
                <span className="text-[16px] font-[500] text-[#e7e7e2]">Shipper</span>
                <span className="mono text-[14px] text-[#797a75]">operation complete</span>
              </div>
              <div className="mt-5 space-y-3 text-[15px] leading-[1.7]">
                <div className="flex items-center justify-between border-b border-white/[0.07] pb-3">
                  <span className="text-[#8d8e88]">Build</span>
                  <span className="text-[#79e3a6]">Passed</span>
                </div>
                <div className="flex items-center justify-between border-b border-white/[0.07] pb-3">
                  <span className="text-[#8d8e88]">Health check</span>
                  <span className="text-[#79e3a6]">Passed</span>
                </div>
                <div className="flex items-center justify-between border-b border-white/[0.07] pb-3">
                  <span className="text-[#8d8e88]">Route</span>
                  <span className="text-[#79e3a6]">Verified</span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-[#8d8e88]">Live URL</span>
                  <span className="text-[#ecece7]">store.example.com</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </Container>
    </Section>
  );
}

function Agents5() {
  return (
    <Section id="agents" className="section-rule">
      <Container>
        <SectionIntro
          title={<>One key per agent. No shared password.</>}
          body="Give every agent its own access. Limit it to an app, a capability, and an expiry date, then revoke it when the work is done."
          align="center"
        />
        <div className="mx-auto mt-16 grid max-w-[1000px] gap-px overflow-hidden rounded-2xl border border-white/[0.1] bg-white/[0.08] md:grid-cols-3">
          {([
            ["Create", "Name the agent and choose what it can do.", Key01Icon],
            ["Scope", "Limit it to one app and the actions it needs.", ShieldCheckIcon],
            ["Revoke", "Expire or revoke without touching production.", LockKeyIcon],
          ] as const).map(([title, body, Icon], index) => (
            <div key={title} className="bg-[#080909] p-8">
              <div className="flex items-center justify-between">
                <HugeiconsIcon icon={Icon} size={24} strokeWidth={1.5} className="text-[#a8a9a3]" />
                <span className="mono text-[15px] text-[#85867f]">{String(index + 1).padStart(2, "0")}</span>
              </div>
              <h3 className="mt-8 text-[24px] font-[520] tracking-[-0.03em] text-[#ecece7]">{title}</h3>
              <p className="mt-2.5 text-[16px] leading-[1.7] text-[#969791]">{body}</p>
            </div>
          ))}
        </div>
        <div className="mt-12 flex justify-center">
          <ActionPrimary href="#get-shipper" />
        </div>
      </Container>
    </Section>
  );
}

function Agents6() {
  return (
    <Section id="agents" className="section-rule">
      <Container>
        <div className="grid items-center gap-16 lg:grid-cols-[1.1fr_0.9fr]">
          <div>
            <SectionIntro
              title={<>Agents shouldn't need your SSH key.</>}
              body="SSH is a master key. Shipper gives an agent a narrow, revocable path to the actions it actually needs."
              size="lg"
            />
            <div className="mt-10 grid gap-3 sm:grid-cols-3">
              {[
                ["Scoped", "Only the apps you choose."],
                ["Auditable", "Every action has an owner."],
                ["Revocable", "One click ends access."],
              ].map(([title, body]) => (
                <div key={title} className="rounded-xl border border-white/[0.09] bg-[#080909] p-5">
                  <div className="text-[16px] font-[500] text-[#e7e7e2]">{title}</div>
                  <div className="mt-2 text-[14px] leading-[1.6] text-[#8d8e88]">{body}</div>
                </div>
              ))}
            </div>
            <div className="mt-9">
              <ActionPrimary href="#get-shipper" />
            </div>
          </div>
          <div className="rounded-2xl border border-white/[0.12] bg-[#080909] p-8">
            <div className="text-[15px] text-[#8d8e88]">Instead of this</div>
            <div className="mono mt-3 rounded-lg border border-[#f08c90]/20 bg-[#f08c90]/[0.04] p-4 text-[15px] text-[#f0a3a6]">
              ssh root@your-server
            </div>
            <div className="mt-7 text-[15px] text-[#8d8e88]">Let the agent use this</div>
            <div className="mono mt-3 rounded-lg border border-[#79e3a6]/20 bg-[#79e3a6]/[0.04] p-4 text-[15px] text-[#b9f0cd]">
              shipper deploy --app store --wait
            </div>
            <div className="mt-7 border-t border-white/[0.08] pt-6">
              <div className="flex items-center justify-between text-[14px]">
                <span className="text-[#8d8e88]">Token scope</span>
                <span className="text-[#c3c4be]">store · deploy · logs</span>
              </div>
              <div className="mt-3 flex items-center justify-between text-[14px]">
                <span className="text-[#8d8e88]">Expires</span>
                <span className="text-[#c3c4be]">90 days</span>
              </div>
            </div>
          </div>
        </div>
      </Container>
    </Section>
  );
}

export function Agents({ variant = 1 }: { variant?: number }) {
  switch (variant) {
    case 2:
      return <Agents2 />;
    case 3:
      return <Agents3 />;
    case 4:
      return <Agents4 />;
    case 5:
      return <Agents5 />;
    case 6:
      return <Agents6 />;
    default:
      return <Agents1 />;
  }
}
