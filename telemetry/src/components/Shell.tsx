import Image from "next/image";
import Link from "next/link";

export function Brand() {
  return (
    <Link href="/" className="flex h-9 shrink-0 items-center gap-2.5 rounded-control pr-2 text-[15px] font-[640] tracking-[-0.28px] text-text">
      <span className="grid size-[30px] place-items-center rounded-[9px] bg-[#232323] shadow-[0_1px_2px_#0000005c,inset_0_1px_0_#ffffff12,inset_0_0_0_1px_#ffffff1a]">
        <Image src="/shipper.svg" alt="" width={16} height={16} priority />
      </span>
      <span>
        ShipperKit <span className="hidden text-secondary sm:inline">Telemetry</span>
      </span>
    </Link>
  );
}

export function Topbar({ children }: { children?: React.ReactNode }) {
  return (
    <header className="sticky top-0 z-20 flex h-[54px] items-center gap-3 bg-shell sm:gap-6 px-4 md:px-[max(32px,calc((100vw-1160px)/2))]">
      <Brand />
      {children}
    </header>
  );
}

export function Frame({ children }: { children: React.ReactNode }) {
  return (
    <div className="mx-1.5 mb-1.5 min-h-[calc(100dvh-60px)] rounded-t-panel rounded-b-card border border-page-border bg-page">
      <main className="mx-auto w-full max-w-[1160px] px-4 pt-8 pb-16 md:px-8">{children}</main>
    </div>
  );
}
