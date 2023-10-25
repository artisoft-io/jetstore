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
            tableHeight: double.infinity)
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
            tableHeight: double.infinity)
      ],
    ],
    formValidatorDelegate: workspaceHomeFormValidator,
    formActionsDelegate: workspaceHomeFormActions,
  ),
  // Add Workspace Dialog
  FormKeys.addWorkspace: FormConfig(
    key: FormKeys.addWorkspace,
    title: "Add / Update Workspace",
    useListView: true,
    actions: [
      FormActionConfig(
          key: ActionKeys.addWorkspaceOk,
          label: "Add / Update",
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
        FormInputFieldConfig(
            key: FSK.wsURI,
            label: "Worksapce URI",
            hint: "Repository where the workspace is versioned",
            flex: 1,
            autofocus: false,
            obscureText: false,
            defaultValue: globalWorkspaceUri,
            isReadOnlyEval: () => globalWorkspaceUri.isNotEmpty,
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

  // Commit Workspace Dialog
  FormKeys.commitWorkspace: FormConfig(
    key: FormKeys.commitWorkspace,
    title: "Commit and Push Workspace Changes to Repository",
    useListView: true,
    actions: [
      FormActionConfig(
          key: ActionKeys.commitWorkspaceOk,
          label: "Commit & Push",
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
        FormInputFieldConfig(
            key: FSK.gitCommitMessage,
            label: "Commit Message",
            hint: "Git commit message, keep it short",
            flex: 1,
            autofocus: false,
            obscureText: false,
            defaultValue: 'Commit from JetStore UI',
            textRestriction: TextRestriction.none,
            maxLength: 120),
      ],
    ],
    formValidatorDelegate: workspaceIDEFormValidator,
    formActionsDelegate: workspaceIDEFormActions,
  ),

  // Do Git Command Workspace Dialog
  FormKeys.doGitCommandWorkspace: FormConfig(
    key: FormKeys.doGitCommandWorkspace,
    title: "Do Git Command in Local Workspace",
    useListView: true,
    actions: [
      FormActionConfig(
          key: ActionKeys.doGitCommandWorkspaceOk,
          label: "Execute",
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
        FormInputFieldConfig(
            key: FSK.gitCommand,
            flex: 10,
            label: "Git Commands",
            hint: "Enter git commands to execute",
            maxLines: 10,
            maxLength: 25000,
            autofocus: false,
            textRestriction: TextRestriction.none),
      ],
    ],
    formValidatorDelegate: workspaceIDEFormValidator,
    formActionsDelegate: workspaceIDEFormActions,
  ),

  // Do Git Status Workspace Dialog
  FormKeys.doGitStatusWorkspace: FormConfig(
    key: FormKeys.doGitStatusWorkspace,
    title: "Do Git Status in Local Workspace",
    useListView: true,
    actions: [
      FormActionConfig(
          key: ActionKeys.doGitStatusWorkspaceOk,
          label: "Git Status",
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
        FormInputFieldConfig(
            key: FSK.gitCommand,
            flex: 10,
            label: "Git Status Commands",
            hint: "Enter git commands to execute",
            maxLines: 1,
            maxLength: 100,
            autofocus: false,
            isReadOnlyEval: () => globalWorkspaceUri.isNotEmpty,
            defaultValue: "git status",
            textRestriction: TextRestriction.none),
      ],
    ],
    formValidatorDelegate: workspaceIDEFormValidator,
    formActionsDelegate: workspaceIDEFormActions,
  ),

  // Push Only Workspace Changes Dialog
  FormKeys.pushOnlyWorkspace: FormConfig(
    key: FormKeys.pushOnlyWorkspace,
    title: "Push Workspace Changes to Repository Without Compiling Workspace",
    useListView: true,
    actions: [
      FormActionConfig(
          key: ActionKeys.pushOnlyWorkspaceOk,
          label: "Push Changes",
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
    ],
    formValidatorDelegate: workspaceIDEFormValidator,
    formActionsDelegate: workspaceIDEFormActions,
  ),

  // Pull Workspace Changes Dialog
  FormKeys.pullWorkspace: FormConfig(
    key: FormKeys.pullWorkspace,
    title: "Pull Workspace Changes from Repository",
    useListView: true,
    actions: [
      FormActionConfig(
          key: ActionKeys.pullWorkspaceOk,
          label: "Pull Changes",
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
    ],
    formValidatorDelegate: workspaceIDEFormValidator,
    formActionsDelegate: workspaceIDEFormActions,
  ),

  // View Last Git Log Workspace Dialog
  FormKeys.viewGitLogWorkspace: FormConfig(
    key: FormKeys.viewGitLogWorkspace,
    title: "Last Git Log of Workspace Changes",
    useListView: true,
    actions: [
      FormActionConfig(
          key: ActionKeys.dialogCancel,
          label: "Close",
          buttonStyle: ActionStyle.primary,
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
        FormInputFieldConfig(
            key: FSK.lastGitLog,
            label: "Last Git Log",
            hint: "Git log of last successful git operation",
            flex: 1,
            autofocus: true,
            obscureText: false,
            isReadOnly: true,
            textRestriction: TextRestriction.none,
            maxLines: 15,
            maxLength: 1000000),
      ],
    ],
    formValidatorDelegate: workspaceIDEFormValidator,
    formActionsDelegate: workspaceIDEFormActions,
  ),

  // Export Client Config Dialog
  FormKeys.exportWorkspaceClientConfig: FormConfig(
    key: FormKeys.exportWorkspaceClientConfig,
    title: "Export Client Configuration from DB to Workspace",
    useListView: true,
    actions: [
      FormActionConfig(
          key: ActionKeys.exportClientConfigOk,
          label: "Export Client Config",
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
            hint: "Workspace where to export the client configuration",
            flex: 1,
            autofocus: true,
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
        FormDropdownFieldConfig(
            key: FSK.client,
            items: [
              DropdownItemConfig(label: 'Select a Client'),
            ],
            dropdownItemsQuery:
                "SELECT client FROM jetsapi.client_registry ORDER BY client ASC LIMIT 150"),
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

  // Add Workspace File Dialog
  FormKeys.addWorkspaceFile: FormConfig(
    key: FormKeys.addWorkspaceFile,
    title: "Add Workspace File",
    useListView: true,
    actions: [
      FormActionConfig(
          key: ActionKeys.addWorkspaceFilesOk,
          label: "Add File",
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
            key: FSK.wsDbSourceFileName,
            label: "File Name",
            hint: "Enter file name, keeping the directory prefix",
            flex: 1,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 200),
      ],
    ],
    // constraint to be from FormKeys.wsDataModelForm since it's a tab
    formValidatorDelegate: workspaceIDEFormValidator,
    formActionsDelegate:   workspaceIDEFormActions,
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
      FormTabConfig(
          label: 'Data Model Files',
          inputField: FormDataTableFieldConfig(
              key: DTKeys.wsDataModelFilesTable,
              dataTableConfig: DTKeys.wsDataModelFilesTable)),
    ],
    formValidatorDelegate: workspaceIDEFormValidator,
    formActionsDelegate:   workspaceIDEFormActions,
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
      FormTabConfig(
          label: 'Jet Rules Files',
          inputField: FormDataTableFieldConfig(
              key: DTKeys.wsJetRulesFilesTable,
              dataTableConfig: DTKeys.wsJetRulesFilesTable)),
    ],
    formValidatorDelegate: workspaceIDEFormValidator,
    formActionsDelegate: workspaceIDEFormActions,
  ),

};

FormConfig? getWorkspaceFormConfig(String key) {
  var config = _formConfigurations[key];
  return config;
}
