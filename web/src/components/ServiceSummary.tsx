import { For, Show, type JSX } from "solid-js";
import type { Service } from "../types";
import { Status } from "./Controls";
import { Icon } from "./Icon";

export default function ServiceSummary(props: {
  services: Service[];
  status?: (service: Service) => string;
  open?: (name: string) => void;
  routes?: (service: Service) => JSX.Element;
}) {
  return <div class="service-grid" role="list">
    <For each={props.services}>{service => {
      const facts = () => <>
        <span class="service-card-heading"><Icon name="box" /><strong>{service.name}</strong>
          <Show when={props.status}><Status value={props.status!(service)} /></Show>
          <Show when={props.open}><Icon name="chevron" /></Show>
        </span>
        <span class="service-card-image" title={service.image}>{service.image || "Build from source"}</span>
        <span class="service-card-facts">
          <span>{service.oneShot ? "Job" : service.port ? "Port " + service.port : "Internal"}</span>
          <Show when={service.volumes?.length}><span>{service.volumes.length} {service.volumes.length === 1 ? "volume" : "volumes"}</span></Show>
        </span>
      </>;
      return <div role="listitem" class="service-card">
        <Show when={props.open} fallback={<div class="service-card-body">{facts()}</div>}>
          <button type="button" class="service-card-body" aria-label={"Open service " + service.name} onClick={() => props.open!(service.name)}>{facts()}</button>
        </Show>
        {props.routes?.(service)}
      </div>;
    }}</For>
  </div>;
}
