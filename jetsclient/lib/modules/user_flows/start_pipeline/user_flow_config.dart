import 'package:jetsclient/models/user_flow_config.dart';
import 'package:jetsclient/modules/form_config_impl.dart';
import 'package:jetsclient/modules/user_flows/start_pipeline/form_action_delegates.dart';
import 'package:jetsclient/utils/constants.dart';

final Map<String, UserFlowConfig> _userFlowConfigurations = {
  //
  UserFlowKeys.startPipelineUF:
      UserFlowConfig(startAtKey: "select_pipeline_config", states: {
    "select_pipeline_config": UserFlowState(
        key: "select_pipeline_config",
        description: 'Select a pipeline configuration',
        formConfig: getFormConfig(FormKeys.spSelectPipelineConfigUF),
        actionDelegate: startPipelineFormActionsUF,
        stateAction: ActionKeys.spPipelineSelected,
        defaultNextState: "select_main_data_source"),
    "select_main_data_source": UserFlowState(
        key: "select_main_data_source",
        description: 'Select the main data source',
        formConfig: getFormConfig(FormKeys.spSelectMainDataSourceUF),
        actionDelegate: startPipelineFormActionsUF,
        stateAction: ActionKeys.spPrepareStartPipeline,
        choices: [
          IsNotExpression(
              expression: IsNullOrEmptyExpression(
                  lhsStateKey: FSK.mergedProcessInputKeys, nextState: ''),
              nextState: 'select_merged_data_sources'),
        ],
        defaultNextState: "summaryUF"),
    "select_merged_data_sources": UserFlowState(
        key: "select_merged_data_sources",
        description: 'Select the merged data sources',
        formConfig: getFormConfig(FormKeys.spSelectMergedDataSourcesUF),
        actionDelegate: startPipelineFormActionsUF,
        stateAction: ActionKeys.spPrepareStartPipeline,
        defaultNextState: "summaryUF"),
    "summaryUF": UserFlowState(
        key: "summaryUF",
        description: 'Start Pipeline Summary',
        formConfig: getFormConfig(FormKeys.spSummaryUF),
        actionDelegate: startPipelineFormActionsUF,
        stateAction: ActionKeys.spStartPipelineUF,
        isEnd: true),
  })
};

UserFlowConfig? getStartPipelineUserFlowConfig(String key) {
  return _userFlowConfigurations[key];
}
