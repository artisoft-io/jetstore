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

// Keys for UI pages
const homePath = '/';
const clientAdminPath = '/clientAdmin';
const sourceConfigPath = '/sourceConfig';
const inputSourceMappingPath = '/inputSourceMapping';
const processInputPath = '/processInput';
const domainTableViewerPath = '/domainTableViewer/:table/:session_id';
const filePreviewPath = '/filePreviewPath/:file_key';
const executionStatusDetailsPath = '/executionStatusDetails/:session_id';
const processErrorsPath = '/processErrorsPath/:session_id';

const processConfigPath = '/processConfig';
const pipelineConfigPath = '/pipelineConfig';
const pageNotFoundPath = '/404';
const loginPath = '/login';
const registerPath = '/register';
const userAdminPath = '/userAdmin';

final Map<String, Widget> jetsRoutesMap = {
  // Home Screen
  homePath: ScreenWithForm(
      key: const Key(ScreenKeys.home),
      screenPath: JetsRouteData(homePath),
      screenConfig: getScreenConfig(ScreenKeys.home),
      formConfig: getFormConfig(FormKeys.home),
      formValidatorDelegate: homeFormValidator,
      formActionsDelegate: homeFormActions),

  // Client & Organization Admin
  clientAdminPath: ScreenWithForm(
      key: const Key(ScreenKeys.clientAdmin),
      screenPath: JetsRouteData(clientAdminPath),
      screenConfig: getScreenConfig(ScreenKeys.clientAdmin),
      formConfig: getFormConfig(FormKeys.clientAdmin),
      // Using source config validator and actions since no widget here
      formValidatorDelegate: sourceConfigValidator,
      formActionsDelegate: sourceConfigActions),

  // Source Config
  sourceConfigPath: ScreenWithForm(
      key: const Key(ScreenKeys.sourceConfig),
      screenPath: JetsRouteData(sourceConfigPath),
      screenConfig: getScreenConfig(ScreenKeys.sourceConfig),
      formConfig: getFormConfig(FormKeys.sourceConfig),
      formValidatorDelegate: sourceConfigValidator,
      formActionsDelegate: sourceConfigActions),

  // Input Source Mapping
  inputSourceMappingPath: ScreenWithForm(
      key: const Key(ScreenKeys.inputSourceMapping),
      screenPath: JetsRouteData(inputSourceMappingPath),
      screenConfig: getScreenConfig(ScreenKeys.inputSourceMapping),
      formConfig: getFormConfig(FormKeys.inputSourceMapping),
      formValidatorDelegate: processInputFormValidator,
      formActionsDelegate: processInputFormActions),

  // Process Input
  processInputPath: ScreenWithForm(
      key: const Key(ScreenKeys.processInput),
      screenPath: JetsRouteData(processInputPath),
      screenConfig: getScreenConfig(ScreenKeys.processInput),
      formConfig: getFormConfig(FormKeys.processInput),
      formValidatorDelegate: processInputFormValidator,
      formActionsDelegate: processInputFormActions),

  // Process Config and Client Rule Config
  processConfigPath: ScreenWithForm(
      key: const Key(ScreenKeys.processConfig),
      screenPath: JetsRouteData(processConfigPath),
      screenConfig: getScreenConfig(ScreenKeys.processConfig),
      formConfig: getFormConfig(FormKeys.processConfig),
      formValidatorDelegate: processConfigFormValidator,
      formActionsDelegate: processConfigFormActions),

  // Pipeline Config
  pipelineConfigPath: ScreenOne(
      key: const Key(ScreenKeys.pipelineConfig),
      screenPath: JetsRouteData(pipelineConfigPath),
      screenConfig: getScreenConfig(ScreenKeys.pipelineConfig),
      validatorDelegate: pipelineConfigFormValidator,
      actionsDelegate: pipelineConfigFormActions,
      tableConfig: getTableConfig(DTKeys.pipelineConfigTable),),

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

  // User Adminstration Screen
  userAdminPath: ScreenWithForm(
      key: const Key(ScreenKeys.userAdmin),
      screenPath: JetsRouteData(userAdminPath),
      screenConfig: getScreenConfig(ScreenKeys.userAdmin),
      formConfig: getFormConfig(FormKeys.userAdmin),
      formValidatorDelegate: (formState, p2, p3, p4) => null,
      formActionsDelegate: userAdminFormActions),

  // Domain Table Viewer
  domainTableViewerPath: ScreenOne(
      key: const Key(ScreenKeys.fileRegistryTable),
      screenPath: JetsRouteData(domainTableViewerPath),
      screenConfig: getScreenConfig(ScreenKeys.fileRegistryTable),
      validatorDelegate: (formState, p2, p3, p4) => null,
      actionsDelegate: (context, formKey, formState, actionKey, {group = 0}) async {},
      tableConfig: getTableConfig(DTKeys.inputTable)),

  // File Preview
  filePreviewPath: ScreenOne(
      key: const Key(ScreenKeys.filePreview),
      screenPath: JetsRouteData(filePreviewPath),
      screenConfig: getScreenConfig(ScreenKeys.filePreview),
      validatorDelegate: (formState, p2, p3, p4) => null,
      actionsDelegate: (context, formKey, formState, actionKey, {group = 0}) async {},
      tableConfig: getTableConfig(DTKeys.inputFileViewerTable)),

  // Pipeline Execution Status Details Viewer
  executionStatusDetailsPath: ScreenOne(
      key: const Key(ScreenKeys.execStatusDetailsTable),
      screenPath: JetsRouteData(executionStatusDetailsPath),
      screenConfig: getScreenConfig(ScreenKeys.execStatusDetailsTable),
      validatorDelegate: (formState, p2, p3, p4) => null,
      actionsDelegate: (context, formKey, formState, actionKey, {group = 0}) async {},
      tableConfig: getTableConfig(DTKeys.pipelineExecDetailsTable)),

  // Pipeline Execution Status Details Viewer
  processErrorsPath: ScreenOne(
      key: const Key(ScreenKeys.processErrorsTable),
      screenPath: JetsRouteData(processErrorsPath),
      screenConfig: getScreenConfig(ScreenKeys.processErrorsTable),
      validatorDelegate: (formState, p2, p3, p4) => null,
      actionsDelegate: (context, formKey, formState, actionKey, {group = 0}) async {},
      tableConfig: getTableConfig(DTKeys.processErrorsTable)),

  // Page Not Found
  pageNotFoundPath: const MessageScreen(message: "Opps 404!"),
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
        child: Text(message, style: Theme.of(context).textTheme.displayMedium),
      ),
    );
  }
}
