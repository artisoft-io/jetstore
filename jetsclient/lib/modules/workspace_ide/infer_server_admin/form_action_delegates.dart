import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/components/dialogs.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/routes/export_routes.dart';
import 'package:jetsclient/utils/constants.dart';

/// Validation and Actions delegates for the Infer Server Admin screen.
///
/// The screen is a thin console over the /inferServer endpoint: the request box holds an
/// envelope naming an action, the server maps that action to an Ollama route it knows.
/// See jets/apiserver/api_infer_server.go for the allow list.

const _jsonEncoder = JsonEncoder.withIndent('  ');

/// Templates behind the macro buttons. Each is a complete, submittable envelope, so a
/// macro click followed by Submit is a working request with no editing needed.
///
/// stream is false on pull because Ollama streams NDJSON progress by default, which this
/// screen has no way to render — it wants a single json answer at the end.
const _macros = <String, Map<String, dynamic>>{
  ActionKeys.inferMacroListModels: {'action': 'list_models', 'body': {}},
  ActionKeys.inferMacroPullModel: {
    'action': 'pull_model',
    'body': {'model': 'granite4.1:3b', 'stream': false},
  },
  ActionKeys.inferMacroShowModel: {
    'action': 'show_model',
    'body': {'model': 'granite4.1:3b'},
  },
  ActionKeys.inferMacroDeleteModel: {
    'action': 'delete_model',
    'body': {'model': 'granite4.1:3b'},
  },
};

String? inferServerAdminFormValidator(
    JetsFormState formState, int group, String key, dynamic v) {
  switch (key) {
    case FSK.inferRequest:
      // Marking the key is what drives Submit: isFormValid reads the marked keys, not
      // the returned message, and enableOnlyWhenFormValid reads isFormValid.
      final value = v is String ? v : null;
      if (value == null || value.trim().isEmpty) {
        formState.markFormKeyAsInvalid(group, key);
        return "A request must be provided.";
      }
      formState.markFormKeyAsValid(group, key);
      return null;

    // Written by the delegate, never by the user.
    case FSK.inferResponse:
    case FSK.inferServerStatusLabel:
      return null;

    default:
      print(
          'Oops Infer Server Admin Form has no validator configured for form field $key');
  }
  return null;
}

/// True when the server is up. Start is offered in every other state, Stop in every state
/// but stopped: both underlying aws calls are idempotent, so acting on a server that is
/// mid-transition is harmless, and the looser rule cannot strand the screen with both
/// buttons dead if a transition stalls.
bool isInferServerRunning(JetsFormState formState) =>
    formState.getValue(0, FSK.inferServerState) == 'running';

bool isInferServerStopped(JetsFormState formState) =>
    formState.getValue(0, FSK.inferServerState) == 'stopped';

/// Post an envelope to the infer server endpoint. Returns the decoded body, or null when
/// the call failed — in which case the error has already been put on screen.
Future<Map<String, dynamic>?> _postInferServer(
    JetsFormState formState, Object envelope) async {
  final result = await HttpClientSingleton().sendRequest(
      path: ServerEPs.inferServerEP,
      token: JetsRouterDelegate().user.token,
      encodedJsonBody: jsonEncode(envelope));
  if (result.statusCode == 200) {
    final body = Map<String, dynamic>.from(result.body as Map<String, dynamic>);
    // The apiserver stamps a refreshed jwt into every response and the http client has
    // already consumed it. Drop it here: this response is rendered into a text box with a
    // copy button, and the session token has no business on the clipboard.
    body.remove('token');
    return body;
  }
  // The apiserver reports the reason in the body; surface it rather than a bare code,
  // since "server is stopped" and "you lack the capability" are both routine here.
  final reason = result.body is Map ? result.body['error'] : null;
  final message = reason ?? 'Request failed with status ${result.statusCode}.';
  formState.setValueAndNotify(0, FSK.inferResponse, message.toString());
  return null;
}

