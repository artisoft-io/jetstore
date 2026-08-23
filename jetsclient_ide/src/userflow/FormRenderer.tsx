/**
 * A form document, rendered. Task F.0a.
 *
 * **The last piece of R2, and the smallest of the four.** The schema, the
 * validator, the interpreter and the engine were all built in Phase 2 and all
 * driven from tests; what nothing did was put a `.form.json` on a screen. This
 * walks `form.rows`, maps each field kind to the widget track A built, and
 * renders `form.actions` as buttons.
 *
 * ## Seven field kinds, four widgets, and the three that are not widgets
 *
 * `text`, `dropdown` and `dataTable` are A.3's and A.4's, and `typeahead` is
 * I.2b's. `label`, `spacer` and F.2's `button` are layout — they carry no key,
 * hold no value and cannot be validated, which is why `valueFieldsOf` exists to
 * exclude them (`form.ts`). The 55 `PaddingConfig` and 12 `TextFieldConfig`
 * instances the corpus counts (I-12) are the second largest thing in a form
 * after the inputs, so they are not an afterthought — and the three
 * `FormActionConfig` instances it counts are why a button is a field kind at
 * all.
 *
 * ## Where a query-backed field's items come from
 *
 * Not from here. The host supplies `queryRows`, because *running* a form's queries
 * is a screen's job — it needs the api client, a re-run trigger and somewhere to
 * put a failure — and the rule is the one the error list already follows: a
 * renderer decides what a field looks like, and a caller decides what is true.
 * `formQueries.ts` holds the plan and `FlowRunner` runs it.
 *
 * ## Errors are the caller's, not this component's
 *
 * `validateForm` is a pure function over the form and the state, and *when* it
 * runs is a flow decision: `ufNext` validates and `ufStartFlow` does not
 * (`engine.ts`, `step`). So the runner owns the error list and passes it down.
 * A renderer that validated on every keystroke would show a required-field error
 * before the user had typed anything, which is not what the Flutter app does.
 *
 * ## Each `dataTable` field is a component of its own, and it has to be
 *
 * `useTableBinding` is a hook, so one per field, so one component per field —
 * a loop calling hooks would break the first time a form had two tables, which
 * `loadFilesUF` does across its two states and `pipelineConfigUF` does within
 * one.
 */

import type { ReactNode } from "react";

import { ActionBar } from "../datatable/ActionBar";
import { DataTable } from "../datatable/DataTable";
import type { FormState } from "../datatable/formState";
import type { ActionRequest } from "../datatable/actionDispatch";
import { useTableBinding } from "../datatable/useTableBinding";
import type { DataTableFetcher } from "../datatable/useDataTable";
import type { ActionConfig, JetsRow, TableConfig } from "../datatable/types";
import { ActionButton } from "../shell/capabilities";
import { Dropdown } from "../widgets/Dropdown";
import { TextInput } from "../widgets/TextInput";
import { Typeahead } from "../widgets/Typeahead";
import type { Field, Form, FormAction } from "./form";
import { itemsFromRows } from "./formQueries";
import type { FieldError } from "./validateForm";

/** Everything the renderer needs that is not the form. */
export interface FormHost {
  formState: FormState;
  group: number;
  /**
   * The rows one of the form's named queries returned, or undefined while it has
   * not run. Task I.2b — `FlowRunner` reads them off `FormState.queryRows`.
   */
  queryRows(name: string): JetsRow[] | undefined;
  /** True while any of the form's queries is in flight; item sources wait. */
  queriesLoading: boolean;
  /** Resolves a `dataTable` field's `table` to the configuration it names. */
  tableConfig(key: string): TableConfig;
  fetcher: DataTableFetcher;
  /** Named predicates for a table action's `isEnabled` — `actions/registry.ts`. */
  predicates: Readonly<Record<string, (formState: FormState, group: number) => boolean>>;
  /** Per-column display filters, by column name, for the table this field names. */
  cellFilters(tableKey: string): Record<string, (value: string | null) => string | null>;
  /** A table action's button was pressed. */
  onTableAction(request: ActionRequest, action: ActionConfig): void;
  /** A form action's button was pressed. */
  onFormAction(action: FormAction): void;
  /** Whether the form as a whole passes — `enableOnlyWhenFormValid` reads this. */
  formValid: boolean;
  /**
   * How many validation groups to draw. Task F.1.
   *
   * One for every form in the corpus but `fmMappingFormUF`, which draws its rows
   * once per row of `repeat.from`'s result — so the count is the **query's**, not
   * the document's and not `FormState.groupCount`.
   *
   * **Not the store's, and the difference is visible.** `resizeFormState` grows
   * and never shrinks (I.1, and the Dart does the same), so a store that has held
   * ten rows still reports ten after a re-query returns two. The Dart has the
   * same store behaviour and does not have the same bug, because what it draws is
   * the list its row builder returned rather than the group count. This is that
   * list's length.
   */
  groupCount: number;
  /** True while an action is running; every button waits. */
  busy: boolean;
}

