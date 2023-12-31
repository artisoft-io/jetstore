import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/models/user_flow_config.dart';
import 'package:jetsclient/modules/user_flows/client_registry/form_action_delegates.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/form_config.dart';

/// Form Config for Client Config UF Module

final Map<String, FormConfig> _formConfigurations = {
  FormKeys.ufSelectClientOrVendor: FormConfig(
    key: FormKeys.ufSelectClientOrVendor,
    useListView: true,
    actions: standardActions,
    inputFields: [
      [
        PaddingConfig(height: 2*defaultPadding),
      ],
      [
        FormDataTableFieldConfig(
            key: FSK.ufClientOrVendorOption,
            dataTableConfig: FSK.ufClientOrVendorOption),
      ],
    ],
    formValidatorDelegate: clientRegistryFormValidator,
    formActionsDelegate: doNothingAction, // overriden by UserFlowState.actionDelegate
  ),
  FormKeys.ufCreateClient: FormConfig(
    key: FormKeys.ufCreateClient,
    useListView: true,
    actions: standardActions,
    inputFields: [
      [
        PaddingConfig(height: 2*defaultPadding),
      ],
      [
        TextFieldConfig(
            label: "Enter the client name:",
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
        PaddingConfig(height: 2*defaultPadding),
      ],
      [
        TextFieldConfig(
            label: "Enter the client details (optional):",
            maxLines: 1,
            topMargin: 0,
            bottomMargin: 0),
      ],
      [
        FormInputFieldConfig(
            key: FSK.ufClientDetails,
            label: "Client Details",
            hint: "Optional notes",
            flex: 1,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 80,
            useDefaultFont: true),
      ],
      [
        PaddingConfig(height: 2*defaultPadding),
      ],
    ],
    formValidatorDelegate: clientRegistryFormValidator,
    formActionsDelegate: doNothingAction, // overriden by UserFlowState.actionDelegate
  ),
  FormKeys.ufSelectClient: FormConfig(
    key: FormKeys.ufSelectClient,
    actions: standardActions,
    inputFields: [
      [
        FormDataTableFieldConfig(
            key: FSK.client,
            tableHeight: double.infinity,
            dataTableConfig: FSK.client),
      ],
    ],
    formValidatorDelegate: clientRegistryFormValidator,
    formActionsDelegate: doNothingAction, // overriden by UserFlowState.actionDelegate
  ),
  // Add vendor/org dialog
  FormKeys.ufVendor: FormConfig(
    key: FormKeys.ufVendor,
    useListView: true,
    actions: [
      FormActionConfig(
          key: ActionKeys.crAddVendorOk,
          capability: "client_config",
          label: "Add",
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
        PaddingConfig(height: 2*defaultPadding),
      ],
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
            autofocus: false,
            isReadOnly: true,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 20,
            useDefaultFont: true),
      ],
      [
        PaddingConfig(height: 2*defaultPadding),
      ],
      [
        FormInputFieldConfig(
            key: FSK.org,
            label: "Vendor/Org Name",
            hint: "Organization name as a short name",
            flex: 1,
            autofocus: true,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 20,
            useDefaultFont: true),
      ],
      [
        PaddingConfig(height: 2*defaultPadding),
      ],
      [
        FormInputFieldConfig(
            key: FSK.ufVendorDetails,
            label: "Details",
            hint: "Optional notes",
            flex: 1,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 80,
            useDefaultFont: true),
      ],
    ],
    formValidatorDelegate: clientRegistryFormValidator,
    formActionsDelegate: clientRegistryAddOrgFormActions,
  ),
  FormKeys.ufShowVendor: FormConfig(
    key: FormKeys.ufShowVendor,
    actions: [
      FormActionConfig(
          key: ActionKeys.ufPrevious,
          label: "Previous",
          buttonStyle: ActionStyle.ufPrimary,
          leftMargin: defaultPadding,
          rightMargin: betweenTheButtonsPadding),
      FormActionConfig(
          key: ActionKeys.ufCompleted,
          label: "Done",
          buttonStyle: ActionStyle.ufSecondary,
          leftMargin: defaultPadding,
          rightMargin: defaultPadding),
    ],
    inputFields: [
      [
        FormDataTableFieldConfig(
            key: FSK.org,
            tableHeight: double.infinity,
            dataTableConfig: FSK.org),
      ],
    ],
    formValidatorDelegate: alwaysValidForm,
    formActionsDelegate: doNothingAction, // overriden by UserFlowState.actionDelegate
  ),
};

FormConfig? getClientRegistryFormConfig(String key) {
  return _formConfigurations[key];
}
