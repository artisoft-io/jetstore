import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/models/user_flow_config.dart';
import 'package:jetsclient/modules/user_flows/workspace_pull/form_action_delegates.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/form_config.dart';

/// Form Config for Workspace Pull UF Module

final Map<String, FormConfig> _formConfigurations = {
  FormKeys.wpPullWorkspaceUF: FormConfig(
    key: FormKeys.wpPullWorkspaceUF,
    actions: standardActions,
    useListView: true,
    inputFields: [
      [
        FormInputFieldConfig(
            key: FSK.wsName,
            label: "Workspace Name",
            hint: "Workspace name is used as the workspace key",
            flex: 1,
            autofocus: false,
            obscureText: false,
            isReadOnly: true,
            textRestriction: TextRestriction.none,
            maxLength: 20),
        FormInputFieldConfig(
            key: FSK.wsURI,
            label: "Worksapce URI",
            hint: "Repository where the workspace is versioned",
            flex: 1,
            autofocus: false,
            obscureText: false,
            isReadOnly: true,
            textRestriction: TextRestriction.none,
            maxLength: 120),
      ],
      [
        PaddingConfig(height: defaultPadding),
      ],
      [
        FormDataTableFieldConfig(
            key: DTKeys.otherWorkspaceActionOptions,
            dataTableConfig: DTKeys.otherWorkspaceActionOptions),
      ],
    ],
    formValidatorDelegate: alwaysValidForm,
    formActionsDelegate: doNothingAction,
  ),
  FormKeys.wpConfirmPullWorkspaceUF: FormConfig(
    key: FormKeys.wpConfirmPullWorkspaceUF,
    actions: [
      FormActionConfig(
          key: ActionKeys.ufPrevious,
          label: "Previous",
          buttonStyle: ActionStyle.ufPrimary,
          leftMargin: defaultPadding,
          rightMargin: betweenTheButtonsPadding),
      FormActionConfig(
          key: ActionKeys.ufCompleted,
          label: "Comfirm",
          buttonStyle: ActionStyle.ufSecondary,
          leftMargin: defaultPadding,
          rightMargin: defaultPadding),
    ],
    useListView: true,
    inputFields: [
      [
        FormInputFieldConfig(
            key: FSK.wsName,
            label: "Workspace Name",
            hint: "Workspace name is used as the workspace key",
            flex: 1,
            autofocus: false,
            obscureText: false,
            isReadOnly: true,
            textRestriction: TextRestriction.none,
            maxLength: 20),
        FormInputFieldConfig(
            key: FSK.wsURI,
            label: "Worksapce URI",
            hint: "Repository where the workspace is versioned",
            flex: 1,
            autofocus: false,
            obscureText: false,
            isReadOnly: true,
            textRestriction: TextRestriction.none,
            maxLength: 120),
      ],
      [
        PaddingConfig(height: defaultPadding),
      ],
      [
        FormDataTableFieldConfig(
            key: DTKeys.wpPullWorkspaceConfirmOptions,
            dataTableConfig: DTKeys.wpPullWorkspaceConfirmOptions)
      ],
    ],
    formValidatorDelegate: alwaysValidForm,
    formActionsDelegate: doNothingAction,
  ),

  // Load Client Config UF
  FormKeys.wpLoadConfigUF: FormConfig(
    key: FormKeys.wpLoadConfigUF,
    actions: standardActions,
    useListView: false,
    inputFieldsV2: [
      FormFieldRowConfig(flex:0, rowConfig: [
        FormInputFieldConfig(
            key: FSK.wsName,
            label: "Workspace Name",
            hint: "Workspace name is used as the workspace key",
            flex: 1,
            autofocus: false,
            obscureText: false,
            isReadOnly: true,
            textRestriction: TextRestriction.none,
            maxLength: 20),
        FormInputFieldConfig(
            key: FSK.wsURI,
            label: "Worksapce URI",
            hint: "Repository where the workspace is versioned",
            flex: 1,
            autofocus: false,
            obscureText: false,
            isReadOnly: true,
            textRestriction: TextRestriction.none,
            maxLength: 120),
      ]),
      FormFieldRowConfig(flex:1, rowConfig: [
        FormDataTableFieldConfig(
            key: FSK.wpClientList,
            dataTableConfig: FSK.wpClientList,
            tableHeight: double.infinity),
      ]),
      FormFieldRowConfig(flex:0, rowConfig: [
        PaddingConfig(),
        PaddingConfig(),
        FormActionConfig(
            key: ActionKeys.wpLoadAllClientConfigUF,
            label: "Load All Clients Config",
            buttonStyle: ActionStyle.ufSecondary,
            leftMargin: defaultPadding,
            rightMargin: defaultPadding),
      ]),
    ],
    formValidatorDelegate: loadConfigFormValidator,
    formActionsDelegate: doNothingAction,
  ),
  FormKeys.wpConfirmLoadConfigUF: FormConfig(
    key: FormKeys.wpConfirmLoadConfigUF,
    actions: [
      FormActionConfig(
          key: ActionKeys.ufPrevious,
          label: "Previous",
          buttonStyle: ActionStyle.ufPrimary,
          leftMargin: defaultPadding,
          rightMargin: betweenTheButtonsPadding),
      FormActionConfig(
          key: ActionKeys.ufCompleted,
          label: "Comfirm",
          buttonStyle: ActionStyle.ufSecondary,
          leftMargin: defaultPadding,
          rightMargin: defaultPadding),
    ],
    useListView: false,
    inputFieldsV2: [
      FormFieldRowConfig(flex:0, rowConfig: [
        FormInputFieldConfig(
            key: FSK.wsName,
            label: "Workspace Name",
            hint: "Workspace name is used as the workspace key",
            flex: 1,
            autofocus: false,
            obscureText: false,
            isReadOnly: true,
            textRestriction: TextRestriction.none,
            maxLength: 20),
        FormInputFieldConfig(
            key: FSK.wsURI,
            label: "Worksapce URI",
            hint: "Repository where the workspace is versioned",
            flex: 1,
            autofocus: false,
            obscureText: false,
            isReadOnly: true,
            textRestriction: TextRestriction.none,
            maxLength: 120),
      ]),
      FormFieldRowConfig(flex:1, rowConfig: [
        FormDataTableFieldConfig(
            key: FSK.wpClientListRO,
            dataTableConfig: FSK.wpClientListRO,
            tableHeight: double.infinity),
      ]),
    ],
    formValidatorDelegate: alwaysValidForm,
    formActionsDelegate: doNothingAction,
  ),
};

FormConfig? getWorkspacePullFormConfig(String key) {
  return _formConfigurations[key];
}
