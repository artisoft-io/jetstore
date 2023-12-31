import 'package:flutter/material.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/modules/actions/config_delegates.dart';
import 'package:jetsclient/modules/actions/process_errors_delegates.dart';
import 'package:jetsclient/modules/actions/query_tool_screen_delegates.dart';
import 'package:jetsclient/modules/actions/source_config_delegates.dart';
import 'package:jetsclient/modules/actions/user_delegates.dart';
import 'package:jetsclient/modules/user_flows/client_registry/form_config.dart';
import 'package:jetsclient/modules/user_flows/configure_files/form_config.dart';
import 'package:jetsclient/modules/user_flows/file_mapping/form_config.dart';
import 'package:jetsclient/modules/user_flows/load_files/form_config.dart';
import 'package:jetsclient/modules/user_flows/pipeline_config/form_config.dart';
import 'package:jetsclient/modules/user_flows/start_pipeline/form_config.dart';
import 'package:jetsclient/modules/user_flows/workspace_pull/form_config.dart';

import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/form_config.dart';
import 'package:jetsclient/modules/workspace_ide/form_config.dart';

final Map<String, FormConfig> _formConfigurations = {
  // Home Form (actionless)
  FormKeys.home: FormConfig(
    key: FormKeys.home,
    actions: [
      // Action-less form
    ],
    formTabsConfig: [
      FormTabConfig(
          label: 'File Loader Status',
          inputField: FormDataTableFieldConfig(
              key: DTKeys.inputLoaderStatusTable,
              dataTableConfig: DTKeys.inputLoaderStatusTable)),
      FormTabConfig(
          label: 'Pipeline Execution Status',
          inputField: FormDataTableFieldConfig(
              key: DTKeys.pipelineExecStatusTable,
              dataTableConfig: DTKeys.pipelineExecStatusTable)),
      FormTabConfig(
          label: 'Data Registry',
          inputField: FormDataTableFieldConfig(
              key: DTKeys.inputRegistryTable,
              dataTableConfig: DTKeys.inputRegistryTable)),
    ],
    formValidatorDelegate: homeFormValidator,
    formActionsDelegate: homeFormActions,
  ),

  // Login Form
  FormKeys.login: FormConfig(
    key: FormKeys.login,
    useListView: true,
    actions: [
      FormActionConfig(
          key: ActionKeys.login,
          label: "Sign in",
          buttonStyle: ActionStyle.dialogOk,
          leftMargin: defaultPadding,
          rightMargin: betweenTheButtonsPadding),
      FormActionConfig(
          key: ActionKeys.register,
          label: "Register",
          buttonStyle: ActionStyle.dialogCancel,
          leftMargin: betweenTheButtonsPadding,
          rightMargin: defaultPadding),
    ],
    inputFields: [
      [
        FormInputFieldConfig(
            key: FSK.userEmail,
            label: "Email",
            hint: "Your email address",
            autofocus: true,
            autofillHints: [AutofillHints.email],
            obscureText: false,
            textRestriction: TextRestriction.allLower,
            maxLength: 80,
            useDefaultFont: true)
      ],
      [
        FormInputFieldConfig(
            key: FSK.userPassword,
            label: "Password",
            hint: "Your password",
            autofocus: false,
            autofillHints: [AutofillHints.password],
            obscureText: true,
            textRestriction: TextRestriction.none,
            maxLength: 80,
            useDefaultFont: true)
      ],
    ],
    formValidatorDelegate: loginFormValidator,
    formActionsDelegate: loginFormActions,
  ),
  // User Registration Form
  FormKeys.register: FormConfig(
    key: FormKeys.register,
    useListView: true,
    actions: [
      FormActionConfig(
          key: ActionKeys.register,
          label: "Register",
          buttonStyle: ActionStyle.dialogOk,
          leftMargin: defaultPadding,
          rightMargin: defaultPadding),
    ],
    inputFields: [
      [
        FormInputFieldConfig(
            key: FSK.userName,
            label: "Name",
            hint: "Enter your name",
            flex: 1,
            autofocus: true,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 80,
            useDefaultFont: true),
        FormInputFieldConfig(
            key: FSK.userEmail,
            label: "Email",
            hint: "Your email address",
            flex: 1,
            autofocus: false,
            autofillHints: [AutofillHints.email],
            obscureText: false,
            textRestriction: TextRestriction.allLower,
            maxLength: 80,
            useDefaultFont: true),
      ],
      [
        FormInputFieldConfig(
            key: FSK.userPassword,
            label: "Password",
            hint: "Your password",
            flex: 1,
            autofocus: false,
            autofillHints: [AutofillHints.newPassword],
            obscureText: true,
            textRestriction: TextRestriction.none,
            maxLength: 80,
            useDefaultFont: true),
        FormInputFieldConfig(
            key: FSK.userPasswordConfirm,
            label: "Password Confirmation",
            hint: "Re-enter your password",
            flex: 1,
            autofocus: false,
            autofillHints: [AutofillHints.newPassword],
            obscureText: true,
            textRestriction: TextRestriction.none,
            maxLength: 80,
            useDefaultFont: true),
      ],
    ],
    formValidatorDelegate: registrationFormValidator,
    formActionsDelegate: registrationFormActions,
  ),
  // User Git Profile Form
  FormKeys.userGitProfile: FormConfig(
    key: FormKeys.userGitProfile,
    useListView: true,
    actions: [
      FormActionConfig(
          key: ActionKeys.submitGitProfileOk,
          capability: "user_profile",
          label: "Submit",
          buttonStyle: ActionStyle.dialogOk,
          leftMargin: defaultPadding,
          rightMargin: defaultPadding),
    ],
    inputFields: [
      [
        FormInputFieldConfig(
            key: FSK.gitName,
            label: "Name",
            hint: "Enter your name for git commits",
            flex: 1,
            autofocus: true,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 80,
            useDefaultFont: true),
        FormInputFieldConfig(
            key: FSK.gitHandle,
            label: "Git Handle",
            hint: "Your git handle (user name) for git commit",
            flex: 1,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.allLower,
            maxLength: 60,
            useDefaultFont: true),
      ],
      [
        FormInputFieldConfig(
            key: FSK.gitEmail,
            label: "Email",
            hint: "Your email address for git commit",
            flex: 1,
            autofocus: false,
            autofillHints: [AutofillHints.email],
            obscureText: false,
            textRestriction: TextRestriction.allLower,
            maxLength: 80,
            useDefaultFont: true),
      ],
      [
        FormInputFieldConfig(
            key: FSK.gitToken,
            label: "Github Token",
            hint: "Github token to use as password",
            flex: 1,
            autofocus: false,
            obscureText: true,
            textRestriction: TextRestriction.none,
            maxLength: 120,
            useDefaultFont: true),
      ],
      [
        FormInputFieldConfig(
            key: FSK.gitTokenConfirm,
            label: "Github Token Confirmation",
            hint: "Re-enter your github token",
            flex: 1,
            autofocus: false,
            obscureText: true,
            textRestriction: TextRestriction.none,
            maxLength: 120,
            useDefaultFont: true),
      ],
    ],
    formValidatorDelegate: gitProfileFormValidator,
    formActionsDelegate: gitProfileFormActions,
  ),
  // User Administration Form (actionless -- user table has the actions)
  FormKeys.userAdmin: FormConfig(
    key: FormKeys.userAdmin,
    actions: [],
    inputFields: [
      [
        FormDataTableFieldConfig(
            key: DTKeys.usersTable,
            dataTableConfig: DTKeys.usersTable,
            tableHeight: double.infinity)
      ],
    ],
    formValidatorDelegate: alwaysValidForm,
    formActionsDelegate: userAdminFormActions,
  ),
  // User Administration Form - Edit User Profile Dialog
  FormKeys.editUserProfile: FormConfig(
    key: FormKeys.editUserProfile,
    actions: [
      FormActionConfig(
          key: ActionKeys.editUserProfileOk,
          capability: "user_profile",
          label: "Submit",
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
    useListView: true,
    inputFields: [
      [
        FormInputFieldConfig(
            key: FSK.userName,
            label: "Name",
            hint: "User name",
            flex: 1,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            isReadOnly: true,
            useDefaultFont: true,
            maxLength: 80),
        FormInputFieldConfig(
            key: FSK.userEmail,
            label: "Email",
            hint: "User email",
            flex: 1,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            isReadOnly: true,
            useDefaultFont: true,
            maxLength: 80),
      ],
      [
        FormDropdownFieldConfig(
            key: FSK.isActive,
            items: [
              DropdownItemConfig(label: 'Select User Status...'),
              DropdownItemConfig(label: 'Active', value: '1'),
              DropdownItemConfig(label: 'Inactive', value: '0'),
            ],
            flex: 1,
            defaultItemPos: 0),
      ],
      [
        PaddingConfig(height: defaultPadding * 4),
      ],
      [
        FormDataTableFieldConfig(
            key: DTKeys.userRolesTable, dataTableConfig: DTKeys.userRolesTable)
      ],
    ],
    formValidatorDelegate: userAdminValidator,
    formActionsDelegate: userAdminFormActions,
  ),

  // Load All Files
  FormKeys.loadAllFiles: FormConfig(
    key: FormKeys.loadAllFiles,
    title: "Load all files within a time period",
    actions: [
      FormActionConfig(
          key: ActionKeys.loadAllFilesOk,
          capability: "run_pipelines",
          label: "Load All Files",
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
        FormDataTableFieldConfig(
            key: FSK.fromSourcePeriodKey,
            tableHeight: double.infinity,
            dataTableConfig: FSK.fromSourcePeriodKey),
        FormDataTableFieldConfig(
            key: FSK.toSourcePeriodKey,
            tableHeight: double.infinity,
            dataTableConfig: FSK.toSourcePeriodKey),
      ],
      // [
      //   FormDataTableFieldConfig(
      //       key: DTKeys.fileKeyStagingMultiLoadTable,
      //       tableHeight: 600,
      //       dataTableConfig: DTKeys.fileKeyStagingMultiLoadTable)
      // ],
    ],
    formValidatorDelegate: loadAllFilesValidator,
    formActionsDelegate: loadAllFilesActions,
  ),

  // Rule Configv2 - Action-less form to select Rule Configv2 to Edit or do Add
  FormKeys.rulesConfigv2: FormConfig(
    key: FormKeys.rulesConfigv2,
    actions: [],
    inputFields: [
      [
        FormDataTableFieldConfig(
            key: DTKeys.ruleConfigv2Table,
            dataTableConfig: DTKeys.ruleConfigv2Table,
            tableHeight: double.infinity),
      ],
    ],
    formValidatorDelegate: ruleConfigv2FormValidator,
    formActionsDelegate: ruleConfigv2FormActions,
  ),

  // Rule Configuration v2 - Dialog to edit rule_configv2.rule_config_json
  FormKeys.rulesConfigv2Dialog: FormConfig(
    key: FormKeys.rulesConfigv2Dialog,
    title: "Rule Configuration",
    actions: [
      FormActionConfig(
          key: ActionKeys.ruleConfigv2Ok,
          capability: "client_config",
          label: "Save",
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
    useListView: true,
    inputFields: [
      [
        FormDropdownFieldConfig(
            key: FSK.client,
            items: [
              DropdownItemConfig(label: 'Select a Client'),
            ],
            autovalidateMode: AutovalidateMode.onUserInteraction,
            makeReadOnlyWhenHasSelectedValue: true,
            dropdownItemsQuery:
                "SELECT client FROM jetsapi.client_registry ORDER BY client ASC LIMIT 150"),
        FormDropdownFieldConfig(
            key: FSK.processName,
            returnedModelCacheKey: FSK.processConfigCache,
            items: [
              DropdownItemConfig(label: 'Select a process'),
            ],
            autovalidateMode: AutovalidateMode.onUserInteraction,
            makeReadOnlyWhenHasSelectedValue: true,
            dropdownItemsQuery:
                "SELECT process_name, key FROM jetsapi.process_config ORDER BY process_name ASC LIMIT 100"),
      ],
      [
        FormInputFieldConfig(
            key: FSK.ruleConfigJson,
            flex: 10,
            label: "Rule Configuration CSV / Json",
            hint:
                "Enter a valid json array or csv with headers of configuration objects",
            maxLines: 25,
            maxLength: 51200,
            autofocus: false,
            textRestriction: TextRestriction.none,
            defaultValue: "[]"),
      ],
    ],
    formValidatorDelegate: ruleConfigv2FormValidator,
    formActionsDelegate: ruleConfigv2FormActions,
  ),

  // ruleConfig - Dialog to enter rule config triples
  // This form is depricated, keeping here as an example of a dynamic form
  // where a row of controls is added dynamically
  FormKeys.rulesConfig: FormConfig(
    key: FormKeys.rulesConfig,
    title: "Rule Configuration",
    actions: [
      FormActionConfig(
          key: ActionKeys.ruleConfigOk,
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
    queries: {
      "inputFieldsQuery":
          "SELECT subject, predicate, object, rdf_type FROM jetsapi.rule_config WHERE client = '{client}' AND process_name = '{process_name}' ORDER BY subject ASC, predicate ASC, object ASC LIMIT 300",
    },
    inputFieldsQuery: "inputFieldsQuery",
    stateKeyPredicates: [FSK.client, FSK.processName],
    formWithDynamicRows: true,
    inputFieldRowBuilder: (index, inputFieldRow, formState) {
      var isLastRow = inputFieldRow == null;
      inputFieldRow ??= List<String?>.filled(4, '');
      // set the default values to the formState
      formState.setValue(index, FSK.subject, inputFieldRow[0]);
      formState.setValue(index, FSK.predicate, inputFieldRow[1]);
      formState.setValue(index, FSK.object, inputFieldRow[2]);
      formState.setValue(index, FSK.rdfType, inputFieldRow[3]);
      // print("Form BUILDER savedState row $inputFieldRow");
      return [
        [
          // NOTE: ** if the layout of the input field row changes,
          // need to also reflect the change in config_delegate.dart
          // for the Add Row action. **
          // subject
          if (!isLastRow)
            FormInputFieldConfig(
              key: FSK.subject,
              label: "Subject",
              hint: "Rule config subject",
              group: index,
              flex: 2,
              autovalidateMode: AutovalidateMode.always,
              autofocus: false,
              obscureText: false,
              textRestriction: TextRestriction.none,
              maxLength: 512,
            ),
          if (isLastRow) TextFieldConfig(label: '', flex: 2),
          // predicate
          if (!isLastRow)
            FormInputFieldConfig(
              key: FSK.predicate,
              label: "Predicate",
              hint: "Rule config predicate",
              group: index,
              flex: 2,
              autovalidateMode: AutovalidateMode.always,
              autofocus: false,
              obscureText: false,
              textRestriction: TextRestriction.none,
              maxLength: 512,
            ),
          if (isLastRow) TextFieldConfig(label: '', flex: 2),
          // object
          if (!isLastRow)
            FormInputFieldConfig(
              key: FSK.object,
              label: "Object",
              hint: "Rule config object",
              group: index,
              flex: 2,
              autovalidateMode: AutovalidateMode.always,
              autofocus: false,
              obscureText: false,
              textRestriction: TextRestriction.none,
              maxLength: 512,
            ),
          if (isLastRow) TextFieldConfig(label: '', flex: 2),
          // rdf type
          if (!isLastRow)
            FormDropdownFieldConfig(
              key: FSK.rdfType,
              group: index,
              flex: 1,
              autovalidateMode: AutovalidateMode.always,
              items: FormDropdownFieldConfig.rdfDropdownItems,
              defaultItemPos: 0,
            ),
          if (isLastRow) TextFieldConfig(label: '', flex: 1),
          // add / delete row button
          FormActionConfig(
            key: isLastRow
                ? ActionKeys.ruleConfigAdd
                : ActionKeys.ruleConfigDelete,
            group: isLastRow ? 0 : index,
            flex: 1,
            label: isLastRow ? 'Add Row' : '',
            labelByStyle: {
              ActionStyle.alternate: 'Delete',
              ActionStyle.danger: 'Confirm',
            },
            buttonStyle:
                isLastRow ? ActionStyle.secondary : ActionStyle.alternate,
            leftMargin: defaultPadding,
            rightMargin: defaultPadding,
          ),
        ],
      ];
    },
    formValidatorDelegate: processConfigFormValidator,
    formActionsDelegate: processConfigFormActions,
  ),

  // Process & Rules Config (actionless)
  FormKeys.processConfig: FormConfig(
    key: FormKeys.processConfig,
    actions: [
      // Action-less form
    ],
    inputFields: [
      [
        FormDataTableFieldConfig(
            key: DTKeys.clientsAndProcessesTableView,
            tableHeight: 400,
            dataTableConfig: DTKeys.clientsAndProcessesTableView),
      ],
      [
        FormDataTableFieldConfig(
            key: DTKeys.ruleConfigTable,
            tableHeight: double.infinity,
            dataTableConfig: DTKeys.ruleConfigTable)
      ],
    ],
    formValidatorDelegate: processConfigFormValidator,
    formActionsDelegate: processConfigFormActions,
  ),

  // Show Pipeline Failure Details - Dialog
  FormKeys.showFailureDetails: FormConfig(
    key: FormKeys.showFailureDetails,
    title: "Pipeline Failure Details",
    actions: [
      FormActionConfig(
          key: ActionKeys.dialogCancel,
          label: "Close",
          buttonStyle: ActionStyle.dialogOk,
          leftMargin: betweenTheButtonsPadding,
          rightMargin: defaultPadding),
    ],
    inputFields: [
      [
        FormInputFieldConfig(
            key: FSK.failureDetails,
            label: "Failure Details",
            hint: "This is read only",
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLines: 20,
            maxLength: 4000000),
      ],
    ],
    formValidatorDelegate: homeFormValidator,
    formActionsDelegate: homeFormActions,
  ),

  // View Process Errors (table as actionless form)
  FormKeys.viewProcessErrors: FormConfig(
    key: FormKeys.viewProcessErrors,
    actions: [
      // Action-less form
    ],
    inputFields: [
      [
        FormDataTableFieldConfig(
            key: DTKeys.processErrorsTable,
            dataTableConfig: DTKeys.processErrorsTable,
            tableHeight: double.infinity)
      ],
    ],
    formValidatorDelegate: alwaysValidForm,
    formActionsDelegate: processErrorsActions,
  ),

  // View Input Records for a domain key from Process Errors (table as actionless dialog)
  FormKeys.viewInputRecords: FormConfig(
    key: FormKeys.viewInputRecords,
    title: "Input Records for a Domain Key",
    actions: [
      FormActionConfig(
          key: ActionKeys.dialogCancel,
          label: "Close",
          buttonStyle: ActionStyle.dialogOk,
          leftMargin: betweenTheButtonsPadding,
          rightMargin: defaultPadding),
    ],
    queries: {
      "inputFieldsQuery": """
          WITH per AS (
            SELECT 1 AS order,
              'Main Input' AS label,
              main_input_registry_key AS key
            FROM jetsapi.pipeline_execution_status pes
            WHERE pes.key = {pipeline_execution_status_key}
            UNION
            SELECT 2 AS order,
              'Merged Input' AS label,
              unnest(merged_input_registry_keys) AS key
            FROM jetsapi.pipeline_execution_status pes
            WHERE pes.key = {pipeline_execution_status_key}
          ),
          pcr AS (
            SELECT 3 AS order,
              'Injected Input' AS label,
              unnest(pc.injected_process_input_keys) AS key
            FROM jetsapi.pipeline_execution_status pe,
              jetsapi.pipeline_config pc
            WHERE pe.key = {pipeline_execution_status_key}
              AND pe.pipeline_config_key = pc.key
          )
          SELECT t.order,
            t.label,
            t.table_name,
            t.lookback_periods,
            t.session_id,
            {pipeline_execution_status_key} AS pipeline_execution_status_key,
            '{object_type}' AS object_type,
            '{domain_key}' AS domain_key
          FROM (
              SELECT per.order,
                per.label,
                pi.table_name,
                pi.lookback_periods,
                ir.session_id
              FROM jetsapi.process_input pi,
                jetsapi.input_registry ir,
                per
              WHERE ir.key = per.key
                AND ir.client = pi.client
                AND ir.org = pi.org
                AND ir.object_type = pi.object_type
                AND ir.table_name = pi.table_name
              UNION
              SELECT pcr.order,
                pcr.label,
                pi.table_name,
                pi.lookback_periods,
                NULL AS session_id
              FROM jetsapi.process_input pi,
                pcr
              WHERE pi.key = pcr.key
            ) AS t
          ORDER BY t.order ASC""",
    },
    inputFieldsQuery: "inputFieldsQuery",
    stateKeyPredicates: [
      FSK.pipelineExectionStatusKey,
      FSK.objectType,
      FSK.domainKey,
    ],
    // inputFieldRow: [order, label, table_name, lookback_periods, session_id,
    //                 pipeline_execution_status_key, object_type, domain_key]
    inputFieldRowBuilder: (index, inputFieldRow, formState) {
      assert(inputFieldRow != null,
          'viewInputRecords form builder: error inputFieldRow should not be null');
      if (inputFieldRow == null) {
        return [];
      }
      // set table predicate values to the formState
      formState.setValue(index, FSK.label, inputFieldRow[1]);
      formState.setValue(index, FSK.tableName, inputFieldRow[2]);
      formState.setValue(index, FSK.lookbackPeriods, inputFieldRow[3]);
      formState.setValue(index, FSK.sessionId, inputFieldRow[4]);
      formState.setValue(
          index, FSK.pipelineExectionStatusKey, inputFieldRow[5]);
      formState.setValue(index, FSK.objectType, inputFieldRow[6]);
      formState.setValue(index, FSK.domainKey, inputFieldRow[7]);
      formState.setValue(
          index, FSK.domainKeyColumn, '${inputFieldRow[6]}:domain_key');
      return [
        [
          FormDataTableFieldConfig(
              key: DTKeys.inputRecordsFromProcessErrorTable + index.toString(),
              group: index,
              dataTableConfig: DTKeys.inputRecordsFromProcessErrorTable,
              tableHeight: 400)
        ],
      ];
    },
    formValidatorDelegate: alwaysValidForm,
    formActionsDelegate: processErrorsActions,
  ),

  // View process_errors.rete_session_triples from Process Errors (table as actionless dialog)
  FormKeys.viewReteTriples: FormConfig(
    key: FormKeys.viewReteTriples,
    title: "Rete Session as Triples",
    actions: [
      FormActionConfig(
          key: ActionKeys.dialogCancel,
          label: "Close",
          buttonStyle: ActionStyle.dialogOk,
          leftMargin: betweenTheButtonsPadding,
          rightMargin: defaultPadding),
    ],
    inputFields: [
      [
        FormDataTableFieldConfig(
            key: DTKeys.reteSessionTriplesTable,
            dataTableConfig: DTKeys.reteSessionTriplesTable,
            tableHeight: 1000)
      ],
    ],
    formValidatorDelegate: alwaysValidForm,
    formActionsDelegate: processErrorsActions,
  ),

  FormKeys.viewReteTriplesV2: FormConfig(
    key: FormKeys.viewReteTriplesV2,
    title: "Rete Session Explorer",
    actions: [
      FormActionConfig(
          key: ActionKeys.dialogCancel,
          label: "Close",
          buttonStyle: ActionStyle.dialogOk,
          leftMargin: betweenTheButtonsPadding,
          rightMargin: defaultPadding),
    ],
    inputFields: [
      [
        FormDataTableFieldConfig(
            key: DTKeys.reteSessionRdfTypeTable,
            flex: 1,
            dataTableConfig: DTKeys.reteSessionRdfTypeTable,
            tableHeight: double.infinity),
        FormDataTableFieldConfig(
            key: DTKeys.reteSessionEntityKeyTable,
            flex: 1,
            dataTableConfig: DTKeys.reteSessionEntityKeyTable,
            tableHeight: double.infinity),
        FormDataTableFieldConfig(
            key: DTKeys.reteSessionEntityDetailsTable,
            flex: 2,
            dataTableConfig: DTKeys.reteSessionEntityDetailsTable,
            tableHeight: double.infinity),
      ],
    ],
    formValidatorDelegate: alwaysValidForm,
    formActionsDelegate: processErrorsActions,
  ),

  // Query Tool Input Form
  FormKeys.queryToolInputForm: FormConfig(
    key: FormKeys.queryToolInputForm,
    title: "Query Tool",
    actions: [
      FormActionConfig(
          key: ActionKeys.queryToolOk,
          capability: "workspace_ide",
          label: "Submit Query",
          buttonStyle: ActionStyle.dialogOk,
          leftMargin: betweenTheButtonsPadding,
          rightMargin: defaultPadding),
      FormActionConfig(
          key: ActionKeys.queryToolDdlOk,
          label: "Submit DDL",
          buttonStyle: ActionStyle.dialogOk,
          leftMargin: betweenTheButtonsPadding,
          rightMargin: defaultPadding),
    ],
    inputFields: [
      [
        FormInputFieldConfig(
            key: FSK.rawQuery,
            label: "Query",
            hint: "Paste query",
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLines: 10,
            maxLength: 4000000),
      ],
    ],
    formValidatorDelegate: queryToolFormValidator,
    formActionsDelegate: queryToolFormActions,
  ),

  // Query Tool Result Viewer Form
  FormKeys.queryToolResultViewForm: FormConfig(
    key: FormKeys.queryToolResultViewForm,
    actions: [],
    inputFields: [
      [
        FormDataTableFieldConfig(
            key: DTKeys.queryToolResultSetTable,
            dataTableConfig: DTKeys.queryToolResultSetTable,
            tableHeight: double.infinity)
      ],
    ],
    formValidatorDelegate: queryToolFormValidator,
    formActionsDelegate: queryToolFormActions,
  ),
};

FormConfig getFormConfig(String key) {
  var config = _formConfigurations[key];
  if (config != null) return config;
  config = getWorkspaceFormConfig(key);
  if (config != null) return config;
  config = getClientRegistryFormConfig(key);
  if (config != null) return config;
  config = getConfigureFileFormConfig(key);
  if (config != null) return config;
  config = getFileMappingFormConfig(key);
  if (config != null) return config;
  config = getPipelineConfigFormConfig(key);
  if (config != null) return config;
  config = getLoadFilesFormConfig(key);
  if (config != null) return config;
  config = getStartPipelineFormConfig(key);
  if (config != null) return config;
  config = getWorkspacePullFormConfig(key);
  if (config != null) return config;
  throw Exception(
      'ERROR: Invalid program configuration: form configuration $key not found');
}
