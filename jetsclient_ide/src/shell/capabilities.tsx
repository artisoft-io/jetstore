/**
 * The capability model: `role_capability` resolved at login, applied to the UI.
 *
 * Task A.2. The capabilities arrive with the login response
 * (`jets/apiserver/api_users.go`), resolved from `jetsapi.role_capability`, and
 * `ApiClient` already holds them. What was missing was one place that decides
 * what a missing capability *does* to a control, and one control that obeys it.
 *
 * ## This is presentation, not enforcement
 *
 * Stated at the top because it is the thing most easily forgotten as this
 * becomes convenient to use. The server is the enforcement point, and the
 * assessment's §3.5 records that client-side checks here are cosmetic. Worse,
 * `/dataTable` reads are not capability-gated at all server-side (I-2) — so a
 * disabled button is a courtesy to an honest user, and nothing else. **Never add
 * a check here in place of one on the server.**
 *
 * ## Disabled, not hidden — and A.1 got this wrong
 *
 * All three places the Flutter app gates on a capability *disable* the control
 * and leave it visible:
 *
 *  - menu entries — `onPressed: doIt ? … : null` (`screens/base_screen.dart:155`)
 *  - data-table actions — same shape (`components/data_table.dart:27`)
 *  - form buttons — via `isOn` (`components/form_button.dart:73`)
 *
 * A.1's shell filtered nav items out of the list instead. That was a divergence
 * introduced without noticing, and this task reverses it: a control the user
 * cannot use stays visible and inert, which tells them the screen exists and
 * that access is the thing they lack. Hiding it makes the application look
 * different to different people with no way to tell why.
 *
 * The one improvement on the original: the disabled control carries a `title`
 * naming the capability. The Flutter app disables silently, which leaves the
 * user to guess.
 */

import { createContext, useContext, type ReactNode } from "react";

import type { ApiClient } from "../api/client";

export interface Permission {
  allowed: boolean;
  /** Why not, for a tooltip. Absent when allowed. */
  reason?: string;
}

const ALLOWED: Permission = { allowed: true };

/**
 * Whether a control gated on `capability` may be used.
 *
 * `undefined` means the control requires no capability, which is how the Flutter
 * configurations express it — `capability` is nullable there and 33 of the 42
 * gated sites simply omit it.
 *
 * An **empty string** is a configuration error rather than an absent
 * requirement, and is denied. That matches the server, which refuses outright:
 * *"unauthorized, configuration error: missing capability on sql statement"*
 * (`jets/datatable/data_table_action.go:433`). It diverges from the Flutter app,
 * which would allow an admin through because its callers test `isAdmin` before
 * consulting the capability at all — a difference no configuration in the corpus
 * exercises, since every one of the 42 sites names a real capability.
 */
export function permissionFor(api: ApiClient, capability?: string): Permission {
  if (capability === undefined || capability === null) return ALLOWED;
  if (capability === "") {
    return { allowed: false, reason: "Misconfigured: no capability named" };
  }
  if (api.can(capability)) return ALLOWED;
  return { allowed: false, reason: `Requires the ${capability} capability` };
}

const ApiContext = createContext<ApiClient | null>(null);

export function ApiProvider({ api, children }: { api: ApiClient; children: ReactNode }) {
  return <ApiContext.Provider value={api}>{children}</ApiContext.Provider>;
}

export function useApi(): ApiClient {
  const api = useContext(ApiContext);
  if (!api) throw new Error("useApi must be used inside the app shell");
  return api;
}

/** The permission for a control, re-read on every render so a re-login lands. */
export function useCan(capability?: string): Permission {
  return permissionFor(useApi(), capability);
}

export interface ActionButtonProps
  extends Omit<React.ButtonHTMLAttributes<HTMLButtonElement>, "disabled"> {
  capability?: string;
  /** Reasons of the caller's own — a pristine form, a request in flight. */
  disabled?: boolean;
  children: ReactNode;
}

/**
 * A button that obeys the capability model.
 *
 * Disabled reasons compose: the caller's `disabled` and a missing capability
 * both suppress the action, and the capability is the one that explains itself,
 * because it is the one the user can do something about.
 */
export function ActionButton({
  capability,
  disabled = false,
  children,
  className,
  ...rest
}: ActionButtonProps) {
  const permission = useCan(capability);
  const blocked = disabled || !permission.allowed;
  return (
    <button
      type="button"
      className={className ?? "btn"}
      disabled={blocked}
      {...(permission.reason ? { title: permission.reason } : {})}
      {...rest}
    >
      {children}
    </button>
  );
}
