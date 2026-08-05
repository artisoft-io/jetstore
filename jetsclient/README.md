# jetsclient

The JetStore UI: a Flutter web app served by the Go apiserver
(`jets/apiserver/`), which hosts the compiled bundle from its own static routes
and answers the app's json endpoints on the same origin.

This file describes how the app is put together and, more usefully, where it
does not behave the way you would expect. It is written for someone about to
add a screen.

## The one idea

**Screens are data, not widgets.** You almost never write a `StatefulWidget`.
You write configuration objects — a `ScreenConfig`, a `FormConfig`, maybe a
`TableConfig` — plus two delegate functions, and register them in lookup maps. A
handful of generic screen widgets interpret that configuration at runtime.

The payoff is that a new screen is a few hundred lines of declarations. The cost
is that the framework only does what it was built to do: when a screen needs
behaviour the config cannot express, you extend the framework rather than
writing a one-off widget. Every such extension so far is an opt-in field
defaulting to the previous behaviour. Keep it that way — these widgets are
shared by every screen in the app.

## Layout

```
lib/
  main.dart                     entry point; derives the server origin from the browser url
  http_client.dart              HttpClientSingleton, the only way to talk to the server
  routes/                       router delegate, route data, the route table
  models/                       the configuration classes (the "language" screens are written in)
  components/                   the generic widgets that interpret configuration
  screens/                      the screen shells
  modules/                      the actual screen definitions, grouped by feature
    *_config_impl.dart          top-level registries with fallback chains into modules
    actions/                    delegate implementations for the older screens
    user_flows/<feature>/       wizard-style flows
    workspace_ide/              the Workspace IDE screens
```

`modules/` is where you work. Everything above it is machinery.

## Routing

`JetsRouterDelegate` is a singleton `RouterDelegate` holding app-wide state that
outlives any screen: the logged-in `user`, the selected client, home-page
filters, split-view weights.

`jetsRoutesMap` in `routes/jets_routes_app.dart` maps a path to an already
constructed screen widget. Note the consequence: **the screen widgets are built
when that map is first touched, at startup**, so `getScreenConfig` and
`getFormConfig` run then too, and an unknown key throws at launch rather than on
navigation. That is a feature — a misconfigured screen fails loudly and
immediately instead of when a user happens to click.

Navigation is `JetsRouterDelegate()(JetsRouteData(path, params: ...))`.

Two things that bite:

- **The page stack is derived from path segments.** `_buildListPages` splits
  each route on `/` and, for every prefix that is itself a key in
  `jetsRoutesMap`, pushes that page. This is what makes back walk a sensible
  breadcrumb. It also means a literal path nested under a parameterized one
  (`/workspaces/foo` under `/workspaces/:workspace_name/home`) does not behave.
  Screens reached from the Workspace IDE menu use top-level paths —
  `/queryTool`, `/inferServerAdmin` — for this reason.
- **`authRequired` is a substring test**: `!(path.contains('login') ||
  path.contains('register'))` in `routes/jets_route_data.dart`. A future path
  containing either word silently becomes public.

## The screen shells

All extend `BaseScreen`, which supplies the app bar, the left menu, and a
`SplitView`.

| Widget | Use |
|---|---|
| `ScreenOne` | one data table |
| `ScreenWithForm` | one form |
| `ScreenWithMultiForms` | several forms sharing state through `peersFormState` |
| `ScreenWithTabsWithForm` | tabbed, a form per tab |
| `UserFlowScreen` | a wizard driven by a `UserFlowConfig` |

`ScreenConfig.menuEntries` and `adminMenuEntries` are separate lists — the admin
list is used when `user.isAdmin`. They are often the same list; the Workspace
IDE passes `workspaceRegistryMenuEntries` for both.

## The registries

Four top-level lookup functions each try their own map, then fall through a
chain of per-module getters:

| Function | File |
|---|---|
| `getScreenConfig` | `modules/screen_config_impl.dart` |
| `getFormConfig` | `modules/form_config_impl.dart` |
| `getTableConfig` | `modules/data_table_config_impl.dart` |
| `getUserFlowConfig` | `modules/user_flow_config_impl.dart` |

Module getters return nullable; the top-level functions throw at the end of the
chain. **Adding a module means adding a line to the chain plus an import** —
forgetting it is the most common way a new screen fails, and it fails at startup
with a clear message.

## Forms

`FormConfig` holds rows of `FormFieldConfig`, a list of `actions`, and two
delegates. The field subclasses live in `models/form_config.dart`:
`FormInputFieldConfig` (text), `FormDropdownFieldConfig`,
`FormDropdownWithSharedItemsFieldConfig`, `FormTypeaheadFieldConfig`,
`FormDataTableFieldConfig` (a whole table as a field), `TextFieldConfig` (static
label), `PaddingConfig` (spacer), and `FormActionConfig` (a button).

