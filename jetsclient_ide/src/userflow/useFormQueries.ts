/**
 * Running a form's named queries, and re-running them. Task I.2b.
 *
 * The React half of `formQueries.ts`, kept separate from it so the rules — the
 * substitution, the readiness test, the suggestion ordering — are pure functions
 * a test can drive without a DOM. What is here is only the scheduling.
 *
 * ## The re-run trigger is the resolved statement, not the form state
 *
 * `FormState` notifies on every write, and most writes change nothing a query
 * reads. So the effect keys on `plan.signature` — a value derived from the
 * statements after substitution — which collapses the Dart's two guards into one:
 * `predicatePreviousValue`, which skips a re-query when the predicate is
 * unchanged, and `isKeyUpdated`, which skips one when the notification came from
 * the widget that owns the field (`components/dropdown_form_field.dart`,
 * `queryDropdownItems`). Both ask "did the input change"; a signature answers it
 * for every query at once and cannot be forgotten on a third.
 *
 * ## Results go into `FormState`, not into React state
 *
 * A validator escape and an action escape are handed a `FormState` and nothing
 * else (`actions/escapes.ts`, `EscapeContext`), and `mapFileUF`'s validator reads
 * two query results — so the rows have to live where an escape can reach them.
 * That is what `FormState.queryRows` is for, and it is why this hook returns a
 * *version* rather than the rows: the rows are already somewhere, and the version
 * is what tells React they changed.
 */

import { useCallback, useEffect, useState } from "react";

import type { FormState } from "../datatable/formState";
import type { JetsRow } from "../datatable/types";
import type { Form } from "./form";
import { planQueries, runQueries, type QueryPlan, type QueryPoster } from "./formQueries";

const NO_QUERIES: QueryPlan = { ready: {}, waiting: [], signature: "" };

export interface FormQueryState {
  /** The rows a query returned, or undefined while it has not run. */
  rows(name: string): JetsRow[] | undefined;
  /** True while a batch is in flight. */
  loading: boolean;
  /** The last failure, or null. A failure leaves the previous rows in place. */
  error: string | null;
}

export function useFormQueries(
  form: Form | null,
  formState: FormState,
  post: QueryPoster,
): FormQueryState {
  // **The plan is state rather than a ref, and the first render already holds
  // the right one.** With a ref plus a signature the mount sequence posts twice:
  // the running effect sees the initial signature, the recomputing effect then
  // changes it, and the running effect fires again for the same statements. The
  // plan's *identity* is the trigger instead, and it changes only when the
  // resolved statements do — so an unrelated write to form state costs a
  // comparison rather than a request.
  const [plan, setPlan] = useState<QueryPlan>(() =>
    form === null ? NO_QUERIES : planQueries(form, formState),
  );
  const [version, setVersion] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const recompute = (): void => {
      setPlan((current) => {
        const next = form === null ? NO_QUERIES : planQueries(form, formState);
        return next.signature === current.signature ? current : next;
      });
    };
    recompute();
    return formState.subscribe(recompute);
  }, [form, formState]);

  useEffect(() => {
    if (Object.keys(plan.ready).length === 0) return;
    let cancelled = false;
    setLoading(true);
    setError(null);
    void runQueries(plan, post)
      .then((rows) => {
        if (cancelled) return;
        for (const [name, returned] of Object.entries(rows)) {
          formState.setQueryRows(name, returned);
        }
        setVersion((n) => n + 1);
      })
      .catch((failure: unknown) => {
        if (cancelled) return;
        setError(failure instanceof Error ? failure.message : String(failure));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [plan, formState, post]);

  const rows = useCallback(
    (name: string) => {
      // `version` is read so the callback's identity changes when the rows do;
      // a component memoising on it would otherwise keep the first answer.
      void version;
      return formState.queryRows(name);
    },
    [formState, version],
  );

  return { rows, loading, error };
}
