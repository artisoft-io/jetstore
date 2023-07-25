import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/utils/form_config.dart';
import 'package:jetsclient/utils/modules/workspace_ide/screen_delegates.dart';


final Map<String, FormConfig> _formConfigurations = {
  // Workspace Registry Form
  FormKeys.workspaceRegistry: FormConfig(
    key: FormKeys.workspaceRegistry,
    actions: [
      // Action-less form
    ],
    inputFields: [
      [
        // Worksace Registry Table
        FormDataTableFieldConfig(
            key: DTKeys.workspaceRegistryTable,
            dataTableConfig: DTKeys.workspaceRegistryTable,
            tableHeight: 600)
      ],
    ],
    formValidatorDelegate: workspaceIDEFormValidator,
    formActionsDelegate: workspaceIDEFormActions,
  ),
  // Workspace Home Form
  FormKeys.workspaceHome: FormConfig(
    key: FormKeys.workspaceHome,
    actions: [
      // Action-less form
    ],
    inputFields: [
      [
        // Worksace Changes (workspace_changes) Table
        FormDataTableFieldConfig(
            key: DTKeys.workspaceChangesTable,
            dataTableConfig: DTKeys.workspaceChangesTable,
            tableHeight: 600)
      ],
    ],
    formValidatorDelegate: workspaceHomeFormValidator,
    formActionsDelegate: workspaceHomeFormActions,
  ),
  // Add Workspace Dialog
  FormKeys.addWorkspace: FormConfig(
    key: FormKeys.addWorkspace,
    title: "Add Workspace",
    actions: [
      FormActionConfig(
          key: ActionKeys.addWorkspaceOk,
          label: "Insert",
          buttonStyle: ActionStyle.primary,
          leftMargin: defaultPadding,
          rightMargin: betweenTheButtonsPadding),
      FormActionConfig(
          key: ActionKeys.dialogCancel,
          label: "Cancel",
          buttonStyle: ActionStyle.secondary,
          leftMargin: betweenTheButtonsPadding,
          rightMargin: defaultPadding),
    ],
    inputFields: [
      [
        FormInputFieldConfig(
            key: FSK.wsName,
            label: "Workspace Name",
            hint: "Workspace name is used as the workspace key",
            flex: 1,
            autofocus: true,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 20),
      ],
      [
        FormInputFieldConfig(
            key: FSK.wsURI,
            label: "Worksapce URI",
            hint: "Repository where the workspace is versioned",
            flex: 1,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 120),
      ],
      [
        FormInputFieldConfig(
            key: FSK.description,
            label: "Description",
            hint: "Workspace description",
            flex: 1,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 120),
      ],
    ],
    formValidatorDelegate: workspaceIDEFormValidator,
    formActionsDelegate: workspaceIDEFormActions,
  ),

  // Workspace File Editor
  FormKeys.workspaceFileEditor: FormConfig(
    key: FormKeys.workspaceFileEditor,
    // title: "Workspace File Editor",
    actions: [
      FormActionConfig(
          key: ActionKeys.wsSaveFileOk,
          label: "Save",
          buttonStyle: ActionStyle.primary,
          leftMargin: defaultPadding,
          rightMargin: betweenTheButtonsPadding),
    ],
    inputFields: [
      [
        FormInputFieldConfig(
            key: FSK.wsFileEditorContent,
            label: "File Content",
            hint: "Edit or paste file here",
            flex: 1,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLines: 50,
            maxLength: 512000),
      ],
    ],
    formValidatorDelegate: workspaceHomeFormValidator,
    formActionsDelegate: workspaceHomeFormActions,
  ),

  // Workspace Domain Class Table
  FormKeys.wsDataModelForm: FormConfig(
    key: FormKeys.wsDataModelForm,
    actions: [
      // Action-less form
    ],
    formTabsConfig: [
      FormTabConfig(
          label: 'Domain Classes',
          inputField: FormDataTableFieldConfig(
              key: DTKeys.wsDomainClassTable,
              dataTableConfig: DTKeys.wsDomainClassTable)),
      FormTabConfig(
          label: 'Data Properties',
          inputField: FormDataTableFieldConfig(
              key: DTKeys.wsDataPropertyTable,
              dataTableConfig: DTKeys.wsDataPropertyTable)),
      FormTabConfig(
          label: 'Domain Tables',
          inputField: FormDataTableFieldConfig(
              key: DTKeys.wsDomainTableTable,
              dataTableConfig: DTKeys.wsDomainTableTable)),
    ],
    formValidatorDelegate: workspaceIDEFormValidator,
    formActionsDelegate: workspaceIDEFormActions,
  ),

  // Workspace Jet Rules Table
  FormKeys.wsJetRulesForm: FormConfig(
    key: FormKeys.wsJetRulesForm,
    actions: [
      // Action-less form
    ],
    formTabsConfig: [
      FormTabConfig(
          label: 'Jet Rules',
          inputField: FormDataTableFieldConfig(
              key: DTKeys.wsJetRulesTable,
              dataTableConfig: DTKeys.wsJetRulesTable)),
      FormTabConfig(
          label: 'Rule Terms',
          inputField: FormDataTableFieldConfig(
              key: DTKeys.wsRuleTermsTable,
              dataTableConfig: DTKeys.wsRuleTermsTable)),
      FormTabConfig(
          label: 'Files Relationship',
          inputField: FormDataTableFieldConfig(
              key: DTKeys.wsMainSupportFilesTable,
              dataTableConfig: DTKeys.wsMainSupportFilesTable)),
    ],
    formValidatorDelegate: workspaceIDEFormValidator,
    formActionsDelegate: workspaceIDEFormActions,
  ),

};

FormConfig? getWorkspaceFormConfig(String key) {
  var config = _formConfigurations[key];
  return config;
}
