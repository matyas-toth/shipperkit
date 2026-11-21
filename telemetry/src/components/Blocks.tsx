import { IconArrowDownRight, IconArrowUpRight } from "@tabler/icons-react";
import { formatCompact, formatNumber, formatPercent } from "@/lib/format";
import type { Slice } from "@/lib/metrics";
import { SERIES } from "@/lib/colors";


export function Card({ title, figure, className = "", children }: { title: string; figure?: React.ReactNode; className?: string; children: React.ReactNode }) {
  return (
    <section className={`flex min-w-0 flex-col rounded-card border border-border bg-surface p-4 shadow-[inset_0_0.5px_0_#ffffff0d] ${className}`}>
      <header className="mb-3 flex min-h-6 items-baseline justify-between gap-3">
        <h2 className="truncate text-[15px] font-[590] text-text">{title}</h2>
        {figure != null && <span className="tabular shrink-0 text-[13px] font-[520] text-secondary">{figure}</span>}
      </header>
      {children}
    </section>
  );
}

export function SectionTitle({ children }: { children: React.ReactNode }) {
  return <h2 className="mt-10 mb-4 text-[18px] font-[600] tracking-[-0.24px]">{children}</h2>;
}

export function Stat({ label, value, delta, color }: { label: string; value: string; delta?: number | null; color: string }) {
  return (
    <div className="relative min-w-0 overflow-hidden rounded-card border border-border bg-surface px-4 py-3.5 shadow-[inset_0_0.5px_0_#ffffff0d]">
      <div className="flex items-center gap-2 text-[12px] font-[520] text-muted">
        <span className="size-2 shrink-0 rounded-full" style={{ background: color }} aria-hidden />
        <span className="truncate">{label}</span>
      </div>
      <div className="mt-1.5 flex items-baseline gap-2">
        <span className="tabular text-[28px] leading-9 font-[620] tracking-[-0.8px] text-text">{value}</span>
        {delta != null && delta !== 0 && Number.isFinite(delta) && (
          <span className={`tabular inline-flex items-center gap-0.5 text-[12px] font-[560] ${delta > 0 ? "text-success" : "text-danger"}`} title="Change over 7 days">
            {delta > 0 ? <IconArrowUpRight size={14} stroke={2} /> : <IconArrowDownRight size={14} stroke={2} />}
            {formatPercent(Math.abs(delta))}
          </span>
        )}
      </div>
    </div>
  );
}

/** Horizontal bars with direct labels. Values are shares of `total`. */
export function BarList({ items, total, color, format = "count", labelFor = (key: string) => key }: { items: Slice[]; total: number; color: string; format?: "count" | "share"; labelFor?: (key: string) => string }) {
  if (!items.length) return <Empty />;
  const max = Math.max(...items.map(item => item.value), 1);
  return (
    <ul className="grid gap-1">
      {items.map(item => {
        const share = total ? item.value / total : 0;
        return (
          <li key={item.label} title={`${labelFor(item.label)}: ${formatNumber(item.value)} (${formatPercent(share)})`} className="group relative flex h-8 items-center gap-3 rounded-[6px] px-2 transition-colors hover:bg-layer-02">
            <span className="absolute inset-y-1 left-0 rounded-[5px] opacity-[0.28] transition-opacity group-hover:opacity-[0.4]" style={{ width: `${Math.max((item.value / max) * 100, 1.5)}%`, background: color }} aria-hidden />
            <span className="relative min-w-0 flex-1 truncate pl-1 text-[13px] text-text">{labelFor(item.label)}</span>
            <span className="tabular relative text-[13px] font-[520] text-secondary">{format === "share" ? formatPercent(share) : formatCompact(item.value)}</span>
            {format === "count" && <span className="tabular relative w-10 text-right text-[12px] text-muted">{formatPercent(share)}</span>}
          </li>
        );
      })}
    </ul>
  );
}

/** One stacked bar for a small part-to-whole split, with a legend under it. */
export function SplitBar({ items, labelFor = (key: string) => key }: { items: Slice[]; labelFor?: (key: string) => string }) {
  const total = items.reduce((sum, item) => sum + item.value, 0);
  if (!total) return <Empty />;
  return (
    <div className="grid gap-3">
      <div className="flex h-3 gap-[2px] overflow-hidden rounded-full">
        {items.map((item, index) => (
          <span key={item.label} title={`${labelFor(item.label)}: ${formatNumber(item.value)}`} className="h-full first:rounded-l-full last:rounded-r-full" style={{ width: `${(item.value / total) * 100}%`, background: SERIES[index % SERIES.length] }} />
        ))}
      </div>
      <ul className="grid gap-1.5">
        {items.map((item, index) => (
          <li key={item.label} className="flex items-center gap-2 text-[13px]">
            <span className="size-2.5 shrink-0 rounded-[3px]" style={{ background: SERIES[index % SERIES.length] }} aria-hidden />
            <span className="min-w-0 flex-1 truncate text-text">{labelFor(item.label)}</span>
            <span className="tabular text-secondary">{formatNumber(item.value)}</span>
            <span className="tabular w-10 text-right text-[12px] text-muted">{formatPercent(item.value / total)}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}

export function Empty() {
  return <div className="grid min-h-24 flex-1 place-items-center text-[13px] text-muted">No data yet</div>;
}

export function Facts({ items }: { items: { label: string; value: string }[] }) {
  return (
    <dl className="grid grid-cols-2 overflow-hidden rounded-card border border-border bg-surface sm:grid-cols-3 lg:grid-cols-5">
      {items.map(item => (
        <div key={item.label} className="border-border-muted px-4 py-3 shadow-[inset_-1px_-1px_0_var(--color-border-muted)]">
          <dt className="text-[12px] text-muted">{item.label}</dt>
          <dd className="tabular mt-0.5 text-[17px] font-[600] tracking-[-0.2px] text-text">{item.value}</dd>
        </div>
      ))}
    </dl>
  );
}
