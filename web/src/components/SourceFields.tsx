import { createQuery } from "@tanstack/solid-query";
import { For, Show, createEffect, createSignal } from "solid-js";
import { api, post } from "../api";
import { useAuth } from "../context";
import type { Connection, StaticConfig } from "../types";
import { Button, ErrorNotice } from "./Controls";
import { Icon } from "./Icon";
export function useConnections() {
  return createQuery(() => ({
    queryKey: ["connections"],
    queryFn: () => api<{ items: Connection[] }>("/connections"),
  }));
}
export function GitConnectionFields(props: {
  config: StaticConfig;
  change: (key: keyof StaticConfig, value: unknown) => void;
}) {
  const connections = useConnections();
  return (
    <>
      <Show when={connections.data?.items.some(c => c.kind !== "registry")}
        fallback={<a class="button neutral small source-connect" href="/settings#sources"><Icon name="git" />Connect GitHub</a>}>
        <div class="field git-field"><label for="source-connection">Connection</label>
          <select id="source-connection" class="input" value={props.config.sourceId ?? ""} onChange={e => {
            props.change("repository", "");
            props.change("repositoryId", 0);
            props.change("branch", "");
            props.change("sourceId", e.currentTarget.value);
          }}>
            <option value="">Public Git repository</option>
            <For each={connections.data?.items.filter(c => c.kind !== "registry")}>{c => <option value={c.id}>{c.name}</option>}</For>
          </select>
        </div>
      </Show>
      <Show when={props.config.sourceId} keyed>{sourceId => <RepositoryPicker sourceId={sourceId} config={props.config} change={props.change} />}</Show>
    </>
  );
}

function RepositoryPicker(props: {
  sourceId: string;
  config: StaticConfig;
  change: (key: keyof StaticConfig, value: unknown) => void;
}) {
  const [page, setPage] = createSignal(1),
    [rows, setRows] = createSignal<
      {
        id: number;
        full_name: string;
        clone_url: string;
        default_branch: string;
      }[]
    >([]),
    [search, setSearch] = createSignal("");
  const repos = createQuery(() => ({
    queryKey: ["source-repos", props.sourceId, page()],
    queryFn: () =>
      api<{ items: ReturnType<typeof rows>; nextPage: number }>(
        `/connections/${props.sourceId}/repositories?page=${page()}`,
      ),
  }));
  createEffect(() => {
    const result = repos.data;
    if (result)
      setRows((old) => [
        ...new Map([...old, ...result.items].map((r) => [r.id, r])).values(),
      ]);
  });
  const firstLoad = () => repos.isPending && rows().length === 0;
  return (
      <div class="repository-picker field git-field" aria-busy={firstLoad()}>
        <Show when={rows().length > 20}>
          <input
            class="input"
            aria-label="Search repositories"
            placeholder="Search repositories"
            value={search()}
            onInput={(e) => setSearch(e.currentTarget.value)}
          />
        </Show>
        <label for="private-repository">Repository</label>
        <select
          id="private-repository"
          class="input"
          required
          disabled={rows().length === 0}
          aria-busy={firstLoad()}
          value={props.config.repository}
          onChange={(e) => {
            const repo = rows().find(
              (r) => r.clone_url === e.currentTarget.value,
            );
            if (repo) {
              props.change("repository", repo.clone_url);
              props.change("repositoryId", repo.id);
              props.change("branch", repo.default_branch);
            }
          }}
        >
          <option value="">
            {firstLoad() ? "Loading repositories" : repos.isError && !rows().length ? "Repositories unavailable" : !rows().length ? "No repositories available" : "Choose repository"}
          </option>
          <Show
            when={
              props.config.repository &&
              !rows().some((r) => r.clone_url === props.config.repository)
            }
          >
            <option value={props.config.repository}>
              {props.config.repository
                .replace("https://github.com/", "")
                .replace(/\.git$/, "")}
            </option>
          </Show>
          <For
            each={rows().filter((r) =>
              r.full_name.toLowerCase().includes(search().toLowerCase()),
            )}
          >
            {(r) => <option value={r.clone_url}>{r.full_name}</option>}
          </For>
        </select>
        <Show when={repos.data?.nextPage}>
          <Button
            variant="ghost"
            disabled={repos.isFetching}
            onClick={() => setPage(repos.data!.nextPage)}
          >
            Load more repositories
          </Button>
        </Show>
        <Show when={repos.error}>
          <ErrorNotice message={repos.error!.message} retry={() => void repos.refetch()} />
        </Show>
      </div>
  );
}
export function ImageFields(props: {
  config: StaticConfig;
  change: (key: keyof StaticConfig, value: unknown) => void;
}) {
  const connections = useConnections();
  return (
    <>
      <label for="image-reference">Image</label>
      <input
        id="image-reference"
        class="input mono"
        required
        value={props.config.image ?? ""}
        placeholder="ghcr.io/owner/app:latest"
        onInput={(e) => props.change("image", e.currentTarget.value)}
      />
      <Show when={connections.data?.items.some((c) => c.kind === "registry")}>
        <label for="registry-connection">Registry</label>
        <select
          id="registry-connection"
          class="input"
          value={props.config.registryId ?? ""}
          onChange={(e) => props.change("registryId", e.currentTarget.value)}
        >
          <option value="">Public registry</option>
          <For
            each={connections.data?.items.filter((c) => c.kind === "registry")}
          >
            {(c) => <option value={c.id}>{c.name}</option>}
          </For>
        </select>
      </Show>
      <label for="image-platform">Platform</label>
      <select
        id="image-platform"
        class="input"
        value={props.config.platform ?? ""}
        onChange={(e) => props.change("platform", e.currentTarget.value)}
      >
        <option value="">Server default</option>
        <option value="linux/amd64">Linux amd64</option>
        <option value="linux/arm64">Linux arm64</option>
      </select>
    </>
  );
}
export function InlineFields(props: {
  config: StaticConfig;
  applicationId?: string;
  change: (key: keyof StaticConfig, value: unknown) => void;
}) {
  const auth = useAuth(),
    [error, setError] = createSignal("");
  async function edit() {
    try {
      const result = await post<{ content: string }>(
        `/applications/${props.applicationId}/inline/source`,
        {},
        auth.session()?.csrfToken,
      );
      props.change("inlineDockerfile", result.content);
      props.change("inlineKeep", false);
    } catch (e) {
      setError((e as Error).message);
    }
  }
  return (
    <>
      <Show when={error()}>
        <ErrorNotice message={error()} />
      </Show>
      <Show
        when={!props.config.inlineKeep || !!props.config.inlineDockerfile}
        fallback={<Button onClick={() => void edit()}>Edit Dockerfile</Button>}
      >
        <label for="inline-dockerfile">Dockerfile</label>
        <textarea
          id="inline-dockerfile"
          class="input mono compose-editor"
          required
          value={props.config.inlineDockerfile ?? ""}
          onInput={(e) => {
            props.change("inlineDockerfile", e.currentTarget.value);
            props.change("inlineKeep", false);
          }}
        />
      </Show>
    </>
  );
}
