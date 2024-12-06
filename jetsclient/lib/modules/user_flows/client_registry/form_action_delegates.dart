import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/components/dialogs.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/modules/actions/delegate_helpers.dart';
import 'package:jetsclient/screens/user_flow_screen.dart';
import 'package:jetsclient/utils/constants.dart';

/// Validation delegate for Client Config UF
String? clientRegistryFormValidator(
    JetsFormState formState, int group, String key, dynamic v) {
  assert((v is String?) || (v is List<String>?),
      "ClientConfigFormValidator has unexpected data type");
  v = formState.getValue(group, key);
  switch (key) {
    case FSK.ufClientOrVendorOption:
      if (unpack(v) != null) {
        return null;
      }
      return "An option must be selected.";

    case FSK.client:
      if (unpack(v) != null) {
        return null;
      }
      return "Client name is required.";

    case FSK.org:
      if (v != null && v != '') {
        return null;
      }
      return "Vendor/Org name is required.";

    case FSK.ufClientDetails:
    case FSK.ufVendorDetails:
      break;

    default:
      print(
          'Oops clientConfigFormValidator Form Validator has no validator configured for form field $key');
  }
  return null;
}

/// Add Vendor/Org Dialog Action
Future<String?> clientRegistryAddOrgFormActions(BuildContext context,
    GlobalKey<FormState> formKey, JetsFormState formState, String actionKey,
    {group = 0}) async {
  final state = formState.getState(group);
  switch (actionKey) {
    case ActionKeys.crAddVendorOk:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      final stateCopy = Map<String, dynamic>.from(state);
      stateCopy[FSK.details] = state[FSK.ufVendorDetails];
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'insert_rows',
        'fromClauses': [
          <String, String>{'table': 'client_org_registry'}
        ],
        'data': [stateCopy],
      }, toEncodable: (_) => '');
      return postInsertRows(context, formState, encodedJsonBody);

    // Cancel Dialog / Form
    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      break;
    default:
      print('Oops unknown ActionKey for Client Config UF State: $actionKey');
  }
  return null;
}

/// Client Config UF Form Actions - set on UserFlowState
Future<String?> clientRegistryFormActions(
    UserFlowScreenState userFlowScreenState,
    BuildContext context,
    GlobalKey<FormState> formKey,
    JetsFormState formState,
    String actionKey,
    {group = 0}) async {
  final state = formState.getState(group);
  switch (actionKey) {
    case ActionKeys.crAddClientUF:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      state[FSK.details] = state[FSK.ufClientDetails];
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'insert_rows',
        'fromClauses': [
          <String, String>{'table': 'client_registry'}
        ],
        'data': [state],
      }, toEncodable: (_) => '');
      final statusCode = await postSimpleAction(
          context, formState, ServerEPs.dataTableEP, encodedJsonBody);
      state.remove(FSK.details);
      state.remove(FSK.ufClientDetails);
      if (statusCode == 200) return null;
      return "Error while creating client";

    case ActionKeys.crSelectClientUF:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      state[FSK.client] = unpack(state[FSK.client]);
      return null;

    case ActionKeys.crShowVendorUF:
      // Go to Page or State client_org - updating list of visited page
      final visitedPages =
          formState.getValue(group, FSK.ufVisitedPages) as List<String>;
      const nextStateKey = 'show_org';
      visitedPages.add(nextStateKey);
      // print("*** ActionKeys.ufNext visitedPages is now: $visitedPages");
      formState.setValue(group, FSK.ufCurrentPage, visitedPages.length - 1);
      final ufState = userFlowScreenState.userFlowConfig.states[nextStateKey];
      final fConfig = ufState!.formConfig;
      userFlowScreenState.setCurrentUserFlowState(ufState, fConfig);
      return null;

    case ActionKeys.deleteClient:
      // Get confirmation
      var uc = await showConfirmationDialog(
          context, 'Are you sure you want to delete the selected client?');
      if (uc != 'OK') return null;
      state[FSK.client] = unpack(state[FSK.client]);
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'insert_rows',
        'fromClauses': [
          <String, String>{'table': 'delete/client'}
        ],
        'data': [state],
      }, toEncodable: (_) => '');
      formState.clearSelectedRow(group, FSK.client);
      formState.getState(group).remove(FSK.client);
      if (context.mounted) {
        final statusCode = await postSimpleAction(
            context, formState, ServerEPs.dataTableEP, encodedJsonBody);
        state.remove(FSK.client);
        state.remove(FSK.details);
        state.remove(FSK.ufVendorDetails);
        state.remove(FSK.ufClientDetails);
        state.remove(FSK.org);
        if (statusCode == 200) return null;
        return "Error while creating client";
      }
      return null;

    case ActionKeys.deleteOrg:
      // Get confirmation
      var uc = await showConfirmationDialog(context,
          'Are you sure you want to delete the selected organization?');
      if (uc != 'OK') return null;
      state[FSK.client] = unpack(state[FSK.client]);
      state[FSK.org] = unpack(state[FSK.org]);
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'insert_rows',
        'fromClauses': [
          <String, String>{'table': 'delete/org'}
        ],
        'data': [state],
      }, toEncodable: (_) => '');
      formState.clearSelectedRow(group, FSK.org);
      formState.getState(group).remove(FSK.org);
      if (context.mounted) {
        final statusCode = await postSimpleAction(
            context, formState, ServerEPs.dataTableEP, encodedJsonBody);
        state.remove(FSK.details);
        state.remove(FSK.ufVendorDetails);
        state.remove(FSK.org);
        if (statusCode == 200) return null;
        return "Error while creating client";
      }
      return null;

    // Cancel Dialog / Form
    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      break;
    default:
      print('Oops unknown ActionKey for Client Config UF State: $actionKey');
  }
  return null;
}
