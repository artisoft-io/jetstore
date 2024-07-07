import 'package:jetsclient/models/user_flow_config.dart';
import 'package:jetsclient/modules/user_flows/start_pipeline/form_action_delegates.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/form_config.dart';

/// Form Config for Start Pipeline UF Module

final Map<String, FormConfig> _formConfigurations = {

  FormKeys.spSelectPipelineConfigUF: FormConfig(
    key: FormKeys.spSelectPipelineConfigUF,
    actions: standardActions,
    inputFields: [
      [
        FormDataTableFieldConfig(
            key: FSK.pipelineConfigKey,
            dataTableConfig: FSK.pipelineConfigKey,
            tableHeight: double.infinity)
      ],
    ],
    formValidatorDelegate: startPipelineFormValidator,
    formActionsDelegate: doNothingAction,
  ),

  FormKeys.spSelectMainDataSourceUF: FormConfig(
    key: FormKeys.spSelectMainDataSourceUF,
    actions: standardActions,
    inputFields: [
      [
        FormDataTableFieldConfig(
            key: FSK.mainInputRegistryKey,
            dataTableConfig: FSK.mainInputRegistryKey,
            tableHeight: double.infinity)
      ],
    ],
    formValidatorDelegate: startPipelineFormValidator,
    formActionsDelegate: doNothingAction,
  ),

  FormKeys.spSelectMergedDataSourcesUF: FormConfig(
    key: FormKeys.spSelectMainDataSourceUF,
    actions: standardActions,
    inputFields: [
      [
        // Table to show the merge process_input of the selected pipeline above
        // this is informative to the user
        FormDataTableFieldConfig(
            key: DTKeys.mergeProcessInputTable,
            dataTableConfig: DTKeys.mergeProcessInputTable,
            tableHeight: 220),
      ],
      [
        FormDataTableFieldConfig(
            key: FSK.mergedInputRegistryKeys,
            dataTableConfig: FSK.mergedInputRegistryKeys,
            tableHeight: double.infinity)
      ],
    ],
    formValidatorDelegate: startPipelineFormValidator,
    formActionsDelegate: doNothingAction,
  ),

  FormKeys.spSummaryUF: FormConfig(
    key: FormKeys.spSummaryUF,
    title: "Run Pipeline Summary",
    useListView: false,
    actions: [
      FormActionConfig(
          key: ActionKeys.ufPrevious,
          label: "Previous",
          buttonStyle: ActionStyle.ufPrimary,
          leftMargin: defaultPadding,
          rightMargin: betweenTheButtonsPadding),
      FormActionConfig(
          key: ActionKeys.ufCancel,
          label: "Cancel",
          buttonStyle: ActionStyle.ufPrimary,
          leftMargin: betweenTheButtonsPadding,
          rightMargin: defaultPadding),
      FormActionConfig(
          key: ActionKeys.ufCompleted,
          label: "Start Pipeline & Done",
          buttonStyle: ActionStyle.ufSecondary,
          leftMargin: defaultPadding,
          rightMargin: defaultPadding),
    ],
    inputFieldsV2: [
      FormFieldRowConfig(flex:0, rowConfig: [
        FormInputFieldConfig(
            key: FSK.client,
            label: "Client",
            hint: "",
            flex: 1,
            isReadOnly: true,
            autofocus: false,
            obscureText: false,
            maxLength: 60,
            textRestriction: TextRestriction.none,
            useDefaultFont: true),
        FormInputFieldConfig(
            key: FSK.processName,
            label: "Process Name",
            hint: "",
            flex: 1,
            isReadOnly: true,
            autofocus: false,
            obscureText: false,
            maxLength: 100,
            textRestriction: TextRestriction.none,
            useDefaultFont: true),
      ]),
      FormFieldRowConfig(flex:0, rowConfig: [
        FormInputFieldConfig(
            key: FSK.description,
            label: "Description",
            hint: "Pipeline configuration description",
            isReadOnly: true,
            flex: 1,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 1024,
            useDefaultFont: true),
      ]),
      FormFieldRowConfig(flex:3, rowConfig: [
        FormDataTableFieldConfig(
            key: DTKeys.spSummaryDataSources,
            dataTableConfig: DTKeys.spSummaryDataSources,
            tableHeight: 224),
      ]),
      FormFieldRowConfig(flex:2, rowConfig: [
        FormDataTableFieldConfig(
            key: DTKeys.spInjectedProcessInput,
            dataTableConfig: DTKeys.spInjectedProcessInput,
            tableHeight: 224)
      ]),
      // FormFieldRowConfig(flex:0, rowConfig: [
      //   PaddingConfig(),
      //   PaddingConfig(),
      //   FormActionConfig(
      //       key: ActionKeys.spTestPipelineUF,
      //       label: "Test Pipeline & Done",
      //       buttonStyle: ActionStyle.ufSecondary,
      //       leftMargin: defaultPadding,
      //       rightMargin: defaultPadding),
      // ]),
    ],
    formValidatorDelegate: startPipelineFormValidator,
    formActionsDelegate: doNothingAction,
  ),
};

FormConfig? getStartPipelineFormConfig(String key) {
  return _formConfigurations[key];
}
