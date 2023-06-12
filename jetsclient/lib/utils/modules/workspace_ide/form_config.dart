import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/utils/form_config.dart';
import 'package:jetsclient/utils/modules/workspace_ide/screen_delegates.dart';


final Map<String, FormConfig> _formConfigurations = {
  // Workspace Home Form
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
            dataTableConfig: DTKeys.workspaceRegistryTable)
      ],
      [
        // Domain Classes Table
        // FormDataTableFieldConfig(
        //     key: DTKeys.inputLoaderStatusTable,
        //     dataTableConfig: DTKeys.inputLoaderStatusTable)
      ],
      [
        // Domain Tables Table
        // FormDataTableFieldConfig(
        //     key: DTKeys.pipelineExecStatusTable,
        //     dataTableConfig: DTKeys.pipelineExecStatusTable)
      ],
    ],
    formValidatorDelegate: workspaceIDEFormValidator,
    formActionsDelegate: workspaceIDEFormActions,
  ),
  // Add Client Dialog
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

};

FormConfig? getWorkspaceFormConfig(String key) {
  var config = _formConfigurations[key];
  return config;
}
