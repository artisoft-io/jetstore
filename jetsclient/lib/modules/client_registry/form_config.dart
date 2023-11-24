import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/models/user_flow_config.dart';
import 'package:jetsclient/modules/client_registry/form_action_delegates.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/form_config.dart';

/// Form Config for Client Config UF Module

final Map<String, FormConfig> _formConfigurations = {
  FormKeys.ufStartClientRegistry: FormConfig(
    key: FormKeys.ufStartClientRegistry,
    useListView: true,
    actions: [],
    inputFields: [
      [
        PaddingConfig(height: 2*defaultPadding),
      ],
      [
        TextFieldConfig(
            label:
                "This flow will assist you in adding Client and/or Vendor in the Client Registry.",
            maxLines: 1,
            topMargin: 0,
            bottomMargin: 0),
      ],
      [
        PaddingConfig(height: 2*defaultPadding),
      ],
      [
        PaddingConfig(),
        FormActionConfig(
            key: ActionKeys.ufStartFlow,
            label: "Start",
            buttonStyle: ActionStyle.secondary,
            leftMargin: defaultPadding,
            rightMargin: defaultPadding),
      ],
    ],
    formValidatorDelegate: alwaysValidForm,
    formActionsDelegate: doNothingAction, // overriden by UserFlowState.actionDelegate
  ),
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
  FormKeys.ufVendor: FormConfig(
    key: FormKeys.ufVendor,
    useListView: true,
    actions: [
      FormActionConfig(
          key: ActionKeys.ufPrevious,
          label: "Previous",
          buttonStyle: ActionStyle.primary,
          leftMargin: defaultPadding,
          rightMargin: betweenTheButtonsPadding),
      FormActionConfig(
          key: ActionKeys.ufContinueLater,
          label: "Cancel",
          buttonStyle: ActionStyle.primary,
          leftMargin: betweenTheButtonsPadding,
          rightMargin: defaultPadding),
      FormActionConfig(
          key: ActionKeys.ufNext,
          label: "Add Vendor/Org and Next",
          buttonStyle: ActionStyle.secondary,
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
            autofocus: true,
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
            label: "Organization Name",
            hint: "Organization name as a short name",
            flex: 1,
            autofocus: true,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 20,
            useDefaultFont: true),
        FormActionConfig(
            key: ActionKeys.crShowVendorUF,
            label: "View Vendor/Org",
            buttonStyle: ActionStyle.secondary,
            leftMargin: defaultPadding,
            rightMargin: defaultPadding),
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
      [
        PaddingConfig(height: 2*defaultPadding),
      ],
      [
        PaddingConfig(),
        PaddingConfig(),
        FormActionConfig(
            key: ActionKeys.crAddVendorUF,
            label: "Add Vendor/Org and New",
            buttonStyle: ActionStyle.secondary,
            leftMargin: defaultPadding,
            rightMargin: defaultPadding),
      ],
      [
        PaddingConfig(),
      ],
    ],
    formValidatorDelegate: clientRegistryFormValidator,
    formActionsDelegate: doNothingAction,
  ),
  FormKeys.ufShowVendor: FormConfig(
    key: FormKeys.ufShowVendor,
    actions: standardActions,
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

  // Done Page
  FormKeys.ufDoneClientRegistry: FormConfig(
    key: FormKeys.ufDoneClientRegistry,
    title: "Client Registry",
    useListView: true,
    // useListView: true,
    actions: [
      FormActionConfig(
          key: ActionKeys.ufPrevious,
          label: "Previous",
          buttonStyle: ActionStyle.primary,
          leftMargin: defaultPadding,
          rightMargin: betweenTheButtonsPadding),
      FormActionConfig(
          key: ActionKeys.ufCompleted,
          label: "Done",
          buttonStyle: ActionStyle.secondary,
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

FormConfig? getClientRegistryFormConfig(String key) {
  return _formConfigurations[key];
}
