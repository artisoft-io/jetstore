// lib/config.dart
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/models/data_table_config.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/modules/actions/delegate_helpers.dart';

class AppConfig {
  factory AppConfig() => _singlton;
  static final AppConfig _singlton = AppConfig._();
  AppConfig._() {
    // Initialize buttonConfigs from JSON
    buttonConfigs = parseButtonConfig(buttonsConfigJson);
    actionConfigs = getConfigurableActionConfig();
    for (var buttonConfig in buttonConfigs) {
      buttonConfigMap[buttonConfig.key] = buttonConfig;
    }
    for (var actionConfig in actionConfigs) {
      actionConfigMap[actionConfig.actionName!] = actionConfig;
    }
  }

  List<ButtonConfig> buttonConfigs = [];
  List<ActionConfig> actionConfigs = [];
  Map<String, ButtonConfig> buttonConfigMap = {};
  Map<String, ActionConfig> actionConfigMap = {};

  static const String buttonsConfigJson = String.fromEnvironment(
    'BUTTON_CFG_JSON',
    defaultValue: '[]', // Optional default value
  );
  static List<ButtonConfig> buttonsConfig =
      parseButtonConfig(buttonsConfigJson);

  static List<ActionConfig> getConfigurableActionConfig() {
    return buttonsConfig
        .map(
          (e) => ActionConfig(
              actionType: DataTableActionType.doAction,
              actionName: e.key,
              key: e.key,
              label: e.label ?? 'Fetch from Stage to Clipboard',
              isVisibleWhenCheckboxVisible: true,
              isEnabledWhenHavingSelectedRows: true,
              capability: 'run_pipelines',
              style: ActionStyle.secondary),
        )
        .toList();
  }

  ButtonConfig? getButtonConfigByKey(String key) {
    return buttonConfigMap[key];
  }

  ActionConfig? getActionConfigByName(String actionName) {
    return actionConfigMap[actionName];
  }

  /// The action delegate is for [ButtonConfig] actions.
  Future<String?> buttonConfigActions(BuildContext context,
      GlobalKey<FormState> formKey, JetsFormState formState, String actionKey,
      {group = 0}) async {
    // Get the ButtonConfig object from the JetsFormState
    var state = formState.getState(0);
    var bc = AppConfig().getButtonConfigByKey(actionKey);
    if (bc == null) {
      print('Oops unknown action key $actionKey or ButtonConfig not found.');
      return null;
    }

    switch (bc.type) {
      // Fetch file from JetStore stage s3 and put contents in clipboard
      case ActionKeys.fetchFromStage2Clipboard:

        // Validate that path is provided
        if (bc.path == null || bc.path!.isEmpty) {
          return 'Error: No path provided in ButtonConfig for fetching file from Stage';
        }

        // Perform the substitutions in the path if needed
        String filePath = bc.path!;
        if (bc.fskParams != null) {
          for (var param in bc.fskParams!) {
            var value = unpack(state[param]);
            if (value == null) {
              print('Ooops missing value for FSK parameter "{$param}" '
                  'in ButtonConfig path substitution, state:\n$state');
              return 'Error: Missing value for FSK parameter "{$param}" in ButtonConfig path substitution.';
            }
            filePath = filePath.replaceAll('{{$param}}', value);
          }
        }

        // Fetch the file
        print('Fetching file from Stage: $filePath');
        var data = await fetchFileFromStage(context, formState, filePath);
        if (data == null) {
          return 'Error: Failed to fetch file from Stage at path: $filePath';
        }

        // Replace text in returned data if needed
        if (bc.replaceText != null && bc.replaceWith != null) {
          data = data.replaceAll(bc.replaceText!, bc.replaceWith!);
        }

        // Put data contents in clipboard
        await Clipboard.setData(ClipboardData(text: data));
        return null;

      default:
        print('Oops unknown ActionKey for process input form: $actionKey');
    }
    return null;
  }
}

class ButtonConfig {
  final String type;
  final String key;
  final String? label;
  final String? description;
  final String? replaceText;
  final String? replaceWith;
  final String? path;
  final List<String>? fskParams;

  ButtonConfig({
    required this.type,
    required this.key,
    this.label,
    this.description,
    this.replaceText,
    this.replaceWith,
    this.path,
    this.fskParams,
  });

  factory ButtonConfig.fromJson(Map<String, dynamic> json) {
    // Validate types with helpful errors
    if (json['type'] is! String) {
      throw const FormatException(
          'Invalid or missing "type" (String required).');
    }
    if (json['key'] is! String) {
      throw const FormatException(
          'Invalid or missing "key" (String required).');
    }
    // Validate that if fsk_params is present, it should be a List<String>
    if (json.containsKey('fsk_params')) {
      if (json['fsk_params'] is! List<dynamic> ||
          !(json['fsk_params'] as List<dynamic>).every((e) => e is String)) {
        throw const FormatException(
            'Invalid "fsk_params" (List<String> required).');
      }
    }

    return ButtonConfig(
      type: json['type'] as String,
      key: json['key'] as String,
      label: json['label'] as String?,
      description: json['description'] as String?,
      replaceText: json['replace_text'] as String?,
      replaceWith: json['replace_with'] as String?,
      path: json['file_path'] as String?,
      fskParams: (json['fsk_params'] as List<dynamic>?)
          ?.map((e) => e as String)
          .toList(),
    );
  }

  Map<String, dynamic> toJson() => {
        'type': type,
        'key': key,
        'label': label,
        'description': description,
        'replace_text': replaceText,
        'replace_with': replaceWith,
        'file_path': path,
        'fsk_params': fskParams,
      };
}

List<ButtonConfig> parseButtonConfig(String jsonString) {
  try {
    final list = jsonDecode(jsonString);
    if (list is! List<dynamic>) return [];
    return list
        .map((e) => ButtonConfig.fromJson(e as Map<String, dynamic>))
        .toList();
  } on FormatException catch (e) {
    print('Button config json got FormatException:\n$e');
    return []; // invalid JSON or validation failed
  } catch (e) {
    print('Button config json got other exception:\n$e');
    return []; // other errors
  }
}

Future<String?> fetchFileFromStage(
    BuildContext context, JetsFormState formState, String filePath) async {
  var state = formState.getState(0);
  state[FSK.userEmail] = JetsRouterDelegate().user.email;
  state[FSK.stageFilePath] = filePath;
  var encodedJsonBody = jsonEncode(<String, dynamic>{
    'action': 'fetch_file_from_stage',
    'data': [state],
  });

  final result = await postRawAction(
      context, ServerEPs.dataTableEP, encodedJsonBody,
      successMessage: 'Fetch s3 object successfully completed',
      failureMessage: 'Failed to fetch s3 object from stage.');
  if (result.statusCode == 401) return "Not Authorized";
  return result.body['file_content'] as String?;
}
