"use client";

import { useId } from "react";
import { Area, AreaChart, Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis, type TooltipContentProps } from "recharts";
import { formatDay, formatNumber } from "@/lib/format";
import type { DayPoint, Slice } from "@/lib/metrics";
import { SERIES } from "@/lib/colors";

const GRID = "#232424";
const TICK = { fill: "#8a8a85", fontSize: 11 };

type Row = { label: string; color: string; value: number };

function TooltipBox({ title, rows, total }: { title: string; rows: Row[]; total?: number }) {
  return (
    <div className="min-w-40 rounded-[8px] border border-border bg-layer-02/95 px-3 py-2 text-[12px] shadow-[0_12px_32px_#00000080] backdrop-blur">
      <div className="mb-1.5 font-[560] text-text">{title}</div>
      <ul className="grid gap-1">
        {rows.map(row => (
          <li key={row.label} className="flex items-center gap-2">
            <span className="size-2 shrink-0 rounded-[2px]" style={{ background: row.color }} />
            <span className="flex-1 text-secondary">{row.label}</span>
            <span className="tabular font-[560] text-text">{formatNumber(row.value)}</span>
          </li>
        ))}
      </ul>
      {total != null && (
        <div className="mt-1.5 flex justify-between border-t border-border pt-1.5 text-secondary">
          <span>Total</span>
          <span className="tabular font-[560] text-text">{formatNumber(total)}</span>
        </div>
      )}
    </div>
  );
}

function tickEvery(length: number) {
  if (length <= 31) return 6;
  if (length <= 92) return 14;
  return 60;
}

function Frame({ height, label, children }: { height: number; label: string; children: React.ReactElement }) {
  return (
    <div role="img" aria-label={label} style={{ height }} className="-ml-2 w-[calc(100%+8px)]">
      <ResponsiveContainer width="100%" height="100%" initialDimension={{ width: 800, height }}>
        {children}
      </ResponsiveContainer>
    </div>
  );
}

const axes = (length: number) => [
  <CartesianGrid key="grid" vertical={false} stroke={GRID} />,
  <XAxis key="x" dataKey="day" tickFormatter={formatDay} tick={TICK} tickLine={false} axisLine={false} interval={tickEvery(length)} minTickGap={24} dy={6} />,
  <YAxis key="y" allowDecimals={false} tick={TICK} tickLine={false} axisLine={false} width={40} />,
];

/** A single time series as a filled line. */
export function TrendChart({ days, dataKey, name, color, height = 240 }: { days: DayPoint[]; dataKey: "active" | "new" | "upgrades"; name: string; color: string; height?: number }) {
  const gradient = useId();
  return (
    <Frame height={height} label={`${name} per day`}>
      <AreaChart data={days} margin={{ top: 8, right: 20, bottom: 0, left: 0 }}>
        <defs>
          <linearGradient id={gradient} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor={color} stopOpacity={0.32} />
            <stop offset="100%" stopColor={color} stopOpacity={0} />
          </linearGradient>
        </defs>
        {axes(days.length)}
        <Tooltip
          cursor={{ stroke: "#4a4b4b", strokeWidth: 1 }}
          content={({ active, payload }: TooltipContentProps) =>
            active && payload?.length ? <TooltipBox title={formatDay(String(payload[0].payload.day))} rows={[{ label: name, color, value: Number(payload[0].value) }]} /> : null
          }
        />
        <Area type="monotone" dataKey={dataKey} name={name} stroke={color} strokeWidth={2} fill={`url(#${gradient})`} activeDot={{ r: 4, stroke: "#161717", strokeWidth: 2 }} isAnimationActive={false} />
      </AreaChart>
    </Frame>
  );
}

