import { HugeiconsIcon } from "@hugeicons/react";
import {
  GithubIcon,
  Menu01Icon,
} from "@hugeicons/core-free-icons";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { Logo } from "@/components/primitives";

const links = [
  { href: "#product", label: "Product" },
  { href: "#agents", label: "Agents" },
  { href: "#pricing", label: "Pricing" },
  { href: "#faq", label: "FAQ" },
];

const repo = "https://github.com/matyas-toth/shipperkit";
const docs = "/docs/";

function Links({ className }: { className?: string }) {
  return (
    <nav className={cn("flex items-center gap-8", className)} aria-label="Primary">
      {links.map((link) => (
        <a
          key={link.href}
          href={link.href}
          className="text-[15px] font-[460] text-[#a2a39d] transition-colors duration-300 hover:text-[#f4f4f0]"
        >
          {link.label}
        </a>
      ))}
    </nav>
  );
}

function Install({ className, always = false }: { className?: string; always?: boolean }) {
  return (
    <Button
      className={cn(
        always ? "inline-flex" : "hidden sm:inline-flex",
        "h-10 shrink-0 rounded-lg border-white bg-white px-4 text-[14px] font-[540] text-[#070808] hover:bg-white/90",
        className,
      )}
      render={<a href="#get-shipper" />}
    >
      Install Shipper
    </Button>
  );
}

function MobileMenu() {
  return (
    <details className="group relative md:hidden">
      <summary aria-label="Open menu" className="flex size-11 cursor-pointer list-none items-center justify-center rounded-lg border border-white/[0.12] text-[#c3c4be] [&::-webkit-details-marker]:hidden">
        <HugeiconsIcon icon={Menu01Icon} size={18} strokeWidth={1.8} />
      </summary>
      <div className="absolute right-0 top-12 z-50 w-60 rounded-xl border border-white/[0.12] bg-[#0a0b0b] p-3 shadow-2xl">
        <a
          href="#get-shipper"
          className="mb-2 flex h-11 items-center justify-center rounded-lg bg-white text-[15px] font-[540] text-[#070808]"
        >
          Install Shipper
        </a>
        {links.map((link) => (
          <a
            key={link.href}
            href={link.href}
            className="block rounded-lg px-3 py-2.5 text-[16px] text-[#c3c4be] hover:bg-white/[0.06] hover:text-white"
          >
            {link.label}
          </a>
        ))}
        <div className="my-2 h-px bg-white/[0.08]" />
        <a href={docs} className="block rounded-lg px-3 py-2.5 text-[16px] text-[#c3c4be] hover:bg-white/[0.06] hover:text-white">
          Docs
        </a>
        <a href={repo} className="block rounded-lg px-3 py-2.5 text-[16px] text-[#c3c4be] hover:bg-white/[0.06] hover:text-white">
          GitHub
        </a>
      </div>
    </details>
  );
}

export function Nav({ variant = 1, contained = false }: { variant?: number; contained?: boolean }) {
  if (variant === 2) {
    return (
      <header className="absolute inset-x-0 top-0 z-40">
        <div className="shell flex h-[84px] items-center justify-between">
          <Logo />
          <Links className="hidden lg:flex" />
          <div className="flex items-center gap-4">
            <a href={repo} className="hidden items-center gap-2 text-[14px] text-[#9a9b95] transition-colors hover:text-white sm:flex">
              <HugeiconsIcon icon={GithubIcon} size={17} strokeWidth={1.6} />
              GitHub
            </a>
            <Install />
            <MobileMenu />
          </div>
        </div>
      </header>
    );
  }

  if (variant === 3) {
    return (
      <header className="fixed inset-x-0 top-0 z-40 border-b border-white/[0.08] bg-[#050606]/78 backdrop-blur-xl">
        <div className="shell flex h-[72px] items-center justify-between">
          <Logo />
          <div className="flex items-center gap-2 sm:gap-7">
            <Links className="hidden lg:flex" />
            <div className="hidden h-5 w-px bg-white/[0.12] sm:block" />
            <a href={docs} className="hidden text-[14px] text-[#9a9b95] transition-colors hover:text-white sm:block">
              Docs
            </a>
            <Install always className="h-11 px-3 text-[13px] max-[359px]:hidden sm:h-10 sm:px-4 sm:text-[14px]" />
            <MobileMenu />
          </div>
        </div>
      </header>
    );
  }

  if (variant === 4) {
    return (
      <header className="absolute inset-x-0 top-5 z-40 flex justify-center px-5">
        <div className="glass flex h-[64px] w-full max-w-[900px] items-center justify-between gap-6 rounded-2xl border border-white/[0.13] pl-5 pr-2.5 shadow-[0_24px_70px_-24px_rgba(0,0,0,0.9)]">
          <Logo />
          <Links className="hidden gap-7 lg:flex" />
          <div className="flex items-center gap-2">
            <a href={repo} aria-label="Open GitHub" className="hidden size-10 items-center justify-center rounded-lg text-[#9a9b95] hover:bg-white/[0.06] hover:text-white sm:flex">
              <HugeiconsIcon icon={GithubIcon} size={18} strokeWidth={1.6} />
            </a>
            <Install />
            <MobileMenu />
          </div>
        </div>
      </header>
    );
  }

  if (variant === 5) {
    return (
      <header className={cn("inset-x-0 top-0 z-40 border-b border-white/[0.08] bg-[#050606]/80 backdrop-blur-xl", contained ? "absolute" : "fixed")}>
        <div className="shell flex h-[68px] items-center justify-between">
          <Logo />
          <div className="hidden items-center gap-8 md:flex">
            {links.map((link) => (
              <a key={link.href} href={link.href} className="text-[15px] text-[#a2a39d] transition-colors hover:text-white">
                {link.label}
              </a>
            ))}
            <Install />
          </div>
          <MobileMenu />
        </div>
      </header>
    );
  }

  if (variant === 6) {
    return (
      <header className="absolute inset-x-0 top-0 z-40">
        <div className="shell flex h-[88px] items-center justify-between border-b border-white/[0.08]">
          <div className="flex items-center gap-10">
            <Logo />
            <Links className="hidden lg:flex" />
          </div>
          <div className="flex items-center gap-6">
            <a href={repo} className="hidden items-center gap-2 text-[14px] text-[#9a9b95] hover:text-white sm:flex">
              <HugeiconsIcon icon={GithubIcon} size={17} strokeWidth={1.6} />
              Star on GitHub
            </a>
            <Install />
            <MobileMenu />
          </div>
        </div>
      </header>
    );
  }

  return (
    <header className={cn("inset-x-0 top-0 z-50", contained ? "absolute" : "fixed")}>
      <div className="border-b border-white/[0.08] bg-[#050606]/82 backdrop-blur-xl">
        <div className="shell flex h-[72px] items-center justify-between">
          <div className="flex items-center gap-9">
            <Logo />
            <span className="hidden h-5 w-px bg-white/[0.12] md:block" />
            <Links className="hidden md:flex" />
          </div>
          <div className="flex items-center gap-2.5">
            <a href={docs} className="hidden px-2 text-[14px] text-[#9a9b95] transition-colors hover:text-white sm:block">
              Docs
            </a>
            <Install />
            <MobileMenu />
          </div>
        </div>
      </div>
    </header>
  );
}
