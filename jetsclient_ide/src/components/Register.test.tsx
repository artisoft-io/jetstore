/**
 * @vitest-environment jsdom
 *
 * Registration. Task X.4.
 *
 * Three things are worth testing here and only one of them is the form. The
 * rules are the Dart's and are stricter than they look; the status codes decide
 * where a person goes next; and the route must work with **no session**, which is
 * the property that makes this screen X.4's rather than track C's.
 */

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ApiClient, ApiError } from "../api/client";
import { MIN_PASSWORD_LENGTH, Register, registrationProblem } from "./Register";

// Auto-cleanup is not configured globally, so a second render finds the first
// one still mounted — which is a duplicate-element error rather than a failure
// that says so.
afterEach(cleanup);

const GOOD = "correct-horse-battery";

describe("the rules, which are the Dart's", () => {
  it("accepts what registrationFormValidator accepts", () => {
    expect(registrationProblem("Michel", "a@b.co", GOOD, GOOD)).toBeNull();
  });

  it("refuses a one-character name, as `characters.length > 1` does", () => {
    expect(registrationProblem("M", "a@b.co", GOOD, GOOD)).toBe("Enter your name.");
  });

  it("refuses an email of three characters or fewer", () => {
    expect(registrationProblem("Michel", "a@b", GOOD, GOOD)).toBe("Enter your email address.");
  });

  /**
   * **Fourteen, and it is the product's only password rule.** A port that relaxed
   * it would look identical and be weaker, which is exactly the class of change
   * nobody notices in a diff of a new file.
   */
  it("requires fourteen characters, not eight", () => {
    expect(registrationProblem("Michel", "a@b.co", "x".repeat(13), "x".repeat(13))).toBe(
      `Password must be at least ${MIN_PASSWORD_LENGTH} characters.`,
    );
    expect(registrationProblem("Michel", "a@b.co", "x".repeat(14), "x".repeat(14))).toBeNull();
  });

  it("requires the confirmation to match", () => {
    expect(registrationProblem("Michel", "a@b.co", GOOD, `${GOOD}!`)).toBe(
      "The passwords do not match.",
    );
  });

  /**
   * The order is the point: a person fixing the name is not simultaneously told
   * their password is short. The Dart gets this free from per-field validators;
   * one form-level check has to be deliberate about it.
   */
  it("reports the first problem rather than all of them", () => {
    expect(registrationProblem("", "", "", "")).toBe("Enter your name.");
  });
});

function fill(values: Record<string, string>) {
  for (const [label, value] of Object.entries(values)) {
    // Exact, because "Password" is a prefix of "Confirm password" and the loose
    // matcher would find two.
    fireEvent.change(screen.getByLabelText(label, { exact: true }), { target: { value } });
  }
}

