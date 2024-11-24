import 'package:jetsclient/models/user_flow_config.dart';
import 'package:jetsclient/modules/form_config_impl.dart';
import 'package:jetsclient/modules/user_flows/register_file_key/form_action_delegates.dart';
import 'package:jetsclient/utils/constants.dart';

final Map<String, UserFlowConfig> _userFlowConfigurations = {
  //
  UserFlowKeys.registerFileKeyUF:
      UserFlowConfig(startAtKey: "submit_schema_event", states: {
    "submit_schema_event": UserFlowState(
        key: "submit_schema_event",
        description: 'Submit a schema event to register a file key',
        formConfig: getFormConfig(FormKeys.rfkSubmitSchemaEvent),
        actionDelegate: registerFileKeyFormActionsUF,
        stateAction: ActionKeys.rfkSubmitSchemaEventUF,
        isEnd: true),
  })
};

UserFlowConfig? getRegisterFileKeyUserFlowConfig(String key) {
  return _userFlowConfigurations[key];
}
