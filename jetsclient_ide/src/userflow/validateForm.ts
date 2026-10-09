/**
 * Form validation. Task S.5, extended by F.1.
 *
 * A port of the two proof flows' `formValidatorDelegate`s, which between them
 * use exactly two rules — see `form.ts` for why that was the whole vocabulary —
 * plus, since F.1, a **named validator escape** for the form whose every check is
 * a relation between sibling values rather than a property of one field.
 *
 * **Validity is form-wide.** `JetsFormState.isFormValid` (`jets_form_state.dart`,
 * `isFormValid`) asks every field, and `enableOnlyWhenFormValid` reads that one
 * answer; a per-field reading would let a Save button light up on a half-filled
 * form.
 *
 * **And form-wide means every group.** A repeating form is one form with *n*
 * validation groups (`form.ts`, `RepeatSchema`), so the Dart's set of invalid
 * keys is per group and its `isFormValid` walks all of them. Here the walk is the
 * validation itself: `validateForm` is called per group and `isFormValid` is the
 * conjunction over the groups the form actually has. Reproducing the Dart's
 * *marks* — `markFormKeyAsValid` / `markFormKeyAsInvalid` — would be a second
 * copy of the answer to keep in step with the first.
 */

import type { FormState } from "../datatable/formState";
import type { EscapeContext, ValidatorEscape } from "../actions/escapes";
import { valueFieldsOf, type Form } from "./form";

export interface FieldError {
  key: string;
  message: string;
  /** Which validation group the error is in; 0 for a form that does not repeat. */
  group: number;
}

/** `unpack`: a scalar, or the first of a selection array. */
function scalar(raw: unknown): string | null {
  if (raw == null) return null;
  if (typeof raw === "string") return raw;
  if (Array.isArray(raw)) return raw.length > 0 ? scalar(raw[0]) : null;
  return null;
}

/** Everything the escape half needs; absent for a form naming no validator. */
export interface FormValidatorContext {
  validate: ValidatorEscape;
  flowKey: string;
}

export function validateForm(
  form: Form,
  formState: FormState,
  group: number,
  escape?: FormValidatorContext,
): FieldError[] {
  const errors: FieldError[] = [];
  for (const field of valueFieldsOf(form)) {
    const raw = formState.getValue(group, field.key);
    let failed = false;
    for (const rule of field.rules ?? []) {
      if (rule.rule === "required") {
        // Empty means empty whichever shape it arrives in: a cleared text box
        // stores nothing (A.3), and a table with no selection stores nothing.
        const value = scalar(raw);
        if (value === null || value === "") {
          errors.push({ key: field.key, message: rule.message, group });
          failed = true;
          break;
        }
      }
      if (rule.rule === "extendsKey") {
        // Task C.3b. The prefix is another field's value, which is what makes
        // this the first rule that reads outside its own field — see
        // `RuleSchema`.
        const value = scalar(raw) ?? "";
        const prefix = scalar(formState.getValue(group, rule.key));
        if (prefix === null || prefix === "") {
          // A document defect, not a user error, and it says which key is
          // empty. Degrading to "no prefix" would turn this rule into
          // `required` and the form would look validated.
          errors.push({
            key: field.key,
            message: `${rule.message} (no prefix: "${rule.key}" is not set)`,
            group,
          });
          failed = true;
          break;
        }
        if (!value.startsWith(prefix) || value.length <= prefix.length) {
          errors.push({ key: field.key, message: rule.message, group });
          failed = true;
          break;
        }
      }
      if (rule.rule === "json") {
        const value = scalar(raw);
        if (value !== null && value !== "") {
          try {
            JSON.parse(value);
          } catch (error) {
            errors.push({
              key: field.key,
              message: `${rule.message}: ${(error as Error).message}`,
              group,
            });
            failed = true;
            break;
          }
        }
      }
    }
    if (failed || escape === undefined) continue;
    // **The rules run first and the escape second, and only when the rules
    // passed.** A field that is empty and required has one thing wrong with it,
    // and the rule says so in the author's own words; asking the escape as well
    // would put two messages on one field with no ordering between them.
    const context: EscapeContext = { formState, group, flowKey: escape.flowKey };
    const message = escape.validate(context, field.key, raw);
    if (message !== null) errors.push({ key: field.key, message, group });
  }
  return errors;
}

/**
 * Every group's errors, for a form that repeats; the given group's otherwise.
 *
 * **`isFormValid` keeps its per-group signature and this is the addition**, which
 * is a deliberate choice rather than a smaller diff. `isFormValid(form, state,
 * group)` is called from outside this repository — the agentic_ai project drives
 * a projected flow through it (`cpipes/templateApply.test.ts`) — and its meaning
 * for a non-repeating form is unchanged. Widening it in place would have changed
 * the shape of a function another project reads as a contract, to serve one form
 * of the fifty. Two names, one of which is honest about walking the store.
 */
export function validateAllGroups(
  form: Form,
  formState: FormState,
  group: number,
  escape?: FormValidatorContext,
): FieldError[] {
  if (form.repeat === undefined) return validateForm(form, formState, group, escape);
  const errors: FieldError[] = [];
  for (let g = 0; g < formState.groupCount; g += 1) {
    errors.push(...validateForm(form, formState, g, escape));
  }
  return errors;
}

/** Whether the given group passes. See `isWholeFormValid` for a repeating form. */
export const isFormValid = (
  form: Form,
  formState: FormState,
  group: number,
  escape?: FormValidatorContext,
): boolean => validateForm(form, formState, group, escape).length === 0;

/** Whether every group a repeating form draws passes. */
export const isWholeFormValid = (
  form: Form,
  formState: FormState,
  group: number,
  escape?: FormValidatorContext,
): boolean => validateAllGroups(form, formState, group, escape).length === 0;