/** Daily counts as columns. */
export function DailyBars({ days, dataKey, name, color, height = 180 }: { days: DayPoint[]; dataKey: "new" | "upgrades"; name: string; color: string; height?: number }) {
  return (
    <Frame height={height} label={`${name} per day`}>
      <BarChart data={days} margin={{ top: 8, right: 8, bottom: 0, left: 0 }} barCategoryGap={days.length > 120 ? 0 : 2}>
        {axes(days.length)}
        <Tooltip
          cursor={{ fill: "#ffffff08" }}
          content={({ active, payload }: TooltipContentProps) =>
            active && payload?.length ? <TooltipBox title={formatDay(String(payload[0].payload.day))} rows={[{ label: name, color, value: Number(payload[0].value) }]} /> : null
          }
        />
        <Bar dataKey={dataKey} name={name} fill={color} radius={[3, 3, 0, 0]} maxBarSize={18} isAnimationActive={false} />
      </BarChart>
    </Frame>
  );
}

/** Active instances per day stacked by the version they ran. */
export function VersionChart({ days, keys, height = 260 }: { days: DayPoint[]; keys: string[]; height?: number }) {
  const data = days.map(point => ({ day: point.day, ...Object.fromEntries(keys.map(key => [key, point.versions[key] ?? 0])) }));
  const color = (index: number, key: string) => (key === "Other" ? "#5b5c5a" : SERIES[index % SERIES.length]);
  return (
    <div className="grid gap-3">
      <Frame height={height} label="Active instances per day by version">
        <AreaChart data={data} margin={{ top: 8, right: 20, bottom: 0, left: 0 }}>
          {axes(days.length)}
          <Tooltip
            cursor={{ stroke: "#4a4b4b", strokeWidth: 1 }}
            content={({ active, payload }: TooltipContentProps) => {
              if (!active || !payload?.length) return null;
              const rows = payload
                .filter(item => Number(item.value) > 0)
                .map(item => ({ label: String(item.name), color: String(item.color), value: Number(item.value) }))
                .reverse();
              return <TooltipBox title={formatDay(String(payload[0].payload.day))} rows={rows} total={rows.reduce((sum, row) => sum + row.value, 0)} />;
            }}
          />
          {[...keys].reverse().map(key => {
            const index = keys.indexOf(key);
            return <Area key={key} type="monotone" dataKey={key} name={key} stackId="v" stroke={color(index, key)} strokeWidth={1.5} fill={color(index, key)} fillOpacity={0.55} isAnimationActive={false} />;
          })}
        </AreaChart>
      </Frame>
      <ul className="flex flex-wrap gap-x-4 gap-y-1.5 pl-1">
        {keys.map((key, index) => (
          <li key={key} className="flex items-center gap-1.5 font-mono text-[12px] text-secondary">
            <span className="size-2.5 rounded-[3px]" style={{ background: color(index, key) }} />
            {key}
          </li>
        ))}
      </ul>
    </div>
  );
}

/** A histogram over ordered buckets. */
export function Columns({ items, name, color, height = 180 }: { items: Slice[]; name: string; color: string; height?: number }) {
  return (
    <Frame height={height} label={name}>
      <BarChart data={items} margin={{ top: 8, right: 8, bottom: 0, left: 0 }} barCategoryGap="18%">
        <CartesianGrid vertical={false} stroke={GRID} />
        <XAxis dataKey="label" tick={TICK} tickLine={false} axisLine={false} interval={0} dy={6} />
        <YAxis allowDecimals={false} tick={TICK} tickLine={false} axisLine={false} width={40} />
        <Tooltip
          cursor={{ fill: "#ffffff08" }}
          content={({ active, payload }: TooltipContentProps) =>
            active && payload?.length ? <TooltipBox title={String(payload[0].payload.label)} rows={[{ label: name, color, value: Number(payload[0].value) }]} /> : null
          }
        />
        <Bar dataKey="value" name={name} fill={color} radius={[4, 4, 0, 0]} maxBarSize={44} isAnimationActive={false} />
      </BarChart>
    </Frame>
  );
}
