import 'package:flutter/material.dart';
import 'package:jetsclient/models/form_config.dart';
import 'package:jetsclient/modules/workspace_ide/infer_server_admin/form_action_delegates.dart';
import 'package:jetsclient/utils/constants.dart';

/// Form Config for the Infer Server Admin screen.
///
/// Uses inputFieldsV2 rather than inputFields so each row can carry its own flex: the two
/// button rows must take their natural height while the request and response boxes share
/// what is left. Every row of inputFields gets the same flex, which would make the button
/// row as tall as the text areas.
///
/// The six top buttons are FormActionConfig used as form *fields*; only Submit lives in
/// actions, which is what puts it in the bottom right.

const _infer = "infer_server_admin";

/// Start and Stop are gated on the reported state rather than on form validity, which is
/// form-wide and already spoken for by Submit.
FormActionConfig _lifecycleButton(
        {required String key,
        required String label,
        required ActionStyle style,
        required EnabledEvaluator isEnabledEval}) =>
    FormActionConfig(
        key: key,
        capability: _infer,
        label: label,
        buttonStyle: style,
        isEnabledEval: isEnabledEval,
        leftMargin: defaultPadding,
        rightMargin: betweenTheButtonsPadding);

FormActionConfig _macroButton({required String key, required String label}) =>
    FormActionConfig(
        key: key,
        capability: _infer,
        label: label,
        buttonStyle: ActionStyle.secondary,
        leftMargin: betweenTheButtonsPadding,
        rightMargin: betweenTheButtonsPadding);

final Map<String, FormConfig> _formConfigurations = {
  FormKeys.inferServerAdminForm: FormConfig(
    key: FormKeys.inferServerAdminForm,
    // Fetch the server state as soon as the screen is up, so the Start/Stop buttons
    // reflect reality rather than defaulting to something and being wrong.
    onLoadActionKey: ActionKeys.inferServerLoad,
    useListView: false,
    actions: [
      FormActionConfig(
          key: ActionKeys.inferServerSubmit,
          capability: _infer,
          label: "Submit",
          // Paired with the validator marking the request field invalid when empty.
          enableOnlyWhenFormValid: true,
          buttonStyle: ActionStyle.dialogOk,
          leftMargin: betweenTheButtonsPadding,
          rightMargin: defaultPadding),
    ],
    inputFieldsV2: [
      // Lifecycle and macro buttons across the top
      FormFieldRowConfig(rowConfig: [
        _lifecycleButton(
            key: ActionKeys.inferServerStart,
            label: "Start",
            style: ActionStyle.predominentInForm,
            // Offered in every state but running: the underlying call is idempotent, so
            // pressing it mid-transition is harmless, and gating strictly on "stopped"
            // would strand the screen if a transition stalled.
            isEnabledEval: (formState) => !isInferServerRunning(formState)),
        _lifecycleButton(
            key: ActionKeys.inferServerStop,
            label: "Stop",
            style: ActionStyle.danger,
            isEnabledEval: (formState) => !isInferServerStopped(formState)),
        _macroButton(key: ActionKeys.inferMacroListModels, label: "Models"),
        _macroButton(key: ActionKeys.inferMacroPullModel, label: "Pull Model"),
        _macroButton(
            key: ActionKeys.inferMacroShowModel, label: "Model Details"),
        _macroButton(key: ActionKeys.inferMacroDeleteModel, label: "Delete"),
      ]),
      // Status line, written by the delegate and refreshed on demand
      FormFieldRowConfig(rowConfig: [
        FormInputFieldConfig(
            key: FSK.inferServerStatusLabel,
            label: "Infer Server",
            hint: "",
            flex: 5,
            autofocus: false,
            obscureText: false,
            isReadOnly: true,
            syncWithFormState: true,
            defaultValue: "Status: loading...",
            textRestriction: TextRestriction.none,
            maxLines: 1,
            maxLength: 512),
        FormActionConfig(
            key: ActionKeys.inferServerRefresh,
            capability: _infer,
            label: "Refresh",
            buttonStyle: ActionStyle.secondary,
            leftMargin: betweenTheButtonsPadding,
            rightMargin: defaultPadding),
      ]),
      // The request envelope. syncWithFormState is what lets the macro buttons fill it in.
      FormFieldRowConfig(flex: 2, rowConfig: [
        FormInputFieldConfig(
            key: FSK.inferRequest,
            label: "Request",
            hint: "Paste a request envelope, or use a button above",
            autofocus: false,
            obscureText: false,
            syncWithFormState: true,
            // Needed for Submit's enableOnlyWhenFormValid to track what is typed.
            autovalidateMode: AutovalidateMode.always,
            textRestriction: TextRestriction.none,
            maxLines: 10,
            maxLength: 4000000),
      ]),
      FormFieldRowConfig(flex: 3, rowConfig: [
        FormInputFieldConfig(
            key: FSK.inferResponse,
            label: "Response",
            hint: "The infer server's response appears here",
            autofocus: false,
            obscureText: false,
            isReadOnly: true,
            syncWithFormState: true,
            // Model listings and pull results run long; copy is how you get the whole
            // thing out to an editor.
            showCopyToClipboard: true,
            textRestriction: TextRestriction.none,
            maxLines: 20,
            maxLength: 4000000),
      ]),
    ],
    formValidatorDelegate: inferServerAdminFormValidator,
    formActionsDelegate: inferServerAdminFormActions,
  ),
};

/// Every key _formConfigurations holds — the infer server admin screens.
///
/// Exists for `test/screen_config_corpus_test.dart`, which serialises these
/// configurations for the React port. **It enumerates rather than hand-lists**,
/// because the registry's key space is not one constant class: 13 of the user
/// flows' 37 table keys are `FSK` constants rather than `DTKeys`, so a list
/// derived from the declared constants of any one class is incomplete by
/// construction. Nothing in the app reads this.
Iterable<String> get inferServerAdminFormConfigKeys => _formConfigurations.keys;


FormConfig? getInferServerAdminFormConfig(String key) {
  return _formConfigurations[key];
}
