import { For, createSignal } from "solid-js";
import { post } from "../api";
import { useAuth } from "../context";
import type { StaticConfig } from "../types";
import { Button, ErrorNotice } from "./Controls";
import { Show } from "solid-js";
import "./DatabaseResource.css";

const engines = [{id:"postgresql",name:"PostgreSQL",versions:["18","17","16"]},{id:"mysql",name:"MySQL",versions:["8.4"]},{id:"mariadb",name:"MariaDB",versions:["11.8","11.4"]},{id:"redis",name:"Redis",versions:["8","7"]}];
export default function DatabaseResource(props:{prepare:(config:StaticConfig,name:string)=>void|Promise<void>;close:()=>void}){
 const auth=useAuth(),[engine,setEngine]=createSignal(engines[0]),[version,setVersion]=createSignal("18"),[name,setName]=createSignal(""),[busy,setBusy]=createSignal(false),[error,setError]=createSignal("");
 async function prepare(e:SubmitEvent){e.preventDefault();setBusy(true);setError("");try{const result=await post<{config:StaticConfig}>("/database-resources/prepare",{engine:engine().id,version:version()},auth.session()?.csrfToken);await props.prepare(result.config,name().trim()||engine().name);}catch(e){setError((e as Error).message);}finally{setBusy(false);}}
 return <form onSubmit={prepare} class="database-resource-form"><h2>New database</h2><Show when={error()}><ErrorNotice message={error()}/></Show><fieldset disabled={busy()} class="form-fields"><label for="database-engine">Engine</label><select id="database-engine" class="input" value={engine().id} onChange={e=>{const next=engines.find(row=>row.id===e.currentTarget.value)!;setEngine(next);setVersion(next.versions[0]);}}><For each={engines}>{row=><option value={row.id}>{row.name}</option>}</For></select><label for="database-version">Version</label><select id="database-version" class="input" value={version()} onChange={e=>setVersion(e.currentTarget.value)}><For each={engine().versions}>{v=><option>{v}</option>}</For></select><label for="database-name">Name</label><input id="database-name" class="input" maxlength="80" placeholder={engine().name} value={name()} onInput={e=>setName(e.currentTarget.value)}/></fieldset><div class="form-actions"><Button variant="ghost" disabled={busy()} onClick={props.close}>Cancel</Button><Button type="submit" variant="primary" disabled={busy()}>{busy()?"Preparing database":"Review database"}</Button></div></form>;
}
