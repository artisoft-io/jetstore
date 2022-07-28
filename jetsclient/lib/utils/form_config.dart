import 'package:flutter/material.dart';

import 'package:jetsclient/routes/jets_route_data.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/utils/data_table_config.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/screens/components/data_table.dart';
import 'package:jetsclient/screens/components/input_text_form_field.dart';
import 'package:jetsclient/screens/components/text_field.dart';
import 'package:jetsclient/screens/components/dropdown_form_field.dart';

enum TextRestriction { none, allLower, allUpper, digitsOnly }

enum ButtonStyle { primary, secondary, other }

class FormConfig {
  FormConfig(
      {required this.key, required this.inputFields, required this.actions});
  final String key;
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

  List<String?>? findFirst(JetsFormState formState, String key) {
    return formState.findFirst(key);
  }

  JetsFormState makeFormState() {
    return JetsFormState(groupCount: groupCount());
  }
}

abstract class FormFieldConfig {
  FormFieldConfig({
    required this.key,
    required this.group,
    required this.flex,
  });
  final String key;
  final int group;
  final int flex;
  Widget makeFormField({
    required JetsRouteData screenPath,
    required JetsFormState state,
    required ValidatorDelegate validator,
  });
}

class TextFieldConfig extends FormFieldConfig {
  TextFieldConfig(
      {super.key = '', super.group = 0, super.flex = 1, required this.label});
  final String label;

  @override
  Widget makeFormField({
    required JetsRouteData screenPath,
    required JetsFormState state,
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
      super.group = 0,
      super.flex = 1,
      required this.label,
      required this.hint,
      required this.autofocus,
      this.obscureText = false,
      required this.textRestriction,
      required this.maxLength});
  final String label;
  final String hint;
  final bool autofocus;
  final bool obscureText;
  final TextRestriction textRestriction;
  // 0 for unbound
  final int maxLength;