describe("the screen", () => {
  it("posts the three fields the endpoint unmarshals, and lower-cases the email", async () => {
    const onSubmit = vi.fn(async () => {});
    render(<Register onSubmit={onSubmit} onSignIn={() => {}} />);
    fill({
      Name: "Michel",
      Email: "Michel@Artisoft.IO",
      Password: GOOD,
      "Confirm password": GOOD,
    });
    fireEvent.click(screen.getByRole("button", { name: "Create account" }));
    await waitFor(() => expect(onSubmit).toHaveBeenCalled());
    // The name is sent as typed; only the email is lower-cased, which is the
    // Dart's `TextRestriction.allLower` on that field and not on the other.
    expect(onSubmit).toHaveBeenCalledWith("Michel", "michel@artisoft.io", GOOD);
  });

  it("does not post when a rule fails, and says which", async () => {
    const onSubmit = vi.fn(async () => {});
    render(<Register onSubmit={onSubmit} onSignIn={() => {}} />);
    fill({ Name: "Michel", Email: "a@b.co", Password: "short", "Confirm password": "short" });
    fireEvent.click(screen.getByRole("button", { name: "Create account" }));
    expect((await screen.findByRole("alert")).textContent).toBe(
      `Password must be at least ${MIN_PASSWORD_LENGTH} characters.`,
    );
    expect(onSubmit).not.toHaveBeenCalled();
  });

  /**
   * **Registration does not sign you in**, which is the Dart's behaviour: the
   * endpoint mints a token and the client throws it away. Reproduced rather than
   * improved, because auto-signing-in on a form submission is a different
   * security posture than the product has today.
   */
  it("offers sign-in rather than entering the app", async () => {
    const onSignIn = vi.fn();
    render(<Register onSubmit={async () => {}} onSignIn={onSignIn} />);
    fill({ Name: "Michel", Email: "a@b.co", Password: GOOD, "Confirm password": GOOD });
    fireEvent.click(screen.getByRole("button", { name: "Create account" }));
    expect((await screen.findByRole("status")).textContent).toContain("Registration successful");
    fireEvent.click(screen.getByRole("button", { name: "Sign in" }));
    expect(onSignIn).toHaveBeenCalled();
  });

  it("surfaces the server's refusal", async () => {
    const onSubmit = vi.fn(async () => {
      throw new ApiError("That email is already registered. Sign in instead.", 409);
    });
    render(<Register onSubmit={onSubmit} onSignIn={() => {}} />);
    fill({ Name: "Michel", Email: "a@b.co", Password: GOOD, "Confirm password": GOOD });
    fireEvent.click(screen.getByRole("button", { name: "Create account" }));
    expect((await screen.findByRole("alert")).textContent).toContain("already registered");
  });
});

describe("the api client", () => {
  function client(status: number, body: unknown = {}) {
    const calls: { url: string; body: Record<string, unknown> }[] = [];
    const fetchImpl = vi.fn(async (url: string | URL, init?: RequestInit) => {
      calls.push({
        url: String(url),
        body: JSON.parse(String(init?.body ?? "{}")) as Record<string, unknown>,
      });
      return new Response(JSON.stringify(body), { status });
    }) as unknown as typeof fetch;
    return { api: new ApiClient("", fetchImpl), calls };
  }

  it("sends `user_email`, not `email` — the field user.User unmarshals", async () => {
    const { api, calls } = client(200);
    await api.register("Michel", "michel@artisoft.io", GOOD);
    expect(calls[0]?.url).toBe("/register");
    expect(calls[0]?.body).toEqual({
      name: "Michel",
      user_email: "michel@artisoft.io",
      password: GOOD,
    });
  });

  /**
   * The three codes the Dart distinguishes. 409 is the one that matters: *this
   * email is taken* and *something went wrong* send a person to different places,
   * and conflating them costs a support ticket.
   */
  it("tells a taken email apart from a bad one and from a failure", async () => {
    await expect(client(409).api.register("M", "a@b.co", GOOD)).rejects.toThrow(
      "already registered",
    );
    await expect(client(422).api.register("M", "a@b.co", GOOD)).rejects.toThrow(
      "Invalid email or password.",
    );
    await expect(client(500).api.register("M", "a@b.co", GOOD)).rejects.toThrow(
      "Registration failed. Please try again.",
    );
  });

  it("leaves the client signed out on success", async () => {
    const { api } = client(200, { token: "t0", name: "Michel" });
    await api.register("Michel", "michel@artisoft.io", GOOD);
    expect(api.isAuthenticated).toBe(false);
  });
});

describe("reachability", () => {
  /**
   * **The property that makes this X.4's.** Every other screen is inside
   * `AppShell`, which renders the sign-in form for anyone without a user. A
   * person registering has no account, so this route has to render with no
   * session at all — and it is the only one in the app that does.
   */
  it("renders with no session, outside the shell", () => {
    render(
      <MemoryRouter>
        <Register onSubmit={async () => {}} onSignIn={() => {}} />
      </MemoryRouter>,
    );
    expect(screen.getByRole("button", { name: "Create account" })).not.toBeNull();
    expect(screen.queryByLabelText("Confirm password")).not.toBeNull();
  });
});
