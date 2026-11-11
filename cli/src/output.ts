import { createColors } from "picocolors";

let enabled = Boolean(process.stdout.isTTY) && !process.env.NO_COLOR;

export function setColor(value: boolean) {
  enabled = value;
}

export function paint() {
  return createColors(enabled);
}

const ansi = /\u001b\[[0-9;]*m/g;

export function stripAnsi(value: string) {
  return value.replace(ansi, "");
}

export function heading(value: string) {
  return paint().bold(paint().white(value));
}

export function muted(value: string) {
  return paint().dim(value);
}

export function statusText(value: string) {
  const colors = paint();
  const state = value.toLowerCase();
  const dot = "●";
  if (["running", "succeeded", "healthy", "deployed", "ready"].includes(state)) return colors.green(`${dot} ${value}`);
  if (["failed", "unavailable", "error", "missing", "cancelled"].includes(state)) return colors.red(`${dot} ${value}`);
  if (["degraded", "warning", "unknown", "interrupted"].includes(state)) return colors.yellow(`${dot} ${value}`);
  if (["queued", "building", "cloning", "packaging", "starting", "checking", "promoting", "operating", "pending"].includes(state)) return colors.yellow(`${dot} ${value}`);
  if (["stopped", "exited", "idle"].includes(state)) return colors.dim(`${dot} ${value}`);
  return `${dot} ${value}`;
}

export function relativeTime(epoch?: number) {
  if (!epoch) return "never";
  const seconds = Math.max(0, Math.round(Date.now() / 1000 - epoch));
  if (seconds < 60) return `${seconds}s ago`;
  if (seconds < 3600) return `${Math.round(seconds / 60)}m ago`;
  if (seconds < 86400) return `${Math.round(seconds / 3600)}h ago`;
  return `${Math.round(seconds / 86400)}d ago`;
}

export function table(headers: string[], rows: Array<Array<string | number>>) {
  if (!rows.length) return "";
  const widths = headers.map((header, index) => Math.max(stripAnsi(header).length, ...rows.map(row => stripAnsi(String(row[index] ?? "")).length)));
  const format = (cells: Array<string | number>) =>
    cells.map((cell, index) => {
      const value = String(cell ?? "");
      return value + " ".repeat(Math.max(0, widths[index] - stripAnsi(value).length));
    }).join("  ");
  const colors = paint();
  return [
    colors.dim(format(headers)),
    colors.dim(widths.map(width => "─".repeat(width)).join("  ")),
    ...rows.map(row => format(row)),
  ].join("\n");
}

export function printJson(value: unknown) {
  process.stdout.write(JSON.stringify(value, null, 2) + "\n");
}

export function fail(message: string) {
  process.stderr.write(paint().red("shipper: ") + message + "\n");
}
