import { For, Show, createSignal, createUniqueId } from "solid-js";
import type { Command, Hook, StaticConfig } from "../types";
import { Icon } from "./Icon";
export const emptyCommand = (): Command => ({ shell: "", service: "", directory: "", user: "", timeout: 300, mode: "job" });
export function CommandFields(props: { value: Command; change: (c: Command) => void; label?: string; services?: string[]; execution?: boolean }) {
 const id = createUniqueId();
 const update = (next: Partial<Command>) => props.change({ ...props.value, ...next });
 return <div class="command-fields form-fields">
 <Show when={props.services?.length}><label for={id+"service"}>Service</label><select id={id+"service"} class="input" value={props.value.service} onChange={e=>update({service:e.currentTarget.value})}><option value="" disabled>Choose service</option><For each={props.services}>{s=><option value={s}>{s}</option>}</For></select></Show>
 <label for={id+"command"}>{props.label ?? "Command"}</label><Show when={!props.value.argv} fallback={<textarea id={id+"command"} class="input mono command-input" value={JSON.stringify(props.value.argv)} onChange={e=>{try{const argv=JSON.parse(e.currentTarget.value);if(Array.isArray(argv)&&argv.every(v=>typeof v==="string"))update({argv});}catch{e.currentTarget.setCustomValidity("Enter a JSON array of arguments");}}} onInput={e=>e.currentTarget.setCustomValidity("")} />}><textarea id={id+"command"} class="input mono command-input" value={props.value.shell} placeholder="None" onInput={e=>update({shell:e.currentTarget.value})} /></Show>
 <details class="configuration command-options"><summary>Options<Icon name="down" /></summary><div class="form-fields configuration-fields">
 <label for={id+"format"}>Command format</label><select id={id+"format"} class="input" value={props.value.argv ? "argv" : "shell"} onChange={e=>update(e.currentTarget.value==="argv"?{shell:"",argv:[]}:{shell:"",argv:undefined})}><option value="shell">Shell</option><option value="argv">Argument list</option></select>
 <Show when={props.execution}><label for={id+"execution"}>Run in</label><select id={id+"execution"} class="input" value={props.value.mode} onChange={e=>update({mode:e.currentTarget.value as Command["mode"]})}><option value="job">Fresh job container</option><option value="exec">Running service</option></select></Show>
 <label for={id+"cwd"}>Working directory</label><input id={id+"cwd"} class="input mono" placeholder="Image default" value={props.value.directory} onInput={e=>update({directory:e.currentTarget.value})} />
 <label for={id+"user"}>User</label><input id={id+"user"} class="input" placeholder="Image default" value={props.value.user} onInput={e=>update({user:e.currentTarget.value})} />
 <label for={id+"timeout"}>Timeout (seconds)</label><input id={id+"timeout"} class="input" type="number" min="1" max="3600" value={props.value.timeout} onInput={e=>update({timeout:Number(e.currentTarget.value)})} />
 </div></details></div>;
}
export function LifecycleCommands(props: { config: StaticConfig; change: (hooks: Hook[]) => void; services?: string[]; compact?: boolean }) {
 const [service,setService]=createSignal(props.services?.[0] ?? props.config.hooks?.[0]?.command.service ?? "");
 const stages = [{stage:"before_deploy",label:"Before deploy"},{stage:"before_start",label:"Before start"},{stage:"after_deploy",label:"After deploy"},{stage:"before_stop",label:"Before stop"}] as const;
 const value=(stage:Hook["stage"]) => props.config.hooks?.find(h=>h.stage===stage && h.command.service===service())?.command ?? {...emptyCommand(),service:service(),mode:stage==="before_stop"?"exec" as const:"job" as const};
 function update(stage:Hook["stage"],command:Command){const hooks=(props.config.hooks??[]).filter(h=>h.stage!==stage||h.command.service!==service());if(command.shell||command.argv)hooks.push({stage,command:{...command,service:service()}});props.change(hooks);}
 return <section class="lifecycle-settings"><Show when={!props.compact}><h2>Deployment commands</h2></Show><Show when={props.config.method==="compose"}><div class="form-fields"><label for="hook-service">Service</label><input id="hook-service" class="input" list="hook-services" value={service()} onInput={e=>setService(e.currentTarget.value)} /><datalist id="hook-services"><For each={props.services}>{s=><option value={s} />}</For></datalist></div></Show>
 <CommandFields value={value("before_deploy")} label="Before deploy" change={c=>update("before_deploy",c)} />
 <details class="configuration"><summary>More commands<Icon name="down" /></summary><div class="advanced-hooks"><For each={stages.slice(1)}>{h=><CommandFields value={value(h.stage)} label={h.label} execution={h.stage!=="before_start"} change={c=>update(h.stage,c)} />}</For></div></details>
 </section>;
}
