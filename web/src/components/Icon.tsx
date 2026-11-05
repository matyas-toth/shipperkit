import { splitProps, type JSX } from "solid-js";
import { Dynamic } from "solid-js/web";
import { IconCube, IconSearch, IconRefresh, IconChevronRight, IconX, IconCopy, IconCheck, IconSettings,
  IconAdjustmentsHorizontal, IconLogout, IconEye, IconPlug, IconDatabase, IconArrowUp, IconAlertTriangle,
  IconPlus, IconExternalLink, IconArrowLeft, IconBrandGit, IconWorld, IconFileCode, IconRocket, IconTrash,
  IconChevronDown, IconTerminal2, IconDots, IconPlayerPlay, IconPlayerPause, IconDownload, IconStack2, IconFolder } from "@tabler/icons-solidjs";

const icons = { box: IconCube, search: IconSearch, refresh: IconRefresh, chevron: IconChevronRight, close: IconX,
  copy: IconCopy, check: IconCheck, settings: IconSettings, sliders: IconAdjustmentsHorizontal, logout: IconLogout,
  eye: IconEye, plug: IconPlug, database: IconDatabase, arrow: IconArrowUp, warning: IconAlertTriangle,
  plus: IconPlus, external: IconExternalLink, back: IconArrowLeft, git: IconBrandGit, globe: IconWorld,
  code: IconFileCode, deploy: IconRocket, trash: IconTrash, down: IconChevronDown, terminal: IconTerminal2, more: IconDots, play: IconPlayerPlay, pause: IconPlayerPause, download: IconDownload, layers: IconStack2, folder: IconFolder };
export type IconName = keyof typeof icons;
export function Icon(props: JSX.SvgSVGAttributes<SVGSVGElement> & { name: IconName }) {
  const [local, rest] = splitProps(props, ["name", "class"]);
  const className = () => "icon " + (local.class ?? "");
  return <Dynamic component={icons[local.name]} size={16} stroke="1.5" aria-hidden="true" class={className()} {...rest} />;
}
