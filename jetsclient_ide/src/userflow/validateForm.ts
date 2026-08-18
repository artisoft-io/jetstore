/**
 * Form validation. Task S.5.
 *
 * A port of the two proof flows' `formValidatorDelegate`s, which between them
 * use exactly two rules — see `form.ts` for why that is the whole vocabulary.
 *
 * **Validity is form-wide.** `JetsFormState.isFormValid` (`jets_form_state.dart:192`)
 * asks every field, and `enableOnlyWhenFormValid` reads that one answer; a
 * per-field reading would let a Save button light up on a half-filled form.
 */

import type { FormState } from "../datatable/formState";
import { valueFieldsOf, type Form } from "./form";

export interface FieldError {
  key: string;
  message: string;
}

/** `unpack`: a scalar, or the first of a selection array. */
function scalar(raw: unknown): string | null {
  if (raw == null) return null;
  if (typeof raw === "string") return raw;
  if (Array.isArray(raw)) return raw.length > 0 ? scalar(raw[0]) : null;
  return null;
}

export function validateForm(form: Form, formState: FormState, group: number): FieldError[] {
  const errors: FieldError[] = [];
  for (const field of valueFieldsOf(form)) {
    const raw = formState.getValue(group, field.key);
    for (const rule of field.rules ?? []) {
      if (rule.rule === "required") {
        // Empty means empty whichever shape it arrives in: a cleared text box
        // stores nothing (A.3), and a table with no selection stores nothing.
        const value = scalar(raw);
        if (value === null || value === "") {
          errors.push({ key: field.key, message: rule.message });
          break;
        }
      }
      if (rule.rule === "json") {
        const value = scalar(raw);
        if (value !== null && value !== "") {
          try {
            JSON.parse(value);
          } catch (error) {
            errors.push({ key: field.key, message: `${rule.message}: ${(error as Error).message}` });
            break;
          }
        }
      }
    }
  }
  return errors;
}

export const isFormValid = (form: Form, formState: FormState, group: number): boolean =>
  validateForm(form, formState, group).length === 0;
