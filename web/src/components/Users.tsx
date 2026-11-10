import { For, Show, createMemo, createSignal } from "solid-js";
import { Portal } from "solid-js/web";
import { createQuery, useQueryClient } from "@tanstack/solid-query";
import { api, post } from "../api";
import { useAuth } from "../context";
import type { AuditEvent, Role, User } from "../types";
import { ActionMenu, Button, ErrorNotice, IconButton, Status } from "./Controls";
import { Icon, type IconName } from "./Icon";
import { createdDate, operationTime } from "../data";

const roles: { value: Role; label: string }[] = [
  { value: "admin", label: "Admin" },
  { value: "developer", label: "Developer" },
  { value: "viewer", label: "Viewer" },
];
const providers: { id: string; label: string; icon: IconName }[] = [
  { id: "google", label: "Google", icon: "google" },
  { id: "github", label: "GitHub", icon: "github" },
  { id: "discord", label: "Discord", icon: "discord" },
];
const actionLabels: Record<string, string> = {
  account_created: "Account created",
  user_provisioned: "User added",
  role_changed: "Role changed",
  user_disabled: "Disabled",
  user_enabled: "Enabled",
  user_removed: "Removed",
  password_reset: "Password reset",
  password_removed: "Password removed",
  password_changed: "Password changed",
  temporary_password_replaced: "Password set",
  factor_reset: "Two-factor reset",
  factor_enabled: "Two-factor enabled",
  factor_disabled: "Two-factor disabled",
  recovery_codes_replaced: "Recovery codes replaced",
  recovery_code_used: "Recovery code used",
  identity_linked: "Sign-in linked",
  identity_unlinked: "Sign-in unlinked",
  signed_in: "Signed in",
  provider_tested: "Provider tested",
  offline_factor_recovery: "Factor recovered offline",
  offline_password_reset: "Password reset offline",
};
const roleLabels: Record<string, string> = { admin: "Admin", developer: "Developer", viewer: "Viewer" };
function auditDetail(event: AuditEvent) {
  const detail = event.detail ?? "";
  switch (event.action) {
    case "user_provisioned":
    case "password_reset": {
      const [role, method] = detail.split("/");
      const roleLabel = roleLabels[role] ?? role;
      const methodLabel = method === "provider_only" ? "Provider sign-in" : "Temporary password";
      return event.action === "user_provisioned" ? roleLabel + ", " + methodLabel : methodLabel;
    }
    case "role_changed": {
      const [before, after] = detail.split(" to ");
      return (roleLabels[before] ?? before) + " to " + (roleLabels[after] ?? after);
    }
    case "signed_in":
      return "with " + (providers.find(provider => provider.id === detail)?.label ?? detail);
    case "identity_linked":
    case "identity_unlinked":
    case "provider_tested":
      return providers.find(provider => provider.id === detail)?.label ?? detail;
    default:
      return detail;
  }
}
type ConfirmAction = "disable" | "enable" | "delete" | "factor";
const confirmCopy: Record<ConfirmAction, { title: string; action: string }> = {
  disable: { title: "Disable account", action: "Disable" },
  enable: { title: "Enable account", action: "Enable" },
  delete: { title: "Remove user", action: "Remove" },
  factor: { title: "Reset two-factor", action: "Reset" },
};

