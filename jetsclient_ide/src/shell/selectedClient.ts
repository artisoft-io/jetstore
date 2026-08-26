/**
 * The client filter every screen's tables are queried against. Task C.6.
 *
 * ## It is the shell's, not a screen's
 *
 * In Flutter it is a `DropdownButtonFormField` in the **left menu of every
 * screen** (`jetsclient/lib/screens/base_screen.dart`, the *Filter Client*
 * dropdown), writing to `JetsRouterDelegate().selectedClient`. Nothing about it
 * belongs to the screen underneath: `_makeQuery` reads the router
 * (`jetsclient/lib/components/data_table_source.dart`, `_makeQuery`) and adds
 * `client = <selection>` to any table whose columns include one named `client`.
 *
 * So this is `actions/homeFilters.ts`'s shape exactly, and for the same stated
 * reason — **there is one build, so there is one selection**, and a per-screen
 * copy would make *is this table filtered?* depend on who asked. C.6 is the first
 * consumer; three of its tables have the column, and C.9, C.10 and C.13 follow.
 *
 * ## What the query builder does with it, which is not what the name suggests
 *
 * `makeQuery` (`datatable/query.ts`) adds the implicit clause only when **all
 * three** of these hold: a client is selected, the table declares a column named
 * `client`, and no where clause of the table already names that column. An
 * explicit clause wins, which is why the flag starts true and is cleared as the
 * clauses are walked rather than being decided up front.
 *
 * **A null selection is a meaningful state and is the default.** With none, a
 * table with a `client` column is unfiltered — except `inputRegistryTable`, which
 * then gets `client NOT IN ('Any')` instead. That branch is in the query builder
 * already; nothing here needs to know about it.
 *
 * ## Where the list comes from, and what happens when it does not arrive
 *
 * One `raw_query` against `jetsapi.client_registry`, which is what the Dart runs
 * at sign-in (`jetsclient/lib/modules/actions/user_delegates.dart`, the
 * `client_registry` query in the login delegate). This app has no sign-in
 * bootstrap (I-67), so the shell asks once when it mounts.
 *
 * **A failure leaves the picker empty and every table unfiltered**, which is the
 * same state a deployment with no registered clients is in, and is deliberately
 * not a banner: the selection is a *narrowing* of what a user may already see, so
 * losing it shows more rows rather than fewer, and no query the app makes depends
 * on it. The failure that would matter — a selection silently not reaching a
 * query — is a different one, and it is what `Home.test.tsx` mutation-tests.
 */

/** The registry query, verbatim from the Dart's login delegate. */
export const CLIENT_LIST_QUERY =
  "SELECT client FROM jetsapi.client_registry ORDER BY client ASC LIMIT 200";

interface ClientStore {
  selected: string | null;
  clients: string[];
  listeners: Set<() => void>;
}

const store: ClientStore = { selected: null, clients: [], listeners: new Set() };

/** The current selection, for a `QueryContext`. Null means no client filter. */
export const selectedClient = (): string | null => store.selected;

/** The clients the picker offers. Empty until the registry query returns. */
export const clientList = (): readonly string[] => store.clients;

export function setSelectedClient(client: string | null): void {
  store.selected = client === "" ? null : client;
  for (const listener of store.listeners) listener();
}

export function setClientList(clients: readonly string[]): void {
  store.clients = [...clients];
  for (const listener of store.listeners) listener();
}

/** Subscribe to either changing; returns the unsubscribe. */
export function subscribeToClient(listener: () => void): () => void {
  store.listeners.add(listener);
  return () => {
    store.listeners.delete(listener);
  };
}

/** Empties both, as a fresh page load would. Tests, and sign-out. */
export function resetSelectedClient(): void {
  store.selected = null;
  store.clients = [];
  for (const listener of store.listeners) listener();
}
