import { Show, createSignal } from "solid-js";
import { operationTime } from "../data";
import { useUpdates } from "../context";
import { Button, ErrorNotice } from "./Controls";
import { Icon } from "./Icon";

export default function Updates() {
  const updates = useUpdates();
  const [busy, setBusy] = createSignal(false);
  const [error, setError] = createSignal("");
  const data = () => updates.data();
  const available = () => data()?.available;
  const status = () => data()?.status;
  const active = () => !!status() && ["queued", "preparing", "pulling", "checkpoint", "updating", "starting", "verifying", "restarting"].includes(status()!.state);
  const succeeded = () => status()?.state === "succeeded" && status()?.toVersion !== data()?.currentVersion;
  const failed = () => status()?.state === "failed";
  const locked = () => busy() || active() || !data()?.enabled;

  async function check() {
    setBusy(true); setError("");
    try { await updates.check(); } catch (e) { setError((e as Error).message); } finally { setBusy(false); }
  }
  async function save(channel: "main" | "nightly", auto: boolean) {
    setBusy(true); setError("");
    try { await updates.save(channel, auto); } catch (e) { setError((e as Error).message); } finally { setBusy(false); }
  }
  async function apply() {
    setBusy(true); setError("");
    try { await updates.apply(); } catch (e) { setError((e as Error).message); } finally { setBusy(false); }
  }

  return <section class="settings-section updates-section" id="updates">
    <div class="section-toolbar updates-toolbar">
      <h2>Updates</h2>
      <div class="updates-toolbar-actions">
        <span class="updates-last-check">{data()?.checkedAt ? "Checked " + operationTime(data()!.checkedAt, true) : "Not checked yet"}</span>
        <Button small variant="ghost" icon="refresh" disabled={!data()?.enabled || busy() || data()?.checking} aria-busy={data()?.checking} onClick={() => void check()}>{data()?.checking ? "Checking" : "Check for updates"}</Button>
      </div>
    </div>

    <Show when={error() || data()?.error || updates.error()}><ErrorNotice message={error() || data()?.error || updates.error()!.message} retry={() => void check()} /></Show>

    <Show when={data()?.enabled} fallback={updates.pending() ? <span class="loading-line" aria-label="Loading updates" /> : <div class="update-unavailable"><Icon name="server" /><div><strong>Managed updates are unavailable</strong><p>This installation does not have the host update helper. Run updates with the managed installer.</p></div></div>}>
      <Show when={active()}>
        <div class="update-progress-banner" role="status">
          <span class="status-dot is-running" />
          <div><strong>Updating to {status()?.toVersion || available()?.version}</strong><p>{status()?.message || "Preparing the update"}</p></div>
          <Button small variant="ghost" onClick={updates.open}>View progress</Button>
        </div>
      </Show>
      <Show when={succeeded()}>
        <div class="update-success-banner" role="status">
          <span class="update-banner-icon"><Icon name="check" /></span>
          <div><strong>Shipper {status()?.toVersion} is ready</strong><p>Reload to use the new release.</p></div>
          <Button small variant="primary" icon="refresh" onClick={() => location.reload()}>Reload</Button>
        </div>
      </Show>
      <Show when={failed() && !active()}>
        <div class="update-failed-banner" role="alert">
          <span class="update-banner-icon"><Icon name="warning" /></span>
          <div><strong>Update failed</strong><p>{status()?.message || "Shipper restored the previous release."}</p></div>
          <Show when={available()}><Button small variant="ghost" onClick={() => void apply()}>Try again</Button></Show>
          <Button small variant="ghost" onClick={updates.open}>Details</Button>
        </div>
      </Show>
      <Show when={available() && !data()?.auto && !active()}>
        <div class="update-available-banner" role="status">
          <span class="update-banner-icon"><Icon name="download" /></span>
          <div><strong>Update available</strong><p>Shipper {available()!.version} is ready to install. Applications stay online.</p></div>
          <div class="update-banner-actions">
            <a class="button ghost small" href={available()!.releaseUrl} target="_blank" rel="noreferrer">See what changed<Icon name="external" /></a>
            <Button variant="primary" icon="download" disabled={locked()} onClick={() => void apply()}>Update</Button>
          </div>
        </div>
      </Show>
      <Show when={available() && data()?.auto && !active()}>
        <div class="update-auto-banner" role="status">
          <Icon name="refresh" />
          <span>Auto update is on. Shipper will install {available()!.version} automatically.</span>
          <Button small variant="ghost" onClick={() => void apply()}>Update now</Button>
        </div>
      </Show>

      <fieldset class="update-channels" disabled={locked()}>
        <legend>Channel</legend>
        <div class="update-channel-grid">
          <label classList={{ selected: data()?.channel === "main" }}>
            <input type="radio" name="update-channel" value="main" checked={data()?.channel === "main"} onChange={() => void save("main", data()?.auto ?? false)} />
            <span class="update-channel-heading"><span class="update-channel-icon"><Icon name="layers" /></span><strong>Main</strong><span class="update-channel-tag">Default</span></span>
            <span class="update-channel-copy">Tagged Shipper releases, published after review. Recommended for production.</span>
          </label>
          <label classList={{ selected: data()?.channel === "nightly", "is-nightly": true }}>
            <input type="radio" name="update-channel" value="nightly" checked={data()?.channel === "nightly"} onChange={() => void save("nightly", data()?.auto ?? false)} />
            <span class="update-channel-heading"><span class="update-channel-icon"><Icon name="terminal" /></span><strong>Nightly</strong></span>
            <span class="update-channel-copy">Every successful master build, published automatically.</span>
            <span class="update-channel-warning"><Icon name="warning" />Can break things or ship bugs. Use it to test upcoming changes.</span>
          </label>
        </div>
      </fieldset>

      <div class="update-auto-row">
        <div><strong>Auto update</strong><p>Install new versions automatically after the five-minute check.</p></div>
        <label class="update-switch">
          <input type="checkbox" role="switch" aria-label="Auto update" checked={data()?.auto ?? false} disabled={locked()} onChange={e => void save(data()?.channel ?? "main", e.currentTarget.checked)} />
          <span class="update-switch-track" aria-hidden="true" />
        </label>
      </div>

      <dl class="settings-group update-details">
        <div class="setting-row"><dt>Current version</dt><dd class="mono">{data()?.currentVersion || "Loading"}</dd></div>
        <div class="setting-row"><dt>Channel</dt><dd>{data()?.channel === "nightly" ? "Nightly" : "Main"}</dd></div>
        <div class="setting-row"><dt>Check interval</dt><dd>Every 5 minutes</dd></div>
      </dl>
    </Show>
  </section>;
}
