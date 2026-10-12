/**
 * @vitest-environment jsdom
 *
 * Sign in. rules_unit_test_harness I-32, 2026-10-11.
 *
 * **The login is checked by the server, not by the form.** The built-in
 * administrator's login is `admin` by default, which is not an email address;
 * a field of `type="email"` refused it before any request was sent, so the
 * administrator could not sign in to this app at all. Registration still
 * requires an email (`Register.test.tsx`); signing in does not.
 */

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { Login } from "./Login";

afterEach(cleanup);

function signIn(login: string, password: string) {
  const onSubmit = vi.fn().mockResolvedValue(undefined);
  render(<Login onSubmit={onSubmit} version="" />);
  fireEvent.change(screen.getByLabelText("Email or login"), { target: { value: login } });
  fireEvent.change(screen.getByLabelText("Password"), { target: { value: password } });
  fireEvent.click(screen.getByRole("button", { name: "Sign in" }));
  return onSubmit;
}

describe("signing in", () => {
  it("sends a login that is not an email address, as the administrator's is", async () => {
    const onSubmit = signIn("admin", "secret");
    await waitFor(() => expect(onSubmit).toHaveBeenCalledWith("admin", "secret"));
  });

  it("sends an email address unchanged", async () => {
    const onSubmit = signIn("ada@example.com", "secret");
    await waitFor(() => expect(onSubmit).toHaveBeenCalledWith("ada@example.com", "secret"));
  });

  it("does not ask the browser to validate an email format", () => {
    render(<Login onSubmit={vi.fn()} version="" />);
    const field = screen.getByLabelText("Email or login") as HTMLInputElement;
    expect(field.type).toBe("text");
    field.value = "admin";
    expect(field.checkValidity()).toBe(true);
  });

  it("shows the server's refusal rather than deciding itself", async () => {
    const onSubmit = vi.fn().mockRejectedValue(new Error("Invalid User or Password"));
    render(<Login onSubmit={onSubmit} version="" />);
    fireEvent.change(screen.getByLabelText("Email or login"), { target: { value: "nobody" } });
    fireEvent.change(screen.getByLabelText("Password"), { target: { value: "x" } });
    fireEvent.click(screen.getByRole("button", { name: "Sign in" }));
    expect((await screen.findByRole("alert")).textContent).toBe("Invalid User or Password");
  });
});
