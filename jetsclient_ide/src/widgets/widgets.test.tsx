/**
 * @vitest-environment jsdom
 *
 * Tests for the text input and dropdown widgets (task A.3).
 *
 * The assertions that matter are the ones about **form state**, not about
 * markup: these widgets exist to feed the same store the data table publishes
 * into (I-9), and the query builder downstream distinguishes an absent key from
 * an empty one. Getting that wrong would not look wrong on screen.
 */

import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import corpus from "../datatable/fixtures/form_fields.json";
import { FormState } from "../datatable/formState";
import { Dropdown } from "./Dropdown";
import { TextInput, restrict } from "./TextInput";

afterEach(cleanup);

const field = { group: 0, fieldKey: "client" };

describe("restrict", () => {
  it.each([
    ["none", "Ab1", "Ab1"],
    ["allLower", "AbC", "abc"],
    ["allUpper", "AbC", "ABC"],
    ["digitsOnly", "a1b2", "12"],
  ] as const)("%s", (restriction, input, expected) => {
    expect(restrict(input, restriction)).toBe(expected);
  });
});

describe("TextInput", () => {
  it("seeds from the form state and writes edits back", () => {
    const formState = new FormState();
    formState.setValue(0, "client", "acme");
    render(<TextInput formState={formState} {...field} label="Client" />);

    const input = screen.getByLabelText("Client") as HTMLInputElement;
    expect(input.value).toBe("acme");

    fireEvent.change(input, { target: { value: "beta" } });
    expect(formState.getValue(0, "client")).toBe("beta");
  });

  it("clears the binding rather than storing an empty string", () => {
    // A.4a's where-clause fallback tests for *absence*; an empty string would
    // filter on "" and return nothing, which looks like a broken query.
    const formState = new FormState();
    formState.setValue(0, "client", "acme");
    render(<TextInput formState={formState} {...field} label="Client" />);

    fireEvent.change(screen.getByLabelText("Client"), { target: { value: "" } });
    expect(formState.getValue(0, "client")).toBeUndefined();
  });

  it("writes its default back, rather than only displaying it", () => {
    // A form submitted without touching the field must carry the default.
    const formState = new FormState();
    render(<TextInput formState={formState} {...field} label="Client" defaultValue="acme" />);
    expect((screen.getByLabelText("Client") as HTMLInputElement).value).toBe("acme");
    expect(formState.getValue(0, "client")).toBe("acme");
  });

  it("does not overwrite a value the form state already holds", () => {
    const formState = new FormState();
    formState.setValue(0, "client", "held");
    render(<TextInput formState={formState} {...field} label="Client" defaultValue="acme" />);
    expect(formState.getValue(0, "client")).toBe("held");
  });

  it("applies the text restriction as the user types", () => {
    const formState = new FormState();
    render(
      <TextInput formState={formState} {...field} label="Year" textRestriction="digitsOnly" />,
    );
    fireEvent.change(screen.getByLabelText("Year"), { target: { value: "20a26" } });
    expect(formState.getValue(0, "client")).toBe("2026");
  });

  it("renders a textarea when the field wants more than one line", () => {
    // Eight fields in the corpus set maxLines above 1; the largest maxLength is
    // 102,400, so this is not a corner case.
    const formState = new FormState();
    render(<TextInput formState={formState} {...field} label="Notes" maxLines={6} />);
    expect(screen.getByLabelText("Notes").tagName).toBe("TEXTAREA");
  });

  it("ignores form-state changes by default, and follows them when asked", async () => {
    // The Dart seeds once and lets the text box own the value; syncWithFormState
    // is the exception, and no field in the nine flows sets it.
    const formState = new FormState();
    const { unmount } = render(<TextInput formState={formState} {...field} label="Client" />);
    await act(async () => {
      formState.setValue(0, "client", "elsewhere");
      formState.notifyListeners();
    });
    expect((screen.getByLabelText("Client") as HTMLInputElement).value).toBe("");
    unmount();

    const synced = new FormState();
    render(<TextInput formState={synced} {...field} label="Client" syncWithFormState />);
    await act(async () => {
      synced.setValue(0, "client", "elsewhere");
      synced.notifyListeners();
    });
    expect((screen.getByLabelText("Client") as HTMLInputElement).value).toBe("elsewhere");
  });

  it("marks itself invalid and exposes the message", () => {
    const formState = new FormState();
    render(<TextInput formState={formState} {...field} label="Client" error="Required" />);
    expect(screen.getByLabelText("Client").getAttribute("aria-invalid")).toBe("true");
    expect(screen.getByRole("alert").textContent).toBe("Required");
  });
});

