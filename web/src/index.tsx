import { render } from "solid-js/web";
import { ErrorBoundary, Show, onCleanup, onMount } from "solid-js";
import { Router, Route } from "@solidjs/router";
import { QueryClient, QueryClientProvider, createQuery } from "@tanstack/solid-query";
import { api, post, APIError } from "./api";
import { AuthContext } from "./context";
import type { Session } from "./types";
import AuthPage from "./pages/Auth";
import Shell from "./components/Shell";
import Applications from "./pages/Applications";
import Settings from "./pages/Settings";
import NewApplication from "./pages/NewApplication";
import Application from "./pages/Application";
import { Button, ErrorNotice } from "./components/Controls";
import "./styles.css";

const queryClient = new QueryClient({
  defaultOptions: { queries: { staleTime: 30_000, retry: (count, error) => !(error instanceof APIError && error.status < 500) && count < 2, refetchOnWindowFocus: false } },
});
function App() {
  const session = createQuery(() => ({ queryKey: ["session"], queryFn: () => api<Session>("/auth/session"), retry: 1 }));
  function expire() { queryClient.setQueryData<Session>(["session"], { setupRequired: false, user: null }); }
  onMount(() => { window.addEventListener("shipper:session-expired", expire); });
  onCleanup(() => window.removeEventListener("shipper:session-expired", expire));
  async function logout() {
    await post("/auth/logout", {}, session.data?.csrfToken);
    expire();
    queryClient.removeQueries({ predicate: query => query.queryKey[0] !== "session" });
  }
  return <AuthContext.Provider value={{ session: () => session.data, logout }}>
    <Show when={!session.isPending} fallback={<div class="boot-screen" role="status" aria-label="Loading Shipper"><span class="loading-line" /></div>}>
      <Show when={!session.error} fallback={<div class="boot-screen"><ErrorNotice message={session.error?.message ?? "Shipper could not be reached"} retry={() => void session.refetch()} /></div>}>
        <Show when={session.data?.user} fallback={<AuthPage setup={session.data?.setupRequired ?? false} onAuthenticated={value => queryClient.setQueryData(["session"], value)} refreshSession={() => session.refetch()} />}>
          <Router root={Shell}><Route path="/" component={Applications} /><Route path="/settings" component={Settings} />
            <Route path="/new" component={NewApplication} /><Route path="/applications/:id" component={Application} />
            <Route path="*404" component={() => <div class="content content-narrow"><h1>Page not found</h1><Button onClick={() => window.location.assign("/")}>Applications</Button></div>} />
          </Router>
        </Show>
      </Show>
    </Show>
  </AuthContext.Provider>;
}
render(() => <ErrorBoundary fallback={(error, reset) => <div class="boot-screen"><ErrorNotice message="Shipper could not display this page" retry={reset} /></div>}>
  <QueryClientProvider client={queryClient}><App /></QueryClientProvider>
</ErrorBoundary>, document.getElementById("app")!);

