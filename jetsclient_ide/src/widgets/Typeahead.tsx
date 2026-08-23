/**
 * The typeahead widget. Task I.2b, and the one widget the flow corpus needed
 * that track A did not build (F21, I-60).
 *
 * **One field, in one form, in one state** — `fmMappingFormUF`'s `input_column`
 * (`jetsclient/lib/modules/user_flows/file_mapping/form_config.dart`, the
 * `FormTypeaheadFieldConfig` inside `inputFieldRowBuilder`). It was invisible to
 * `widgets.test.tsx` and to the form-field corpus for the same reason: `allFields`
 * does not walk `inputFieldRowBuilder` (`jetsclient/test/corpus_support.dart`,
 * `allFields`), so the corpus reported the form as empty — truthfully, and
 * uselessly for the question F.0b was asking.
 *
 * ## It is a text input with a listbox, not a select
 *
 * The Dart wraps a `FormInputFieldConfig` and hands it to `TypeAheadField`
 * (`models/form_config.dart:437`, `inputFieldConfig`), and the value written to
 * form state is whatever the user typed — `onChanged` fires per keystroke, and
 * choosing a suggestion is a shortcut for typing it. That is why membership of the
 * suggestion list is enforced by `mapFileUF`'s validator and not by this control:
 * a user may type a column that does not exist, and be told so.
 *
 * So this builds on `useFormField` exactly as `TextInput` does rather than
 * sharing markup with `Dropdown`. The two are cousins in the Dart's class
 * hierarchy and not in behaviour.
 *
 * ## The listbox
 *
 * A combobox in the ARIA sense: an `input` owning a `listbox`, opened on focus and
 * on typing, closed on Escape or on blur. Keyboard is arrow keys plus Enter, which
 * the Flutter widget gets from `TypeAheadField` and which a plain `<input>` does
 * not have.
 *
 * **Escape closes the list and keeps the text.** The Dart binds Escape to
 * `_node.unfocus()` (`components/typeahead_form_field.dart`, `_handleKeyEvent`),
 * which dismisses the suggestions without touching the field — so a user who has
 * typed a column that is not in the list can dismiss the list rather than having
 * their text replaced.
 *
 * **Filtering and ordering are not here.** `suggestionsFor`
 * (`userflow/formQueries.ts`) is the port of `suggestionsCallback`, and it is a
 * pure function over the items so it can be tested without a DOM — which matters,
 * because the priority ordering is the part with a rule in it.
 */

import { useEffect, useId, useMemo, useRef, useState } from "react";

import { useFormField } from "./useFormField";
import { suggestionsFor } from "../userflow/formQueries";
import type { FormState } from "../datatable/formState";

export interface TypeaheadProps {
  formState: FormState;
  group: number;
  fieldKey: string;
  label: string;
  hint?: string;
  /** The suggestions, in query order. */
  items: readonly string[];
  /**
   * A sibling key whose value floats the suggestions resembling it.
   *
   * Read from the *same group*, not from group 0: the Dart reads
   * `formState.getValue(formConfig.group, formConfig.priorityTargetKey!)` and the
   * whole point in `fmMappingFormUF` is that row *i*'s suggestions are ordered by
   * row *i*'s data property.
   */
  priorityKey?: string;
  maxLength?: number;
  /** True while the query filling `items` is in flight. */
  loading?: boolean;
  error?: string;
}

export function Typeahead({
  formState,
  group,
  fieldKey,
  label,
  hint,
  items,
  priorityKey,
  maxLength = 0,
  loading = false,
  error,
}: TypeaheadProps) {
  const id = useId();
  const listId = `${id}-list`;
  const { value, setValue } = useFormField({ formState, group, fieldKey });
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(-1);
  // The pattern is the text since the list was opened, not the value: opening on
  // focus must offer everything, which is the Dart's empty-pattern branch, and a
  // field seeded from form state would otherwise open filtered to itself.
  const [pattern, setPattern] = useState("");
  const blurTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const priorityTarget =
    priorityKey === undefined ? null : asText(formState.getValue(group, priorityKey));

  const suggestions = useMemo(
    () => (open ? suggestionsFor(items, pattern, priorityTarget) : []),
    [open, items, pattern, priorityTarget],
  );

  useEffect(
    () => () => {
      if (blurTimer.current !== null) clearTimeout(blurTimer.current);
    },
    [],
  );

  const choose = (item: string) => {
    setValue(item);
    setPattern("");
    setOpen(false);
    setActive(-1);
  };

  const onKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "Escape") {
      if (open) event.preventDefault();
      setOpen(false);
      setActive(-1);
      return;
    }
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      if (!open) {
        setOpen(true);
        setActive(0);
        return;
      }
      const count = suggestions.length;
      if (count === 0) return;
      const down = event.key === "ArrowDown";
      // Nothing highlighted is its own case rather than index -1 arithmetic:
      // from nothing, Down means the first and Up means the last. Treating -1 as
      // a position puts Up on the *second* item, which is where this started.
      setActive((current) =>
        current < 0 ? (down ? 0 : count - 1) : (current + (down ? 1 : -1) + count) % count,
      );
      return;
    }
    if (event.key === "Enter" && open && active >= 0 && active < suggestions.length) {
      event.preventDefault();
      choose(suggestions[active]!);
    }
  };

  return (
    <div className="field field--typeahead">
      <label htmlFor={id}>{label}</label>
      <input
        id={id}
        type="text"
        role="combobox"
        aria-expanded={open}
        aria-controls={listId}
        aria-autocomplete="list"
        aria-invalid={error != null}
        aria-busy={loading}
        {...(active >= 0 && active < suggestions.length
          ? { "aria-activedescendant": `${listId}-${active}` }
          : {})}
        {...(hint ? { placeholder: hint } : {})}
        {...(maxLength > 0 ? { maxLength } : {})}
        {...(error != null ? { "aria-errormessage": `${id}-error` } : {})}
        value={value}
        onFocus={() => {
          setPattern("");
          setOpen(true);
        }}
        onBlur={() => {
          // Deferred, because a click on a suggestion blurs the input before the
          // option's own handler runs. The Flutter widget has the same problem
          // and solves it inside `TypeAheadField`.
          blurTimer.current = setTimeout(() => {
            setOpen(false);
            setActive(-1);
          }, 0);
        }}
        onChange={(event) => {
          setValue(event.target.value);
          setPattern(event.target.value);
          setOpen(true);
          setActive(-1);
        }}
        onKeyDown={onKeyDown}
      />
      <ul
        id={listId}
        role="listbox"
        aria-label={`${label} suggestions`}
        className="typeahead__list"
        hidden={!open || suggestions.length === 0}
      >
        {suggestions.map((item, index) => (
          <li
            id={`${listId}-${index}`}
            key={item}
            role="option"
            aria-selected={index === active}
            className={index === active ? "typeahead__item is-active" : "typeahead__item"}
            onMouseDown={(event) => {
              // `mousedown` rather than `click`: the deferred blur above would
              // otherwise race a click that lands after the list is hidden.
              event.preventDefault();
              choose(item);
            }}
          >
            {item}
          </li>
        ))}
      </ul>
      {error != null && (
        <p className="field-error" id={`${id}-error`} role="alert">
          {error}
        </p>
      )}
    </div>
  );
}

/** Form state holds `string | string[]`; the priority target is a scalar. */
function asText(raw: unknown): string | null {
  if (typeof raw === "string") return raw;
  if (Array.isArray(raw) && raw.length > 0 && typeof raw[0] === "string") return raw[0];
  return null;
}
