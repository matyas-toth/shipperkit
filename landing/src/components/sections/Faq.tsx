import { HugeiconsIcon } from "@hugeicons/react";
import {
  ArrowDown01Icon,
} from "@hugeicons/core-free-icons";
import { ActionSecondary, Container, Section, SectionIntro, Status } from "@/components/primitives";
import { cn } from "@/lib/utils";

const faqs = [
  {
    q: "What do I need to get started?",
    a: "A supported Ubuntu server and a project to deploy. The installer handles Docker and the proxy. A domain is only needed when you want public HTTPS.",
  },
  {
    q: "Does it work with my stack?",
    a: "Static sites, Dockerfiles, Docker Compose, prebuilt images, and most language projects. If it runs in Docker, it can run on Shipper.",
  },
  {
    q: "Do I need a GitHub account?",
    a: "No. Deploy a local folder, a public Git URL, an image, a Compose file, or an inline Dockerfile. Private GitHub is optional.",
  },
  {
    q: "What happens when a deploy fails?",
    a: "The previous release keeps serving traffic. The failed stage, its logs, and the release that is still live stay together so you can fix the problem.",
  },
  {
    q: "Can my AI agent use it?",
    a: "Yes. The CLI and API expose the same plan, deploy, inspect, and rollback actions with scoped tokens. An agent does not need your SSH key or browser session.",
  },
  {
    q: "How much does it cost?",
    a: "The software is free and MIT licensed. You pay for the server and domain you already use. There is no platform fee, seat count, or build-minute meter.",
  },
  {
    q: "Is HTTPS included?",
    a: "Yes. Point a domain at the server and Shipper handles the certificate and renewal. Local installs can use .localhost names over HTTP.",
  },
  {
    q: "Can I leave later?",
    a: "Yes. Your apps run on ordinary Docker resources, your data stays on your server, and the software is open source. Stopping Shipper does not delete your workloads.",
  },
];

function Wrapper({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <Section id="faq" className={cn("section-rule", className)}>
      {children}
    </Section>
  );
}

function Faq1() {
  return (
    <Wrapper>
      <Container>
        <div className="grid gap-14 lg:grid-cols-[0.8fr_1.2fr] lg:gap-20">
          <div className="lg:sticky lg:top-28 lg:self-start">
            <SectionIntro
              title={<>Questions, answered before you install.</>}
              body="The things a developer actually wants to know before handing over a server."
            />
            <div className="mt-9 flex items-center gap-4">
              <Status label="Open source" />
              <span className="text-[14px] text-[#797a75]">MIT</span>
            </div>
          </div>
          <div className="border-t border-white/[0.1]">
            {faqs.map((item) => (
              <details key={item.q} className="group border-b border-white/[0.1]">
                <summary className="flex cursor-pointer list-none items-start justify-between gap-8 py-7 text-[18px] font-[480] text-[#e7e7e2] transition-colors hover:text-white [&::-webkit-details-marker]:hidden">
                  {item.q}
                  <HugeiconsIcon
                    icon={ArrowDown01Icon}
                    size={20}
                    strokeWidth={1.6}
                    className="mt-1 shrink-0 text-[#797a75] transition-transform duration-300 group-open:rotate-180"
                  />
                </summary>
                <p className="max-w-[68ch] pb-7 pr-10 text-[16px] leading-[1.75] text-[#969791]">{item.a}</p>
              </details>
            ))}
          </div>
        </div>
      </Container>
    </Wrapper>
  );
}

function Faq2() {
  return (
    <Wrapper>
      <Container className="max-w-[920px]">
        <SectionIntro
          title={<>The whole story, in public.</>}
          body="No sales-shaped answers. The README and the docs describe what is supported today."
          align="center"
        />
        <div className="mx-auto mt-14 max-w-[800px] border-t border-white/[0.1]">
          {faqs.map((item) => (
            <details key={item.q} className="group border-b border-white/[0.1]">
              <summary className="flex cursor-pointer list-none items-center gap-6 py-7 [&::-webkit-details-marker]:hidden">
                <span className="flex-1 text-[20px] font-[480] tracking-[-0.02em] text-[#e7e7e2]">{item.q}</span>
                <HugeiconsIcon
                  icon={ArrowDown01Icon}
                  size={20}
                  strokeWidth={1.6}
                  className="shrink-0 text-[#85867f] transition-transform duration-300 group-open:rotate-180"
                />
              </summary>
              <p className="max-w-[68ch] pb-7 text-[16px] leading-[1.75] text-[#969791]">{item.a}</p>
            </details>
          ))}
        </div>
      </Container>
    </Wrapper>
  );
}

