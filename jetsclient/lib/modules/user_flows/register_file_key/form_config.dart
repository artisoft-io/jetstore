import 'package:jetsclient/modules/user_flows/register_file_key/form_action_delegates.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/form_config.dart';

/// Form Config for Load Files UF Module

final Map<String, FormConfig> _formConfigurations = {
  FormKeys.rfkSubmitSchemaEvent: FormConfig(
    key: FormKeys.rfkSubmitSchemaEvent,
    actions: [
      FormActionConfig(
          key: ActionKeys.rfkSubmitSchemaEventUF,
          capability: "client_config",
          label: "Save",
          enableOnlyWhenFormValid: true,
          buttonStyle: ActionStyle.dialogOk,
          leftMargin: defaultPadding,
          rightMargin: betweenTheButtonsPadding),
      FormActionConfig(
          key: ActionKeys.dialogCancel,
          label: "Cancel",
          buttonStyle: ActionStyle.dialogCancel,
          leftMargin: betweenTheButtonsPadding,
          rightMargin: defaultPadding),
    ],
    inputFields: [
      [
        PaddingConfig(height: 2 * defaultPadding),
      ],
      [
        TextFieldConfig(
            label:
                "Enter the file key for the event and paste or enter the Schema Event as json below:",
            maxLines: 2,
            topMargin: 0,
            bottomMargin: 0),
      ],
      [
        FormInputFieldConfig(
            key: FSK.fileKey,
            label: "Event's File Key",
            hint: "S3 file key for the schema event",
            autofocus: true,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 300,
            useDefaultFont: true),
      ],
      [
        PaddingConfig(height: 1 * defaultPadding),
      ],
      [
        FormInputFieldConfig(
            key: FSK.schemaEventJson,
            label: "Schema Event (json)",
            hint: "Schema event json defines the file schema and the source file location to use as file key",
            flex: 1,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLines: 13,
            maxLength: 51200),
      ],
      [
        PaddingConfig(height: 2 * defaultPadding),
      ],
    ],
    formValidatorDelegate: registerFileKeyFormValidator,
    formActionsDelegate: doNothingAction,
  ),
};

FormConfig? getRegisterFileKeyFormConfig(String key) {
  return _formConfigurations[key];
}
