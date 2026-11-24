import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";

/* Small, honest simulations of what Shipper does. Each one can be poked at,
   none of them pretend to be live data. */

export const MARK = "m5.46 35.39 24.09 11.16-23.81 13.25c5.84 2.79 11.9 4.14 17.34 4.14 12.15 0 24.03-6.33 32.02-18.18 7.4-11.21 13.77-20.23 20.54-28.74l-70.18 18.37z";

const hexChars = "0123456789abcdef";
const randomId = (n = 7) => Array.from({ length: n }, () => hexChars[Math.floor(Math.random() * 16)]).join("");
const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

function useInView<T extends Element>(once = true) {
  const ref = useRef<T>(null);
  const [inView, set] = useState(false);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const io = new IntersectionObserver(([e]) => {
      if (e.isIntersecting) {
        set(true);
        if (once) io.disconnect();
      } else if (!once) set(false);
    }, { threshold: 0.35 });
    io.observe(el);
    return () => io.disconnect();
  }, [once]);
  return [ref, inView] as const;
}

function Check({ className = "" }: { className?: string }) {
  return (
    <svg viewBox="0 0 16 16" className={`size-3.5 ${className}`} fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      <path d="M3.5 8.5 6.5 11.5 12.5 4.5" />
    </svg>
  );
}
function Cross({ className = "" }: { className?: string }) {
  return (
    <svg viewBox="0 0 16 16" className={`size-3.5 ${className}`} fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" aria-hidden>
      <path d="M4.5 4.5 11.5 11.5M11.5 4.5 4.5 11.5" />
    </svg>
  );
}
export function Lock({ open = false, className = "" }: { open?: boolean; className?: string }) {
  return (
    <svg viewBox="0 0 16 16" className={`size-3.5 ${className}`} fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden>
      <rect x="3" y="7" width="10" height="7" rx="1.5" />
      <path d={open ? "M5.5 7V5a2.5 2.5 0 0 1 5-.6" : "M5.5 7V5a2.5 2.5 0 0 1 5 0v2"} style={{ transition: "d 300ms" }} />
    </svg>
  );
}

function Switch({ checked, onChange, children }: { checked: boolean; onChange: (v: boolean) => void; children: ReactNode }) {
  return (
    <button type="button" role="switch" aria-checked={checked} onClick={() => onChange(!checked)} className="inline-flex cursor-pointer items-center gap-2 text-[13px] text-ink-2">
      <span className="w-switch" />
      {children}
    </button>
  );
}

/* ---------- 1. Deploy pipeline with a breakable build ---------- */

const STAGES = ["Clone", "Build", "Start", "Health check", "Live"];

