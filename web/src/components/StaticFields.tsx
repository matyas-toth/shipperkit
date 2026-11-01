import { Show } from "solid-js";
import type { StaticConfig } from "../types";
import { Icon } from "./Icon";

export const emptyConfig = (): StaticConfig => ({ repository: "", branch: "", publishDirectory: ".", routing: "files", htaccess: false, domain: "", https: false });
export default function StaticFields(props: { config: StaticConfig; change: (key: keyof StaticConfig, value: string | boolean) => void; advanced?: boolean; acme: boolean }) {
  return <>
    <label for="repository">Git repository</label><input class="input" id="repository" type="url" required placeholder="https://github.com/owner/website" autocomplete="off" value={props.config.repository} onInput={e => props.change("repository", e.currentTarget.value)} />
    <Show when={!props.advanced}>
      <label for="domain">Domain</label><input class="input" id="domain" placeholder="Automatic .localhost address" value={props.config.domain} onInput={e => { props.change("domain", e.currentTarget.value); if (e.currentTarget.value.endsWith(".localhost")) props.change("https", false); }} />
      <Show when={props.config.domain && !props.config.domain.endsWith(".localhost")}><label class="check-field"><input type="checkbox" checked={props.config.https} disabled={!props.acme} onChange={e => props.change("https", e.currentTarget.checked)} />HTTPS</label></Show>
    </Show>
    <details class="configuration" open={props.advanced}><summary>Configuration<Icon name="down" /></summary><div class="form-fields configuration-fields">
      <label for="branch">Branch</label><input class="input" id="branch" placeholder="Default branch" value={props.config.branch} onInput={e => props.change("branch", e.currentTarget.value)} />
      <label for="publish-directory">Publish directory</label><input class="input" id="publish-directory" required value={props.config.publishDirectory} onInput={e => props.change("publishDirectory", e.currentTarget.value)} />
      <label for="routing">Routing</label><select class="input" id="routing" value={props.config.routing} onChange={e => props.change("routing", e.currentTarget.value)}><option value="files">File based</option><option value="spa">SPA fallback</option><option value="strict">Strict</option></select>
      <label class="check-field"><input type="checkbox" checked={props.config.htaccess} onChange={e => props.change("htaccess", e.currentTarget.checked)} />.htaccess rewrites</label>
    </div></details>
  </>;
}
