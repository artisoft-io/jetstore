import 'package:jetsclient/models/user_flow_config.dart';
import 'package:jetsclient/modules/form_config_impl.dart';
import 'package:jetsclient/modules/user_flows/home_filters/form_action_delegates.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/utils/constants.dart';

final Map<String, UserFlowConfig> _userFlowConfigurations = {
  // UF to set the home filters for pipeline execution status table
  UserFlowKeys.homeFiltersUF: UserFlowConfig(
    startAtKey: "select_process",
    states: {
      "select_process": UserFlowState(
          key: "select_process",
          description: 'Select process filter',
          formConfig: getFormConfig(FormKeys.hfSelectProcessUF),
          actionDelegate: homeFiltersFormActionsUF,
          stateAction: ActionKeys.hfSelectProcessUF,
          defaultNextState: "select_status"),
      "select_status": UserFlowState(
          key: "select_status",
          description: 'Select status filter',
          formConfig: getFormConfig(FormKeys.hfSelectStatusUF),
          actionDelegate: homeFiltersFormActionsUF,
          stateAction: ActionKeys.hfSelectStatusUF,
          defaultNextState: "select_file_key_filter"),
      "select_file_key_filter": UserFlowState(
          key: "select_file_key_filter",
          description: 'Select file key filter',
          formConfig: getFormConfig(FormKeys.hfSelectFileKeyFilterUF),
          actionDelegate: homeFiltersFormActionsUF,
          stateAction: ActionKeys.hfSelectFileKeyFilterUF,
          defaultNextState: "select_time_window"),
      "select_time_window": UserFlowState(
          key: "select_time_window",
          description: 'Select time windows of pipeline execution',
          formConfig: getFormConfig(FormKeys.hfSelectTimeWindowUF),
          actionDelegate: homeFiltersFormActionsUF,
          stateAction: ActionKeys.hfSelectTimeWindowUF,
          defaultNextState: "view_status_table"),
      "view_status_table": UserFlowState(
          key: "view_status_table",
          description: 'Pipeline Execution Status',
          formConfig: getFormConfig(FormKeys.hfViewStatusTableUF),
          actionDelegate: homeFiltersFormActionsUF,
          isEnd: true),
    },
    formStateInitializer: (formState) {
      final state = JetsRouterDelegate().homeFiltersState;
      state.forEach((key, value) {
        formState.setValue(0, key, value);
      });
    },
  ),
};

UserFlowConfig? getHomeFiltersUserFlowConfig(String key) {
  return _userFlowConfigurations[key];
}