export function PipelineWidget({ autoplay = true }: { autoplay?: boolean }) {
  const [ref, inView] = useInView<HTMLDivElement>();
  const [step, setStep] = useState(-1);
  const [failed, setFailed] = useState(false);
  const [broken, setBroken] = useState(false);
  const [serving, setServing] = useState("4f8ac21");
  const [candidate, setCandidate] = useState("9c0e2a4");
  const [busy, setBusy] = useState(false);
  const [elapsed, setElapsed] = useState(0);
  const run = useRef(0);

  const deploy = useCallback(async (breakIt: boolean) => {
    const id = ++run.current;
    const next = randomId();
    setCandidate(next);
    setFailed(false);
    setBusy(true);
    const started = performance.now();
    const tick = setInterval(() => setElapsed((performance.now() - started) / 1000), 100);
    for (let i = 0; i < STAGES.length; i++) {
      if (run.current !== id) break;
      setStep(i);
      await sleep(i === 1 ? 1300 : 750);
      if (breakIt && i === 1) {
        setFailed(true);
        break;
      }
      if (i === STAGES.length - 1) setServing(next);
    }
    clearInterval(tick);
    if (run.current === id) setBusy(false);
  }, []);

  useEffect(() => {
    if (inView && autoplay) deploy(false);
  }, [inView, autoplay, deploy]);

  const done = !busy && step === STAGES.length - 1 && !failed;
  const fillTo = failed ? 1 : Math.max(0, step);
  return (
    <div ref={ref} className="w-card select-none">
      <div className="w-toolbar">
        <span className="flex items-center gap-2"><span className="t-mono text-ink">store</span><span className="text-mute">main</span></span>
        <div className="flex items-center gap-3">
          <Switch checked={broken} onChange={setBroken}>Break the build</Switch>
          <button type="button" className="w-btn is-primary" disabled={busy} onClick={() => deploy(broken)}>Deploy</button>
        </div>
      </div>
      <div className="px-5 pb-5 pt-7 sm:px-7">
        <div className="relative">
          <div className="absolute left-[10px] right-[10px] top-[10px] h-[2px] rounded bg-white/[0.07]" />
          <div
            className="absolute left-[10px] top-[10px] h-[2px] rounded transition-[width,background-color] duration-700 ease-[var(--ease-out)]"
            style={{ width: `calc((100% - 20px) * ${fillTo / (STAGES.length - 1)})`, background: failed ? "var(--red)" : "var(--green)" }}
          />
          <ol className="relative flex justify-between">
            {STAGES.map((stage, i) => {
              const isFail = failed && i === 1;
              const passed = step > i || (done && i <= step) || (!busy && !failed && step === i && i === STAGES.length - 1);
              const active = busy && step === i && !isFail;
              const skipped = failed && i > 1;
              return (
                <li key={stage} className="flex w-0 flex-col items-center gap-3">
                  <span
                    className="relative grid size-[22px] place-items-center rounded-full border transition-all duration-500"
                    style={{
                      borderColor: isFail ? "var(--red)" : passed ? "var(--green)" : active ? "var(--text)" : "var(--line-2)",
                      background: isFail ? "rgba(240,140,144,.14)" : passed ? "rgba(121,227,166,.14)" : "#0d0e0e",
                      color: isFail ? "var(--red)" : "var(--green)",
                    }}
                  >
                    {isFail ? <Cross /> : passed ? <Check /> : active ? <span className="size-1.5 rounded-full bg-ink" /> : null}
                    {active && <span className="absolute inset-0 rounded-full border border-white/50" style={{ animation: "pulse-ring 1.2s var(--ease-out) infinite" }} />}
                  </span>
                  <span className={`whitespace-nowrap text-[12px] sm:text-[13px] transition-colors ${skipped ? "text-white/25" : isFail ? "text-red" : passed || active ? "text-ink" : "text-mute"}`}>{stage}</span>
                </li>
              );
            })}
          </ol>
        </div>
        <div className="mt-7 grid gap-3 sm:grid-cols-2">
          <div className="rounded-[10px] border border-line bg-[#0b0c0c] p-4">
            <div className="text-[12px] text-mute">Serving</div>
            <div className="mt-1.5 flex items-center gap-2">
              <span className="dot" />
              <span key={serving} className="t-mono text-[15px] text-ink" style={{ animation: "fade-up 500ms var(--ease-out)" }}>{serving}</span>
            </div>
          </div>
          <div className={`rounded-[10px] border p-4 transition-colors duration-500 ${failed ? "border-red/30 bg-red/[0.06]" : "border-line bg-[#0b0c0c]"}`}>
            <div className="text-[12px] text-mute">{failed ? "Failed and kept offline" : done ? "Deployed" : busy ? "Deploying" : "Ready"}</div>
            <div className="mt-1.5 flex items-center justify-between gap-2">
              <span className={`t-mono text-[15px] ${failed ? "text-red" : "text-ink"}`}>{candidate}</span>
              <span className="t-mono text-[13px] tabular-nums text-mute">{elapsed.toFixed(1)}s</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

/* ---------- 2. Traffic through the proxy, redeploy without dropping requests ---------- */

const APPS = [
  { name: "store", domain: "store.example.com", y: 60 },
  { name: "api", domain: "api.example.com", y: 150 },
  { name: "blog", domain: "blog.example.com", y: 240 },
];

export function RouterWidget() {
  const [versions, setVersions] = useState([3, 7, 12]);
  const [swapping, setSwapping] = useState<number | null>(null);
  const [hover, setHover] = useState<number | null>(null);
  const [served, setServed] = useState(48210);
  const [ref, inView] = useInView<HTMLDivElement>(false);

  useEffect(() => {
    if (!inView) return;
    const t = setInterval(() => setServed((n) => n + 3 + Math.floor(Math.random() * 9)), 220);
    return () => clearInterval(t);
  }, [inView]);

  const redeploy = async (i: number) => {
    if (swapping !== null) return;
    setSwapping(i);
    await sleep(1400);
    setVersions((v) => v.map((n, j) => (j === i ? n + 1 : n)));
    await sleep(500);
    setSwapping(null);
  };

  const pathFor = (y: number) => `M150 ${y} C 205 ${y}, 215 150, 268 150 M 312 150 C 365 150, 375 ${y}, 430 ${y}`;
  return (
    <div ref={ref} className="w-card">
      <div className="w-toolbar">
        <span>Click an app to redeploy it</span>
        <span className="t-mono tabular-nums text-ink">{served.toLocaleString("en-US")} <span className="text-mute">requests, 0 dropped</span></span>
      </div>
      <svg viewBox="0 0 580 300" className="block w-full" role="img" aria-label="Requests flowing from domains through the Shipper proxy to app containers">
        <defs>
          <linearGradient id="rw-line" x1="0" x2="1">
            <stop offset="0" stopColor="#fff" stopOpacity=".06" />
            <stop offset=".5" stopColor="#fff" stopOpacity=".16" />
            <stop offset="1" stopColor="#fff" stopOpacity=".06" />
          </linearGradient>
        </defs>
        {APPS.map((app, i) => (
          <g key={app.name} opacity={hover === null || hover === i ? 1 : 0.35} style={{ transition: "opacity 300ms" }}>
            <path id={`rw-in-${i}`} d={`M150 ${app.y} C 205 ${app.y}, 215 150, 268 150`} fill="none" stroke="url(#rw-line)" strokeWidth="1.5" />
            <path id={`rw-out-${i}`} d={`M312 150 C 365 150, 375 ${app.y}, 428 ${app.y}`} fill="none" stroke="url(#rw-line)" strokeWidth="1.5" />
            {inView &&
              [0, 1, 2].map((k) => (
                <circle key={k} r="2.6" fill={swapping === i ? "#e8c07d" : "#79e3a6"}>
                  <animateMotion dur={`${1.6 + i * 0.25}s`} begin={`${k * 0.55 + i * 0.2}s`} repeatCount="indefinite" path={pathFor(app.y)} keyPoints="0;1" keyTimes="0;1" calcMode="linear" />
                </circle>
              ))}
            <g onMouseEnter={() => setHover(i)} onMouseLeave={() => setHover(null)}>
              <rect x="8" y={app.y - 15} width="142" height="30" rx="7" fill="#151616" stroke="rgba(255,255,255,.1)" />
              <g transform={`translate(18 ${app.y - 6})`} stroke="#79e3a6" fill="none" strokeWidth="1.3">
                <rect x="0" y="5" width="9" height="7" rx="1.3" />
                <path d="M2 5V3.5a2.5 2.5 0 0 1 5 0V5" />
              </g>
              <text x="34" y={app.y + 4.5} fontSize="11.5" fill="#d8d8d3" fontFamily="var(--font-mono)">{app.domain}</text>
            </g>
            <g
              className="cursor-pointer"
              onClick={() => redeploy(i)}
              onMouseEnter={() => setHover(i)}
              onMouseLeave={() => setHover(null)}
              role="button"
              tabIndex={0}
              aria-label={`Redeploy ${app.name}`}
              onKeyDown={(e) => (e.key === "Enter" || e.key === " ") && redeploy(i)}
            >
              <rect x="430" y={app.y - 22} width="140" height="44" rx="9" fill={hover === i ? "#1b1c1c" : "#141515"} stroke={swapping === i ? "rgba(232,192,125,.6)" : "rgba(255,255,255,.12)"} style={{ transition: "all 300ms" }} />
              <circle cx="447" cy={app.y - 5} r="3.2" fill={swapping === i ? "#e8c07d" : "#79e3a6"} />
              <text x="458" y={app.y - 1} fontSize="13" fill="#f4f4f0" fontWeight="600">{app.name}</text>
              <text x="447" y={app.y + 13} fontSize="10.5" fill="#8d8e88" fontFamily="var(--font-mono)">
                {swapping === i ? `v${versions[i]} → v${versions[i] + 1}` : `release v${versions[i]}`}
              </text>
              <text x="556" y={app.y + 4} fontSize="14" fill="#8d8e88" textAnchor="end">↻</text>
            </g>
          </g>
        ))}
        <g>
          <circle cx="290" cy="150" r="34" fill="#0d0e0e" stroke="rgba(255,255,255,.16)" />
          <circle cx="290" cy="150" r="34" fill="none" stroke="#79e3a6" strokeOpacity=".35" strokeDasharray="4 6">
            <animateTransform attributeName="transform" type="rotate" from="0 290 150" to="360 290 150" dur="18s" repeatCount="indefinite" />
          </circle>
          <path d={MARK} fill="#f4f4f0" transform="translate(277 137) scale(0.32)" />
          <text x="290" y="203" fontSize="11" fill="#8d8e88" textAnchor="middle">Shipper proxy</text>
        </g>
      </svg>
    </div>
  );
}

/* ---------- 3. Release history, restore any healthy release ---------- */

const HISTORY = [
  { id: "2e4c6a8", ok: true, when: "Nov 2" },
  { id: "d6a4f79", ok: true, when: "Nov 6" },
  { id: "9c0e2a4", ok: false, when: "Nov 7" },
  { id: "7b2d1c9", ok: true, when: "Nov 8" },
  { id: "a91f03e", ok: true, when: "Nov 9" },
  { id: "c47e5b2", ok: false, when: "Nov 9" },
  { id: "4f8ac21", ok: true, when: "Nov 10" },
];

export function RollbackWidget() {
  const [live, setLive] = useState(HISTORY.length - 1);
  const [note, setNote] = useState("4f8ac21 is serving");
  const [tip, setTip] = useState<number | null>(null);
  const pick = (i: number) => {
    const r = HISTORY[i];
    if (!r.ok) {
      setNote(`${r.id} failed its build, so it never went live`);
      return;
    }
    if (i === live) return;
    setLive(i);
    setNote(`Restored ${r.id} in ${(1.4 + Math.random()).toFixed(1)}s`);
  };
  const pct = (i: number) => (i / (HISTORY.length - 1)) * 100;
  return (
    <div className="w-card select-none">
      <div className="w-toolbar">
        <span>Release history</span>
        <span key={note} className="truncate text-ink" style={{ animation: "fade-up 400ms var(--ease-out)" }}>{note}</span>
      </div>
      <div className="px-6 pb-7 pt-12 sm:px-10">
        <div className="relative h-[58px]">
          <div className="absolute inset-x-0 top-[27px] h-px bg-white/10" />
          <div
            className="absolute -top-8 -translate-x-1/2 rounded-full bg-ink px-2.5 py-0.5 text-[11.5px] font-semibold text-black transition-[left] duration-700 ease-[var(--ease-out)]"
            style={{ left: `${pct(live)}%` }}
          >
            Live
          </div>
          {HISTORY.map((r, i) => (
            <button
              key={r.id}
              type="button"
              onClick={() => pick(i)}
              onMouseEnter={() => setTip(i)}
              onMouseLeave={() => setTip(null)}
              className="group absolute top-[14px] grid size-[28px] -translate-x-1/2 cursor-pointer place-items-center rounded-full"
              style={{ left: `${pct(i)}%` }}
              aria-label={`${r.ok ? "Restore" : "Failed release"} ${r.id}`}
            >
              <span
                className="size-[12px] rounded-full border-2 transition-all duration-300 group-hover:scale-125"
                style={{
                  borderColor: !r.ok ? "var(--red)" : i === live ? "var(--green)" : "rgba(255,255,255,.35)",
                  background: i === live ? "var(--green)" : !r.ok ? "rgba(240,140,144,.2)" : "#0d0e0e",
                  boxShadow: i === live ? "0 0 0 6px rgba(121,227,166,.12)" : "none",
                }}
              />
              <span className={`t-mono absolute top-[34px] whitespace-nowrap text-[10.5px] transition-opacity sm:text-[11px] ${tip === i || i === live ? "opacity-100" : "opacity-0 sm:opacity-60"} ${r.ok ? "text-ink-2" : "text-red"}`}>{r.id}</span>
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}

/* ---------- 4. Scheduled backups to the server or S3 ---------- */

export function BackupWidget() {
  const [target, setTarget] = useState<"server" | "s3">("s3");
  const [rows, setRows] = useState([
    { id: 1, at: "03:00", size: "182 MB" },
    { id: 2, at: "02:00", size: "181 MB" },
    { id: 3, at: "01:00", size: "181 MB" },
  ]);
  const [copy, setCopy] = useState<number | null>(null);
  const [flying, setFlying] = useState(0);
  const [ref, inView] = useInView<HTMLDivElement>(false);
  const hour = useRef(4);

  useEffect(() => {
    if (!inView) return;
    const t = setInterval(() => {
      setFlying((n) => n + 1);
      setTimeout(() => {
        const h = hour.current++ % 24;
        setRows((r) => [{ id: Date.now(), at: `${String(h).padStart(2, "0")}:00`, size: `${182 + (h % 3)} MB` }, ...r].slice(0, 4));
      }, 1100);
    }, 3200);
    return () => clearInterval(t);
  }, [inView]);

  return (
    <div ref={ref} className="w-card">
      <div className="w-toolbar">
        <span className="t-mono text-ink">store-db</span>
        <div className="flex rounded-[8px] border border-line p-0.5">
          {(["server", "s3"] as const).map((k) => (
            <button key={k} type="button" onClick={() => setTarget(k)} className={`cursor-pointer rounded-[6px] px-2.5 py-1 text-[12.5px] transition-colors ${target === k ? "bg-white/10 text-ink" : "text-mute hover:text-ink"}`}>
              {k === "server" ? "This server" : "S3 storage"}
            </button>
          ))}
        </div>
      </div>
      <svg viewBox="0 0 520 120" className="block w-full">
        <g transform="translate(70 60)">
          <ellipse cx="0" cy="-22" rx="30" ry="9" fill="#151616" stroke="rgba(255,255,255,.25)" />
          <path d="M-30 -22v44c0 5 13.4 9 30 9s30-4 30-9v-44" fill="#121313" stroke="rgba(255,255,255,.25)" />
          <path d="M-30 0c0 5 13.4 9 30 9s30-4 30-9" fill="none" stroke="rgba(255,255,255,.15)" />
          <ellipse cx="0" cy="-22" rx="30" ry="9" fill="#1a1b1b" stroke="rgba(255,255,255,.25)" />
        </g>
        <path d="M120 60 H400" stroke="rgba(255,255,255,.12)" strokeDasharray="3 5" />
        {inView && (
          <g key={flying}>
            <rect x="-9" y="-7" width="18" height="14" rx="3" fill="#79e3a6">
              <animateMotion dur="1.1s" path="M120 60 H400" fill="freeze" keySplines="0.16 1 0.3 1" calcMode="spline" keyTimes="0;1" keyPoints="0;1" />
              <animate attributeName="opacity" values="0;1;1;0" keyTimes="0;.1;.85;1" dur="1.1s" fill="freeze" />
            </rect>
          </g>
        )}
        <g transform="translate(450 60)" style={{ transition: "opacity 300ms" }}>
          {target === "s3" ? (
            <g>
              <path d="M-30 -24h60l-7 50h-46z" fill="#121313" stroke="rgba(255,255,255,.25)" />
              <ellipse cx="0" cy="-24" rx="30" ry="7" fill="#1a1b1b" stroke="rgba(255,255,255,.25)" />
              <text x="0" y="8" fontSize="11" fill="#8d8e88" textAnchor="middle" fontFamily="var(--font-mono)">s3</text>
            </g>
          ) : (
            <g>
              <rect x="-32" y="-26" width="64" height="52" rx="7" fill="#121313" stroke="rgba(255,255,255,.25)" />
              <path d="M-22 -10h44M-22 2h44M-22 14h28" stroke="rgba(255,255,255,.15)" />
              <circle cx="20" cy="14" r="2.5" fill="#79e3a6" />
            </g>
          )}
        </g>
      </svg>
      <ul className="border-t border-line">
        {rows.map((r, i) => (
          <li key={r.id} className="flex items-center justify-between gap-3 border-b border-line px-4 py-2.5 text-[13px] last:border-b-0" style={{ animation: i === 0 ? "fade-up 500ms var(--ease-out)" : undefined }}>
            <span className="flex items-center gap-2"><span className="dot" /><span className="text-ink">Today, {r.at}</span></span>
            <span className="t-mono text-mute">{r.size}</span>
            <button type="button" onClick={() => setCopy(r.id)} className="cursor-pointer text-ink-2 transition-colors hover:text-ink">
              {copy === r.id ? "Restored as store-db-copy" : "Restore as copy"}
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}

/* ---------- 5. One server, many workloads ---------- */

const ADDABLE = {
  App: [["store", "Next.js", 180], ["api", "Go", 60], ["docs", "Static", 20], ["admin", "Rails", 260], ["worker", "Python", 120]],
  Database: [["postgres", "PostgreSQL", 220], ["cache", "Redis", 40], ["events", "ClickHouse", 380], ["mongo", "MongoDB", 260]],
  Service: [["n8n", "Workflows", 300], ["uptime", "Uptime Kuma", 90], ["umami", "Analytics", 140], ["immich", "Photos", 520]],
} as const;
type Kind = keyof typeof ADDABLE;
type Block = { key: number; kind: Kind; name: string; what: string; mb: number };

export function ServerWidget({ ram = 4096 }: { ram?: number }) {
  const [blocks, setBlocks] = useState<Block[]>([
    { key: 1, kind: "App", name: "store", what: "Next.js", mb: 180 },
    { key: 2, kind: "Database", name: "postgres", what: "PostgreSQL", mb: 220 },
    { key: 3, kind: "Service", name: "n8n", what: "Workflows", mb: 300 },
  ]);
  const counter = useRef(10);
  const used = blocks.reduce((n, b) => n + b.mb, 0) + 420;
  const add = (kind: Kind) => {
    const pool = ADDABLE[kind];
    const taken = new Set(blocks.map((b) => b.name));
    const pick = pool.find(([n]) => !taken.has(n)) ?? pool[Math.floor(Math.random() * pool.length)];
    if (used + pick[2] > ram || blocks.length >= 12) return;
    setBlocks((b) => [...b, { key: counter.current++, kind, name: taken.has(pick[0]) ? `${pick[0]}-${counter.current}` : pick[0], what: pick[1], mb: pick[2] }]);
  };
  const color: Record<Kind, string> = { App: "var(--green)", Database: "var(--blue)", Service: "var(--amber)" };
  return (
    <div className="w-card">
      <div className="w-toolbar">
        <span className="flex items-center gap-2"><span className="dot" />vps-01 <span className="text-mute">4 GB · 2 vCPU</span></span>
        <div className="flex gap-1.5">
          {(Object.keys(ADDABLE) as Kind[]).map((k) => (
            <button key={k} type="button" className="w-btn" onClick={() => add(k)}>+ {k}</button>
          ))}
        </div>
      </div>
      <div className="p-4">
        <div className="grid min-h-[168px] grid-cols-2 content-start gap-2 sm:grid-cols-3">
          {blocks.map((b) => (
            <button
              key={b.key}
              type="button"
              onClick={() => setBlocks((all) => all.filter((x) => x.key !== b.key))}
              className="group flex cursor-pointer items-center justify-between gap-2 rounded-[9px] border border-line bg-[#0c0d0d] px-3 py-2.5 text-left transition-colors hover:border-white/20"
              style={{ animation: "pop-in 450ms var(--ease-out)" }}
              aria-label={`Remove ${b.name}`}
            >
              <span className="min-w-0">
                <span className="flex items-center gap-1.5 text-[13px] text-ink"><span className="size-1.5 rounded-full" style={{ background: color[b.kind] }} />{b.name}</span>
                <span className="block truncate text-[11.5px] text-mute">{b.what}</span>
              </span>
              <span className="t-mono text-[11px] text-mute group-hover:hidden">{b.mb}M</span>
              <span className="hidden text-[13px] text-mute group-hover:block">×</span>
            </button>
          ))}
        </div>
        <div className="mt-4">
          <div className="mb-1.5 flex justify-between text-[12px] text-mute">
            <span>Memory</span>
            <span className="t-mono tabular-nums">{(used / 1024).toFixed(1)} / {(ram / 1024).toFixed(0)} GB</span>
          </div>
          <div className="h-1.5 overflow-hidden rounded-full bg-white/[0.06]">
            <div className="h-full rounded-full transition-[width] duration-700 ease-[var(--ease-out)]" style={{ width: `${(used / ram) * 100}%`, background: used / ram > 0.85 ? "var(--amber)" : "var(--green)" }} />
          </div>
        </div>
      </div>
    </div>
  );
}

/* ---------- 6. Service Library search ---------- */

export const LOGOS: [string, string][] = [
  ["n8n", "n8n.png"], ["Immich", "immich.svg"], ["Supabase", "supabase.svg"], ["Grafana", "grafana.svg"], ["Uptime Kuma", "uptime-kuma.svg"],
  ["Ghost", "ghost.svg"], ["Nextcloud", "nextcloud.svg"], ["Gitea", "gitea.svg"], ["MinIO", "minio.svg"], ["PocketBase", "pocketbase.svg"],
  ["Umami", "umami.svg"], ["Vaultwarden", "vaultwarden.svg"], ["Jellyfin", "jellyfin.svg"], ["Outline", "outline.svg"], ["Metabase", "metabase.svg"],
  ["Appwrite", "appwrite.svg"], ["Directus", "directus.svg"], ["Strapi", "strapi.svg"], ["WordPress", "wordpress.svg"], ["Mattermost", "mattermost.svg"],
  ["Listmonk", "listmonk.svg"], ["Home Assistant", "home-assistant.svg"], ["Open WebUI", "open-webui.svg"], ["Plane", "plane.svg"], ["Twenty", "twenty.svg"],
  ["Paperless", "paperless.svg"], ["Meilisearch", "meilisearch.svg"], ["Keycloak", "keycloak.svg"], ["Cal.com", "calcom.svg"], ["AFFiNE", "affine.svg"],
];

export function LibraryWidget({ columns = 6 }: { columns?: number }) {
  const [q, setQ] = useState("");
  const [picked, setPicked] = useState<string | null>(null);
  const list = useMemo(() => LOGOS.filter(([n]) => n.toLowerCase().includes(q.trim().toLowerCase())), [q]);
  return (
    <div className="w-card">
      <div className="w-toolbar">
        <input className="w-input w-full max-w-[260px]" placeholder="Search 365 services" value={q} onChange={(e) => setQ(e.target.value)} aria-label="Search services" />
        <span className="hidden text-mute sm:block">{picked ? <span className="text-ink">{picked} is ready to deploy</span> : "Secrets are generated for you"}</span>
      </div>
      <div className="grid gap-2 p-3" style={{ gridTemplateColumns: `repeat(auto-fill, minmax(${columns > 5 ? 96 : 120}px, 1fr))` }}>
        {list.map(([name, file]) => (
          <button
            key={name}
            type="button"
            onClick={() => setPicked(name)}
            className={`flex cursor-pointer flex-col items-center gap-2 rounded-[10px] border px-2 py-3.5 transition-all duration-300 hover:-translate-y-0.5 hover:border-white/20 ${picked === name ? "border-green/50 bg-green/[0.06]" : "border-line bg-[#0c0d0d]"}`}
            style={{ animation: "pop-in 380ms var(--ease-out)" }}
          >
            <img src={`/media/logos/${file}`} alt="" className="size-8 object-contain" loading="lazy" />
            <span className="w-full truncate text-center text-[12px] text-ink-2">{name}</span>
          </button>
        ))}
        {list.length === 0 && <div className="col-span-full py-8 text-center text-[13px] text-mute">Not in this sample. The full library has 365.</div>}
      </div>
    </div>
  );
}

/* ---------- 7. Installer with host picker ---------- */

const HOSTS = [
  ["Ubuntu 24.04", "Ubuntu 24.04.3 LTS"],
  ["Ubuntu 22.04", "Ubuntu 22.04.5 LTS"],
  ["Debian 13", "Debian GNU/Linux 13 (trixie)"],
  ["Debian 12", "Debian GNU/Linux 12 (bookworm)"],
] as const;
export const INSTALL = "curl -fsSL https://shipperkit.dev/install.sh | sudo bash";

export function InstallWidget({ picker = true }: { picker?: boolean }) {
  const [host, setHost] = useState(0);
  const [arm, setArm] = useState(false);
  const [ref, inView] = useInView<HTMLDivElement>();
  const [shown, setShown] = useState(0);
  const lines = [
    { t: "Install plan", c: "text-ink font-semibold" },
    { t: `• Host        ${HOSTS[host][1]} ${arm ? "arm64" : "amd64"}` },
    { t: "• Docker      install from the signed repository" },
    { t: "• Dashboard   http://203.0.113.10:8080" },
    { t: "• Firewall    allow TCP 80, 443 and 8080" },
    { t: "" },
    { t: "Shipper is ready.", c: "text-green" },
    { t: "  Open        http://203.0.113.10:8080", c: "text-ink" },
  ];
  useEffect(() => {
    if (!inView) return;
    setShown(0);
    let i = 0;
    const t = setInterval(() => {
      i++;
      setShown(i);
      if (i >= lines.length) clearInterval(t);
    }, i === 0 ? 300 : 160);
    return () => clearInterval(t);
  }, [inView, host, arm]);
  return (
    <div ref={ref} className="w-card">
      <div className="w-toolbar">
        <span className="flex gap-1.5"><i className="size-2.5 rounded-full bg-white/15" /><i className="size-2.5 rounded-full bg-white/15" /><i className="size-2.5 rounded-full bg-white/15" /></span>
        <button type="button" data-copy={INSTALL} className="cursor-pointer text-ink-2 hover:text-ink"><span data-copy-label>Copy</span></button>
      </div>
      {picker && (
        <div className="flex flex-wrap items-center gap-1.5 border-b border-line px-4 py-3">
          {HOSTS.map(([label], i) => (
            <button key={label} type="button" onClick={() => setHost(i)} className={`cursor-pointer rounded-full border px-2.5 py-1 text-[12.5px] transition-colors ${host === i ? "border-white/25 bg-white/10 text-ink" : "border-line text-mute hover:text-ink"}`}>{label}</button>
          ))}
          <span className="mx-1 h-4 w-px bg-line" />
          {["x86-64", "ARM64"].map((label, i) => (
            <button key={label} type="button" onClick={() => setArm(i === 1)} className={`cursor-pointer rounded-full border px-2.5 py-1 text-[12.5px] transition-colors ${arm === (i === 1) ? "border-white/25 bg-white/10 text-ink" : "border-line text-mute hover:text-ink"}`}>{label}</button>
          ))}
        </div>
      )}
      <pre className="t-mono m-0 min-h-[236px] overflow-x-auto whitespace-pre p-4 text-[12.5px] leading-[1.75] text-ink-2 sm:text-[13px]">
        <span className="text-mute">$ </span><span className="text-ink">{INSTALL}</span>{"\n\n"}
        {lines.slice(0, shown).map((l, i) => (
          <span key={i} className={`block ${l.c ?? ""}`} style={{ animation: "fade-up 300ms var(--ease-out)" }}>{l.t || " "}</span>
        ))}
        {shown < lines.length && <span className="caret text-ink-2" />}
      </pre>
    </div>
  );
}

/* ---------- 8. CLI for you and your agents ---------- */

const RUNS: Record<string, { cmd: string; out: [string, string?][] }> = {
  deploy: {
    cmd: "shipper deploy . --app store",
    out: [["Uploading 214 files", "text-mute"], ["● building   9feda2e", "text-mute"], ["● starting   9feda2e", "text-mute"], ["● checking   9feda2e", "text-mute"], ["● succeeded  9feda2e in 24s", "text-green"], ["https://store.example.com", "text-ink"]],
  },
  logs: {
    cmd: "shipper logs store --lines 4",
    out: [["12:04:11  GET /cart 200 18ms"], ["12:04:12  GET /api/products 200 9ms"], ["12:04:14  POST /checkout 201 41ms"], ["12:04:15  GET /cart 200 12ms"]],
  },
  doctor: {
    cmd: "shipper doctor store",
    out: [["✓ Docker connection", "text-green"], ["✓ Release artifact", "text-green"], ["✗ Readiness check", "text-red"], ["  The candidate started, but its HTTP health check did not pass.", "text-mute"], ["  4f8ac21 is still serving", "text-ink"]],
  },
  restart: {
    cmd: "shipper restart store",
    out: [["Restarting store", "text-mute"], ["● running  store", "text-green"]],
  },
};

export function AgentWidget({ initial = "deploy" }: { initial?: keyof typeof RUNS }) {
  const [ref, inView] = useInView<HTMLDivElement>();
  const [run, setRun] = useState<string>(initial);
  const [typed, setTyped] = useState(0);
  const [lines, setLines] = useState(0);
  const token = useRef(0);
  const start = useCallback(async (name: string) => {
    const id = ++token.current;
    setRun(name);
    setTyped(0);
    setLines(0);
    const r = RUNS[name];
    for (let i = 1; i <= r.cmd.length; i++) {
      if (token.current !== id) return;
      setTyped(i);
      await sleep(22);
    }
    for (let i = 1; i <= r.out.length; i++) {
      await sleep(380);
      if (token.current !== id) return;
      setLines(i);
    }
  }, []);
  useEffect(() => {
    if (inView) start(initial);
  }, [inView]);
  const r = RUNS[run];
  return (
    <div ref={ref} className="w-card">
      <div className="w-toolbar">
        <span className="t-mono truncate text-[12px]">token <span className="text-ink">deploy:store</span></span>
        <div className="flex gap-1">
          {Object.keys(RUNS).map((k) => (
            <button key={k} type="button" aria-pressed={run === k} className="w-btn !h-7 !px-2.5 !text-[12px]" onClick={() => start(k)}>{k}</button>
          ))}
        </div>
      </div>
      <pre className="t-mono m-0 min-h-[210px] overflow-x-auto whitespace-pre p-4 text-[12.5px] leading-[1.75] text-ink-2 sm:text-[13px]">
        <span className="text-mute">$ </span><span className="text-ink">{r.cmd.slice(0, typed)}</span>
        {typed < r.cmd.length && <span className="caret ml-px text-ink-2" />}
        {"\n"}
        {r.out.slice(0, lines).map(([t, c], i) => (
          <span key={`${run}-${i}`} className={`block ${c ?? ""}`} style={{ animation: "fade-up 300ms var(--ease-out)" }}>{t}</span>
        ))}
      </pre>
    </div>
  );
}

/* ---------- 9. Managed databases ---------- */

const ENGINES = [
  { name: "PostgreSQL", logo: "postgresql.svg", scheme: "postgres", port: 5432, user: "store" },
  { name: "MySQL", logo: "mysql.svg", scheme: "mysql", port: 3306, user: "store" },
  { name: "MariaDB", logo: "mariadb.svg", scheme: "mysql", port: 3306, user: "store" },
  { name: "MongoDB", logo: "mongodb.svg", scheme: "mongodb", port: 27017, user: "store" },
  { name: "Redis", logo: "redis.svg", scheme: "redis", port: 6379, user: "default" },
  { name: "ClickHouse", logo: "clickhouse.svg", scheme: "clickhouse", port: 9000, user: "store" },
];
const alnum = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789";

export function DatabaseWidget() {
  const [engine, setEngine] = useState(0);
  const [pub, setPub] = useState(false);
  const [secret, setSecret] = useState("q7Rk2mWv9TxL4cNp");
  const [rotating, setRotating] = useState(false);
  const rotate = async () => {
    if (rotating) return;
    setRotating(true);
    const target = Array.from({ length: 16 }, () => alnum[Math.floor(Math.random() * alnum.length)]).join("");
    for (let f = 0; f <= 16; f++) {
      setSecret(target.slice(0, f) + Array.from({ length: 16 - f }, () => alnum[Math.floor(Math.random() * alnum.length)]).join(""));
      await sleep(45);
    }
    setRotating(false);
  };
  const e = ENGINES[engine];
  const host = pub ? "db.example.com" : "store-db";
  return (
    <div className="w-card">
      <div className="flex gap-1 overflow-x-auto border-b border-line p-2">
        {ENGINES.map((x, i) => (
          <button key={x.name} type="button" onClick={() => setEngine(i)} className={`flex shrink-0 cursor-pointer items-center gap-2 rounded-[8px] px-3 py-2 text-[13px] transition-colors ${engine === i ? "bg-white/[0.08] text-ink" : "text-mute hover:text-ink"}`}>
            <img src={`/media/logos/${x.logo}`} alt="" className="size-4 object-contain" />
            {x.name}
          </button>
        ))}
      </div>
      <div className="space-y-4 p-5">
        <div>
          <div className="mb-2 text-[12px] text-mute">Connection string</div>
          <div className="t-mono overflow-x-auto whitespace-nowrap rounded-[8px] border border-line bg-[#0a0b0b] px-3 py-2.5 text-[12.5px] text-ink-2">
            {e.scheme}://{e.user}:<span className={rotating ? "text-amber" : "text-ink"}>{secret}</span>@<span key={host} className="text-ink" style={{ animation: "fade-up 300ms var(--ease-out)" }}>{host}</span>:{e.port}
            {e.scheme !== "redis" && "/store"}
          </div>
        </div>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <Switch checked={pub} onChange={setPub}>{pub ? "Public on db.example.com" : "Private network only"}</Switch>
          <button type="button" className="w-btn" onClick={rotate} disabled={rotating}>Rotate password</button>
        </div>
      </div>
    </div>
  );
}

/* ---------- 10. Domains and HTTPS ---------- */

export function DomainWidget() {
  const [domain, setDomain] = useState("store.example.com");
  const [step, setStep] = useState(3);
  const [live, setLive] = useState("store.example.com");
  const busy = step > 0 && step < 3;
  const add = async () => {
    const d = domain.trim().toLowerCase().replace(/^https?:\/\//, "").replace(/\/.*$/, "") || "store.example.com";
    setDomain(d);
    setStep(0);
    for (let i = 1; i <= 3; i++) {
      await sleep(i === 2 ? 1100 : 650);
      setStep(i);
    }
    setLive(d);
  };
  const steps = ["DNS points at this server", "Certificate issued by Let's Encrypt", "Serving over HTTPS"];
  return (
    <div className="w-card">
      <form className="flex gap-2 border-b border-line p-3" onSubmit={(e) => { e.preventDefault(); add(); }}>
        <input className="w-input t-mono flex-1 text-[13px]" value={domain} onChange={(e) => setDomain(e.target.value)} aria-label="Domain" spellCheck={false} />
        <button type="submit" className="w-btn is-primary" disabled={busy}>Add domain</button>
      </form>
      <ul className="space-y-2.5 p-4">
        {steps.map((s, i) => {
          const ok = step > i;
          const active = step === i && busy || (step === 0 && i === 0);
          return (
            <li key={s} className={`flex items-center gap-3 text-[13.5px] transition-colors duration-300 ${ok ? "text-ink" : "text-mute"}`}>
              <span className={`grid size-5 place-items-center rounded-full border transition-all duration-300 ${ok ? "border-green/60 bg-green/10 text-green" : "border-line"}`}>
                {ok ? <Check className="size-3" /> : active ? <span className="size-1.5 animate-pulse rounded-full bg-ink" /> : null}
              </span>
              {s}
            </li>
          );
        })}
      </ul>
      <div className="mx-4 mb-4 flex items-center gap-2 rounded-[8px] border border-line bg-[#0a0b0b] px-3 py-2 text-[13px]">
        <span className={step >= 3 ? "text-green" : "text-mute"}><Lock open={step < 3} /></span>
        <span className="t-mono truncate text-ink-2">https://<span className="text-ink">{step >= 3 ? live : domain}</span></span>
      </div>
    </div>
  );
}

/* ---------- Bonus: a status board that ticks ---------- */

export function StatusWidget() {
  const [ref, inView] = useInView<HTMLDivElement>(false);
  const [beat, setBeat] = useState(0);
  useEffect(() => {
    if (!inView) return;
    const t = setInterval(() => setBeat((b) => b + 1), 1400);
    return () => clearInterval(t);
  }, [inView]);
  const rows = [
    ["store", "Next.js", "store.example.com", 0.42],
    ["api", "Go", "api.example.com", 0.18],
    ["postgres", "PostgreSQL 17", "private", 0.31],
    ["n8n", "Service Library", "flows.example.com", 0.55],
    ["uptime", "Service Library", "status.example.com", 0.12],
  ] as const;
  return (
    <div ref={ref} className="w-card">
      <div className="w-toolbar"><span>Applications</span><span className="flex items-center gap-2"><span className="dot" />All running</span></div>
      <ul>
        {rows.map(([name, kind, where, load], i) => {
          const v = Math.max(0.04, Math.min(0.95, load + Math.sin(beat * 0.9 + i * 1.7) * 0.12));
          return (
            <li key={name} className="grid grid-cols-[1fr_auto] items-center gap-3 border-b border-line px-4 py-3 last:border-b-0 sm:grid-cols-[1.1fr_1fr_90px]">
              <span className="min-w-0"><span className="flex items-center gap-2 text-[14px] text-ink"><span className="dot" />{name}</span><span className="block text-[12px] text-mute">{kind}</span></span>
              <span className="t-mono hidden truncate text-[12px] text-ink-2 sm:block">{where}</span>
              <span className="flex items-center gap-2">
                <span className="h-1 w-14 overflow-hidden rounded-full bg-white/[0.07]"><span className="block h-full rounded-full bg-white/50 transition-[width] duration-1000 ease-[var(--ease-out)]" style={{ width: `${v * 100}%` }} /></span>
                <span className="t-mono w-8 text-right text-[11px] tabular-nums text-mute">{Math.round(v * 100)}%</span>
              </span>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

/* ---------- Tabbed product tour over real screenshots ---------- */

export const SHOTS = [
  { key: "applications", label: "Applications", alt: "Applications in the Shipper dashboard" },
  { key: "deploy", label: "Deploy", alt: "Deploying a new application" },
  { key: "releases", label: "Releases", alt: "A failed deployment next to the release that is still serving" },
  { key: "databases", label: "Databases", alt: "A managed MongoDB database" },
  { key: "backups", label: "Backups", alt: "Backup schedules and history" },
  { key: "service-library", label: "Service Library", alt: "The Service Library" },
];

export function ShotTabs({ start = 0, auto = true }: { start?: number; auto?: boolean }) {
  const [i, setI] = useState(start);
  const [ref, inView] = useInView<HTMLDivElement>(false);
  const touched = useRef(false);
  useEffect(() => {
    if (!auto || !inView || touched.current) return;
    const t = setInterval(() => setI((n) => (n + 1) % SHOTS.length), 4200);
    return () => clearInterval(t);
  }, [auto, inView]);
  return (
    <div ref={ref}>
      <div className="mb-5 flex justify-center">
        <div className="flex max-w-full gap-1 overflow-x-auto rounded-[10px] border border-line bg-[#0c0d0d] p-1">
          {SHOTS.map((s, k) => (
            <button
              key={s.key}
              type="button"
              onClick={() => { touched.current = true; setI(k); }}
              className={`relative shrink-0 cursor-pointer overflow-hidden rounded-[7px] px-3 py-1.5 text-[13px] transition-colors ${i === k ? "bg-white/[0.09] text-ink" : "text-mute hover:text-ink"}`}
            >
              {s.label}
            </button>
          ))}
        </div>
      </div>
      <div className="shot relative aspect-[1440/900]">
        {SHOTS.map((s, k) => (
          <img
            key={s.key}
            src={`/media/shots/${s.key}.webp`}
            alt={s.alt}
            width={1440}
            height={900}
            loading={k === start ? "eager" : "lazy"}
            className="absolute inset-0 size-full transition-[opacity,transform] duration-700 ease-[var(--ease-out)]"
            style={{ opacity: i === k ? 1 : 0, transform: i === k ? "none" : "scale(1.012)" }}
          />
        ))}
      </div>
    </div>
  );
}
