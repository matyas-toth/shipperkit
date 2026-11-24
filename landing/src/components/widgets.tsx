import { useCallback, useEffect, useRef, useState } from "react";

/* Interactive pieces of the landing page. They simulate Shipper and never
   pretend to be live data. */

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

/* The CLI, as you or a coding agent would use it. */

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

/* Product tour over real screenshots. It rotates until someone picks a tab. */

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