  @override
  Widget makeFormField({
    required JetsRouteData screenPath,
    required JetsFormState state,
    required ValidatorDelegate validator,
  }) {
    return JetsTextFormField(
      key: Key(key),
      formFieldConfig: this,
      onChanged: (p0) => state.setValueAndNotify(group, key, p0),
      validator: (String? value) => validator(group, key, value),
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
/// Dropdown Widget, [items] must be provided, with
/// perhaps a blank item to invite the user to make a selection.
/// If the [dropdownItemsQuery] is not null, it will be used
/// to query the server to obtain items that are appended to [items]
class FormDropdownFieldConfig extends FormFieldConfig {
  FormDropdownFieldConfig({
    required super.key,
    super.group = 0,
    super.flex = 1,
    this.defaultItemPos = 0,
    this.dropdownItemsQuery,
    required this.items,
  });
  final String? dropdownItemsQuery;
  final int defaultItemPos;
  final List<DropdownItemConfig> items;
  bool dropdownItemLoaded = false;

  @override
  Widget makeFormField({
    required JetsRouteData screenPath,
    required JetsFormState state,
    required ValidatorDelegate validator,
  }) {
    return JetsDropdownButtonFormField(
      key: Key(key),
      screenPath: screenPath,
      formFieldConfig: this,
      onChanged: (p0) => state.setValueAndNotify(group, key, p0),
      validator: (String? value) => validator(group, key, value),
      flex: flex,
    );
  }
}

class FormDataTableFieldConfig extends FormFieldConfig {
  FormDataTableFieldConfig(
      {required super.key,
      super.group = 0,
      super.flex = 1,
      this.tableWidth = double.infinity,
      this.tableHeight = 400,
      required this.dataTableConfig});
  final double tableWidth;
  final double tableHeight;
  final String dataTableConfig;

  @override
  Widget makeFormField({
    required JetsRouteData screenPath,
    required JetsFormState state,
    required ValidatorDelegate validator,
  }) {
    return Expanded(
      child: SizedBox(
        width: tableWidth,
        height: tableHeight,
        child: JetsDataTableWidget(
          key: Key(key),
          screenPath: screenPath,
          formFieldConfig: this,
          tableConfig: getTableConfig(dataTableConfig),
          formState: state,
          validator: (WidgetField? value) => validator(group, key, value),
        ),
      ),
    );
  }
}

class FormActionConfig {
  FormActionConfig(
      {required this.key, required this.label, required this.buttonStyle});
  final String key;
  final String label;
  final ButtonStyle buttonStyle;
}

final Map<String, FormConfig> _formConfigurations = {
  //* DEMO FORM
  "dataTableDemoForm": FormConfig(
    key: "dataTableDemoForm",
    actions: [
      FormActionConfig(
          key: "dataTableDemoAction1",
          label: "Do it!",
          buttonStyle: ButtonStyle.primary),
    ],
    inputFields: [
      [
        FormInputFieldConfig(
            key: "source_loc",
            label: "S3 Folder",
            hint: "Folder where the files are dropped",
            flex: 2,
            autofocus: true,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 80), // ],
        FormDropdownFieldConfig(key: 'client', items: [
          DropdownItemConfig(label: ''),
        ],
        dropdownItemsQuery: "SELECT client FROM jetsapi.client_registry ORDER BY client ASC LIMIT 50"),
      ],
      [
        FormInputFieldConfig(
            key: "table_name",
            label: "Client Table Name",
            hint: "Table where client file is loaded into",
            flex: 2,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 60), // ],
        FormInputFieldConfig(
            key: "grouping_column",
            label: "Grouping Column",
            hint: "Column containing Member Key",
            flex: 1,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 60), // ],
      ],
      [
        FormDataTableFieldConfig(
            key: "dataTableDemoMainTable",
            dataTableConfig: "dataTableDemoMainTableConfig")
      ],
      [
        FormDataTableFieldConfig(
            key: "dataTableDemoSupportTable",
            dataTableConfig: "dataTableDemoSupportTableConfig")
      ],
    ],
  ),
  FormKeys.login: FormConfig(
    key: FormKeys.login,
    actions: [
      FormActionConfig(
          key: ActionKeys.login,
          label: "Sign in",
          buttonStyle: ButtonStyle.primary),
      FormActionConfig(
          key: ActionKeys.register,
          label: "Register",
          buttonStyle: ButtonStyle.secondary),
    ],
    inputFields: [
      [
        FormInputFieldConfig(
            key: FSK.userEmail,
            label: "Email",
            hint: "Your email address",
            autofocus: true,
            obscureText: false,
            textRestriction: TextRestriction.allLower,
            maxLength: 80)
      ],
      [
        FormInputFieldConfig(
            key: FSK.userPassword,
            label: "Password",
            hint: "Your password",
            autofocus: false,
            obscureText: true,
            textRestriction: TextRestriction.none,
            maxLength: 80)
      ],
    ],
  ),
  FormKeys.register: FormConfig(
    key: FormKeys.register,
    actions: [
      FormActionConfig(
          key: ActionKeys.register,
          label: "Register",
          buttonStyle: ButtonStyle.primary),
    ],
    inputFields: [
      [
        TextFieldConfig(label: 'This is the First Item'),
        FormInputFieldConfig(
            key: FSK.userName,
            label: "Name",
            hint: "Enter your name",
            flex: 2,
            autofocus: true,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 80), // ],
        // [
        FormInputFieldConfig(
            key: FSK.userEmail,
            label: "Email",
            hint: "Your email address",
            flex: 2,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.allLower,
            maxLength: 80),
        //* REMOVE THIS DEMO CODE
        FormDropdownFieldConfig(key: 'emailType', items: [
          DropdownItemConfig(label: 'Work', value: 'work'),
          DropdownItemConfig(label: 'Home', value: 'home'),
        ]),
      ],
      [
        TextFieldConfig(label: 'This is the Second Item'),
        FormInputFieldConfig(
            key: FSK.userPassword,
            label: "Password",
            hint: "Your password",
            flex: 2,
            autofocus: false,
            obscureText: true,
            textRestriction: TextRestriction.none,
            maxLength: 80), // ],
        // [
        FormInputFieldConfig(
            key: FSK.userPasswordConfirm,
            label: "Password Confirmation",
            hint: "Re-enter your password",
            flex: 2,
            autofocus: false,
            obscureText: true,
            textRestriction: TextRestriction.none,
            maxLength: 80),
        //* REMOVE THIS DEMO CODE
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
