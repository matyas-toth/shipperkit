"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";

/** Links that rewrite one search param, styled as a segmented control. */
export function Segmented({ param, options, fallback }: { param: string; options: { value: string; label: string }[]; fallback: string }) {
  const params = useSearchParams();
  const current = params.get(param) ?? fallback;
  return <SegmentedView param={param} options={options} current={options.some(o => o.value === current) ? current : fallback} params={params.toString()} />;
}

export function SegmentedView({ param, options, current, params = "" }: { param: string; options: { value: string; label: string }[]; current: string; params?: string }) {
  return (
    <div className="flex h-9 items-center gap-0.5 rounded-[9px] border border-border bg-deep p-[3px]">
      {options.map(option => {
        const next = new URLSearchParams(params);
        next.set(param, option.value);
        next.delete("page");
        const active = option.value === current;
        return (
          <Link
            key={option.value}
            href={`?${next.toString()}`}
            prefetch
            scroll={false}
            aria-current={active ? "true" : undefined}
            className={`tabular flex h-full items-center rounded-[6px] px-3 text-[13px] font-[540] transition-colors ${active ? "bg-layer-03 text-text shadow-[inset_0_0.5px_0_#ffffff1a]" : "text-muted hover:text-text"}`}
          >
            {option.label}
          </Link>
        );
      })}
    </div>
  );
}
