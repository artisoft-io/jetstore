/**
 * The git profile screen. Task C.14.
 *
 * `ScreenWithForm` over `FormKeys.userGitProfile` in the Flutter app, reached
 * from the app bar rather than from a menu
 * (`jetsclient/lib/components/app_bar.dart`, the `userGitProfilePath` button).
 * It is where a user tells JetStore which name, address and handle to put on the
 * git commits the workspace IDE makes on their behalf, and which token to
 * authenticate them with.
 *
 * ## The route carries no parameters, and that is the one deliberate divergence
 *
 * Flutter's route is
 * `/userGitProfile/:user_email/:git_name/:git_email/:git_handle`, and the app bar
 * fills all four out of `JetsRouterDelegate().user` before navigating. **This
 * route is `/git-profile` and carries none of them**, for three reasons in
 * ascending order of weight:
 *
 *  - The repository's own conventions treat personal data in a url as a thing to
 *    avoid, and these four are a person's name, work address and account handle.
 *    A url is the most widely logged, cached and shoulder-surfed string a web
 *    application has.
 *  - **The parameters are seed values and nothing else.** No delegate reads them
 *    back: `gitProfileFormActions` posts `formState.getState(0)` with
 *    `user_email` **overwritten from the session**
 *    (`jetsclient/lib/modules/actions/user_delegates.dart`,
 *    `gitProfileFormActions`), so even in Flutter the identity that decides which
 *    row is written comes from the token rather than from the url.
 *  - This app already holds all four on the same authority Flutter does. They
 *    arrive with the sign-in response and sit on `ApiClient.currentUser`
 *    (`api/client.ts`, `User.gitProfile`), which is the React equivalent of the
 *    `UserModel` the app bar reads. Passing them through the url would be
 *    copying a value out of the client and back into it.
 *
 * **What that costs while both apps ship: nothing, and the reason is worth
 * stating rather than assuming.** Nothing hands a user from Flutter to this
 * screen — `migratedUserFlows` is empty and this is not a flow — and no Flutter
 * route would have to change for it to. If track X ever wants the handoff before
 * it retires `jetsclient`, the app bar would drop its `params` map and point at
 * `/ide/git-profile`; it would not need to learn a parameter shape, because there
 * is none to learn.
 *
 * ## What actually gates it
 *
 * `screen_reachability.json` gives `userGitProfileScreen` an empty `access` list,
 * and that is a statement about the *configuration* rather than a claim that
 * anyone may reach it (I-20). Read off the code, the gates are:
 *
 *  - **the screen**: an authenticated session. The Flutter app bar refuses to
 *    navigate unless `user.isAuthenticated`; here the shell's session gate is the
 *    same check one level up, so a signed-out user never renders this.
 *  - **the save**: the `user_profile` capability, on both ends. The Flutter form
 *    action declares `capability: "user_profile"`
 *    (`jetsclient/lib/modules/form_config_impl.dart`, `FormKeys.userGitProfile`)
 *    and the server requires it independently —
 *    `sqlInsertStmts["update/user_git_profile"]` carries
 *    `Capability: "user_profile"` (`jets/datatable/sql_stmts.go`). So the button
 *    below is `ActionButton`'s courtesy and the refusal is the server's, which is
 *    the split `shell/capabilities.tsx` states.
 *
 * ## No form document
 *
 * Five fields, one button, one cross-field rule. This is an ordinary routed
 * screen with plain markup rather than a `.form.json` through `FormRenderer` —
 * see `QueryTool.tsx`, which declines the same thing for a stronger reason and
 * carries the argument.
 */