function Faq3() {
  return (
    <Wrapper>
      <Container>
        <SectionIntro
          title={<>Start with the practical parts.</>}
          body="The answers that change whether Shipper fits your server and your workflow."
          size="sm"
        />
        <div className="mt-10 max-w-[900px]">
          {faqs.slice(0, 6).map((item) => (
            <details key={item.q} className="group border-b border-white/[0.1]">
              <summary className="flex cursor-pointer list-none items-center gap-4 py-5 text-[16px] text-[#ecece7] [&::-webkit-details-marker]:hidden md:py-6 md:text-[18px]">
                <span className="flex-1">{item.q}</span>
                <HugeiconsIcon icon={ArrowDown01Icon} size={17} strokeWidth={1.6} className="shrink-0 text-[#8d8e88] transition-transform duration-300 group-open:rotate-180" />
              </summary>
              <p className="max-w-[72ch] pb-5 text-[15px] leading-[1.7] text-[#a2a39d] md:text-[16px]">{item.a}</p>
            </details>
          ))}
        </div>
      </Container>
    </Wrapper>
  );
}

function Faq4() {
  return (
    <Wrapper>
      <Container>
        <div className="grid gap-16 lg:grid-cols-[1.1fr_0.9fr]">
          <div>
            <SectionIntro
              title={<>Read the boundary before the pitch.</>}
              body="These are the answers that decide whether Shipper belongs on your server."
            />
            <div className="mt-10">
              {faqs.slice(0, 5).map((item) => (
                <div key={item.q} className="border-t border-white/[0.1] py-7 last:border-b">
                  <h3 className="text-[20px] font-[500] tracking-[-0.025em] text-[#e7e7e2]">{item.q}</h3>
                  <p className="mt-3 max-w-[60ch] text-[16px] leading-[1.75] text-[#969791]">{item.a}</p>
                </div>
              ))}
            </div>
          </div>
          <div className="lg:pt-28">
            <div className="rounded-2xl border border-white/[0.1] bg-[#080909] p-8">
              <h3 className="text-[28px] font-[520] leading-tight tracking-[-0.035em] text-[#ecece7]">
                Read the operations guide.
              </h3>
              <p className="mt-4 text-[16px] leading-[1.7] text-[#969791]">
                It lists what Shipper can repair, what it refuses to guess, and
                which failures still need an operator.
              </p>
              <div className="mt-8">
                <ActionSecondary href="/docs/diagnostics/">
                  Open the operations guide
                </ActionSecondary>
              </div>
              <div className="mt-8 flex items-center gap-3 border-t border-white/[0.08] pt-6">
                <Status label="Public documentation" />
              </div>
            </div>
          </div>
        </div>
      </Container>
    </Wrapper>
  );
}

function Faq5() {
  return (
    <Wrapper>
      <Container className="max-w-[980px]">
        <SectionIntro
          title={<>The console answers the same questions.</>}
          body="The CLI and the API expose the same facts as the dashboard, with stable output and a clear result."
        />
        <div className="mt-12 space-y-3">
          {faqs.slice(0, 6).map((item) => (
            <details key={item.q} className="group overflow-hidden rounded-xl border border-white/[0.1] bg-[#080909]">
              <summary className="flex cursor-pointer list-none items-center gap-5 px-6 py-5 text-[17px] text-[#e7e7e2] [&::-webkit-details-marker]:hidden">
                <span className="mono text-[14px] text-[#79e3a6]">?</span>
                <span className="flex-1">{item.q}</span>
                <HugeiconsIcon icon={ArrowDown01Icon} size={18} strokeWidth={1.6} className="text-[#85867f] transition-transform duration-300 group-open:rotate-180" />
              </summary>
              <div className="border-t border-white/[0.08] px-6 py-5 text-[15px] leading-[1.75] text-[#969791]">
                {item.a}
              </div>
            </details>
          ))}
        </div>
      </Container>
    </Wrapper>
  );
}

function Faq6() {
  return (
    <Wrapper>
      <Container>
        <div className="grid gap-14 lg:grid-cols-[0.7fr_1.3fr] lg:gap-20">
          <SectionIntro
            title={<>Short answers for long servers.</>}
            body="The details matter at 3 a.m. Keep the docs close."
          />
          <div className="grid gap-x-12 gap-y-0 sm:grid-cols-2">
            {faqs.map((item) => (
              <details key={item.q} className="group border-t border-white/[0.1] py-5">
                <summary className="flex cursor-pointer list-none items-start justify-between gap-4 text-[17px] font-[500] text-[#dcdcd6] [&::-webkit-details-marker]:hidden">
                  {item.q}
                  <HugeiconsIcon icon={ArrowDown01Icon} size={18} strokeWidth={1.6} className="mt-1 shrink-0 text-[#85867f] transition-transform duration-300 group-open:rotate-180" />
                </summary>
                <p className="mt-3 pr-6 text-[15px] leading-[1.7] text-[#8d8e88]">{item.a}</p>
              </details>
            ))}
          </div>
        </div>
      </Container>
    </Wrapper>
  );
}

export function Faq({ variant = 1 }: { variant?: number }) {
  switch (variant) {
    case 2:
      return <Faq2 />;
    case 3:
      return <Faq3 />;
    case 4:
      return <Faq4 />;
    case 5:
      return <Faq5 />;
    case 6:
      return <Faq6 />;
    default:
      return <Faq1 />;
  }
}
