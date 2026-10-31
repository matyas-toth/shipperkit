import { splitProps, type JSX } from "solid-js";
import { Icon, type IconName } from "./Icon";
import { statusLabel } from "../data";

export function Button(props: JSX.ButtonHTMLAttributes<HTMLButtonElement> & { variant?: "neutral" | "ghost"; small?: boolean; icon?: IconName }) {
  const [local, rest] = splitProps(props, ["variant", "small", "icon", "class", "children"]);
  return <button type="button" class={"button " + (local.variant ?? "neutral") + (local.small ? " small" : "") + " " + (local.class ?? "")} {...rest}>
    {local.icon && <Icon name={local.icon} />}{local.children}
  </button>;
}
export function IconButton(props: JSX.ButtonHTMLAttributes<HTMLButtonElement> & { icon: IconName; label: string; ghost?: boolean }) {
  const [local, rest] = splitProps(props, ["icon", "label", "ghost", "class"]);
  return <button type="button" class={"button icon-button " + (local.ghost ? "ghost" : "neutral") + " " + (local.class ?? "")}
    aria-label={local.label} title={local.label} {...rest}><Icon name={local.icon} /></button>;
}
export function Status(props: { value: string }) {
  return <span class={"status status-" + props.value}><span class="status-dot" />{statusLabel(props.value)}</span>;
}
export function ErrorNotice(props: { message: string; retry?: () => void }) {
  return <div class="error-notice" role="alert"><Icon name="warning" /><span>{props.message}</span>
    {props.retry && <Button variant="ghost" small onClick={props.retry}>Retry</Button>}</div>;
}

