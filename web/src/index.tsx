import { render } from "solid-js/web";
import { ErrorBoundary, Show, lazy, onCleanup, onMount } from "solid-js";
import { Router, Route } from "@solidjs/router";
import { QueryClient, QueryClientProvider, createQuery } from "@tanstack/solid-query";
import { api, post, APIError } from "./api";
import { AuthContext } from "./context";
import type { Session } from "./types";
import AuthPage from "./pages/Auth";
import FactorGate from "./pages/FactorGate";
import Shell from "./components/Shell";
import Applications from "./pages/Applications";
import Settings from "./pages/Settings";
import System from "./pages/System";
import Application from "./pages/Application";
import { Button, ErrorNotice } from "./components/Controls";
import {clearVolatileDrafts} from "./drafts";
import "./styles.css";

const NewApplication = lazy(() => import("./pages/NewApplication"));

const queryClient = new QueryClient({
  defaultOptions: { queries: { staleTime: 30_000, retry: (count, error) => !(error instanceof APIError && error.status < 500) && count < 2, refetchOnWindowFocus: false } },
});
function App() {
  const session = createQuery(() => ({ queryKey: ["session"], queryFn: () => api<Session>("/auth/session"), retry: 1 }));
  function expire() { queryClient.setQueryData<Session>(["session"], { setupRequired: false, user: null });clearVolatileDrafts(); }
  const refresh = () => void session.refetch();
  function accept(value: Session) {
    queryClient.setQueryData(["session"], value);
    if (value.authStep) {
      clearVolatileDrafts();
      queryClient.removeQueries({ predicate: query => query.queryKey[0] !== "session" });
    }
  }
  onMount(() => {
    window.addEventListener("shipper:session-expired", expire);
    window.addEventListener("shipper:session-changed", refresh);
  });
  onCleanup(() => {
    window.removeEventListener("shipper:session-expired", expire);
    window.removeEventListener("shipper:session-changed", refresh);
  });
  async function logout() {
    await post("/auth/logout", {}, session.data?.csrfToken);
    expire();
    queryClient.removeQueries({ predicate: query => query.queryKey[0] !== "session" });
  }
  return <AuthContext.Provider value={{ session: () => session.data, logout, accept }}>
    <Show when={!session.isPending} fallback={<div class="boot-screen" role="status" aria-label="Loading Shipper"><span class="loading-line" /></div>}>
      <Show when={!session.error || session.data?.user} fallback={<div class="boot-screen"><ErrorNotice message={session.error?.message ?? "Shipper could not be reached"} retry={() => void session.refetch()} /></div>}>
        <Show when={session.data?.user} fallback={<AuthPage setup={session.data?.setupRequired ?? false} onAuthenticated={accept} refreshSession={() => session.refetch()} />}>
          <Show when={!session.data?.authStep && !session.data?.recoveryCodes?.length} fallback={<FactorGate />}>
            <Router root={Shell}><Route path="/" component={Applications} /><Route path="/settings" component={Settings} /><Route path="/system" component={System}/>
              <Route path="/new" component={NewApplication} /><Route path="/applications/:id" component={Application} />
              <Route path="*404" component={() => <div class="content content-narrow"><h1>Page not found</h1><Button onClick={() => window.location.assign("/")}>Applications</Button></div>} />
            </Router>
          </Show>
        </Show>
      </Show>
    </Show>
  </AuthContext.Provider>;
}
render(() => <ErrorBoundary fallback={(error, reset) => <div class="boot-screen"><ErrorNotice message="Shipper could not display this page" retry={reset} /></div>}>
  <QueryClientProvider client={queryClient}><App /></QueryClientProvider>
</ErrorBoundary>, document.getElementById("app")!);

