import 'package:flutter/material.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/models/form_config.dart';
import 'package:jetsclient/models/user_flow_config.dart';
import 'package:jetsclient/modules/actions/config_delegates.dart';
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
    formValidatorDelegate: pipelineConfigFormValidator,
    formActionsDelegate: doNothingAction, // overriden by UserFlowState.actionDelegate
  ),
  FormKeys.pcAddPipelineConfigUF: FormConfig(
    key: FormKeys.pcAddPipelineConfigUF,
    useListView: true,
    actions: standardActions,
    inputFields: [
      [
        PaddingConfig(height: 2*defaultPadding),
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
    formValidatorDelegate: pipelineConfigFormValidator,
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
    formValidatorDelegate: pipelineConfigFormValidator,
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
            key: FSK.mainProcessInputKey,
            dataTableConfig: FSK.mainProcessInputKey,
            tableHeight: double.infinity),
      ],
    ],
    formValidatorDelegate: pipelineConfigFormValidator,
    formActionsDelegate:
        doNothingAction, // overriden by UserFlowState.actionDelegate
  ),
  // View Merged Input
  FormKeys.pcViewMergeProcessInputsUF: FormConfig(
    key: FormKeys.pcViewMergeProcessInputsUF,
    actions: standardActions,
    useListView: true,
    inputFields: [
      [
        FormDataTableFieldConfig(
            key: DTKeys.pcViewMergedProcessInputKeys,
            dataTableConfig: DTKeys.pcViewMergedProcessInputKeys),
      ],
      [
        PaddingConfig(height: 2*defaultPadding),
      ],
      [
        FormActionConfig(
            key: ActionKeys.pcGotToAddMergeProcessInputUF,
            label: "Add Input to Merge",
            buttonStyle: ActionStyle.secondary,
            leftMargin: defaultPadding,
            rightMargin: defaultPadding),
        PaddingConfig(),
        PaddingConfig(),
      ],
      [
        PaddingConfig(),
      ],
    ],
    formValidatorDelegate: pipelineConfigFormValidator,
    formActionsDelegate:
        doNothingAction, // overriden by UserFlowState.actionDelegate
  ),
  // View Injected Input
  FormKeys.pcViewInjectedProcessInputsUF: FormConfig(
    key: FormKeys.pcViewInjectedProcessInputsUF,
    actions: standardActions,
    useListView: true,
    inputFields: [
      [
        FormDataTableFieldConfig(
            key: DTKeys.pcViewInjectedProcessInputKeys,
            dataTableConfig: DTKeys.pcViewInjectedProcessInputKeys),
      ],
      [
        PaddingConfig(height: 2*defaultPadding),
      ],
      [
        FormActionConfig(
            key: ActionKeys.pcGotToAddInjectedProcessInputUF,
            label: "Add Input to Inject",
            buttonStyle: ActionStyle.secondary,
            leftMargin: defaultPadding,
            rightMargin: defaultPadding),
        PaddingConfig(),
        PaddingConfig(),
      ],
      [
        PaddingConfig(),
      ],
    ],
    formValidatorDelegate: pipelineConfigFormValidator,
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
    formValidatorDelegate: pipelineConfigFormValidator,
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
    formValidatorDelegate: pipelineConfigFormValidator,
    formActionsDelegate:
        doNothingAction, // overriden by UserFlowState.actionDelegate
  ),
  FormKeys.pcAutomationUF: FormConfig(
    key: FormKeys.pcAutomationUF,
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
          label: "Save and Finish",
          buttonStyle: ActionStyle.secondary,
          leftMargin: betweenTheButtonsPadding,
          rightMargin: defaultPadding),
    ],
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
            defaultItemPos: 0),
      ],
      [
        FormDropdownFieldConfig(
            key: FSK.automated,
            items: [
              DropdownItemConfig(label: 'Select automation mode'),
              DropdownItemConfig(label: 'Automated', value: '1'),
              DropdownItemConfig(label: 'Manual', value: '0'),
            ],
            flex: 1,
            defaultItemPos: 0),
      ],
    ],
    formValidatorDelegate: pipelineConfigFormValidator,
    formActionsDelegate:
        doNothingAction, // overriden by UserFlowState.actionDelegate
  ),
  // Done Page
  FormKeys.pcDonePipelineConfigUF: FormConfig(
    key: FormKeys.pcDonePipelineConfigUF,
    title: "Pipeline Configuration",
    useListView: true,
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
        PaddingConfig(height: 2*defaultPadding),
      ],
      [
        TextFieldConfig(
            label: "Congratulation, your Pipeline Configuration is completed.",
            maxLines: 1,
            topMargin: 0,
            bottomMargin: 0),
      ],
    ],
    formValidatorDelegate: alwaysValidForm,
    formActionsDelegate: doNothingAction,
  ),
};

FormConfig? getPipelineConfigFormConfig(String key) {
  return _formConfigurations[key];
}