function generatePassword() {
  const bytes = crypto.getRandomValues(new Uint8Array(18));
  return btoa(String.fromCharCode(...bytes)).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

export default function Users() {
  const client = useQueryClient();
  const auth = useAuth();
  const [error, setError] = createSignal("");
  const [busy, setBusy] = createSignal(false);
  const [creating, setCreating] = createSignal(false);
  const [email, setEmail] = createSignal("");
  const [role, setRole] = createSignal<Role>("developer");
  const [mode, setMode] = createSignal<"temporary" | "provider">("temporary");
  const [password, setPassword] = createSignal(generatePassword());
  const [created, setCreated] = createSignal<User>();
  const [resetting, setResetting] = createSignal<User>();
  const [resetPassword, setResetPassword] = createSignal(generatePassword());
  const [resetDone, setResetDone] = createSignal(false);
  const [activity, setActivity] = createSignal<User>();
  const [confirming, setConfirming] = createSignal<{ action: ConfirmAction; user: User }>();
  const query = createQuery(() => ({ queryKey: ["users"], queryFn: () => api<{ items: User[] }>("/users") }));
  const audit = createQuery(() => ({
    queryKey: ["users", activity()?.id, "audit"],
    queryFn: () => api<{ items: AuditEvent[] }>("/users/" + activity()!.id + "/audit"),
    enabled: !!activity(),
  }));
  const items = createMemo(() => query.data?.items ?? []);

  function openCreate() {
    setEmail(""); setRole("developer"); setMode("temporary"); setPassword(generatePassword());
    setCreated(undefined); setError(""); setCreating(true);
  }
  function closeCreate() {
    setCreated(undefined); setPassword(""); setCreating(false); setError("");
  }
  async function createUser(event: SubmitEvent) {
    event.preventDefault();
    if (busy()) return;
    setBusy(true); setError("");
    try {
      const user = await post<User>("/users", { email: email(), role: role(), password: mode() === "temporary" ? password() : "" });
      setCreated(user);
      await client.invalidateQueries({ queryKey: ["users"] });
    } catch (e) { setError((e as Error).message); }
    finally { setBusy(false); }
  }
  async function updateRole(user: User, value: string) {
    setError("");
    try {
      await post("/users/" + user.id + "/role", { role: value });
      await client.invalidateQueries({ queryKey: ["users"] });
      if (user.id === auth.session()?.user?.id) await client.invalidateQueries({ queryKey: ["session"] });
    } catch (e) { setError((e as Error).message); await query.refetch(); }
  }
  async function run(action: string, user: User) {
    setBusy(true); setError("");
    try {
      await post("/users/" + user.id + "/" + action, {});
      const self = user.id === auth.session()?.user?.id;
      setConfirming(undefined);
      await client.invalidateQueries({ queryKey: ["users"] });
      if (self && (action === "disable" || action === "delete")) {
        try { await auth.logout(); } catch { /* the disabled session is already revoked */ }
        window.location.assign("/");
      }
    } catch (e) { setError((e as Error).message); }
    finally { setBusy(false); }
  }
  function openReset(user: User) {
    setResetting(user); setResetPassword(generatePassword()); setResetDone(false); setError("");
  }
  async function resetPasswordFor(event: SubmitEvent) {
    event.preventDefault();
    if (busy() || !resetting()) return;
    setBusy(true); setError("");
    try {
      await post("/users/" + resetting()!.id + "/password", { password: resetPassword() });
      setResetDone(true);
      await client.invalidateQueries({ queryKey: ["users"] });
    } catch (e) { setError((e as Error).message); }
    finally { setBusy(false); }
  }
  async function copyPassword(value: string) {
    try { await navigator.clipboard.writeText(value); }
    catch { setError("The password could not be copied"); }
  }
  return <section class="settings-section" id="users">
    <div class="section-toolbar"><h2>Users</h2><Button small variant="primary" icon="plus" onClick={openCreate}>Add user</Button></div>
    <Show when={error() && !creating() && !resetting()}><ErrorNotice message={error()} /></Show>
    <div class="table-tray users-tray">
      <table class="inventory-table users-table">
        <thead><tr><th>User</th><th>Role</th><th>Sign-in</th><th>Last active</th><th aria-label="Actions" /></tr></thead>
        <tbody>
          <For each={items()}>{user => {
            const initial = user.email.slice(0, 1).toUpperCase();
            const signIns = () => user.identities?.length ?? 0;
            return <tr>
              <td><span class="user-identity"><span class="avatar user-avatar">{initial}</span><span class="user-name"><strong>{user.email}</strong>
                <Show when={user.disabled}><Status value="disabled" /></Show>
                <Show when={!user.disabled && user.passwordChangeRequired}><Status value="warning" label="Set password" /></Show>
              </span></span></td>
              <td><select class="input role-select" aria-label={"Role for " + user.email} value={user.role}
                onChange={event => void updateRole(user, event.currentTarget.value)}>
                <For each={roles}>{item => <option value={item.value}>{item.label}</option>}</For>
              </select></td>
              <td><div class="signin-chips">
                <Show when={user.hasPassword}><span class="signin-chip" title="Password"><Icon name="lock" />Password</span></Show>
                <For each={providers.filter(provider => user.identities?.some(identity => identity.provider === provider.id))}>{provider =>
                  <span class="signin-chip" title={provider.label}><Icon name={provider.icon} />{provider.label}</span>
                }</For>
                <Show when={!user.hasPassword && !signIns()}><span class="signin-chip is-empty">None</span></Show>
              </div></td>
              <td class="date-cell"><Show when={user.lastLoginAt} fallback={<span class="no-value">Never</span>}>{value => <time datetime={new Date(value() * 1000).toISOString()} title={createdDate(value())}>{operationTime(value())}</time>}</Show></td>
              <td class="row-chevron"><ActionMenu label={"Actions for " + user.email} icon="more">
                <button onClick={() => openReset(user)}>Reset password</button>
                <Show when={user.hasPassword}><button onClick={() => void run("password/remove", user)}>Remove password</button></Show>
                <button onClick={() => setActivity(user)}>View activity</button>
                <Show when={user.factorEnabled}><button onClick={() => setConfirming({ action: "factor", user })}>Reset two-factor</button></Show>
                <Show when={user.disabled} fallback={<button onClick={() => setConfirming({ action: "disable", user })}>Disable account</button>}>
                  <button onClick={() => setConfirming({ action: "enable", user })}>Enable account</button>
                </Show>
                <button class="danger" onClick={() => setConfirming({ action: "delete", user })}>Remove user</button>
              </ActionMenu></td>
            </tr>;
          }}</For>
          <Show when={!items().length}><tr class="empty-row"><td colspan="5"><Show when={!query.isPending} fallback={<span class="loading-line" />}>No users</Show></td></tr></Show>
        </tbody>
      </table>
    </div>

    <Show when={creating()}><Portal>
      <div class="drawer-scrim" onClick={closeCreate} />
      <div class="drawer user-drawer" role="dialog" aria-modal="true" aria-labelledby="add-user-title">
        <div class="drawer-toolbar"><h2 id="add-user-title">{created() ? "User created" : "Add user"}</h2>
          <div class="drawer-actions"><IconButton icon="close" label="Close" ghost onClick={closeCreate} /></div>
        </div>
        <Show when={created()} fallback={
          <form class="drawer-scroll user-form" onSubmit={createUser}>
            <div class="form-fields">
              <label for="new-user-email">Email</label>
              <input id="new-user-email" class="input" type="email" autocomplete="off" required value={email()} onInput={e => setEmail(e.currentTarget.value)} />
              <label for="new-user-role">Role</label>
              <select id="new-user-role" class="input" value={role()} onChange={e => setRole(e.currentTarget.value as Role)}>
                <For each={roles}>{item => <option value={item.value}>{item.label}</option>}</For>
              </select>
              <fieldset class="choice-grid user-access">
                <legend>Sign-in</legend>
                <label class="choice-card" classList={{ selected: mode() === "temporary" }}><input type="radio" name="access" checked={mode() === "temporary"} onChange={() => setMode("temporary")} />Temporary password</label>
                <label class="choice-card" classList={{ selected: mode() === "provider" }}><input type="radio" name="access" checked={mode() === "provider"} onChange={() => setMode("provider")} />Provider sign-in only</label>
              </fieldset>
              <Show when={mode() === "temporary"}>
                <label for="new-user-password">Temporary password</label>
                <div class="password-field">
                  <input id="new-user-password" class="input mono" type="text" autocomplete="off" required minlength={12} maxlength={72} value={password()} onInput={e => setPassword(e.currentTarget.value)} />
                  <span class="password-actions">
                    <IconButton icon="refresh" label="Generate password" ghost onClick={() => setPassword(generatePassword())} />
                    <IconButton icon="copy" label="Copy password" ghost onClick={() => void copyPassword(password())} />
                  </span>
                </div>
              </Show>
            </div>
            <Show when={error()}><ErrorNotice message={error()} /></Show>
            <div class="form-actions"><Button variant="ghost" disabled={busy()} onClick={closeCreate}>Cancel</Button><Button type="submit" variant="primary" disabled={busy()}>Add user</Button></div>
          </form>
        }>{user => <div class="drawer-scroll">
          <dl class="details-list">
            <div><dt>Email</dt><dd>{user().email}</dd></div>
            <div><dt>Role</dt><dd>{roles.find(item => item.value === user().role)?.label}</dd></div>
            <Show when={mode() === "temporary"}><div><dt>Temporary password</dt><dd class="mono">{password()}</dd></div></Show>
          </dl>
          <div class="form-actions">
            <Show when={mode() === "temporary"}><Button variant="ghost" icon="copy" onClick={() => void copyPassword(password())}>Copy password</Button></Show>
            <Button variant="primary" onClick={closeCreate}>Done</Button>
          </div>
        </div>}</Show>
      </div>
    </Portal></Show>

    <Show when={resetting()}>{user => <Portal>
      <div class="drawer-scrim" onClick={() => setResetting(undefined)} />
      <div class="drawer user-drawer" role="dialog" aria-modal="true" aria-labelledby="reset-user-title">
        <div class="drawer-toolbar"><h2 id="reset-user-title">Reset password</h2>
          <div class="drawer-actions"><IconButton icon="close" label="Close" ghost onClick={() => setResetting(undefined)} /></div>
        </div>
        <form class="drawer-scroll user-form" onSubmit={resetPasswordFor}>
          <dl class="details-list"><div><dt>Email</dt><dd>{user().email}</dd></div></dl>
          <Show when={!resetDone()}>
            <div class="form-fields">
              <label for="reset-user-password">Temporary password</label>
              <div class="password-field">
                <input id="reset-user-password" class="input mono" type="text" autocomplete="off" required minlength={12} maxlength={72} value={resetPassword()} onInput={e => setResetPassword(e.currentTarget.value)} />
                <span class="password-actions">
                  <IconButton icon="refresh" label="Generate password" ghost onClick={() => setResetPassword(generatePassword())} />
                  <IconButton icon="copy" label="Copy password" ghost onClick={() => void copyPassword(resetPassword())} />
                </span>
              </div>
            </div>
            <Show when={error()}><ErrorNotice message={error()} /></Show>
            <div class="form-actions"><Button variant="ghost" disabled={busy()} onClick={() => setResetting(undefined)}>Cancel</Button><Button type="submit" variant="primary" disabled={busy()}>Reset password</Button></div>
          </Show>
          <Show when={resetDone()}><div class="form-actions">
            <Button variant="ghost" icon="copy" onClick={() => void copyPassword(resetPassword())}>Copy password</Button>
            <Button variant="primary" onClick={() => setResetting(undefined)}>Done</Button>
          </div></Show>
        </form>
      </div>
    </Portal>}</Show>

    <Show when={activity()}>{user => <Portal>
      <div class="drawer-scrim" onClick={() => setActivity(undefined)} />
      <div class="drawer user-drawer" role="dialog" aria-modal="true" aria-labelledby="user-activity-title">
        <div class="drawer-toolbar"><h2 id="user-activity-title">Activity</h2>
          <div class="drawer-actions"><IconButton icon="close" label="Close" ghost onClick={() => setActivity(undefined)} /></div>
        </div>
        <div class="drawer-scroll">
          <div class="activity-user">{user().email}</div>
          <Show when={audit.error}><ErrorNotice message={audit.error!.message} retry={() => void audit.refetch()} /></Show>
          <dl class="audit-list">
            <For each={audit.data?.items}>{event =>
              <div><dt><span>{actionLabels[event.action] ?? event.action}</span>{auditDetail(event) ? <span class="audit-detail">{auditDetail(event)}</span> : null}</dt><dd>{createdDate(event.createdAt)}</dd></div>
            }</For>
          </dl>
          <Show when={audit.data && !audit.data.items.length}><p class="no-value">No activity</p></Show>
        </div>
      </div>
    </Portal>}</Show>

    <Show when={confirming()}>{value => <Portal>
      <div class="drawer-scrim" onClick={() => setConfirming(undefined)} />
      <div class="confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="confirm-user-title">
        <header><h2 id="confirm-user-title">{confirmCopy[value().action].title}</h2></header>
        <p>{value().action === "delete" ? "Remove " + value().user.email + " and revoke every session?" : value().action === "disable" ? "Disable " + value().user.email + " and revoke every session?" : confirmCopy[value().action].title + " for " + value().user.email + "?"}</p>
        <Show when={error()}><ErrorNotice message={error()} /></Show>
        <div class="form-actions">
          <Button variant="ghost" disabled={busy()} onClick={() => setConfirming(undefined)}>Cancel</Button>
          <Button variant="primary" disabled={busy()} onClick={() => {
            const item = value();
            if (item.action === "delete") void run("delete", item.user);
            else if (item.action === "factor") void run("factor/reset", item.user);
            else void run(item.action, item.user);
          }}>{confirmCopy[value().action].action}</Button>
        </div>
      </div>
    </Portal>}</Show>
  </section>;
}
