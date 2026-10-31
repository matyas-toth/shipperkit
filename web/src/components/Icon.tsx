import { splitProps, type JSX } from "solid-js";

const paths = {
  ship: "M2 10h12l-2 4H4l-2-4ZM4 8l4-6v6H4Zm6-4 3 4h-3V4Z",
  box: "m8 1.75 5.5 3v6.5l-5.5 3-5.5-3v-6.5l5.5-3Zm0 6.5v6m-5.5-9.5 5.5 3 5.5-3M5.25 3.25l5.5 3v2.5",
  search: "M7 11.5a4.5 4.5 0 1 0 0-9 4.5 4.5 0 0 0 0 9Zm3.25-1.25L14 14",
  refresh: "M13.5 5.5A5.75 5.75 0 1 0 13.6 10M13.5 2v3.5H10",
  chevron: "m6 4 4 4-4 4",
  close: "m4 4 8 8m0-8-8 8",
  copy: "M6 6h7v8H6V6ZM10 6V2H3v8h3",
  check: "m3 8 3.5 3.5L13 4.5",
  settings: "M6.5 2h3l.5 2 1.5 1 2-.5 1.5 2.5-1.5 1.5v1.75L15 12.5 13.5 15l-2-.5L10 15H6l-.5-.5-2 .5L2 12.5l1.5-2.25V8.5L2 7l1.5-2.5 2 .5 1-1 .5-2ZM10.5 8.5a2.5 2.5 0 1 0-5 0 2.5 2.5 0 0 0 5 0Z",
  sliders: "M2 4h5m3 0h4M2 12h8m3 0h1M7 2v4m3 4v4",
  logout: "M7 2H3v12h4m3-10 4 4-4 4m-4-4h8",
  eye: "M1.5 8s2.25-4.5 6.5-4.5 6.5 4.5 6.5 4.5-2.25 4.5-6.5 4.5S1.5 8 1.5 8Zm8.5 0a2 2 0 1 0-4 0 2 2 0 0 0 4 0Z",
  plug: "M5 1v4m6-4v4M3.5 5h9v2a4.5 4.5 0 0 1-9 0V5ZM8 11.5V15",
  database: "M13.5 4c0 1.25-2.5 2.25-5.5 2.25S2.5 5.25 2.5 4 5 1.75 8 1.75s5.5 1 5.5 2.25Zm-11 0v8c0 1.25 2.5 2.25 5.5 2.25s5.5-1 5.5-2.25V4m-11 4c0 1.25 2.5 2.25 5.5 2.25s5.5-1 5.5-2.25",
  arrow: "M8 13V3m-4 4 4-4 4 4",
  warning: "M8 2 15 14H1L8 2Zm0 4v4m0 1.5v.5",
} as const;
export type IconName = keyof typeof paths;
export function Icon(props: JSX.SvgSVGAttributes<SVGSVGElement> & { name: IconName }) {
  const [local, rest] = splitProps(props, ["name", "class"]);
  return <svg viewBox="0 0 16 16" width="16" height="16" aria-hidden="true"
    fill={local.name === "ship" ? "currentColor" : "none"} stroke={local.name === "ship" ? "none" : "currentColor"}
    stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round" class={"icon " + (local.class ?? "")} {...rest}>
    <path d={paths[local.name]} />
  </svg>;
}

