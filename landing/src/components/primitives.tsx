import * as React from "react";
import { HugeiconsIcon } from "@hugeicons/react";
import {
  ArrowRight01Icon,
  ArrowUpRight01Icon,
  CheckmarkCircle02Icon,
  Copy01Icon,
  Tick02Icon,
} from "@hugeicons/core-free-icons";
import { cn } from "@/lib/utils";

export const INSTALL_COMMAND =
  "curl -fsSL https://shipperkit.dev/install.sh | sudo bash";

export function Container({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return <div className={cn("shell", className)}>{children}</div>;
}

export function Section({
  id,
  children,
  className,
}: {
  id?: string;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <section id={id} className={cn("section scroll-mt-24", className)}>
      {children}
    </section>
  );
}

export function Logo({
  className,
  showText = true,
  size = 24,
}: {
  className?: string;
  showText?: boolean;
  size?: number;
}) {
  return (
    <a href="/" aria-label="Shipper home" className={cn("inline-flex items-center gap-3", className)}>
      <svg width={size} height={size} viewBox="0 0 81 81" fill="none" aria-hidden="true" className="shrink-0">
        <path
          d="m5.46 35.39 24.09 11.16-23.81 13.25c5.84 2.79 11.9 4.14 17.34 4.14 12.15 0 24.03-6.33 32.02-18.18 7.4-11.21 13.77-20.23 20.54-28.74l-70.18 18.37z"
          fill="currentColor"
        />
      </svg>
      {showText && (
        <span className="text-[18px] font-[560] tracking-[-0.035em]">Shipper</span>
      )}
    </a>
  );
}

export function SectionIntro({
  title,
  body,
  align = "left",
  className,
  size = "md",
}: {
  title: React.ReactNode;
  body?: React.ReactNode;
  align?: "left" | "center";
  className?: string;
  size?: "sm" | "md" | "lg";
}) {
  return (
    <div
      className={cn(
        "flex flex-col gap-6",
        align === "center" && "items-center text-center",
        className,
      )}
    >
      <h2
        className={cn(
          "max-w-[21ch] text-[#ecece7] text-[clamp(2rem,4vw,3rem)]",
          size === "sm" && "text-[clamp(1.7rem,3.2vw,2.3rem)]",
          size === "lg" && "max-w-[17ch] text-[clamp(2.6rem,4.8vw,3.55rem)]",
          align === "center" && "mx-auto max-w-[20ch]",
        )}
      >
        {title}
      </h2>
      {body && (
        <p className="max-w-[62ch] text-[17px] leading-[1.7] text-[#b3b4ad] md:text-[18px]">
          {body}
        </p>
      )}
    </div>
  );
}

export function Status({
  label,
  tone = "success",
  pulse = false,
  className,
}: {
  label: string;
  tone?: "success" | "warning" | "danger" | "muted" | "info";
  pulse?: boolean;
  className?: string;
}) {
  const color = {
    success: "#79e3a6",
    warning: "#e8c07d",
    danger: "#f08c90",
    muted: "#8a8b86",
    info: "#9dc4f0",
  }[tone];
  return (
    <span
      className={cn(
        "inline-flex items-center gap-2.5 text-[14px] font-[500] text-[#c2c3bd]",
        className,
      )}
    >
      <span
        className={cn("size-[6px] shrink-0 rounded-full", pulse && "animate-pulse-dot")}
        style={{
          background: color,
          boxShadow: `0 0 0 4px color-mix(in oklab, ${color} 11%, transparent)`,
        }}
      />
      {label}
    </span>
  );
}

export function CopyButton({
  value = INSTALL_COMMAND,
  label = "Copy",
  className,
  iconOnly = false,
  compactIcon = false,
}: {
  value?: string;
  label?: string;
  className?: string;
  iconOnly?: boolean;
  compactIcon?: boolean;
}) {
  return (
    <button
      type="button"
      data-copy={value}
      aria-label={label === "Copy" ? "Copy install command" : label}
      className={cn(
        "inline-flex h-11 shrink-0 items-center gap-2.5 rounded-lg border border-white/[0.14] bg-white/[0.05] px-4 text-[14px] font-[500] text-[#e7e7e2] transition-colors hover:border-white/[0.26] hover:bg-white/[0.09]",
        iconOnly && "w-11 justify-center px-0",
        className,
      )}
    >
      <span data-copy-icon className="inline-flex">
        <HugeiconsIcon icon={Copy01Icon} size={16} strokeWidth={1.6} />
      </span>
      <span data-copy-label className={cn(iconOnly && "sr-only", compactIcon && "sr-only sm:not-sr-only")}>{label}</span>
    </button>
  );
}

export function CommandBar({
  command = INSTALL_COMMAND,
  className,
  showCopy = true,
}: {
  command?: string;
  className?: string;
  showCopy?: boolean;
}) {
  return (
    <div
      className={cn(
        "w-full rounded-xl border border-white/[0.12] bg-[#0a0b0b] p-2 shadow-[inset_0_1px_0_rgba(255,255,255,0.04)]",
        className,
      )}
    >
      <div className="flex flex-col gap-1">
        <div className="flex items-center justify-between gap-2 px-2 py-1">
          <div className="flex min-w-0 items-center gap-2.5">
            <span className="mono shrink-0 text-[14px] text-[#85867f]">$</span>
            <code className="mono min-w-0 overflow-x-auto whitespace-nowrap text-[12px] leading-[1.7] text-[#e1e1db] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden xl:text-[13px]">
              <span className="sm:hidden">curl -fsSL \</span>
              <span className="hidden sm:inline">{command}</span>
            </code>
          </div>
          {showCopy && (
            <>
              <CopyButton iconOnly className="shrink-0 xl:hidden" />
              <CopyButton className="hidden shrink-0 xl:inline-flex" />
            </>
          )}
        </div>
        <div className="px-2 pb-1 pl-[30px] sm:hidden">
          <code className="mono block whitespace-pre text-[12px] leading-[1.7] text-[#e1e1db]">
            https://shipperkit.dev/install.sh \
            <br />| sudo bash
          </code>
        </div>
      </div>
    </div>
  );
}

export function TerminalInstall({ className }: { className?: string }) {
  return (
    <span className={className}>
      <span className="block sm:hidden">
        curl -fsSL
        <br />
        https://shipperkit.dev/install.sh
        <br />| sudo bash
      </span>
      <span className="hidden sm:block">
        curl -fsSL https://shipperkit.dev/install.sh
        <br />| sudo bash
      </span>
    </span>
  );
}

export function ActionPrimary({
  href = "#get-shipper",
  children = "Install Shipper",
  className,
  external = false,
}: {
  href?: string;
  children?: React.ReactNode;
  className?: string;
  external?: boolean;
}) {
  return (
    <a
      href={href}
      className={cn("button-primary group", className)}
      {...(external ? { target: "_blank", rel: "noreferrer" } : {})}
    >
      {children}
      <span className="button-icon">
        <HugeiconsIcon icon={ArrowRight01Icon} size={17} strokeWidth={1.8} />
      </span>
    </a>
  );
}

export function ActionSecondary({
  href,
  children,
  className,
  external = false,
}: {
  href: string;
  children: React.ReactNode;
  className?: string;
  external?: boolean;
}) {
  return (
    <a
      href={href}
      className={cn("button-secondary group", className)}
      {...(external ? { target: "_blank", rel: "noreferrer" } : {})}
    >
      {children}
      <HugeiconsIcon
        icon={ArrowUpRight01Icon}
        size={16}
        strokeWidth={1.7}
        className="opacity-70 transition-transform duration-300 group-hover:-translate-y-0.5 group-hover:translate-x-0.5"
      />
    </a>
  );
}

export function Screenshot({
  src,
  alt,
  className,
  frameClassName,
  priority = false,
  plain = false,
  fade = false,
}: {
  src: string;
  alt: string;
  className?: string;
  frameClassName?: string;
  priority?: boolean;
  plain?: boolean;
  fade?: boolean;
}) {
  return (
    <figure
      className={cn(
        "relative m-0 min-w-0 max-w-full rounded-[22px] border border-white/[0.14] bg-white/[0.045] p-2 shadow-[0_50px_120px_-50px_rgba(0,0,0,1)]",
        frameClassName,
      )}
    >
      <div className="relative overflow-hidden rounded-[14px] border border-white/[0.09] bg-[#0a0b0b]">
        {!plain && (
          <div className="screen-bar">
            <div className="flex items-center gap-2">
              <span className="screen-dot" />
              <span className="screen-dot" />
              <span className="screen-dot" />
            </div>
            <div className="mono ml-2 flex h-7 flex-1 items-center gap-2 rounded-md border border-white/[0.07] bg-black/25 px-3 text-[14px] text-[#898a85]">
              <span className="size-1.5 rounded-full bg-[#79e3a6]" />
              shipper.local
            </div>
          </div>
        )}
        <img
          src={src}
          alt={alt}
          width={1440}
          height={900}
          loading={priority ? "eager" : "lazy"}
          decoding="async"
          className={cn("shot h-auto w-full", className)}
        />
        {fade && (
          <div className="pointer-events-none absolute inset-x-0 bottom-0 h-20 bg-gradient-to-t from-[#0a0b0b] to-transparent" />
        )}
      </div>
    </figure>
  );
}

export function AsciiField({
  className,
  cols = 72,
  rows = 28,
  seed = 7,
  opacity = 0.14,
}: {
  className?: string;
  cols?: number;
  rows?: number;
  seed?: number;
  opacity?: number;
}) {
  const chars = "-/\\|:+·01{}[]";
  let state = seed;
  const next = () => {
    state = (state * 1664525 + 1013904223) % 4294967296;
    return state / 4294967296;
  };
  const lines: string[] = [];
  for (let row = 0; row < rows; row += 1) {
    let line = "";
    for (let col = 0; col < cols; col += 1) {
      line += next() > 0.62 ? chars[Math.floor(next() * chars.length)] : " ";
    }
    lines.push(line);
  }
  return (
    <pre
      aria-hidden="true"
      className={cn("ascii-field pointer-events-none absolute select-none", className)}
      style={{ opacity }}
    >
      {lines.join("\n")}
    </pre>
  );
}

export function Sparkline({
  points = "0,26 12,22 24,24 36,17 48,19 60,12 72,14 84,8 96,11 108,5",
  className,
  stroke = "#f4f4f0",
  fill = true,
  height = 48,
  width = 132,
}: {
  points?: string;
  className?: string;
  stroke?: string;
  fill?: boolean;
  height?: number;
  width?: number;
}) {
  const gradientId = `spark-${React.useId().replace(/[^a-zA-Z0-9_-]/g, "")}`;
  return (
    <svg
      viewBox={`0 0 ${width} ${height}`}
      className={cn("block w-full", className)}
      preserveAspectRatio="none"
      aria-hidden="true"
    >
      {fill && (
        <polygon
          points={`${points} ${width},${height} 0,${height}`}
          fill={`url(#${gradientId})`}
          opacity="0.28"
        />
      )}
      <polyline points={points} fill="none" stroke={stroke} strokeWidth="1.2" vectorEffect="non-scaling-stroke" />
      {fill && (
        <defs>
          <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor={stroke} stopOpacity="0.6" />
            <stop offset="100%" stopColor={stroke} stopOpacity="0" />
          </linearGradient>
        </defs>
      )}
    </svg>
  );
}

export function Bars({
  values = [34, 52, 41, 67, 50, 74, 62, 86, 70, 92],
  className,
  tone = "#79e3a6",
}: {
  values?: number[];
  className?: string;
  tone?: string;
}) {
  return (
    <div className={cn("flex h-12 items-end gap-1.5", className)} aria-hidden="true">
      {values.map((value, index) => (
        <span
          key={index}
          className="w-full rounded-[2px]"
          style={{
            height: `${value}%`,
            background: index === values.length - 1 ? tone : "rgba(255,255,255,0.15)",
          }}
        />
      ))}
    </div>
  );
}

export function Ring({
  value = 68,
  label,
  sub,
  size = 128,
  tone = "#f4f4f0",
}: {
  value?: number;
  label: string;
  sub?: string;
  size?: number;
  tone?: string;
}) {
  const radius = 42;
  const circumference = 2 * Math.PI * radius;
  const offset = circumference - (value / 100) * circumference;
  return (
    <div className="flex items-center gap-5">
      <svg width={size} height={size} viewBox="0 0 100 100" aria-hidden="true">
        <circle cx="50" cy="50" r={radius} fill="none" stroke="rgba(255,255,255,0.1)" strokeWidth="1.2" />
        <circle
          cx="50"
          cy="50"
          r={radius}
          fill="none"
          stroke={tone}
          strokeWidth="2"
          strokeDasharray={circumference}
          strokeDashoffset={offset}
          strokeLinecap="round"
          transform="rotate(-90 50 50)"
        />
        <circle cx="50" cy="50" r="1.5" fill={tone} />
      </svg>
      <div className="flex flex-col">
        <span className="mono text-[28px] leading-none tnum">{label}</span>
        {sub && <span className="mt-2.5 text-[14px] leading-snug text-[#85867f]">{sub}</span>}
      </div>
    </div>
  );
}

export function Check({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex items-start gap-3">
      <HugeiconsIcon
        icon={Tick02Icon}
        size={18}
        strokeWidth={1.7}
        className="mt-[3px] shrink-0 text-[#79e3a6]"
      />
      <span className="text-[16px] leading-[1.6] text-[#c3c4be]">{children}</span>
    </div>
  );
}

export function CheckBadge({ children }: { children: React.ReactNode }) {
  return (
    <span className="inline-flex items-center gap-2.5 rounded-full border border-white/[0.1] bg-white/[0.035] px-4 py-2 text-[14px] text-[#c3c4be]">
      <HugeiconsIcon icon={CheckmarkCircle02Icon} size={16} strokeWidth={1.6} className="text-[#79e3a6]" />
      {children}
    </span>
  );
}

export function ArrowLink({
  href,
  children,
  className,
  external = false,
}: {
  href: string;
  children: React.ReactNode;
  className?: string;
  external?: boolean;
}) {
  return (
    <a
      href={href}
      className={cn(
        "group inline-flex items-center gap-2 text-[15px] font-[500] text-[#d4d4ce] transition-colors hover:text-white",
        className,
      )}
      {...(external ? { target: "_blank", rel: "noreferrer" } : {})}
    >
      {children}
      <HugeiconsIcon
        icon={ArrowUpRight01Icon}
        size={16}
        strokeWidth={1.7}
        className="transition-transform duration-300 group-hover:-translate-y-0.5 group-hover:translate-x-0.5"
      />
    </a>
  );
}

export function CodeWindow({
  lines,
  title = "deployment.log",
  className,
}: {
  lines: React.ReactNode[];
  title?: string;
  className?: string;
}) {
  return (
    <div className={cn("overflow-hidden rounded-xl border border-white/[0.1] bg-[#090a0a]", className)}>
      <div className="flex h-12 items-center justify-between border-b border-white/[0.07] px-4">
        <div className="flex items-center gap-2">
          <span className="screen-dot" />
          <span className="screen-dot" />
          <span className="screen-dot" />
        </div>
        <span className="mono text-[14px] text-[#7f807b]">{title}</span>
        <span className="w-10" />
      </div>
      <div className="overflow-x-auto p-5">
        {lines.map((line, index) => (
          <div key={index} className="mono flex min-w-max gap-5 text-[14px] leading-[2] text-[#aeb0a9]">
            <span className="w-5 shrink-0 text-right text-[#797a75]">{index + 1}</span>
            <span>{line}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

export function Divider({ className }: { className?: string }) {
  return <div className={cn("h-px w-full bg-white/[0.08]", className)} />;
}
