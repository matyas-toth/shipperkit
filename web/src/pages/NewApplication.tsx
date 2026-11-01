import { A, useNavigate } from "@solidjs/router";
import { createQuery } from "@tanstack/solid-query";
import { Show, createSignal, onMount } from "solid-js";
import { createStore } from "solid-js/store";
import { api, post } from "../api";
import type { Deployment, ProxyConfig } from "../types";
import { useAuth } from "../context";
import { Button, ErrorNotice } from "../components/Controls";
import { Icon } from "../components/Icon";
import StaticFields, { emptyConfig } from "../components/StaticFields";

export default function NewApplication() {
  const auth = useAuth(), navigate = useNavigate(), [config, setConfig] = createStore(emptyConfig());
  const [name, setName] = createSignal(""), [pending, setPending] = createSignal(false), [error, setError] = createSignal("");
  const proxy = createQuery(() => ({ queryKey: ["proxy"], queryFn: () => api<ProxyConfig>("/proxy") }));
  onMount(() => { document.title = "New application | Shipper"; });
  async function submit(e: SubmitEvent) {
    e.preventDefault(); if (pending()) return; setPending(true); setError("");
    try { const d = await post<Deployment>("/applications", { name: name(), config }, auth.session()?.csrfToken); navigate("/applications/" + d.applicationId + "?tab=deployments"); }
    catch (e) { setError((e as Error).message); } finally { setPending(false); }
  }
  return <div class="content content-form"><A href="/" class="back-link"><Icon name="back" />Applications</A><header class="page-heading"><h1>New application</h1></header>
    <form onSubmit={submit}><Show when={error()}><ErrorNotice message={error()} /></Show><div class="form-fields">
      <label for="name">Name</label><input class="input" id="name" maxlength={80} placeholder="Repository name" value={name()} onInput={e => setName(e.currentTarget.value)} />
      <StaticFields config={config} change={(key, value) => setConfig(key, value as never)} acme={!!proxy.data?.acmeEmail} />
    </div><div class="form-actions"><A href="/" class="button ghost">Cancel</A><Button type="submit" icon="deploy" disabled={pending()}>{pending() ? "Creating application" : "Deploy"}</Button></div></form>
  </div>;
}
