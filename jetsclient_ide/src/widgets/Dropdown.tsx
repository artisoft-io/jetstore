/**
 * The dropdown widget. Task A.3.
 *
 * Eleven `FormDropdownFieldConfig` instances across the nine flows — the one
 * figure of the assessment's field table the corpus confirms unchanged. Their
 * item lists are short: 1 to 9 entries.
 *
 * Options set by at least one field: `dropdownItemsQuery` (5), `isReadOnly` (4),
 * `defaultItemPos` (2), `returnedModelCacheKey` (2), `stateKeyPredicates` (1).
 * `whereStateContains` and `makeReadOnlyWhenHasSelectedValue` are declared and
 * set by none.
 *
 * ## Where the items come from is not this widget's business
 *
 * **Five of the eleven populate themselves from a named server query**, and that
 * is the one part of A.3 that is not small. It is deliberately not built here.
 * The query is named on the *form* (`dropdownItemsQueries`), resolved against
 * the server, and optionally cached under `returnedModelCacheKey` — so it is
 * form-level machinery whose home is the flow schema S.1 defines, alongside
 * `stateKeyPredicates`, which re-runs the query when a form-state key changes.
 *
 * This widget takes `items` as a prop. A static list and a query result reach it
 * the same way, which is what keeps it a widget rather than a second data layer.
 * Recorded as I-11 so the omission is a scheduled piece of work rather than a
 * gap someone rediscovers.
 */

import { useEffect, useId, useRef } from "react";

import { asText, useFormField } from "./useFormField";
import type { FormState } from "../datatable/formState";

export interface DropdownItem {
  /** Written to form state. The Dart carries label and value separately. */
  value: string;
  label: string;
}

export interface DropdownProps {
  formState: FormState;
  group: number;
  fieldKey: string;
  label: string;
  items: DropdownItem[];
  /** Index of the item selected when the form state holds nothing. */
  defaultItemPos?: number;
  isReadOnly?: boolean;
  /** True while a `dropdownItemsQuery` is in flight; the control waits. */
  loading?: boolean;
  error?: string;
}

export function Dropdown({
  formState,
  group,
  fieldKey,
  label,
  items,
  defaultItemPos = 0,
  isReadOnly = false,
  loading = false,
  error,
}: DropdownProps) {
  const id = useId();
  const fallback = items[defaultItemPos]?.value;
  const { value, setValue } = useFormField({
    formState,
    group,
    fieldKey,
    ...(fallback !== undefined ? { defaultValue: fallback } : {}),
  });

  // Items arriving late — from a query — must not silently leave the field on a
  // value that is no longer offered, and must seed the default they now imply.
  //
  // The separator is NUL because it is the one character an item value cannot
  // contain, and it is written `\0` rather than as a literal byte: a raw NUL in
  // the source makes `grep` classify the whole file as binary and skip it
  // without saying so, which in this project is a measurement hazard (I-46).
  //
  // **It reads the form state, not `value`** (jetstore_maintenance_02 `I-30`,
  // 2026-10-27). This is a passive effect, so it can run after a change event
  // that landed between the commit that drew the items and the effect itself:
  // the event writes the store at once, but `value` here is still the render's.
  // Reading `value` then put the default back over the user's choice. A test
  // that picks an item as soon as the options are drawn lost that race under
  // load; `widgets.test.tsx` makes it deterministic.
  const seededFor = useRef<string>("");
  useEffect(() => {
    const signature = items.map((i) => i.value).join("\0");
    if (items.length === 0 || seededFor.current === signature) return;
    seededFor.current = signature;
    const current = asText(formState.getValue(group, fieldKey));
    if (current === "" || !items.some((i) => i.value === current)) {
      const next = items[defaultItemPos]?.value;
      if (next !== undefined) setValue(next);
    }
  }, [items, defaultItemPos, value, setValue, formState, group, fieldKey]);

  return (
    <div className={`field${isReadOnly ? " field--readonly" : ""}`}>
      <label htmlFor={id}>{label}</label>
      <select
        id={id}
        value={value}
        disabled={isReadOnly || loading}
        aria-invalid={error != null}
        aria-busy={loading}
        {...(error != null ? { "aria-errormessage": `${id}-error` } : {})}
        onChange={(e) => setValue(e.target.value)}
      >
        {loading && <option value="">Loading…</option>}
        {!loading && items.length === 0 && <option value="">No choices</option>}
        {!loading &&
          items.map((item) => (
            <option key={item.value} value={item.value}>
              {item.label}
            </option>
          ))}
      </select>
      {error != null && (
        <p className="field-error" id={`${id}-error`} role="alert">
          {error}
        </p>
      )}
    </div>
  );
}
