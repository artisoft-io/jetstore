import 'package:flutter/material.dart';
import 'package:jetsclient/screens/components/data_table.dart';
import 'package:jetsclient/screens/components/input_text_form_field.dart';
import 'package:jetsclient/screens/components/text_field.dart';
import 'package:jetsclient/utils/data_table_config.dart';

import '../routes/jets_route_data.dart';
import '../screens/components/dropdown_form_field.dart';

enum TextRestriction { none, allLower, allUpper, digitsOnly }

enum ButtonStyle { primary, secondary, other }

typedef FormStateMap = List<Map<String, dynamic>>;
typedef ValidatorDelegate = String? Function(int, String, String?);

abstract class FormFieldConfig {
  FormFieldConfig({
    required this.key,
    this.group = 0,
    this.flex = 1,
  });
  final String key;
  final int group;
  final int flex;
  Widget makeFormField({
    required JetsRouteData formPath,
    required FormStateMap state,
    required ValidatorDelegate validator,
  });
}

class TextFieldConfig extends FormFieldConfig {
  TextFieldConfig({super.key = '', required this.label, super.flex});
  final String label;

  @override
  Widget makeFormField({
    required JetsRouteData formPath,
    required FormStateMap state,
    required ValidatorDelegate validator,
  }) {
    return JetsTextField(
      fieldConfig: this,
      flex: flex,
    );
  }
}

class FormInputFieldConfig extends FormFieldConfig {
  FormInputFieldConfig(
      {required super.key,
      required this.label,
      required this.hint,
      required this.autofocus,
      this.obscureText = false,
      required this.textRestriction,
      required this.maxLength,
      super.flex});
  final String label;
  final String hint;
  final bool autofocus;
  final bool obscureText;
  final TextRestriction textRestriction;
  // 0 for unbound
  final int maxLength;

  @override
  Widget makeFormField({
    required JetsRouteData formPath,
    required FormStateMap state,
    required ValidatorDelegate validator,
  }) {
    return JetsTextFormField(
      inputFieldConfig: this,
      onChanged: (p0) => state[group][key] = p0,
      validatorDelegate: validator,
      flex: flex,
    );
  }
}

class DropdownItemConfig {
  DropdownItemConfig({
    required this.label,
    this.value,
  });
  final String label;
  final String? value;
}

class FormDropdownFieldConfig extends FormFieldConfig {
  FormDropdownFieldConfig({
    required super.key,
    this.defaultItemPos = 0,
    required this.items,
    super.flex,
  });
  final int defaultItemPos;
  final List<DropdownItemConfig> items;

  @override
  Widget makeFormField({
    required JetsRouteData formPath,
    required FormStateMap state,
    required ValidatorDelegate validator,
  }) {
    return JetsDropdownButtonFormField(
      inputFieldConfig: this,
      onChanged: (p0) => state[group][key] = p0,
      validatorDelegate: validator,
      flex: flex,
    );
  }
}

// class FormDataTableFieldConfig extends FormFieldConfig {
//   FormDataTableFieldConfig({
//     required super.key, 
//     required this.dataTableConfig,
//   });
//   final String dataTableConfig;

//   @override
//   Widget makeFormField({
//     required JetsRouteData formPath,
//     required FormStateMap state,
//     required ValidatorDelegate validator,
//   }) {
//     return JetsDataTableWidget(
//       tablePath: formPath,
//       tableConfig: getTableConfig(dataTableConfig),
//       state passing is missing!
//     );
//   }
// }

class FormActionConfig {
  FormActionConfig(
      {required this.key, required this.label, required this.buttonStyle});
  final String key;
  final String label;
  final ButtonStyle buttonStyle;
}

class FormConfig {
  FormConfig(
      {required this.key,
      this.title,
      required this.inputFields,
      required this.actions});
  final String key;
  final String? title;
  final List<List<FormFieldConfig>> inputFields;
  final List<FormActionConfig> actions;

  int groupCount() {
    var unique = <int>{};
    for (int i = 0; i < inputFields.length; i++) {
      for (int j = 0; j < inputFields[i].length; j++) {
        unique.add(inputFields[i][j].group);
      }
    }
    return unique.length;
  }

  dynamic findFirst(FormStateMap formData, String key) {
    for (int i = 0; i < formData.length; i++) {
      if (formData[i].containsKey(key)) {
        return formData[i][key];
      }
    }
  }

  FormStateMap makeFormData() {
    return FormStateMap.filled(groupCount(), <String, dynamic>{});
  }
}

final Map<String, FormConfig> _formConfigurations = {
  'login': FormConfig(
    key: 'login',
    title: 'Please Sign In',
    actions: [
      FormActionConfig(
          key: "login", label: "Sign in", buttonStyle: ButtonStyle.primary),
      FormActionConfig(
          key: "register",
          label: "Register",
          buttonStyle: ButtonStyle.secondary),
    ],
    inputFields: [
      [
        FormInputFieldConfig(
            key: "email",
            label: "Email",
            hint: "Your email address",
            autofocus: true,
            obscureText: false,
            textRestriction: TextRestriction.allLower,
            maxLength: 80)
      ],
      [
        FormInputFieldConfig(
            key: "password",
            label: "Password",
            hint: "Your password",
            autofocus: false,
            obscureText: true,
            textRestriction: TextRestriction.none,
            maxLength: 80)
      ],
    ],
  ),
  'register': FormConfig(
    key: 'register',
    title: 'Welcome, Please Register',
    actions: [
      FormActionConfig(
          key: "register", label: "Register", buttonStyle: ButtonStyle.primary),
    ],
    inputFields: [
      [
        TextFieldConfig(label: 'This is the First Item'),
        FormInputFieldConfig(
            key: "name",
            label: "Name",
            hint: "Enter your name",
            flex: 2,
            autofocus: true,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 80), // ],
        // [
        FormInputFieldConfig(
            key: "email",
            label: "Email",
            hint: "Your email address",
            flex: 2,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.allLower,
            maxLength: 80),
        FormDropdownFieldConfig(key: 'emailType', items: [
          DropdownItemConfig(label: 'Work', value: 'work'),
          DropdownItemConfig(label: 'Home', value: 'home'),
        ]),
      ],
      [
        TextFieldConfig(label: 'This is the Second Item'),
        FormInputFieldConfig(
            key: "password",
            label: "Password",
            hint: "Your password",
            flex: 2,
            autofocus: false,
            obscureText: true,
            textRestriction: TextRestriction.none,
            maxLength: 80), // ],
        // [
        FormInputFieldConfig(
            key: "passwordConfirmation",
            label: "Password Confirmation",
            hint: "Re-enter your password",
            flex: 2,
            autofocus: false,
            obscureText: true,
            textRestriction: TextRestriction.none,
            maxLength: 80),
        FormDropdownFieldConfig(key: 'emailType', items: [
          DropdownItemConfig(label: ''),
          DropdownItemConfig(label: 'Work', value: 'work'),
          DropdownItemConfig(label: 'Home', value: 'home'),
        ]),
      ],
    ],
  ),
};

FormConfig getFormConfig(String key) {
  var config = _formConfigurations[key];
  if (config == null) {
    throw Exception(
        'ERROR: Invalid program configuration: form configuration $key not found');
  }
  return config;
}
