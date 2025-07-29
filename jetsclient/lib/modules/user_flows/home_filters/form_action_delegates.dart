import 'package:flutter/material.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/modules/user_flows/home_filters/form_action_helpers.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/screens/user_flow_screen.dart';
import 'package:jetsclient/modules/actions/delegate_helpers.dart';
import 'package:jetsclient/utils/constants.dart';

/// Validation delegate for Client Config UF
String? homeFiltersFormValidator(
    JetsFormState formState, int group, String key, dynamic v) {
  assert((v is String?) || (v is List<String>?),
      "homeFiltersFormValidator has unexpected data type");
  var homeFiltersState = JetsRouterDelegate().homeFiltersState;
  switch (key) {
    // optional keys
    case DTKeys.hfProcessTableUF:
      if (v == null) {
        homeFiltersState.remove(FSK.processName);
      } else {
        homeFiltersState[FSK.processName] = v;
        homeFiltersState[key] = v;
      }
      return null;

    case DTKeys.hfStatusTableUF:
      if (v == null) {
        homeFiltersState.remove(FSK.status);
      } else {
        homeFiltersState[FSK.status] = formState.getValue(group, FSK.status);
        homeFiltersState[key] = v;
      }
      return null;

    case FSK.hfStartTime:
    case FSK.hfStartOffset:
    case FSK.hfEndTime:
    case FSK.hfEndOffset:
      if (v == null) {
        homeFiltersState.remove(key);
      } else {
        homeFiltersState[key] = v;
      }
      return null;

    // required keys
    case DTKeys.hfFileKeyFilterTypeTableUF:
      if (v != null) {
        var fkFilterType = formState.getValue(group, FSK.hfFileKeyMatchType);
        if (fkFilterType != null) {
          homeFiltersState[FSK.hfFileKeyMatchType] = fkFilterType;
          homeFiltersState[key] = v;
          return null;
        }
      }
      return "Select an option";
    case FSK.hfFileKeySubstring:
      final fkFilterType =
          unpack(formState.getValue(group, DTKeys.hfFileKeyFilterTypeTableUF));
      if (fkFilterType != null && fkFilterType != 'None') {
        String? fkSubstring = unpack(v);
        if (fkSubstring == null || fkSubstring.isEmpty) {
          return "Enter a file key fragment";
        } else {
          homeFiltersState[key] = fkSubstring;
        }
      } else {
        homeFiltersState.remove(key);
      }
      return null;

    default:
      print(
          'Oops homeFiltersFormValidator Form Validator has no validator configured for form field $key');
  }
  return null;
}

/// Home Filters UF Form Actions - set on UserFlowState
Future<String?> homeFiltersFormActionsUF(
    UserFlowScreenState userFlowScreenState,
    BuildContext context,
    GlobalKey<FormState> formKey,
    JetsFormState formState,
    String actionKey,
    {group = 0}) async {
  switch (actionKey) {
    case ActionKeys.hfSelectProcessUF:
    case ActionKeys.hfSelectStatusUF:
    case ActionKeys.hfSelectFileKeyFilterUF:
    case ActionKeys.hfSelectTimeWindowUF:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      updateHomeFilters(context, formState);
      return null;

    // Cancel Dialog / Form
    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      break;
    default:
      print('Oops unknown ActionKey for File Mapping UF State: $actionKey');
  }
  return null;
}
