import 'package:flutter/material.dart';
import 'package:adaptive_theme/adaptive_theme.dart';
import 'package:jetsclient/routes/export_routes.dart';
import 'package:jetsclient/screens/screen_form.dart';
import 'package:jetsclient/screens/screen_one.dart';
import 'package:jetsclient/screens/screen_delegates/user_delegates.dart';
import 'package:jetsclient/screens/screen_delegates/config_delegates.dart';
import 'package:jetsclient/models/user.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/utils/data_table_config.dart';
import 'package:jetsclient/utils/form_config.dart';
import 'package:jetsclient/utils/screen_config.dart';

const PARAM_CHAR = ':';

// Key pages
const homePath = '/';
const sourceConfigPath = '/sourceConfig';
const processInputPath = '/processInput';
const inputMappingPath = '/processInput/mapping/:pi';
const processConfigPath = '/processConfig';
const ruleConfigPath = '/processConfig/ruleConfig/:pc';
const pipelineConfigPath = '/pipelineConfig';
const pageNotFoundPath = '/404';
const loginPath = '/login';
const registerPath = '/register';

// Old test pages
const pipelinePath = '/dataPipelines';
const mappingConfigPath = '/mappingConfig';
const fileRegistryPath = '/fileRegistry';
const fileRegistryTablePath = '/fileRegistry/table/:table';

final Map<String, Widget> jetsRoutesMap = {
  // Home Screen
  homePath: ScreenWithForm(
      key: const Key(ScreenKeys.home),
      screenPath: JetsRouteData(homePath),
      screenConfig: getScreenConfig(ScreenKeys.home),
      formConfig: getFormConfig(FormKeys.home),
      formValidatorDelegate: (context, formState, p2, p3, p4) => null,
      formActionsDelegate: homeFormActions),
  // Source Config
  sourceConfigPath: ScreenWithForm(
      key: const Key(ScreenKeys.sourceConfig),
      screenPath: JetsRouteData(sourceConfigPath),
      screenConfig: getScreenConfig(ScreenKeys.sourceConfig),
      formConfig: getFormConfig(FormKeys.sourceConfig),
      formValidatorDelegate: sourceConfigFormValidator,
      formActionsDelegate: sourceConfigFormActions),

  // Login Screen
  loginPath: ScreenWithForm(
      key: const Key(ScreenKeys.login),
      screenPath: JetsRouteData(loginPath),
      screenConfig: getScreenConfig(ScreenKeys.login),
      formConfig: getFormConfig(FormKeys.login),
      formValidatorDelegate: loginFormValidator,
      formActionsDelegate: loginFormActions),
  // Register Screen
  registerPath: ScreenWithForm(
      key: const Key(ScreenKeys.register),
      screenPath: JetsRouteData(registerPath),
      screenConfig: getScreenConfig(ScreenKeys.register),
      formConfig: getFormConfig(FormKeys.register),
      formValidatorDelegate: registrationFormValidator,
      formActionsDelegate: registrationFormActions),

  // Page Not Found
  pageNotFoundPath: const MessageScreen(message: "Opps 404!"),

  //* DEMO
  // Pipeline Screen
  pipelinePath: ScreenOne(
      key: const Key(ScreenKeys.pipelines),
      screenPath: JetsRouteData(pipelinePath),
      screenConfig: getScreenConfig(ScreenKeys.pipelines),
      validatorDelegate: (context, formState, p2, p3, p4) => null,
      actionsDelegate: (context, formKey, formState, actionKey) {},
      tableConfig: getTableConfig(DTKeys.pipelineDemo)),
  //* TEST SCREEN
  // mappingConfigPath: TestScreen(
  //     key: const Key('testScreen'),
  //     screenPath: JetsRouteData(mappingConfigPath),
  //     screenConfig: getScreenConfig("testScreen"),
  //     formConfig: getFormConfig("dataTableDemoForm")),
  // File Registry Screen
  fileRegistryPath: ScreenOne(
      key: const Key(ScreenKeys.fileRegistry),
      screenPath: JetsRouteData(fileRegistryPath),
      screenConfig: getScreenConfig(ScreenKeys.fileRegistry),
      validatorDelegate: (context, formState, p2, p3, p4) => null,
      actionsDelegate: (context, formKey, formState, actionKey) {},
      tableConfig: getTableConfig(DTKeys.registryDemo)),
  // File Registry Table Screen
  fileRegistryTablePath: ScreenOne(
      key: const Key(ScreenKeys.fileRegistryTable),
      screenPath: JetsRouteData(fileRegistryTablePath),
      screenConfig: getScreenConfig(ScreenKeys.fileRegistryTable),
      validatorDelegate: (context, formState, p2, p3, p4) => null,
      actionsDelegate: (context, formKey, formState, actionKey) {},
      tableConfig: getTableConfig(DTKeys.inputTable)),
};
const noAuthRequiredPaths = {loginPath, registerPath, pageNotFoundPath};

class MessageScreen extends StatelessWidget {
  final String message;
  const MessageScreen({required this.message, super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        automaticallyImplyLeading: false,
        title: const Text('JetStore Workspace'),
        actions: <Widget>[
          IconButton(
            icon: const Icon(Icons.dark_mode_sharp),
            tooltip: 'Toggle Theme',
            onPressed: () {
              AdaptiveTheme.of(context).toggleThemeMode();
            },
          ),
          IconButton(
            icon: const Icon(Icons.logout_sharp),
            tooltip: 'Log Out',
            onPressed: () {
              var user = UserModel();
              user.name = "";
              user.email = "";
              user.password = "";
              user.token = "";
              JetsRouterDelegate().user = user;
              JetsRouterDelegate()(JetsRouteData(loginPath));
            },
          ),
        ],
      ),
      body: Center(
        child: Text(message, style: Theme.of(context).textTheme.headline2),
      ),
    );
  }
}