Each subclass implements `makeFormField`, which is how a config becomes a
widget. To add a field type you add a subclass; you do not touch the form.

### inputFields vs inputFieldsV2

Two row formats, and the choice matters more than it looks:

- `inputFields` — `List<List<FormFieldConfig>>`. **Every row gets the same
  flex**, so a row of buttons ends up as tall as a row of text areas.
- `inputFieldsV2` — `List<FormFieldRowConfig>`, each with its own `flex`
  (default 0, meaning natural height). Use this whenever rows should differ in
  height.

Two more traps:

- **More than 5 rows switches the layout to a `ListView`** (`inputFields.length
  > 5 || useListView == true`), which scrolls and drops the per-row flex
  behaviour. Adding a sixth row can silently change how a screen lays out.
- **`FormConfig.groupCount()` only counts `inputFields`.** A form built entirely
  from `inputFieldsV2` reports 0 groups, which `JetsFormState` coerces to 1.
  Fine for single-group forms; be aware if you ever want validation groups with
  V2 rows.

### The two delegates

```dart
String? validator(JetsFormState formState, int group, String key, dynamic v);
Future<String?> actions(BuildContext, GlobalKey<FormState>, JetsFormState,
                        String actionKey, {group});
```

Both switch on a key and both `print` a complaint in their `default` branch —
that is the house style for "this screen is misconfigured", and it is why the
analyzer reports `avoid_print` across the module tree.

**The validator has a side effect, and it is the important part.** Returning an
error string only paints a message under the field. What actually drives
`isFormValid()` is the explicit call:

```dart
case FSK.myField:
  if (value == null || value.isEmpty) {
    formState.markFormKeyAsInvalid(group, key);   // <- this
    return "Must be provided.";
  }
  formState.markFormKeyAsValid(group, key);       // <- and this
  return null;
```

Omit those and `enableOnlyWhenFormValid` buttons are enabled on an empty form.
The field also needs an `autovalidateMode` (`always` or `onUserInteraction`) or
the validator never runs before submit.

## JetsFormState

A `ChangeNotifier` holding, per validation group, a `Map<String, dynamic>` of
widget key to value. Values are `String?` for text and dropdowns, `List<String>`
for multi-select tables — casting is the caller's job.

- `setValue` — write, no notification
- `setValueAndNotify` — write and notify
- `getValue`
- `addCacheValue` / `getCacheValue` — non-field data, e.g. dropdown item lists
  fetched from the server
- `markFormKeyAsValid` / `markFormKeyAsInvalid` — feed `isFormValid()`
- `parentFormState` — set for dialogs opened from a screen
- `peersFormState` — sibling forms in `ScreenWithMultiForms`; this is how the
  Query Tool's input form hands a query to its result table

Because it is a `ChangeNotifier` that widgets listen to, notifying during a
build would rebuild during build. The convention for absorbing that is a
zero-duration `Future.delayed` plus a `mounted` check — see
`_JetsFormButtonState._handleStateChange`. Follow it in any new listener.

### The write-once text field

`JetsTextFormField` seeds its `TextEditingController` from the form state in
`initState` and, by default, never reads it again. **Setting a value
programmatically will not change what the user sees** unless the field opts in
with `syncWithFormState: true`. This surprises everyone once.

Other opt-in flags on `FormInputFieldConfig`: `isReadOnlyEval` (read-only as a
function of state), `showCopyToClipboard` (copy button over the top right
corner, for values too long to read in place).

## Buttons

