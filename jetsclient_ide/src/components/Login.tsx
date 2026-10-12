import { useState, type FormEvent } from "react";

interface Props {
  onSubmit: (email: string, password: string) => Promise<void>;
  version: string;
  /**
   * Where *Create an account* goes, or absent for a build with no registration.
   *
   * Optional rather than required (task X.4) because this component is the
   * session gate and is rendered by `AppShell`, which has a router above it and
   * no reason to know about one. The link is a plain href for the same reason:
   * `/register` is outside the shell's route tree, so a `<Link>` here would be
   * navigating a router this component does not belong to.
   */
  registerHref?: string;
}

export function Login({ onSubmit, version, registerHref }: Props) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await onSubmit(email, password);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Sign in failed.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="login-page">
      <form className="login-card" onSubmit={submit}>
        {/*
          **JetStore, not Workspace IDE.** The app was named after one of its
          screens while it *was* one screen; it now serves the whole product and
          this is the first thing anyone sees. The url is still `/ide/` — that is
          I-26 and task X.2 — so this heading is half of the rename and says so
          rather than waiting for the other half.
        */}
        <h1>JetStore</h1>
        <p className="login-sub">Sign in with your JetStore account.</p>

        {/*
          **A login, not an email, and checked by the server rather than here.**
          The apiserver's built-in administrator signs in as `admin` by default
          (`-adminEmail`, jets/apiserver/main.go), which `type="email"` refused
          before the request was ever sent. Whether a login and password are
          valid is the database's to say (`Server.Login`); only registration
          requires an email address (`Register.tsx`, and `User.Validate` on the
          server). rules_unit_test_harness I-32, 2026-10-11.
        */}
        <label htmlFor="email">Email or login</label>
        <input
          id="email"
          type="text"
          autoCapitalize="none"
          autoCorrect="off"
          spellCheck={false}
          autoComplete="username"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          required
          autoFocus
        />

        <label htmlFor="password">Password</label>
        <input
          id="password"
          type="password"
          autoComplete="current-password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
        />

        {error && (
          <p className="login-error" role="alert">
            {error}
          </p>
        )}

        <button type="submit" className="btn btn-primary" disabled={busy}>
          {busy ? "Signing in…" : "Sign in"}
        </button>
        {registerHref !== undefined && (
          <p className="login-sub">
            No account? <a href={registerHref}>Create one</a>.
          </p>
        )}
        {version && <p className="login-version">JetStore {version}</p>}
      </form>
    </div>
  );
}
