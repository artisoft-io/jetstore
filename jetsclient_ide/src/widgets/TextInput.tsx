/**
 * The text input widget. Task A.3.
 *
 * **Scoped by measurement, not by guesswork.**
 * `jetsclient/test/form_field_corpus_test.dart` serialises every field the nine
 * flows declare; across the 46 `FormInputFieldConfig` instances it reports, the
 * options actually set are:
 *
 * | Option | Set on |
 * |---|---|
 * | `useDefaultFont` | 26 |
 * | `isReadOnly` | 23 |
 * | `autofocus` | 8 |
 * | `maxLines` (> 1) | 8 |
 * | `defaultValue` | 4 |
 * | `textRestriction: digitsOnly` | 2 |
 *
 * Five declared options are set by **no** field in any flow — `obscureText`,
 * `autofillHints`, `syncWithFormState`, `showCopyToClipboard` and
 * `isReadOnlyEval`. Four are implemented anyway because they are one line each
 * and their absence would be a silent behaviour change if a flow adopted one;
 * `isReadOnlyEval` is not, because it is a Dart closure and the port has nowhere
 * to put it — see the note at the bottom.
 *
 * `maxLength` ranges from 10 to 102,400 across the corpus, so the field is not
 * a short-string control with a long-text special case; it is one control with a
 * limit.
 */

import { useId } from "react";

import { useFormField } from "./useFormField";
import type { FormState } from "../datatable/formState";

/** `TextRestriction` (`models/form_config.dart:17`). */
export type TextRestriction = "none" | "allLower" | "allUpper" | "digitsOnly";

export interface TextInputProps {
  formState: FormState;
  group: number;
  fieldKey: string;
  label: string;
  hint?: string;
  /** 0 means unbounded, as in the Dart. */
  maxLength?: number;
  maxLines?: number;
  textRestriction?: TextRestriction;
  defaultValue?: string;
  autofocus?: boolean;
  isReadOnly?: boolean;
  obscureText?: boolean;
  syncWithFormState?: boolean;
  /** A validation message from the form; the widget does not decide validity. */
  error?: string;
}

/** Applies a `TextRestriction` as the user types, as the Dart formatters do. */
export function restrict(value: string, restriction: TextRestriction): string {
  switch (restriction) {
    case "allLower":
      return value.toLowerCase();
    case "allUpper":
      return value.toUpperCase();
    case "digitsOnly":
      return value.replace(/\D/g, "");
    case "none":
      return value;
  }
}

export function TextInput({
  formState,
  group,
  fieldKey,
  label,
  hint,
  maxLength = 0,
  maxLines = 1,
  textRestriction = "none",
  defaultValue,
  autofocus = false,
  isReadOnly = false,
  obscureText = false,
  syncWithFormState = false,
  error,
}: TextInputProps) {
  const id = useId();
  const { value, setValue } = useFormField({
    formState,
    group,
    fieldKey,
    ...(defaultValue !== undefined ? { defaultValue } : {}),
    syncWithFormState,
  });

  const onChange = (raw: string) => setValue(restrict(raw, textRestriction));
  const shared = {
    id,
    value,
    readOnly: isReadOnly,
    autoFocus: autofocus,
    "aria-invalid": error != null,
    ...(hint ? { placeholder: hint } : {}),
    ...(maxLength > 0 ? { maxLength } : {}),
    ...(error != null ? { "aria-errormessage": `${id}-error` } : {}),
  };

  return (
    <div className={`field${isReadOnly ? " field--readonly" : ""}`}>
      <label htmlFor={id}>{label}</label>
      {maxLines > 1 ? (
        <textarea {...shared} rows={maxLines} onChange={(e) => onChange(e.target.value)} />
      ) : (
        <input
          {...shared}
          type={obscureText ? "password" : "text"}
          onChange={(e) => onChange(e.target.value)}
        />
      )}
      {error != null && (
        <p className="field-error" id={`${id}-error`} role="alert">
          {error}
        </p>
      )}
    </div>
  );
}

/**
 * **`isReadOnlyEval` is not implemented, and that is a decision.**
 *
 * It is a Dart closure — `bool Function(JetsFormState)` — so it cannot cross
 * into a configuration file any more than `cellFilter` or the action delegates
 * could. No field in the nine flows sets it, so nothing is lost today. When one
 * needs to, the answer is the same as everywhere else in this port: express the
 * condition in the flow schema S.1 defines, not as a function pointer.
 */