import { useCallback, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";

import { ApiError, type ApiClient, type GitProfile } from "../api/client";
import { ActionButton } from "../shell/capabilities";
import { useNotifications } from "../shell/notifications";
import { restrict, type TextRestriction } from "../widgets/TextInput";

/** The capability the save requires, on both ends. */
export const USER_PROFILE = "user_profile";

/** The registered statement the save posts to. `jets/datatable/sql_stmts.go`. */
const UPDATE_STATEMENT = "update/user_git_profile";

interface Draft {
  gitName: string;
  gitEmail: string;
  gitHandle: string;
  gitToken: string;
  gitTokenConfirm: string;
}

/**
 * `gitProfileFormValidator`, field for field
 * (`jetsclient/lib/modules/actions/user_delegates.dart`).
 *
 * The thresholds are the Dart's and are reproduced rather than tidied: a name
 * must exceed one character, an address and a handle must exceed three, a token
 * must exceed five, and the confirmation must equal the token. None of them is a
 * format check — an address here is a git identity string, not an account, and
 * the server stores whatever it is given.
 */
export function validateDraft(draft: Draft): Partial<Record<keyof Draft, string>> {
  const errors: Partial<Record<keyof Draft, string>> = {};
  if (draft.gitName.length <= 1) {
    errors.gitName = draft.gitName.length === 1 ? "Name is too short." : "Name must be provided.";
  }
  if (draft.gitEmail.length <= 3) errors.gitEmail = "Email must be provided.";
  if (draft.gitHandle.length <= 3) {
    errors.gitHandle = "Git handle (user name) must be provided.";
  }
  // **The token is required even when only the name is being changed**, and that
  // is the server's shape rather than a strict form: the update writes
  // `git_token` unconditionally and only encrypts a non-empty one
  // (`jets/datatable/data_table_action.go`, the `user_git_profile` case), so
  // saving with the field blank would overwrite the stored token with the empty
  // string. The Dart validator guards the same edge.
  if (draft.gitToken.length <= 5) errors.gitToken = "Git token must be provided";
  if (draft.gitTokenConfirm !== draft.gitToken) {
    errors.gitTokenConfirm = "Git tokens does not match.";
  }
  return errors;
}

interface FieldProps {
  id: keyof Draft;
  label: string;
  hint: string;
  maxLength: number;
  restriction?: TextRestriction;
  secret?: boolean;
  autoFocus?: boolean;
  value: string;
  error?: string;
  onChange(value: string): void;
}

function Field({
  id,
  label,
  hint,
  maxLength,
  restriction = "none",
  secret = false,
  autoFocus = false,
  value,
  error,
  onChange,
}: FieldProps) {
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      <input
        id={id}
        name={id}
        type={secret ? "password" : "text"}
        value={value}
        placeholder={hint}
        maxLength={maxLength}
        autoFocus={autoFocus}
        // `allLower` on the handle and the address, as the Dart's input
        // formatters apply it — while typing rather than on submit, so what the
        // user sees is what is sent.
        onChange={(e) => onChange(restrict(e.target.value, restriction))}
        {...(error ? { "aria-invalid": true, "aria-errormessage": `${id}-error` } : {})}
      />
      {error != null && (
        <p className="field-error" id={`${id}-error`} role="alert">
          {error}
        </p>
      )}
    </div>
  );
}