export interface FormRendererProps {
  form: Form;
  host: FormHost;
  errors: FieldError[];
}

export function FormRenderer({ form, host, errors }: FormRendererProps): ReactNode {
  // A repeating form draws its rows once per validation group; every other form
  // draws them once, in the host's group. The two are the same loop.
  const groups =
    form.repeat === undefined
      ? [host.group]
      : Array.from({ length: Math.max(0, host.groupCount) }, (_, index) => index);

  return (
    <section className="uf-form">
      {form.title !== undefined && <h2 className="uf-form__title">{form.title}</h2>}

      {form.repeat !== undefined && groups.length === 0 && !host.queriesLoading && (
        // Zero rows and nothing in flight. Either the query returned none or a
        // parameter it needs is missing — and the second is what a user reaching
        // a parameterised flow route by hand gets.
        <p className="uf-form__empty" role="status">
          Nothing to map yet.
        </p>
      )}

      {groups.map((group) => (
        <FormGroup key={group} form={form} host={host} group={group} errors={errors} />
      ))}

      <div className="uf-form__actions" role="group" aria-label="Form actions">
        {/* The inverse gate below is a real behaviour rather than a stylistic
            mirror: "Save as Draft" is offered *because* the worksheet does not
            validate (`form_button.dart`). Both gates live in
            `FormActionButton` now, so an inline `button` field reads them the
            same way — see the `button` case in `FieldView`. */}
        {form.actions.map((action) => (
          <FormActionButton key={action.action} action={action} host={host} />
        ))}
      </div>
    </section>
  );
}

/**
 * One validation group's worth of the form.
 *
 * Split out of `FormRenderer` so a repeating form is a loop over this rather than
 * a `group` threaded through every call below it, and so the group boundary is
 * where React's reconciliation boundary is — a row added or removed at the end
 * must not re-key the rows before it.
 */
function FormGroup({
  form,
  host,
  group,
  errors,
}: {
  form: Form;
  host: FormHost;
  group: number;
  errors: FieldError[];
}): ReactNode {
  const errorFor = (key: string): string | undefined =>
    errors.find((e) => e.key === key && e.group === group)?.message;
  const scoped: FormHost = group === host.group ? host : { ...host, group };

  return (
    <>
      {form.rows.map((row, rowIndex) => (
        // Rows are positional and a field carries no identity of its own beyond
        // its key, which `label` and `spacer` do not have — so the index is the
        // key, and reordering a row re-renders it. That is Phase 0's unstable
        // form-field key finding pointing the other way: there, a *generated*
        // key changed every render; here the position is the identity.
        <div className="uf-form__row" key={rowIndex}>
          {row.map((field, fieldIndex) => (
            <FieldView
              key={fieldIndex}
              field={field}
              host={scoped}
              error={"key" in field ? errorFor(field.key) : undefined}
            />
          ))}
        </div>
      ))}
    </>
  );
}

