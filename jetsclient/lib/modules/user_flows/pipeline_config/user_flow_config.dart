import 'package:jetsclient/models/user_flow_config.dart';
import 'package:jetsclient/modules/form_config_impl.dart';
import 'package:jetsclient/modules/user_flows/pipeline_config/form_action_delegates.dart';
import 'package:jetsclient/utils/constants.dart';

final Map<String, UserFlowConfig> _userFlowConfigurations = {
  //
  UserFlowKeys.pipelineConfigUF:
      UserFlowConfig(startAtKey: "select_add_or_edit", states: {
    "select_add_or_edit": UserFlowState(
        key: "select_add_or_edit",
        description: 'Select between add or edit pipeline_config',
        formConfig: getFormConfig(FormKeys.pcAddOrEditPipelineConfigUF),
        actionDelegate: pipelineConfigFormActionsUF,
        choices: [
          Expression(lhsStateKey: FSK.pcAddOrEditPipelineConfigOption,
          op: Operator.equals,
          rhsValue: FSK.ufAddOption,
          isRhsStateKey: false,
          nextState: 'add_pipeline_config'),
          Expression(lhsStateKey: FSK.pcAddOrEditPipelineConfigOption,
          op: Operator.equals,
          rhsValue: FSK.ufEditOption,
          isRhsStateKey: false,
          nextState: 'select_pipeline_config'),
        ]),
    "add_pipeline_config": UserFlowState(
        key: "add_pipeline_config",
        description: 'Add Pipeline Config',
        formConfig: getFormConfig(FormKeys.pcAddPipelineConfigUF),
        actionDelegate: pipelineConfigFormActionsUF,
        stateAction: ActionKeys.pcAddPipelineConfigUF,
        defaultNextState: "select_main_process_input"),
    "select_pipeline_config": UserFlowState(
        key: "select_pipeline_config",
        description: 'Select an existing Pipeline Config for mapping',
        formConfig: getFormConfig(FormKeys.pcSelectPipelineConfigUF),
        actionDelegate: pipelineConfigFormActionsUF,
        stateAction: ActionKeys.pcSelectPipelineConfigUF,
        defaultNextState: "select_main_process_input"),
    "select_main_process_input": UserFlowState(
        key: "select_main_process_input",
        description: 'Select the main process input',
        formConfig: getFormConfig(FormKeys.pcSelectMainProcessInputUF),
        actionDelegate: pipelineConfigFormActionsUF,
        stateAction: ActionKeys.pcSelectMainProcessInputUF,
        defaultNextState: "view_merge_process_inputs"),
    "view_merge_process_inputs": UserFlowState(
        key: "view_merge_process_inputs",
        description: 'View the merge process inputs',
        formConfig: getFormConfig(FormKeys.pcViewMergeProcessInputsUF),
        actionDelegate: pipelineConfigFormActionsUF,
        defaultNextState: "view_injected_process_inputs"),
    "view_injected_process_inputs": UserFlowState(
        key: "view_injected_process_inputs",
        description: 'View the merge process inputs',
        formConfig: getFormConfig(FormKeys.pcViewInjectedProcessInputsUF),
        actionDelegate: pipelineConfigFormActionsUF,
        defaultNextState: "set_pipeline_automation"),
    "set_pipeline_automation": UserFlowState(
        key: "set_pipeline_automation",
        description: 'Set pipeline automation',
        formConfig: getFormConfig(FormKeys.pcAutomationUF),
        actionDelegate: pipelineConfigFormActionsUF,
        stateAction: ActionKeys.pcSavePipelineConfigUF,
        defaultNextState: "doneUF"),
    "doneUF": UserFlowState(
        key: "doneUF",
        description: 'User Flow Completed',
        formConfig: getFormConfig(FormKeys.pcDonePipelineConfigUF),
        actionDelegate: pipelineConfigFormActionsUF,
        isEnd: true),
    // "new_main_process_inputs": UserFlowState(
    //     key: "new_main_process_inputs",
    //     description: 'Add a main process inputs',
    //     formConfig: getFormConfig(FormKeys.pcNewProcessInputsUF),
    //     actionDelegate: pipelineConfigFormActionsUF,
    //     stateAction: ActionKeys.pcNewMainProcessInputUF,
    //     defaultNextState: "select_main_process_input"),
    "add_merge_process_inputs": UserFlowState(
        key: "add_merge_process_inputs",
        description: 'Add a merge process inputs',
        formConfig: getFormConfig(FormKeys.pcAddMergeProcessInputsUF),
        actionDelegate: pipelineConfigFormActionsUF,
        stateAction: ActionKeys.pcAddMergeProcessInputUF,
        defaultNextState: "view_merge_process_inputs"),
    // "new_merge_process_inputs": UserFlowState(
    //     key: "new_merge_process_inputs",
    //     description: 'Add a new merge process inputs',
    //     formConfig: getFormConfig(FormKeys.pcNewProcessInputsUF),
    //     actionDelegate: pipelineConfigFormActionsUF,
    //     stateAction: ActionKeys.pcNewMergeProcessInputUF,
    //     defaultNextState: "view_merge_process_inputs"),
    "add_injected_process_inputs": UserFlowState(
        key: "add_injected_process_inputs",
        description: 'Add an injected process inputs',
        formConfig: getFormConfig(FormKeys.pcAddInjectedProcessInputsUF),
        actionDelegate: pipelineConfigFormActionsUF,
        stateAction: ActionKeys.pcAddInjectedProcessInputUF,
        defaultNextState: "view_injected_process_inputs"),
    // "new_injected_process_inputs": UserFlowState(
    //     key: "new_injected_process_inputs",
    //     description: 'Add a new injected process inputs',
    //     formConfig: getFormConfig(FormKeys.pcNewProcessInputsUF),
    //     actionDelegate: pipelineConfigFormActionsUF,
    //     stateAction: ActionKeys.pcNewInjectedProcessInputUF,
    //     defaultNextState: "view_injected_process_inputs"),
  })
};

UserFlowConfig? getPipelineConfigUserFlowConfig(String key) {
  return _userFlowConfigurations[key];
}