const items = [
  { value: "csv", label: "CSV" },
  { value: "parquet", label: "Parquet" },
];

describe("Dropdown", () => {
  it("selects the default item and writes it to the form state", () => {
    const formState = new FormState();
    render(
      <Dropdown
        formState={formState}
        group={0}
        fieldKey="format"
        label="Format"
        items={items}
        defaultItemPos={1}
      />,
    );
    expect((screen.getByLabelText("Format") as HTMLSelectElement).value).toBe("parquet");
    expect(formState.getValue(0, "format")).toBe("parquet");
  });

  it("keeps a value the form state already holds", () => {
    const formState = new FormState();
    formState.setValue(0, "format", "csv");
    render(
      <Dropdown formState={formState} group={0} fieldKey="format" label="Format" items={items} defaultItemPos={1} />,
    );
    expect((screen.getByLabelText("Format") as HTMLSelectElement).value).toBe("csv");
  });

  it("writes the selection through", () => {
    const formState = new FormState();
    render(<Dropdown formState={formState} group={0} fieldKey="format" label="Format" items={items} />);
    fireEvent.change(screen.getByLabelText("Format"), { target: { value: "parquet" } });
    expect(formState.getValue(0, "format")).toBe("parquet");
  });

  it("waits while a query is in flight rather than showing an empty list", () => {
    // Five of the eleven dropdowns populate from a named server query.
    const formState = new FormState();
    render(
      <Dropdown formState={formState} group={0} fieldKey="format" label="Format" items={[]} loading />,
    );
    const select = screen.getByLabelText("Format") as HTMLSelectElement;
    expect(select.disabled).toBe(true);
    expect(select.getAttribute("aria-busy")).toBe("true");
  });

  it("re-seeds when items arrive late and the held value is not among them", () => {
    // The failure this prevents: a query returns and the field keeps a value the
    // user can no longer see, which the form then submits.
    const formState = new FormState();
    formState.setValue(0, "format", "gone");
    const { rerender } = render(
      <Dropdown formState={formState} group={0} fieldKey="format" label="Format" items={[]} loading />,
    );
    rerender(
      <Dropdown formState={formState} group={0} fieldKey="format" label="Format" items={items} />,
    );
    expect(formState.getValue(0, "format")).toBe("csv");
  });
});

describe("the field corpus", () => {
  const forms = (corpus as { forms: Record<string, { fields: { type: string }[] }> }).forms;
  const fields = Object.values(forms).flatMap((f) => f.fields);

  it("is the 50 flow forms, and A.3 covers the two scalar input types in them", () => {
    expect(Object.keys(forms)).toHaveLength(50);
    const counts = fields.reduce<Record<string, number>>((acc, f) => {
      acc[f.type] = (acc[f.type] ?? 0) + 1;
      return acc;
    }, {});
    expect(counts["FormInputFieldConfig"]).toBe(46);
    expect(counts["FormDropdownFieldConfig"]).toBe(11);
    // The data-table field is A.4's, and its 37 match the 37 table configurations
    // exactly — the cross-check that says the traversal reaches everything.
    expect(counts["FormDataTableFieldConfig"]).toBe(37);
  });

  it("contains none of the three exotic field types", () => {
    // `FormTypeaheadFieldConfig` and `FormDropdownWithSharedItemsFieldConfig`
    // have three constructor sites between them, all inside `file_mapping`'s
    // `inputFieldRowBuilder` — a closure that builds rows per record at run
    // time, so they are unreachable statically. `file_mapping` is the flow the
    // plan already excludes from Phase 2's proof flows.
    for (const type of ["FormTypeaheadFieldConfig", "FormDropdownWithSharedItemsFieldConfig"]) {
      expect(fields.some((f) => f.type === type)).toBe(false);
    }
  });
});