function FieldView({
  field,
  host,
  error,
}: {
  field: Field;
  host: FormHost;
  error?: string;
}): ReactNode {
  switch (field.field) {
    case "label":
      return (
        <p className="uf-form__label">
          {"text" in field
            ? field.text
            : // A repeating row's heading, written by the seed escape into this
              // group's form state — see `form.ts`, the `label` union.
              (asText(host.formState.getValue(host.group, field.fromKey)) ?? "")}
        </p>
      );
    case "spacer":
      return <div className="uf-form__spacer" aria-hidden="true" />;
    case "text":
      return (
        <TextInput
          formState={host.formState}
          group={host.group}
          fieldKey={field.key}
          label={field.label}
          {...(field.hint !== undefined ? { hint: field.hint } : {})}
          {...(field.maxLines !== undefined ? { maxLines: field.maxLines } : {})}
          {...(field.maxLength !== undefined ? { maxLength: field.maxLength } : {})}
          {...(field.autofocus !== undefined ? { autofocus: field.autofocus } : {})}
          {...(field.isReadOnly !== undefined ? { isReadOnly: field.isReadOnly } : {})}
          {...(error !== undefined ? { error } : {})}
        />
      );
    case "dropdown": {
      // The literal list first and the query's rows after it, which is what both
      // Dart paths do — the literal carries the "Select a…" prompt and the query
      // carries the choices (`form.ts`, `itemsFrom`).
      const fetched =
        field.itemsFrom === undefined ? [] : itemsFromRows(host.queryRows(field.itemsFrom));
      return (
        <Dropdown
          formState={host.formState}
          group={host.group}
          fieldKey={field.key}
          label={field.label}
          items={[...field.items, ...fetched]}
          loading={field.itemsFrom !== undefined && host.queriesLoading}
          {...(field.defaultItemPos !== undefined ? { defaultItemPos: field.defaultItemPos } : {})}
          {...(field.isReadOnly !== undefined ? { isReadOnly: field.isReadOnly } : {})}
          {...(error !== undefined ? { error } : {})}
        />
      );
    }
    case "typeahead":
      return (
        <Typeahead
          formState={host.formState}
          group={host.group}
          fieldKey={field.key}
          label={field.label}
          items={itemsFromRows(host.queryRows(field.itemsFrom)).map((i) => i.value)}
          loading={host.queriesLoading}
          {...(field.hint !== undefined ? { hint: field.hint } : {})}
          {...(field.priorityKey !== undefined ? { priorityKey: field.priorityKey } : {})}
          {...(field.maxLength !== undefined ? { maxLength: field.maxLength } : {})}
          {...(error !== undefined ? { error } : {})}
        />
      );
    case "dataTable":
      return (
        <FormDataTable
          fieldKey={field.key}
          tableKey={field.table}
          host={host}
          {...(error !== undefined ? { error } : {})}
        />
      );
    case "button":
      // **The same component the action bar renders, deliberately.** A
      // `FormActionConfig` in `inputFields` and one in `actions` are one Dart
      // class (`form.ts`, the `button` variant), so a divergence here would be a
      // divergence the document cannot express.
      return <FormActionButton action={field} host={host} />;
  }
}

/**
 * One button, wherever it sits. Task F.2.
 *
 * Factored out of the action bar's map so the bar and an inline `button` field
 * cannot drift on the two enable gates — which is the pair `mapperOk` and
 * `mapperDraft` proved are independent branches rather than one flag (I-76).
 */
function FormActionButton({ action, host }: { action: FormAction; host: FormHost }): ReactNode {
  return (
    <ActionButton
      className={`btn btn--${action.style ?? "primary"}`}
      disabled={
        host.busy ||
        (action.enableOnlyWhenFormValid === true && !host.formValid) ||
        (action.enableOnlyWhenFormNotValid === true && host.formValid)
      }
      {...(action.capability !== undefined ? { capability: action.capability } : {})}
      onClick={() => host.onFormAction(action)}
    >
      {action.label}
    </ActionButton>
  );
}

/**
 * One `dataTable` field: A.4's widget bound to this form.
 *
 * **The field key and the table key are separate parameters even though every
 * shipping form sets them equal.** `lfSourceConfigTable` is both the field's key
 * in form state and the configuration it names, in all three proof forms — but
 * they are different things: one addresses a value, the other addresses a
 * document that two flows may share. Collapsing them would make a second form
 * naming the same table impossible to write.
 */
function FormDataTable({
  fieldKey,
  tableKey,
  host,
  error,
}: {
  fieldKey: string;
  tableKey: string;
  host: FormHost;
  error?: string;
}): ReactNode {
  const config = host.tableConfig(tableKey);
  const binding = useTableBinding({
    config,
    field: { group: host.group, key: fieldKey },
    formState: host.formState,
    fetcher: host.fetcher,
  });

  const selectedIndex = binding.selection.findIndex(Boolean);
  const selectedRow = selectedIndex === -1 ? undefined : binding.rows[selectedIndex];
  const selectedRowCount = binding.selection.filter(Boolean).length;

  return (
    <div className="uf-form__table">
      <DataTable
        config={config}
        state={binding}
        modes={binding.modes}
        cellFilters={host.cellFilters(tableKey)}
        actions={
          <ActionBar
            actions={config.actions}
            context={{
              selectedRowCount,
              checkboxVisible: binding.modes.checkboxVisible,
              // `blocked` is `hasBlockingFilter`, which is this predicate
              // inverted — A.4c's answer reused rather than recomputed.
              whereClauseSatisfied: !binding.blocked,
              formState: host.formState,
              predicates: host.predicates,
            }}
            {...(selectedRow !== undefined ? { selectedRow } : {})}
            onAction={host.onTableAction}
          />
        }
      />
      {error !== undefined && (
        <p className="field-error" role="alert">
          {error}
        </p>
      )}
    </div>
  );
}

/** Form state holds `string | string[]`; a label shows the first. */
function asText(raw: unknown): string | null {
  if (typeof raw === "string") return raw;
  if (Array.isArray(raw) && raw.length > 0 && typeof raw[0] === "string") return raw[0];
  return null;
}
