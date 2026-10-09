/**
 * @vitest-environment jsdom
 *
 * The typeahead widget. Task I.2b.
 *
 * The ordering and filtering rules are `suggestionsFor`'s and are tested without
 * a DOM in `userflow/formQueries.test.ts`. What is asserted here is the part that
 * needs one: that the value written to **form state** is what the user typed or
 * chose, and that the listbox opens and closes when the Flutter widget's does.
 *
 * The form-state assertions matter more than the markup ones for the reason A.3
 * gives: the query builder downstream distinguishes an absent key from an empty
 * one, and getting that wrong does not look wrong on screen.
 */

import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { FormState } from "../datatable/formState";
import { Typeahead } from "./Typeahead";

afterEach(cleanup);

const COLUMNS = ["member_id", "member_dob", "claim_id"];

function setup(props: Partial<React.ComponentProps<typeof Typeahead>> = {}) {
  const formState = new FormState();
  render(
    <Typeahead
      formState={formState}
      group={0}
      fieldKey="input_column"
      label="Input Column"
      items={COLUMNS}
      {...props}
    />,
  );
  return { formState, input: screen.getByLabelText("Input Column") as HTMLInputElement };
}

const options = () => screen.queryAllByRole("option").map((o) => o.textContent);

describe("Typeahead", () => {
  it("seeds from the form state and writes edits back", () => {
    const formState = new FormState();
    formState.setValue(0, "input_column", "claim_id");
    render(
      <Typeahead
        formState={formState}
        group={0}
        fieldKey="input_column"
        label="Input Column"
        items={COLUMNS}
      />,
    );
    const input = screen.getByLabelText("Input Column") as HTMLInputElement;
    expect(input.value).toBe("claim_id");

    fireEvent.change(input, { target: { value: "member_id" } });
    expect(formState.getValue(0, "input_column")).toBe("member_id");
  });

  it("accepts a value that is not in the list", () => {
    // The Dart writes through on every keystroke; membership is enforced by
    // `mapFileUF`'s validator, which is what lets the user be *told* the column
    // does not exist rather than silently prevented from saying so.
    const { formState, input } = setup();
    fireEvent.change(input, { target: { value: "not_a_column" } });
    expect(formState.getValue(0, "input_column")).toBe("not_a_column");
  });

  it("clears the binding rather than storing an empty string", () => {
    const { formState, input } = setup();
    fireEvent.change(input, { target: { value: "x" } });
    fireEvent.change(input, { target: { value: "" } });
    expect(formState.getValue(0, "input_column")).toBeUndefined();
  });

  it("offers every suggestion on focus and filters as the user types", () => {
    const { input } = setup();
    expect(options()).toEqual([]);
    fireEvent.focus(input);
    expect(options()).toEqual(COLUMNS);
    fireEvent.change(input, { target: { value: "dob" } });
    expect(options()).toEqual(["member_dob"]);
  });

  it("orders by the priority target when the pattern is empty", () => {
    // `priorityTargetKey` is read from the *same group*, which is what makes a
    // repeating form order row i's suggestions by row i's data property.
    const formState = new FormState();
    formState.setValue(0, "data_property", "claim:id");
    render(
      <Typeahead
        formState={formState}
        group={0}
        fieldKey="input_column"
        label="Input Column"
        items={COLUMNS}
        priorityKey="data_property"
      />,
    );
    fireEvent.focus(screen.getByLabelText("Input Column"));
    expect(options()).toEqual(["member_id", "claim_id", "member_dob"]);
  });

  it("chooses a suggestion on mousedown and closes the list", () => {
    const { formState, input } = setup();
    fireEvent.focus(input);
    fireEvent.mouseDown(screen.getByText("member_dob"));
    expect(formState.getValue(0, "input_column")).toBe("member_dob");
    expect(options()).toEqual([]);
    expect(input.value).toBe("member_dob");
  });

  it("moves through the list with the arrow keys and takes one with Enter", () => {
    const { formState, input } = setup();
    fireEvent.focus(input);
    fireEvent.keyDown(input, { key: "ArrowDown" });
    fireEvent.keyDown(input, { key: "ArrowDown" });
    fireEvent.keyDown(input, { key: "Enter" });
    expect(formState.getValue(0, "input_column")).toBe("member_dob");
  });

  it("wraps at the end of the list", () => {
    const { formState, input } = setup();
    fireEvent.focus(input);
    // From nothing highlighted, Up means the last suggestion rather than the
    // second — index -1 is "no position", not a position to step from.
    fireEvent.keyDown(input, { key: "ArrowUp" });
    fireEvent.keyDown(input, { key: "Enter" });
    expect(formState.getValue(0, "input_column")).toBe("claim_id");

    fireEvent.focus(input);
    fireEvent.keyDown(input, { key: "ArrowUp" });
    fireEvent.keyDown(input, { key: "ArrowDown" }); // wraps past the end
    fireEvent.keyDown(input, { key: "Enter" });
    expect(formState.getValue(0, "input_column")).toBe("member_id");
  });

  it("closes on Escape and keeps the text", () => {
    // `_handleKeyEvent` binds Escape to `unfocus()`, which dismisses the list
    // without touching the field — so a user who typed a column that is not
    // offered can get the list out of the way.
    const { formState, input } = setup();
    fireEvent.change(input, { target: { value: "not_a_column" } });
    expect(options().length).toBe(0); // nothing matches, but the list is open
    fireEvent.change(input, { target: { value: "member" } });
    expect(options().length).toBe(2);
    fireEvent.keyDown(input, { key: "Escape" });
    expect(options()).toEqual([]);
    expect(input.value).toBe("member");
    expect(formState.getValue(0, "input_column")).toBe("member");
  });

  it("closes on blur, deferred so a click on a suggestion still lands", async () => {
    const { input } = setup();
    fireEvent.focus(input);
    fireEvent.blur(input);
    // The list is still there synchronously; that is the whole point.
    expect(options()).toEqual(COLUMNS);
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 1));
    });
    expect(options()).toEqual([]);
  });

  it("is a combobox and says whether it is expanded", () => {
    const { input } = setup();
    expect(input.getAttribute("role")).toBe("combobox");
    expect(input.getAttribute("aria-expanded")).toBe("false");
    fireEvent.focus(input);
    expect(input.getAttribute("aria-expanded")).toBe("true");
  });

  it("reports a validation message from the form", () => {
    setup({ error: "Input Column is not valid." });
    expect(screen.getByRole("alert").textContent).toBe("Input Column is not valid.");
    expect(screen.getByLabelText("Input Column").getAttribute("aria-invalid")).toBe("true");
  });

  it("says it is busy while its query is in flight", () => {
    setup({ loading: true, items: [] });
    expect(screen.getByLabelText("Input Column").getAttribute("aria-busy")).toBe("true");
  });
});
