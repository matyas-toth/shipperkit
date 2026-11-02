import { A, useNavigate } from "@solidjs/router";
import { createQuery } from "@tanstack/solid-query";
import { Show, createEffect, createSignal, onMount } from "solid-js";
import { createStore } from "solid-js/store";
import { api, post } from "../api";
import type { Application, Deployment, ProxyConfig } from "../types";
import Variables from "../components/Variables";
import { useAuth } from "../context";
import { Button, ErrorNotice } from "../components/Controls";
import { Icon } from "../components/Icon";
import StaticFields, { emptyConfig } from "../components/StaticFields";
import { clearDraft, readDraft, safeConfigDraft, saveDraft } from "../drafts";

export default function NewApplication() {
  const auth = useAuth(), navigate = useNavigate(), key = "new:" + auth.session()?.user?.id;
  const draft = readDraft<{ name: string; config: ReturnType<typeof emptyConfig> }>(key);
  const [config, setConfig] = createStore({ ...emptyConfig(), ...draft?.config });
  const [name, setName] = createSignal(draft?.name ?? ""), [pending, setPending] = createSignal(false), [error, setError] = createSignal("");
  createEffect(() => saveDraft(key, { name: name(), config: safeConfigDraft(config) }));
  const proxy = createQuery(() => ({ queryKey: ["proxy"], queryFn: () => api<ProxyConfig>("/proxy") }));
  onMount(() => { document.title = "New application | Shipper"; });
  async function submit(e: SubmitEvent) {
    e.preventDefault(); if (pending()) return; setPending(true); setError("");
    try { const d = await post<Deployment>("/applications", { name: name(), config }, auth.session()?.csrfToken); clearDraft(key);clearDraft("variables:" + auth.session()?.user?.id + ":new"); navigate("/applications/" + d.applicationId + "?tab=deployments"); }
    catch (e) { setError((e as Error).message); } finally { setPending(false); }
  }
  return <div class="content content-form"><A href="/" class="back-link"><Icon name="back" />Applications</A><header class="page-heading"><h1>New application</h1></header>
    <form onSubmit={submit}><Show when={error()}><ErrorNotice message={error()} /></Show><div class="form-fields">
      <label for="name">Name</label><input class="input" id="name" maxlength={80} placeholder={config.composeSource === "paste" ? "Application name" : "Repository name"} value={name()} onInput={e => setName(e.currentTarget.value)} />
      <StaticFields config={config} change={(key, value) => setConfig(key, value as never)} acme={!!proxy.data?.acmeEmail} />
    </div><details class="configuration new-variables"><summary>.env<Icon name="down" /></summary><Variables application={{ id: "new", config } as Application} busy={pending()} embedded change={variables => setConfig("variables", variables)} /></details><div class="form-actions"><A href="/" class="button ghost">Cancel</A><Button type="submit" icon="deploy" disabled={pending()}>{pending() ? "Creating application" : "Deploy"}</Button></div></form>
  </div>;
}
