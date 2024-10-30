import 'package:flutter/material.dart';
import 'package:jetsclient/models/form_config.dart';
import 'package:jetsclient/models/user_flow_config.dart';
import 'package:jetsclient/modules/actions/config_delegates.dart';
import 'package:jetsclient/modules/user_flows/pipeline_config/form_action_delegates.dart';
import 'package:jetsclient/utils/constants.dart';

/// Form Config for Pipeline Config UF Module

final Map<String, FormConfig> _formConfigurations = {
  FormKeys.pcAddOrEditPipelineConfigUF: FormConfig(
    key: FormKeys.pcAddOrEditPipelineConfigUF,
    useListView: true,
    actions: standardActions,
    inputFields: [
      [
        PaddingConfig(height: 2*defaultPadding),
      ],
      [
        FormDataTableFieldConfig(
            key: FSK.pcAddOrEditPipelineConfigOption,
            dataTableConfig: FSK.pcAddOrEditPipelineConfigOption),
      ],
    ],
    formValidatorDelegate: pipelineConfigFormValidatorUF,
    formActionsDelegate: doNothingAction, // overriden by UserFlowState.actionDelegate
  ),
  FormKeys.pcAddPipelineConfigUF: FormConfig(
    key: FormKeys.pcAddPipelineConfigUF,
    useListView: true,
    actions: standardActions,
    inputFields: [
      [
        PaddingConfig(height: defaultPadding),
      ],
      [
        FormDropdownFieldConfig(
            key: FSK.client,
            items: [
              DropdownItemConfig(label: 'Select a Client'),
            ],
            autovalidateMode: AutovalidateMode.onUserInteraction,
            dropdownItemsQuery:
                "SELECT client FROM jetsapi.client_registry ORDER BY client ASC LIMIT 150"),
      ],
      [
        PaddingConfig(height: defaultPadding),
      ],
      [
        FormDropdownFieldConfig(
            key: FSK.processName,
            returnedModelCacheKey: FSK.processConfigCache,
            items: [
              DropdownItemConfig(label: 'Select a process'),
            ],
            autovalidateMode: AutovalidateMode.onUserInteraction,
            dropdownItemsQuery:
                "SELECT process_name, key FROM jetsapi.process_config ORDER BY process_name ASC LIMIT 100"),
      ],
      [
        PaddingConfig(height: defaultPadding),
      ],
      [
        FormInputFieldConfig(
            key: FSK.description,
            label: "Description",
            hint: "Pipeline configuration description",
            flex: 2,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 512,
            useDefaultFont: true),
      ],
      [
        PaddingConfig(height: defaultPadding),
      ],
    ],
    formValidatorDelegate: pipelineConfigFormValidatorUF,
    formActionsDelegate: doNothingAction, // overriden by UserFlowState.actionDelegate
  ),

  FormKeys.pcSelectPipelineConfigUF: FormConfig(
    key: FormKeys.pcSelectPipelineConfigUF,
    actions: standardActions,
    useListView: false,
    inputFields: [
      [
        FormDataTableFieldConfig(
            key: DTKeys.pcPipelineConfigTable,
            dataTableConfig: DTKeys.pcPipelineConfigTable,
            tableHeight: double.infinity),
      ],
    ],
    formValidatorDelegate: pipelineConfigFormValidatorUF,
    formActionsDelegate:
        doNothingAction, // overriden by UserFlowState.actionDelegate
  ),

  FormKeys.pcSelectMainProcessInputUF: FormConfig(
    key: FormKeys.pcSelectMainProcessInputUF,
    actions: standardActions,
    useListView: false,
    inputFields: [
      [
        FormDataTableFieldConfig(
            key: DTKeys.pcMainProcessInputKey,
            dataTableConfig: DTKeys.pcMainProcessInputKey,
            tableHeight: double.infinity),
      ],
    ],
    formValidatorDelegate: pipelineConfigFormValidatorUF,
    formActionsDelegate:
        doNothingAction, // overriden by UserFlowState.actionDelegate
  ),
  // View Merged Input
  FormKeys.pcViewMergeProcessInputsUF: FormConfig(
    key: FormKeys.pcViewMergeProcessInputsUF,
    actions: standardActions,
    useListView: false,
    inputFieldsV2: [
      FormFieldRowConfig(flex:1, rowConfig: [
        FormDataTableFieldConfig(
            key: DTKeys.pcViewMergedProcessInputKeys,
            dataTableConfig: DTKeys.pcViewMergedProcessInputKeys,
            tableHeight: double.infinity),
      ]),
      FormFieldRowConfig(rowConfig: [
        FormActionConfig(
            key: ActionKeys.pcGotToAddMergeProcessInputUF,
            label: "Add Data Source to Merge",
            buttonStyle: ActionStyle.predominentInForm,
            leftMargin: defaultPadding,
            rightMargin: defaultPadding),
        PaddingConfig(),
        PaddingConfig(),
      ]),
    ],
    formValidatorDelegate: pipelineConfigFormValidatorUF,
    formActionsDelegate:
        doNothingAction, // overriden by UserFlowState.actionDelegate
  ),
  // View Injected Input
  FormKeys.pcViewInjectedProcessInputsUF: FormConfig(
    key: FormKeys.pcViewInjectedProcessInputsUF,
    actions: standardActions,
    useListView: false,
    inputFieldsV2: [
      FormFieldRowConfig(flex:1, rowConfig: [
        FormDataTableFieldConfig(
            key: DTKeys.pcViewInjectedProcessInputKeys,
            dataTableConfig: DTKeys.pcViewInjectedProcessInputKeys,
            tableHeight: double.infinity),
      ]),
      FormFieldRowConfig(rowConfig: [PaddingConfig()]),
      FormFieldRowConfig(rowConfig: [
        FormActionConfig(
            key: ActionKeys.pcGotToAddInjectedProcessInputUF,
            label: "Add Data Source for Historical Data",
            buttonStyle: ActionStyle.predominentInForm,
            leftMargin: defaultPadding,
            rightMargin: defaultPadding),
        PaddingConfig(),
        PaddingConfig(),
      ]),
    ],
    formValidatorDelegate: pipelineConfigFormValidatorUF,
    formActionsDelegate:
        doNothingAction, // overriden by UserFlowState.actionDelegate
  ),
  FormKeys.pcAddMergeProcessInputsUF: FormConfig(
    key: FormKeys.pcAddMergeProcessInputsUF,
    actions: standardActions,
    useListView: false,
    inputFields: [
      [
        FormDataTableFieldConfig(
            key: DTKeys.pcMergedProcessInputKeys,
            dataTableConfig: DTKeys.pcMergedProcessInputKeys,
            tableHeight: double.infinity),
      ],
    ],
    formValidatorDelegate: pipelineConfigFormValidatorUF,
    formActionsDelegate:
        doNothingAction, // overriden by UserFlowState.actionDelegate
  ),
  FormKeys.pcAddInjectedProcessInputsUF: FormConfig(
    key: FormKeys.pcAddInjectedProcessInputsUF,
    actions: standardActions,
    useListView: false,
    inputFields: [
      [
        FormDataTableFieldConfig(
            key: DTKeys.pcInjectedProcessInputKeys,
            dataTableConfig: DTKeys.pcInjectedProcessInputKeys,
            tableHeight: double.infinity),
      ],
    ],
    formValidatorDelegate: pipelineConfigFormValidatorUF,
    formActionsDelegate:
        doNothingAction, // overriden by UserFlowState.actionDelegate
  ),
  FormKeys.pcAutomationUF: FormConfig(
    key: FormKeys.pcAutomationUF,
    actions: standardActions,
    useListView: true,
    inputFields: [
      [
        FormDropdownFieldConfig(
            key: FSK.sourcePeriodType,
            items: [
              DropdownItemConfig(label: 'Select execution frequency'),
              DropdownItemConfig(label: 'Monthly', value: 'month_period'),
              DropdownItemConfig(label: 'Weekly', value: 'week_period'),
              DropdownItemConfig(label: 'Daily', value: 'day_period'),
            ],
            flex: 1,
            defaultItemPos: 1),
        FormDropdownFieldConfig(
            key: FSK.automated,
            items: [
              DropdownItemConfig(label: 'Select automation mode'),
              DropdownItemConfig(label: 'Automated', value: '1'),
              DropdownItemConfig(label: 'Manual', value: '0'),
            ],
            flex: 1,
            defaultItemPos: 1),
      ],
      [
        TextFieldConfig(
            label: "Paste or enter the Rule Configuration as json or csv:",
            maxLines: 1,
            topMargin: 0,
            bottomMargin: 0),
      ],
      [
        FormInputFieldConfig(
            key: FSK.ruleConfigJson,
            label: "Rule Configuration (csv or json)",
            hint: "Pipeline-specific Rule Configuration",
            flex: 1,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            defaultValue: '[]',
            maxLines: 13,
            maxLength: 51200),
      ],
      [
        PaddingConfig(height: 2*defaultPadding),
      ],
    ],
    formValidatorDelegate: pipelineConfigFormValidatorUF,
    formActionsDelegate:
        doNothingAction, // overriden by UserFlowState.actionDelegate
  ),
  // New Process Input Dialog
  FormKeys.pcNewProcessInputDialog: FormConfig(
    key: FormKeys.pcNewProcessInputDialog,
    title: "New Process Input",
    useListView: true,
    actions: [
      FormActionConfig(
          key: ActionKeys.addProcessInputOk,
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
    inputFields: [
      [
        FormInputFieldConfig(
            key: FSK.client,
            label: "Client",
            hint: "",
            flex: 1,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            isReadOnly: true,
            maxLength: 25,
            useDefaultFont: true),
        PaddingConfig(),
        PaddingConfig(),
      ],
      [
        FormDataTableFieldConfig(
            key: DTKeys.pcProcessInputRegistry,
            dataTableConfig: DTKeys.pcProcessInputRegistry,
            tableHeight: 380),
      ],
      [
        PaddingConfig(height: defaultPadding),
      ],
      [
        FormInputFieldConfig(
            key: FSK.lookbackPeriods,
            label: "Lookback Periods",
            hint: "Number of periods to include in the rule session",
            defaultValue: "0",
            flex: 1,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.digitsOnly,
            maxLength: 10,
            useDefaultFont: true),
        PaddingConfig(),
        PaddingConfig(),
      ],
      [
        PaddingConfig(),
      ],
    ],
    formValidatorDelegate: pipelineConfigFormValidatorUF,
    formActionsDelegate: processInputFormActions,
  ),
  // New Process Input Dialog for Merge and Injected Process Inuts
  FormKeys.pcNewProcessInputDialog4MI: FormConfig(
    key: FormKeys.pcNewProcessInputDialog4MI,
    title: "New Process Input",
    useListView: true,
    actions: [
      FormActionConfig(
          key: ActionKeys.addProcessInputOk,
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
    inputFields: [
      [
        FormInputFieldConfig(
            key: FSK.client,
            label: "Client",
            hint: "",
            flex: 1,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            isReadOnly: true,
            maxLength: 25,
            useDefaultFont: true),
        PaddingConfig(),
        PaddingConfig(),
      ],
      [
        FormDataTableFieldConfig(
            key: DTKeys.pcProcessInputRegistry4MI,
            dataTableConfig: DTKeys.pcProcessInputRegistry4MI,
            tableHeight: 380),
      ],
      [
        PaddingConfig(height: defaultPadding),
      ],
      [
        FormInputFieldConfig(
            key: FSK.lookbackPeriods,
            label: "Lookback Periods",
            hint: "Number of periods to include in the rule session",
            defaultValue: "0",
            flex: 1,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.digitsOnly,
            maxLength: 10,
            useDefaultFont: true),
        PaddingConfig(),
        PaddingConfig(),
      ],
      [
        PaddingConfig(),
      ],
    ],
    formValidatorDelegate: pipelineConfigFormValidatorUF,
    formActionsDelegate: processInputFormActions,
  ),
  // Summary Page
  FormKeys.pcSummaryUF: FormConfig(
    key: FormKeys.pcSummaryUF,
    title: "Pipeline Configuration Summary",
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
          label: "Save & Done",
          buttonStyle: ActionStyle.ufSecondary,
          leftMargin: betweenTheButtonsPadding,
          rightMargin: defaultPadding),
    ],
    useListView: true,
    inputFields: [
      [
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
      ],
      [
        FormDropdownFieldConfig(
            key: FSK.sourcePeriodType,
            isReadOnly: true,
            items: [
              DropdownItemConfig(label: 'Select execution frequency'),
              DropdownItemConfig(label: 'Monthly', value: 'month_period'),
              DropdownItemConfig(label: 'Weekly', value: 'week_period'),
              DropdownItemConfig(label: 'Daily', value: 'day_period'),
            ],
            flex: 1,
            defaultItemPos: 0),
        FormDropdownFieldConfig(
            key: FSK.automated,
            isReadOnly: true,
            items: [
              DropdownItemConfig(label: 'Select automation mode'),
              DropdownItemConfig(label: 'Automated', value: '1'),
              DropdownItemConfig(label: 'Manual', value: '0'),
            ],
            flex: 1,
            defaultItemPos: 0),
      ],
      [
        FormInputFieldConfig(
            key: FSK.description,
            label: "Description",
            hint: "Pipeline configuration description",
            flex: 2,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 512,
            useDefaultFont: true),
      ],
      [
        PaddingConfig(height: defaultPadding),
      ],
      [
        FormDataTableFieldConfig(
            key: DTKeys.pcSummaryProcessInputs,
            dataTableConfig: DTKeys.pcSummaryProcessInputs,
            tableHeight: 272),
      ],
      [
        FormInputFieldConfig(
            key: FSK.ruleConfigJson,
            label: "Rule Configuration CSV / Json",
            hint:
                "Enter a valid json array or csv with headers of configuration objects",
            isReadOnly: true,
            maxLines: 1,
            maxLength: 51200,
            autofocus: false,
            textRestriction: TextRestriction.none,
            defaultValue: "[]"),
      ],
    ],
    formValidatorDelegate: pipelineConfigFormValidatorUF,
    formActionsDelegate: doNothingAction,
  ),
};

FormConfig? getPipelineConfigFormConfig(String key) {
  return _formConfigurations[key];
}