`FormActionConfig` is used two ways, and the class doc says so: as an entry in
`actions` (rendered in the form's action row, bottom right) or as a field inside
a row (rendered inline). `JetsFormButton.expand` distinguishes them — true for
the action row, false from `makeFormField`, because `JetsForm` already wraps
every field in a `Flexible`, and two `ParentDataWidget`s over one render object
throws.

Enablement is the AND of:

1. `capability` — null, or held by the user, or the user is admin
2. `enableOnlyWhenFormValid` / `enableOnlyWhenFormNotValid` — note this reads
   **form-wide** validity
3. `isEnabledEval(formState)` — the way to gate one button on one value

`buttonStyle` is stored in the form state under the button's own key, so a
delegate can restyle a button at runtime.

`FormConfig.onLoadActionKey`, when set, is dispatched to the actions delegate
after the first frame — for a form that must fetch something before the user
acts.

## Data tables

`TableConfig` describes both the query and the widget: `fromClauses`,
`whereClauses`, `columns`, sorting, paging, plus `actions` (`ActionConfig`).
`JetsDataTableWidget` builds a `/dataTable` request from it — the client
composes a structured query that the server turns into sql.

`ActionConfig.actionType` decides what a table button does: `showDialog`,
`showScreen`, `doAction`, `refreshTable`, `toggleCheckboxVisible`,
`setSessionIdFilter`, and others.

`formStateConfig` publishes the selected row into the enclosing form's state,
which is how a table selection feeds a form.

`refreshOnKeyUpdateEvent` re-runs the query when a listed form state key changes.

## User flows

`UserFlowConfig` is a state machine "greatly inspired from the Amazon States
Language" — states, each with a `FormConfig`, and choice-style conditions
selecting the next state. `standardActions` supplies Previous/Cancel/Next.

Defined under `modules/user_flows/<feature>/`, one directory per flow, each with
`user_flow_config.dart`, `form_config.dart`, `data_table_config.dart`,
`screen_config.dart`, `form_action_delegates.dart`. That five-file shape is the
convention for a new feature module.

## Server communication

`HttpClientSingleton().sendRequest(path:, token:, encodedJsonBody:)` — always a
POST, always json, origin derived in `main.dart` from the browser url (port 8080
in debug).

Two behaviours to know:

- **Every response carries a refreshed jwt.** The client pulls `body['token']`
  and updates the session. The token is still in the map it hands back, so strip
  it before rendering a response body anywhere a user can see or copy it.
- **A 401 redirects to login** from inside the client, clearing the token.

Endpoints are in `ServerEPs` (`utils/constants.dart`): `/dataTable`,
`/purgeData`, `/registerFileKey`, `/inferServer`, `/login`, `/register`.
`/dataTable` is a multiplexer — the `action` field selects among `read`,
`raw_query`, `insert_rows`, `workspace_*` and others; see the switch in
`jets/apiserver/api_tables.go`.

## Capabilities

`user.capabilities` comes from the server at login, resolved from
`jetsapi.role_capability`. The client uses it to disable menu entries
(`base_screen.dart`), form buttons (`form_button.dart`) and table actions
(`data_table.dart`). Admin passes every check. Note these **disable** rather
than hide.

**Client-side capability checks are presentation only.** A disabled button stops
nobody who can post to the endpoint. Server-side enforcement exists in exactly
two places: `DataTableContext.VerifyUserPermission` for sql statements, and
`DoInferServerAction` for the infer server endpoint. `authh` validates the token
and nothing more. **Any new endpoint that mutates something must check the
capability itself**, following one of those two.

Current capabilities: `jetstore_read`, `client_config`, `workspace_ide`,
`run_pipelines`, `user_profile`, `infer_server_admin`. Seeded in
`jets/jets_init_db.sql`, which runs on every deployment carrying UI service
changes, so adding one needs no manual step.

## Constants

`utils/constants.dart` is one large bag of key classes: `ScreenKeys`,
`FormKeys`, `ActionKeys`, `FSK` (form state keys), `DTKeys` (data table keys),
`ServerEPs`, plus `ActionStyle` and the padding constants. Everything is a
string constant, so a typo is a runtime lookup failure, not a compile error. Add
keys here rather than inlining literals.

## Adding a screen

1. `modules/<area>/<feature>/` with `screen_config.dart`, `form_config.dart`,
   `form_action_delegates.dart`, each exposing a nullable `get<Feature>*Config`.
2. Keys in `utils/constants.dart`.
3. A path constant and a `jetsRoutesMap` entry in `routes/jets_routes_app.dart`.
4. A `MenuEntry` in the relevant menu list, with `capability` if it needs one.
5. **Wire the getters into the fallback chains** in `screen_config_impl.dart`
   and `form_config_impl.dart`, imports included.

`modules/workspace_ide/infer_server_admin/` is a small, current example of the
whole shape, including buttons whose enablement depends on server state.

## Build and test

```bash
flutter pub get
flutter analyze                                   # ~129 pre-existing infos, mostly avoid_print
flutter build web --release                       # the real compile check
flutter test --platform chrome                    # NOT plain `flutter test`
```

`flutter test` on the default vm target **cannot load this app** — the import
chain reaches `package:web`, which is web-only. This is pre-existing and affects
the template `widget_test.dart` too. Always pass `--platform chrome`.

There is very little test coverage. `test/form_button_test.dart` covers the
button's two enablement paths and is a usable template: it builds a field the
way `JetsForm` builds a row, which is what makes framework-level regressions
visible.

The compiled bundle is served by the Go apiserver, which lists each asset
explicitly in `jets/apiserver/server.go`. **A new top-level asset needs a route
added there**, or it 404s in production while working under `flutter run`.
