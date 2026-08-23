/**
 * A form document, rendered. Task F.0a.
 *
 * **The last piece of R2, and the smallest of the four.** The schema, the
 * validator, the interpreter and the engine were all built in Phase 2 and all
 * driven from tests; what nothing did was put a `.form.json` on a screen. This
 * walks `form.rows`, maps each field kind to the widget track A built, and
 * renders `form.actions` as buttons.
 *
 * ## Five field kinds, three widgets, and the two that are not widgets
 *
 * `text`, `dropdown` and `dataTable` are A.3's and A.4's. `label` and `spacer`
 * are layout — they carry no key, hold no value and cannot be validated, which is
 * why `valueFieldsOf` exists to exclude them (`form.ts`). The 55 `PaddingConfig`
 * and 12 `TextFieldConfig` instances the corpus counts (I-12) are the second
 * largest thing in a form after the inputs, so they are not an afterthought.
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
import type { ActionConfig, TableConfig } from "../datatable/types";
import { ActionButton } from "../shell/capabilities";
import { Dropdown } from "../widgets/Dropdown";
import { TextInput } from "../widgets/TextInput";
import type { Field, Form, FormAction } from "./form";
import type { FieldError } from "./validateForm";

/** Everything the renderer needs that is not the form. */
export interface FormHost {
  formState: FormState;
  group: number;
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
  /** True while an action is running; every button waits. */
  busy: boolean;
}

export interface FormRendererProps {
  form: Form;
  host: FormHost;
  errors: FieldError[];
}

export function FormRenderer({ form, host, errors }: FormRendererProps): ReactNode {
  const errorFor = (key: string): string | undefined =>
    errors.find((e) => e.key === key)?.message;

  return (
    <section className="uf-form">
      {form.title !== undefined && <h2 className="uf-form__title">{form.title}</h2>}

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
              host={host}
              error={"key" in field ? errorFor(field.key) : undefined}
            />
          ))}
        </div>
      ))}

      <div className="uf-form__actions" role="group" aria-label="Form actions">
        {form.actions.map((action) => (
          <ActionButton
            key={action.action}
            className={`btn btn--${action.style ?? "primary"}`}
            disabled={host.busy || (action.enableOnlyWhenFormValid === true && !host.formValid)}
            {...(action.capability !== undefined ? { capability: action.capability } : {})}
            onClick={() => host.onFormAction(action)}
          >
            {action.label}
          </ActionButton>
        ))}
      </div>
    </section>
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
      return <p className="uf-form__label">{field.text}</p>;
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
          {...(error !== undefined ? { error } : {})}
        />
      );
    case "dropdown":
      return (
        <Dropdown
          formState={host.formState}
          group={host.group}
          fieldKey={field.key}
          label={field.label}
          items={field.items}
          {...(field.defaultItemPos !== undefined ? { defaultItemPos: field.defaultItemPos } : {})}
          {...(field.isReadOnly !== undefined ? { isReadOnly: field.isReadOnly } : {})}
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
  }
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
