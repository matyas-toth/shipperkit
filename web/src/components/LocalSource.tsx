import { Show, createSignal } from "solid-js";
import { useAuth } from "../context";
import { Button, ErrorNotice } from "./Controls";
import { bytes } from "../data";
import { excludedSource, sourceArchive } from "../local-source";

export default function LocalSource(props: {applicationId?:string;value?:string;change:(id:string)=>void}) {
  const auth=useAuth(),[files,setFiles]=createSignal<{name:string;file:File}[]>([]),[error,setError]=createSignal(""),[busy,setBusy]=createSignal(false);
  let picker!:HTMLInputElement;
  async function upload(){setBusy(true);setError("");try{const body=sourceArchive(files());const res=await fetch("/api/sources?application="+encodeURIComponent(props.applicationId||""),{method:"POST",credentials:"same-origin",headers:{"Content-Type":"application/x-tar","X-CSRF-Token":auth.session()?.csrfToken||""},body});const data=await res.json();if(!res.ok)throw new Error(data.error?.message||"Source upload failed");props.change(data.id);setFiles([]);}catch(e){setError((e as Error).message)}finally{setBusy(false)}}
  return <div class="local-source-input"><input ref={element=>{picker=element;picker.setAttribute("webkitdirectory","");}} type="file" multiple hidden aria-label="Project folder" onChange={e=>{const selected=[...(e.currentTarget.files??[])].map(file=>({file,name:file.webkitRelativePath.split("/").slice(1).join("/")||file.name})).filter(v=>!excludedSource(v.name));setFiles(selected);setError("");}}/>
    <Button variant="ghost" disabled={busy()} onClick={()=>picker.click()}>Choose folder</Button>
    <Show when={props.value}><span class="mono">{props.value!.slice(0,12)}</span></Show>
    <Show when={files().length}><details class="configuration" open><summary>{files().length} {files().length===1?"file":"files"} · {bytes(files().reduce((n,v)=>n+v.file.size,0))}</summary><div class="source-file-list mono">{files().map(v=>v.name).join("\n")}</div><Button disabled={busy()} onClick={()=>void upload()}>{busy()?"Uploading":"Use these files"}</Button></details></Show>
    <Show when={error()}><ErrorNotice message={error()}/></Show>
  </div>;
}
