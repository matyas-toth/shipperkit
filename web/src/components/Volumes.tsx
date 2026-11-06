import { Index, createUniqueId } from "solid-js";
import type { PersistentVolume } from "../types";
import { Button, IconButton } from "./Controls";

export default function Volumes(props: { values: PersistentVolume[]; change: (values: PersistentVolume[]) => void }) {
  const id = createUniqueId();
  const update = (index: number, patch: Partial<PersistentVolume>) =>
    props.change(props.values.map((volume, i) => i === index ? { ...volume, ...patch } : volume));
  return <section class="volume-settings">
    <div class="section-toolbar">
      <h2>Persistent volumes</h2>
      <Button icon="plus" variant="ghost" onClick={() => props.change([...props.values, { name: "", path: "" }])}>Add volume</Button>
    </div>
    <Index each={props.values}>{(volume, index) => <div class="volume-row">
      <div class="field"><label for={id + "-name-" + index}>Volume name</label><input id={id + "-name-" + index} class="input" aria-label={"Volume name " + (index + 1)} placeholder="data" value={volume().name} required onInput={e => update(index, {name: e.currentTarget.value})} /></div>
      <div class="field"><label for={id + "-path-" + index}>Container path</label><input id={id + "-path-" + index} class="input mono" aria-label={"Mount path " + (index + 1)} placeholder="/app/data" value={volume().path} required onInput={e => update(index, {path: e.currentTarget.value})} /></div>
      <IconButton icon="trash" label={"Remove volume " + (index + 1)} ghost onClick={() => props.change(props.values.filter((_, i) => i !== index))} />
    </div>}</Index>
  </section>;
}
