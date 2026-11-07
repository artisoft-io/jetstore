/**
 * A form for named, typed values: a runbook's parameters when a run starts, and
 * a `manual` step's declared `records` when a person fills them in. One
 * component because the two are the same declaration — a name, a type from the
 * format's closed list, perhaps a description and a default.
 *
 * Values are parsed by `parseTyped` before they are sent, so a number goes as a
 * number and a list as an array. The server validates them again and its answer
 * is the one that counts.
 */

import { useState, type FormEvent, type ReactNode } from "react";

import { initialText, parseTyped, type ValueType } from "./api";

export interface FieldDecl {
  name: string;
  type: ValueType;
  description?: string;
  default?: unknown;
}

export function TypedFields({
  fields,
  submitLabel,
  busy,
  disabled,
  onSubmit,
  onCancel,
  children,
}: {
  fields: FieldDecl[];
  submitLabel: string;
  busy: boolean;
  /** A reason of the caller's own, e.g. a missing capability. */
  disabled?: boolean;
  onSubmit(values: Record<string, unknown>): void;
  onCancel?: () => void;
  children?: ReactNode;
}) {
  const [text, setText] = useState<Record<string, string | boolean>>(() =>
    Object.fromEntries(fields.map((f) => [f.name, initialText(f.type, f.default)])),
  );
  const [errors, setErrors] = useState<Record<string, string>>({});

  const submit = (event: FormEvent) => {
    event.preventDefault();
    const values: Record<string, unknown> = {};
    const found: Record<string, string> = {};
    for (const f of fields) {
      const parsed = parseTyped(f.type, text[f.name] ?? "");
      if ("error" in parsed) found[f.name] = parsed.error;
      else values[f.name] = parsed.value;
    }
    setErrors(found);
    if (Object.keys(found).length === 0) onSubmit(values);
  };

  return (
    <form className="runbook-form" onSubmit={submit}>
      {children}
      {fields.length === 0 && <p className="empty-sub">Nothing to fill in.</p>}
      {fields.map((f) => {
        const id = `rbf-${f.name}`;
        const value = text[f.name] ?? "";
        const error = errors[f.name];
        const list = /^list</.test(f.type);
        return (
          <div className="field" key={f.name}>
            <label htmlFor={id}>
              <code>{f.name}</code> <span className="runbook-type">{f.type}</span>
              {f.description ? ` — ${f.description}` : ""}
            </label>
            {f.type === "boolean" ? (
              <input
                id={id}
                type="checkbox"
                checked={value === true}
                onChange={(e) => setText({ ...text, [f.name]: e.target.checked })}
              />
            ) : list ? (
              <textarea
                id={id}
                rows={3}
                placeholder="one value per line"
                value={String(value)}
                aria-invalid={error ? true : undefined}
                onChange={(e) => setText({ ...text, [f.name]: e.target.value })}
              />
            ) : (
              <input
                id={id}
                type="text"
                inputMode={f.type === "integer" || f.type === "numeric" || f.type === "session_id" ? "numeric" : undefined}
                placeholder={f.type === "timestamp" ? "2026-11-06T17:00:00Z" : undefined}
                value={String(value)}
                aria-invalid={error ? true : undefined}
                onChange={(e) => setText({ ...text, [f.name]: e.target.value })}
              />
            )}
            {error && <p className="field-error">{error}</p>}
          </div>
        );
      })}
      <div className="runbook-actions">
        <button type="submit" className="btn btn-primary" disabled={busy || disabled}>
          {submitLabel}
        </button>
        {onCancel && (
          <button type="button" className="btn" onClick={onCancel} disabled={busy}>
            Cancel
          </button>
        )}
      </div>
    </form>
  );
}
