import 'package:jetsclient/models/user_flow_config.dart';
import 'package:jetsclient/modules/user_flows/client_registry/form_action_delegates.dart';
import 'package:jetsclient/modules/form_config_impl.dart';
import 'package:jetsclient/utils/constants.dart';

final Map<String, UserFlowConfig> _userFlowConfigurations = {
  //
  UserFlowKeys.clientRegistryUF: UserFlowConfig(startAtKey: "select_client_vendor", states: {
    "select_client_vendor": UserFlowState(
        key: "select_client_vendor",
        description: 'Select between create client or add vendor/org',
        formConfig: getFormConfig(FormKeys.ufSelectClientOrVendor),
        actionDelegate: clientRegistryFormActions,
        choices: [
          Expression(lhsStateKey: FSK.ufClientOrVendorOption,
          op: Operator.equals,
          rhsValue: FSK.ufClientOption,
          isRhsStateKey: false,
          nextState: 'create_client'),
          Expression(lhsStateKey: FSK.ufClientOrVendorOption,
          op: Operator.equals,
          rhsValue: FSK.ufVendorOption,
          isRhsStateKey: false,
          nextState: 'select_client'),
        ]),
    "create_client": UserFlowState(
        key: "create_client",
        description: 'Create client',
        formConfig: getFormConfig(FormKeys.ufCreateClient),
        actionDelegate: clientRegistryFormActions,
        stateAction: ActionKeys.crAddClientUF,
        defaultNextState: "show_org"),
    "select_client": UserFlowState(
        key: "select_client",
        description: 'Select an existing client',
        formConfig: getFormConfig(FormKeys.ufSelectClient),
        actionDelegate: clientRegistryFormActions,
        stateAction: ActionKeys.crSelectClientUF,
        defaultNextState: "show_org"),
    "show_org": UserFlowState(
        key: "show_org",
        description: 'Create org/vendor of client',
        formConfig: getFormConfig(FormKeys.ufShowVendor),
        actionDelegate: clientRegistryFormActions,
        isEnd: true),
  })
};

UserFlowConfig? getScreenConfigUserFlowConfig(String key) {
  return _userFlowConfigurations[key];
}
