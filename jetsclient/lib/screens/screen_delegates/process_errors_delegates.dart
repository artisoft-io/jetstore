import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/screens/components/dialogs.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/screens/screen_delegates/delegate_helpers.dart';

/// User Administration Form Actions
Future<String?> processErrorsActions(BuildContext context,
    GlobalKey<FormState> formKey, JetsFormState formState, String actionKey,
    {int group = 0}) async {
  switch (actionKey) {
    // View Rete Session V1
    case ActionKeys.setupShowReteTriples:
      // Preparing to view rdf graph as triples from the process_errors table
      // Need to:
      //  Fetch process_errors.rete_session_triples by process_errors.key
      var state = formState.getState(0);
      final keyList = state[FSK.key];
      if (keyList == null) {
        print("Error: null process_errors.key (FSK.key) in formState");
        return "Error: null process_errors.key (FSK.key) in formState";
      }
      final key = keyList[0].toString();
      var rawQuery = <String, dynamic>{
        'action': 'raw_query',
      };
      rawQuery['query'] =
          "SELECT rete_session_triples FROM jetsapi.process_errors WHERE key = $key";
      final rows = await queryJetsDataModel(
          context, formState, ServerEPs.dataTableEP, json.encode(rawQuery));
      if (rows == null) {
        return "No rows returned";
      }
      var triples = rows[0][0];
      if (triples != null) {
        formState.setValue(0, FSK.reteSessionTriples, json.decode(triples));
      }
      break;

    // View Rete Session V2 -- Rete Session Explorer
    case ActionKeys.setupShowReteTriplesV2:
      // Preparing to view rdf graph as triples from the process_errors table
      // Need to:
      //  Fetch process_errors.rete_session_triples by process_errors.key
      var state = formState.getState(0);
      final keyList = state[FSK.key];
      if (keyList == null) {
        print("Error: null process_errors.key (FSK.key) in formState");
        return "Error: null process_errors.key (FSK.key) in formState";
      }
      final key = keyList[0].toString();
      var rawQuery = <String, dynamic>{
        'action': 'raw_query',
      };
      rawQuery['query'] =
          "SELECT rete_session_triples FROM jetsapi.process_errors WHERE key = $key";
      final rows = await queryJetsDataModel(
          context, formState, ServerEPs.dataTableEP, json.encode(rawQuery));
      if (rows == null) {
        return "No rows returned";
      }
      final responseModel = rows[0][0];
      if (responseModel != null) {
        final reteSession = json.decode(responseModel);
        formState.setValue(
            0, FSK.reteSessionRdfTypes, json.decode(reteSession['rdf_types']));
        formState.setValue(0, FSK.reteSessionEntityKeyByType,
            reteSession['entity_key_by_type']);
        formState.setValue(0, FSK.reteSessionEntityDetailsByKey,
            reteSession['entity_details_by_key']);
      }
      break;

    // View Rete Session V2 -- Navigate to related entity
    case ActionKeys.reteSessionVisitEntity:
      // Get the object value and type
      var state = formState.getState(0);
      final propertyValue = state[FSK.entityPropertyValue];
      final propertyValueType = state[FSK.entityPropertyValueType];
      if (propertyValue == null || propertyValueType == null) {
        print(
            "Error: unexpected null value for propertyValue or propertyValueType in formState");
        return "Error: unexpected null value for propertyValue or propertyValueType in formState";
      }
      if (propertyValueType[0] != "named_resource") {
        print("Navigating to resources only");
        return null;
      }
      formState.setValueAndNotify(0, FSK.entityKey, propertyValue);
      return null;

    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      break;

    default:
      showAlertDialog(context,
          'Oops unknown ActionKey for processErrorsActions: $actionKey');
  }
  return null;
}
