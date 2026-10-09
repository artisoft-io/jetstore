/**
 * Create an account. Task X.4, and the last screen the Flutter app owned.
 *
 * **It is here rather than in track C because it is a retirement decision.**
 * `/register` is the only path by which a user account comes into existence —
 * `UserAdmin`'s action document has `update/users` and `delete/users` and nothing
 * that inserts one — so retiring `jetsclient` without this screen would mean no
 * new user could ever be created. That is why X.4 lands before X.1 rather than
 * after it, which is the opposite of the order the phase 3 plan's dependency
 * column gives.
 *
 * ## The rules are the Dart's, and they are stricter than they look
 *
 * `registrationFormValidator` (`jetsclient/lib/modules/actions/user_delegates.dart`):
 * a name of more than one character, an email of more than three, a password of
 * **at least fourteen**, and a confirmation that matches. The fourteen is not a
 * placeholder — it is the only password rule the product has, and dropping it
 * here would weaken it while looking like a port.
 *
 * The email field is lower-cased on the way in, matching the Dart's
 * `TextRestriction.allLower`. That is a real behaviour and not cosmetic: the
 * server looks the user up by the string it is given.
 */

import { useState, type FormEvent } from "react";

/** The Dart's `FSK.userPassword` rule: `value.length >= 14`. */
export const MIN_PASSWORD_LENGTH = 14;

interface Props {
  onSubmit: (name: string, email: string, password: string) => Promise<void>;
  onSignIn: () => void;
}

/**
 * The first rule that fails, or null.
 *
 * **Exported and pure so the rules can be tested without a DOM**, and ordered so
 * that a person fixing one problem is not told about a second they have not
 * reached yet — which is what the Dart's per-field validator does by construction
 * and a single form-level check has to do on purpose.
 */
export function registrationProblem(
  name: string,
  email: string,
  password: string,
  confirm: string,
): string | null {
  if (name.trim().length <= 1) return "Enter your name.";
  if (email.trim().length <= 3) return "Enter your email address.";
  if (password.length < MIN_PASSWORD_LENGTH) {
    return `Password must be at least ${MIN_PASSWORD_LENGTH} characters.`;
  }
  if (confirm !== password) return "The passwords do not match.";
  return null;
}

export function Register({ onSubmit, onSignIn }: Props) {
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    const problem = registrationProblem(name, email, password, confirm);
    if (problem !== null) {
      setError(problem);
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await onSubmit(name.trim(), email.trim(), password);
      setDone(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Registration failed.");
    } finally {
      setBusy(false);
    }
  }

  if (done) {
    return (
      <div className="login-page">
        <div className="login-card">
          <h1>JetStore</h1>
          <p className="login-sub" role="status">
            Registration successful. Sign in with your new account.
          </p>
          <button type="button" className="btn btn-primary" onClick={onSignIn}>
            Sign in
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="login-page">
      <form className="login-card" onSubmit={submit}>
        <h1>JetStore</h1>
        <p className="login-sub">Create an account.</p>

        <label htmlFor="reg-name">Name</label>
        <input
          id="reg-name"
          type="text"
          autoComplete="name"
          maxLength={80}
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
          autoFocus
        />

        <label htmlFor="reg-email">Email</label>
        <input
          id="reg-email"
          type="email"
          autoComplete="email"
          maxLength={80}
          value={email}
          // `TextRestriction.allLower` on the Dart field, applied as the user
          // types rather than on submit, so what they see is what is sent.
          onChange={(e) => setEmail(e.target.value.toLowerCase())}
          required
        />

        <label htmlFor="reg-password">Password</label>
        <input
          id="reg-password"
          type="password"
          autoComplete="new-password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
        />
        <p className="login-sub">At least {MIN_PASSWORD_LENGTH} characters.</p>

        <label htmlFor="reg-confirm">Confirm password</label>
        <input
          id="reg-confirm"
          type="password"
          autoComplete="new-password"
          value={confirm}
          onChange={(e) => setConfirm(e.target.value)}
          required
        />

        {error && (
          <p className="login-error" role="alert">
            {error}
          </p>
        )}

        <button type="submit" className="btn btn-primary" disabled={busy}>
          {busy ? "Creating account…" : "Create account"}
        </button>
        <button type="button" className="btn" onClick={onSignIn}>
          Back to sign in
        </button>
      </form>
    </div>
  );
}
