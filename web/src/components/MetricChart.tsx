import { For, Show } from "solid-js";

type Point = { x: number; y: number };

function path(points: Point[]) {
  return points.map((point, index) => `${index === 0 ? "M" : "L"}${point.x.toFixed(1)} ${point.y.toFixed(1)}`).join(" ");
}

function segments(values: Array<number | null>, width: number, height: number, padding: number) {
  const result: Point[][] = [];
  let current: Point[] = [];
  const step = values.length > 1 ? width / (values.length - 1) : width;
  const usable = height - padding * 2;
  values.forEach((value, index) => {
    if (value == null) {
      if (current.length > 1) result.push(current);
      current = [];
      return;
    }
    current.push({ x: index * step, y: padding + (1 - Math.min(100, Math.max(0, value)) / 100) * usable });
  });
  if (current.length > 1) result.push(current);
  return result;
}

export function Sparkline(props: { values: Array<number | null>; state?: string }) {
  const width = 64;
  const height = 18;
  const parts = () => segments(props.values, width, height, 2);
  return (
    <svg class="sparkline" classList={{ warning: props.state === "warning", critical: props.state === "critical" }} viewBox={`0 0 ${width} ${height}`} aria-hidden="true" preserveAspectRatio="none">
      <For each={parts()}>{points => <path d={path(points)} fill="none" stroke="currentColor" stroke-width="1.25" vector-effect="non-scaling-stroke" />}</For>
    </svg>
  );
}

export function MetricChart(props: {
  label: string;
  value: string;
  state?: string;
  values: Array<number | null>;
  detail?: string;
}) {
  const width = 240;
  const height = 72;
  const parts = () => segments(props.values, width, height, 4);
  const hasData = () => parts().length > 0;
  const summary = () => {
    const values = props.values.filter((value): value is number => value != null);
    if (!values.length) return `${props.label} has no samples yet`;
    return `${props.label} 24 hour range, ${Math.round(Math.min(...values))} to ${Math.round(Math.max(...values))} percent`;
  };
  return (
    <section class="metric-chart" classList={{ warning: props.state === "warning", critical: props.state === "critical" }}>
      <header>
        <span>{props.label}</span>
        <strong>{props.value}</strong>
      </header>
      <svg class="metric-chart-plot" viewBox={`0 0 ${width} ${height}`} role="img" aria-label={summary()} preserveAspectRatio="none">
        <line class="metric-chart-guide" x1="0" x2={width} y1={height / 2} y2={height / 2} />
        <Show when={hasData()} fallback={<line class="metric-chart-empty" x1="0" x2={width} y1={height - 4} y2={height - 4} />}>
          <For each={parts()}>{points => (
            <>
              <path class="metric-chart-area" d={`${path(points)} L${points[points.length - 1].x.toFixed(1)} ${height} L${points[0].x.toFixed(1)} ${height} Z`} />
              <path class="metric-chart-line" d={path(points)} fill="none" vector-effect="non-scaling-stroke" />
            </>
          )}</For>
        </Show>
      </svg>
      <Show when={props.detail}>{detail => <span class="metric-chart-detail">{detail()}</span>}</Show>
    </section>
  );
}