/// Read the lifecycle status out of a response and publish it to the form state.
void _applyStatus(JetsFormState formState, Map<String, dynamic>? body) {
  final status = body?['status'];
  if (status is! Map) {
    formState.setValue(0, FSK.inferServerState, 'unknown');
    formState.setValueAndNotify(
        0, FSK.inferServerStatusLabel, 'Status: unavailable');
    return;
  }
  final state = status['state']?.toString() ?? 'unknown';
  formState.setValue(0, FSK.inferServerState, state);
  formState.setValueAndNotify(
      0,
      FSK.inferServerStatusLabel,
      'Status: $state  '
      '(tasks ${status['runningTasks']}/${status['desiredTasks']}, '
      'instances ${status['instanceCount']}/${status['desiredCapacity']})');
}

Future<void> _refreshStatus(JetsFormState formState) async {
  final body = await _postInferServer(formState, {'action': 'server_status'});
  _applyStatus(formState, body);
}

/// Infer Server Admin Form Actions
Future<String?> inferServerAdminFormActions(BuildContext context,
    GlobalKey<FormState> formKey, JetsFormState formState, String actionKey,
    {group = 0}) async {
  switch (actionKey) {
    // Fired from FormConfig.onLoadActionKey, and by the Refresh button.
    case ActionKeys.inferServerLoad:
    case ActionKeys.inferServerRefresh:
      await _refreshStatus(formState);
      return null;

    case ActionKeys.inferServerStart:
    case ActionKeys.inferServerStop:
      final starting = actionKey == ActionKeys.inferServerStart;
      if (!starting) {
        final uc = await showDangerZoneDialog(context,
            'Stop the Infer Server? The GPU instance is terminated and any loaded model is unloaded.');
        if (uc != 'OK') return null;
      }
      formState.setValueAndNotify(0, FSK.inferServerStatusLabel,
          starting ? 'Status: starting...' : 'Status: stopping...');
      final body = await _postInferServer(
          formState, {'action': starting ? 'start_server' : 'stop_server'});
      _applyStatus(formState, body);
      if (body != null && context.mounted) {
        // changed false means it was already in the requested state, which is a
        // successful no-op rather than something to report as an error.
        final changed = body['changed'] == true;
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(
            content: Text(changed
                ? (starting
                    ? 'Infer Server starting, this takes several minutes.'
                    : 'Infer Server stopping, the instance takes several minutes to terminate.')
                : 'Infer Server was already ${starting ? 'running' : 'stopped'}.')));
      }
      return null;

    case ActionKeys.inferMacroListModels:
    case ActionKeys.inferMacroPullModel:
    case ActionKeys.inferMacroShowModel:
    case ActionKeys.inferMacroDeleteModel:
      // Relies on FormInputFieldConfig.syncWithFormState on the request field; without
      // it the state changes and the visible text box does not.
      formState.setValueAndNotify(
          0, FSK.inferRequest, _jsonEncoder.convert(_macros[actionKey]));
      return null;

    case ActionKeys.inferServerSubmit:
      if (!formKey.currentState!.validate()) return null;
      final request = formState.getValue(group, FSK.inferRequest) as String?;
      if (request == null) return null;
      Object envelope;
      try {
        envelope = jsonDecode(request);
      } on FormatException catch (e) {
        formState.setValueAndNotify(0, FSK.inferResponse,
            'The request is not valid json: ${e.message}');
        return null;
      }
      formState.setValueAndNotify(0, FSK.inferResponse, 'Working...');
      final body = await _postInferServer(formState, envelope);
      if (body == null) return null;
      // Lifecycle actions are legal to type by hand, so keep the status in sync when
      // one comes back through the request box rather than through the buttons.
      if (body.containsKey('status')) {
        _applyStatus(formState, body);
      }
      formState.setValueAndNotify(
          0, FSK.inferResponse, _jsonEncoder.convert(body));
      return null;

    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      break;
    default:
      print('Oops unknown ActionKey for Infer Server Admin Form: $actionKey');
  }
  return null;
}
