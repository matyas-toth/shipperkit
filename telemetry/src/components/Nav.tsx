"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { IconChartArea, IconServer2 } from "@tabler/icons-react";

const links = [
  { href: "/", label: "Overview", icon: IconChartArea },
  { href: "/instances", label: "Instances", icon: IconServer2 },
];

export function Nav() {
  const pathname = usePathname();
  return (
    <nav className="flex items-center gap-1 rounded-[9px] p-0.5">
      {links.map(({ href, label, icon: Icon }) => {
        const active = href === "/" ? pathname === "/" : pathname.startsWith(href);
        return (
          <Link
            key={href}
            href={href}
            aria-current={active ? "page" : undefined}
            className={`flex h-8 items-center gap-2 rounded-control px-3 text-[13px] font-[520] transition-colors ${active ? "bg-layer-02 text-text shadow-[inset_0_0_0_1px_#ffffff10]" : "text-[#a6a6a1] hover:bg-surface hover:text-text"}`}
          >
            <Icon size={16} stroke={1.75} className="hidden sm:block" />
            {label}
          </Link>
        );
      })}
    </nav>
  );
}
