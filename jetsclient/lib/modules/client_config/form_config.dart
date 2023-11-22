import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/models/user_flow_config.dart';
import 'package:jetsclient/modules/client_config/form_action_delegates.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/form_config.dart';

/// Form Config for Client Config UF Module

final Map<String, FormConfig> _formConfigurations = {
  FormKeys.ufStartClientRegistry: FormConfig(
    key: FormKeys.ufStartClientRegistry,
    actions: [
      FormActionConfig(
          key: ActionKeys.ufStartFlow,
          label: "Start",
          buttonStyle: ActionStyle.primary,
          leftMargin: defaultPadding,
          rightMargin: betweenTheButtonsPadding),
    ],
    inputFields: [
      [
        TextFieldConfig(
            label: "This flow will assist you in adding Client and/or Vendor in the Client Registry.",
            maxLines: 1,
            topMargin: 0,
            bottomMargin: 0),
      ],
    ],
    formValidatorDelegate: alwaysValidForm,
    formActionsDelegate: doNothingAction,
  ),
  FormKeys.ufClient: FormConfig(
    key: FormKeys.ufClient,
    actions: standardActions,
    inputFields: [
      [
        TextFieldConfig(
            label: "Enter a new Client or select and existing Client.",
            maxLines: 1,
            topMargin: 0,
            bottomMargin: 0),
      ],
      [
        FormInputFieldConfig(
            key: FSK.client,
            label: "Client Name",
            hint: "Client name as a short name",
            flex: 1,
            autofocus: true,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 20,
            useDefaultFont: true),
      ],
      [
        FormInputFieldConfig(
            key: FSK.details,
            label: "Details",
            hint: "Optional notes",
            flex: 1,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 80,
            useDefaultFont: true),
      ],
      [
        FormActionConfig(
            key: ActionKeys.crAddClientUF,
            label: "Add",
            buttonStyle: ActionStyle.primary,
            leftMargin: defaultPadding,
            rightMargin: betweenTheButtonsPadding),
      ],
      [
        FormDataTableFieldConfig(
            key: DTKeys.clientAdminTable,
            tableHeight: double.infinity,
            dataTableConfig: DTKeys.clientAdminTable),
      ],
    ],
    formValidatorDelegate: clientConfigFormValidator,
    formActionsDelegate: doNothingAction,
  ),
  FormKeys.ufVendor: FormConfig(
    key: FormKeys.ufVendor,
    actions: standardActions,
    inputFields: [
      [
        TextFieldConfig(
            label: "Enter a new Vendor (or Org) for the Client.",
            maxLines: 1,
            topMargin: 0,
            bottomMargin: 0),
      ],
      [
        FormInputFieldConfig(
            key: FSK.client,
            label: "Client Name",
            hint: "Client name as a short name",
            flex: 1,
            autofocus: true,
            isReadOnly: true,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 20,
            useDefaultFont: true),
      ],
      [
        FormInputFieldConfig(
            key: FSK.org,
            label: "Organization Name",
            hint: "Organization name as a short name",
            flex: 1,
            autofocus: true,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 20,
            useDefaultFont: true),
      ],
      [
        FormInputFieldConfig(
            key: FSK.details,
            label: "Details",
            hint: "Optional notes",
            flex: 1,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 80,
            useDefaultFont: true),
      ],
      [
        FormActionConfig(
            key: ActionKeys.crAddVendorUF,
            label: "Add",
            buttonStyle: ActionStyle.primary,
            leftMargin: defaultPadding,
            rightMargin: defaultPadding),
      ],
    ],
    formValidatorDelegate: clientConfigFormValidator,
    formActionsDelegate: doNothingAction,
  ),

  // Done Page
  FormKeys.ufDoneClientRegistry: FormConfig(
    key: FormKeys.ufDoneClientRegistry,
    title: "Client Registry",
    useListView: true,
    actions: [
      FormActionConfig(
          key: ActionKeys.ufCompleted,
          label: "Done",
          buttonStyle: ActionStyle.primary,
          leftMargin: defaultPadding,
          rightMargin: defaultPadding),
    ],
    inputFields: [
      [
        TextFieldConfig(
            label: "Congratulation, Client Registry Configuration Completed.",
            maxLines: 1,
            topMargin: 0,
            bottomMargin: 0),
      ],
    ],
    formValidatorDelegate: alwaysValidForm,
    formActionsDelegate: doNothingAction,
  ),
};

FormConfig? getClientConfigFormConfig(String key) {
  return _formConfigurations[key];
}
