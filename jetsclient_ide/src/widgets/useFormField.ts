/**
 * Binding a scalar widget to the form state.
 *
 * Task A.3. **Extends the form state A.4c built rather than introducing a
 * second one** — issue I-9. Two stores in one screen would let the data table
 * publish a selection the inputs cannot see, with each half looking correct in
 * isolation.
 *
 * ## Seed once, then own the value
 *
 * `JetsTextFormField` seeds its text from the form state at `initState` and is
 * the source of truth from then on; the exception is `syncWithFormState`, for
 * values something other than the user writes — a button filling in a template,
 * a server response (`models/form_config.dart:291`). **No field in the nine
 * flows sets it**, so the exception is implemented but unexercised, and the
 * comment says which of the two you are getting.
 *
 * That asymmetry is not an accident to tidy away: a controlled input that reads
 * form state on every render fights the user's cursor whenever anything else
 * writes to the same key, and the Flutter app chose the same way round.
 */

import { useCallback, useEffect, useRef, useState } from "react";

import type { FormState } from "../datatable/formState";

export interface FormFieldBinding {
  value: string;
  /** Writes through to the form state and notifies, as a user edit should. */
  setValue(next: string): void;
}

export interface UseFormFieldOptions {
  formState: FormState;
  group: number;
  fieldKey: string;
  /** Used only when the form state holds nothing for the key at mount. */
  defaultValue?: string;
  /**
   * Re-read the form state when someone else writes to this key. Off by
   * default, matching the Flutter default; no flow field turns it on.
   */
  syncWithFormState?: boolean;
}

/** Form state holds `string | string[]`; a scalar widget shows the first. */
function asText(value: unknown): string {
  if (typeof value === "string") return value;
  if (Array.isArray(value) && value.length > 0 && typeof value[0] === "string") {
    return value[0];
  }
  return "";
}

export function useFormField({
  formState,
  group,
  fieldKey,
  defaultValue,
  syncWithFormState = false,
}: UseFormFieldOptions): FormFieldBinding {
  const [value, setLocal] = useState<string>(() => {
    const held = formState.getValue(group, fieldKey);
    if (held !== undefined && held !== null) return asText(held);
    return defaultValue ?? "";
  });

  // A default is written back, not merely displayed. The Flutter widget does the
  // same, and it matters: a form submitted without touching the field must carry
  // the default rather than nothing.
  const seeded = useRef(false);
  useEffect(() => {
    if (seeded.current) return;
    seeded.current = true;
    const held = formState.getValue(group, fieldKey);
    if ((held === undefined || held === null) && defaultValue != null && defaultValue !== "") {
      formState.setValue(group, fieldKey, defaultValue);
    }
  }, [formState, group, fieldKey, defaultValue]);

  useEffect(() => {
    if (!syncWithFormState) return;
    return formState.subscribe(() => {
      setLocal(asText(formState.getValue(group, fieldKey)));
    });
  }, [formState, group, fieldKey, syncWithFormState]);

  const setValue = useCallback(
    (next: string) => {
      setLocal(next);
      // Empty clears the binding rather than storing "", so a cleared field and
      // an untouched one look the same to the query builder — which is what
      // A.4a's where-clause fallback tests for.
      formState.setValue(group, fieldKey, next === "" ? null : next);
      formState.notifyListeners();
    },
    [formState, group, fieldKey],
  );

  return { value, setValue };
}
