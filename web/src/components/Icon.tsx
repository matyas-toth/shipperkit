import { splitProps, type JSX } from "solid-js";
import { Dynamic } from "solid-js/web";
import { IconCube, IconSearch, IconRefresh, IconChevronRight, IconX, IconCopy, IconCheck, IconSettings,
  IconAdjustmentsHorizontal, IconLogout, IconEye, IconPlug, IconDatabase, IconArrowUp, IconAlertTriangle,
  IconPlus, IconExternalLink, IconArrowLeft, IconBrandGit, IconWorld, IconFileCode, IconRocket, IconTrash,
  IconChevronDown, IconTerminal2 } from "@tabler/icons-solidjs";

const icons = { box: IconCube, search: IconSearch, refresh: IconRefresh, chevron: IconChevronRight, close: IconX,
  copy: IconCopy, check: IconCheck, settings: IconSettings, sliders: IconAdjustmentsHorizontal, logout: IconLogout,
  eye: IconEye, plug: IconPlug, database: IconDatabase, arrow: IconArrowUp, warning: IconAlertTriangle,
  plus: IconPlus, external: IconExternalLink, back: IconArrowLeft, git: IconBrandGit, globe: IconWorld,
  code: IconFileCode, deploy: IconRocket, trash: IconTrash, down: IconChevronDown, terminal: IconTerminal2 };
export type IconName = keyof typeof icons | "ship";
export function Icon(props: JSX.SvgSVGAttributes<SVGSVGElement> & { name: IconName }) {
  const [local, rest] = splitProps(props, ["name", "class"]);
  const className = () => "icon " + (local.class ?? "");
  if (local.name === "ship") return <svg viewBox="0 0 16 16" width="16" height="16" fill="currentColor" aria-hidden="true" class={className()} {...rest}><path d="M2 10h12l-2 4H4l-2-4ZM4 8l4-6v6H4Zm6-4 3 4h-3V4Z" /></svg>;
  return <Dynamic component={icons[local.name]} size={16} stroke="1.5" aria-hidden="true" class={className()} {...rest} />;
}