export function GitProfileScreen({ api }: { api: ApiClient }) {
  const navigate = useNavigate();
  const { setError, setStatus } = useNotifications();

  const profile = api.currentUser?.gitProfile;
  const [draft, setDraft] = useState<Draft>(() => ({
    gitName: profile?.gitName ?? "",
    gitEmail: profile?.gitEmail ?? "",
    gitHandle: profile?.gitHandle ?? "",
    gitToken: "",
    gitTokenConfirm: "",
  }));
  // Shown only after a submit, so a form that has never been touched does not
  // open covered in required-field errors. That is the Flutter behaviour — its
  // action delegate calls `formKey.currentState!.validate()` first and the form
  // is not `autovalidate`.
  const [showErrors, setShowErrors] = useState(false);
  const [busy, setBusy] = useState(false);

  const errors = useMemo(() => validateDraft(draft), [draft]);
  const shown = showErrors ? errors : {};

  const set = useCallback(
    (key: keyof Draft) => (value: string) => setDraft((d) => ({ ...d, [key]: value })),
    [],
  );

  const submit = useCallback(async () => {
    setShowErrors(true);
    if (Object.keys(validateDraft(draft)).length > 0) return;

    const email = api.currentUser?.email ?? "";
    setBusy(true);
    setError(null);
    try {
      /*
       * `insert_rows` against a registered *update* statement — the action names
       * the dispatch arm and `fromClauses[0].table` names the statement, which is
       * how every write in this app reaches `sqlInsertStmts`.
       *
       * **The five keys are the statement's `ColumnKeys` and no more.** The Dart
       * posts the whole of form-state group 0, which carries the confirmation
       * field as well — a second copy of the token on the wire that the server
       * reads no column for. Sending only what the statement names is the one
       * place this screen sends *less* than Flutter, and it is deliberate.
       *
       * `user_email` is the signed-in account's, not a value from the url or the
       * form. The server takes whoever the request names — the statement's
       * `WHERE user_email = $5` is a parameter and `user_profile` is held by
       * three of the four seeded roles — so which email a client puts here is the
       * whole of who gets edited. Recorded as **I-187**.
       */
      await api.dataTable({
        action: "insert_rows",
        fromClauses: [{ table: UPDATE_STATEMENT }],
        data: [
          {
            git_name: draft.gitName,
            git_email: draft.gitEmail,
            git_handle: draft.gitHandle,
            git_token: draft.gitToken,
            user_email: email,
          },
        ],
      });
      const saved: GitProfile = {
        gitName: draft.gitName,
        gitEmail: draft.gitEmail,
        gitHandle: draft.gitHandle,
      };
      api.setGitProfile(saved);
      setStatus("Git profile updated.");
      // Leaving the screen is the Flutter behaviour and it also stops the token
      // sitting in a mounted input for the rest of the session. `/` redirects to
      // the workspace IDE until C.6 lands the home screen this corresponds to.
      void navigate("/");
    } catch (error) {
      setError(
        error instanceof ApiError
          ? `Saving the git profile failed: ${error.message}`
          : "Something went wrong. Please try again.",
      );
    } finally {
      setBusy(false);
    }
  }, [api, draft, navigate, setError, setStatus]);

  return (
    <main className="screen">
      <h1>Edit Git Profile</h1>
      <p className="screen-sub">
        The identity JetStore commits with when the workspace IDE pushes on your behalf.
      </p>

      <form
        className="stack"
        onSubmit={(e) => {
          e.preventDefault();
          void submit();
        }}
      >
        <Field
          id="gitName"
          label="Name"
          hint="Enter your name for git commits"
          maxLength={80}
          autoFocus
          value={draft.gitName}
          {...(shown.gitName ? { error: shown.gitName } : {})}
          onChange={set("gitName")}
        />
        <Field
          id="gitHandle"
          label="Git Handle"
          hint="Your git handle (user name) for git commit"
          maxLength={60}
          restriction="allLower"
          value={draft.gitHandle}
          {...(shown.gitHandle ? { error: shown.gitHandle } : {})}
          onChange={set("gitHandle")}
        />
        <Field
          id="gitEmail"
          label="Email"
          hint="Your email address for git commit"
          maxLength={80}
          restriction="allLower"
          value={draft.gitEmail}
          {...(shown.gitEmail ? { error: shown.gitEmail } : {})}
          onChange={set("gitEmail")}
        />
        <Field
          id="gitToken"
          label="Github Token"
          hint="Github token to use as password"
          maxLength={120}
          secret
          value={draft.gitToken}
          {...(shown.gitToken ? { error: shown.gitToken } : {})}
          onChange={set("gitToken")}
        />
        <Field
          id="gitTokenConfirm"
          label="Github Token Confirmation"
          hint="Re-enter your github token"
          maxLength={120}
          secret
          value={draft.gitTokenConfirm}
          {...(shown.gitTokenConfirm ? { error: shown.gitTokenConfirm } : {})}
          onChange={set("gitTokenConfirm")}
        />

        <div className="screen-actions">
          <ActionButton
            capability={USER_PROFILE}
            className="btn btn-primary"
            disabled={busy}
            onClick={() => void submit()}
          >
            {busy ? "Saving…" : "Submit"}
          </ActionButton>
        </div>
      </form>
    </main>
  );
}
