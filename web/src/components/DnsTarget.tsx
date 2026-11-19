import { Show, createSignal } from "solid-js";
import { IconButton } from "./Controls";

// The address an operator's DNS record must point at, ready to copy.
export default function DnsTarget(props: { address?: string }) {
  const [copied, setCopied] = createSignal(false);
  async function copy() {
    try { await navigator.clipboard.writeText(props.address!); setCopied(true); setTimeout(() => setCopied(false), 1600); } catch { /* clipboard unavailable */ }
  }
  return <Show when={props.address}>{address => <div class="dns-target">
    <span>{address().includes(":") ? "AAAA record" : "A record"}</span>
    <code>{address()}</code>
    <IconButton icon={copied() ? "check" : "copy"} ghost label={copied() ? "Copied" : "Copy server address"} onClick={() => void copy()} />
  </div>}</Show>;
}
