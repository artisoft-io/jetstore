import 'package:flutter/material.dart';
import 'package:adaptive_theme/adaptive_theme.dart';
import 'package:jetsclient/modules/user_flow_config_impl.dart';
import 'package:jetsclient/routes/export_routes.dart';
import 'package:jetsclient/screens/screen_form.dart';
import 'package:jetsclient/screens/screen_multi_form.dart';
import 'package:jetsclient/screens/screen_one.dart';
import 'package:jetsclient/models/user.dart';
import 'package:jetsclient/screens/screen_tab_form.dart';
import 'package:jetsclient/screens/user_flow_screen.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/modules/data_table_config_impl.dart';
import 'package:jetsclient/modules/form_config_impl.dart';
import 'package:jetsclient/modules/screen_config_impl.dart';

const PARAM_CHAR = ':';

// Keys for UI pages
const homePath = '/';
// Expert screen, to be replaced by UFs
// const processInputPath = '/processInput';
const domainTableViewerPath = '/domainTableViewer/:table_name/:session_id';
const queryToolPath = '/queryTool';
const filePreviewPath = '/filePreviewPath/:file_key';
const executionStatusDetailsPath = '/executionStatusDetails/:session_id';
const processErrorsPath = '/processErrors/:session_id';

// Old Rule Config with triples
const processConfigPath = '/processConfig';
// Rule Configv2
const ruleConfigPath = '/ruleConfig';

const pageNotFoundPath = '/404';
const loginPath = '/login';
const registerPath = '/register';
const userAdminPath = '/userAdmin';
const userGitProfilePath =
    '/userGitProfile/:user_email/:git_name/:git_email/:git_handle';

// Workspace IDE paths
const workspaceRegistryPath = '/workspaces';
const workspaceHomePath = '/workspaces/:workspace_name/home';

// User Flow Paths
const ufClientRegistryPath = '/clientRegistryUF/:startAtKey';
const ufSourceConfigPath = '/sourceConfigUF/:startAtKey';
const ufFileMappingPath = '/fileMappingUF';
const ufMappingPath = '/fileMappingUF/mapping/:table_name/:object_type';
const ufPipelineConfigPath = '/pipelineConfigUF';
const ufLoadFilesPath = '/loadFilesUF';
const ufStartPipelinePath = '/startPipelineUF';
const ufPullWorkspacePath =
    '/pullWorkspaceUF/:key/:workspace_name/:workspace_branch/:feature_branch/:workspace_uri';
const ufLoadConfigPath = '/workspaces/loadConfigUF/:workspace_name';

final Map<String, Widget> jetsRoutesMap = {
  // Home Screen
  homePath: ScreenWithForm(
    key: const Key(ScreenKeys.home),
    screenPath: const JetsRouteData(homePath),
    screenConfig: getScreenConfig(ScreenKeys.home),
    formConfig: getFormConfig(FormKeys.home),
  ),

  // // Process Input
  // processInputPath: ScreenWithForm(
  //   key: const Key(ScreenKeys.processInput),
  //   screenPath: const JetsRouteData(processInputPath),
  //   screenConfig: getScreenConfig(ScreenKeys.processInput),
  //   formConfig: getFormConfig(FormKeys.processInput),
  // ),

  // Rule Config
  processConfigPath: ScreenWithForm(
    key: const Key(ScreenKeys.processConfig),
    screenPath: const JetsRouteData(processConfigPath),
    screenConfig: getScreenConfig(ScreenKeys.processConfig),
    formConfig: getFormConfig(FormKeys.processConfig),
  ),

  // Rule Configv2
  ruleConfigPath: ScreenWithForm(
    key: const Key(ScreenKeys.ruleConfigv2),
    screenPath: const JetsRouteData(ruleConfigPath),
    screenConfig: getScreenConfig(ScreenKeys.ruleConfigv2),
    formConfig: getFormConfig(FormKeys.rulesConfigv2),
  ),

  // Workspace IDE - Workspace Registry
  workspaceRegistryPath: ScreenWithForm(
      key: const Key(ScreenKeys.workspaceRegistry),
      screenPath: const JetsRouteData(workspaceRegistryPath),
      screenConfig: getScreenConfig(ScreenKeys.workspaceRegistry),
      formConfig: getFormConfig(FormKeys.workspaceRegistry)),

  // Workspace IDE - Workspace Home
  workspaceHomePath: ScreenWithTabsWithForm(
    key: const Key(ScreenKeys.workspaceHome),
    screenPath: const JetsRouteData(workspaceHomePath),
    screenConfig: getScreenConfig(ScreenKeys.workspaceHome),
    formConfig: getFormConfig(FormKeys.workspaceHome),
  ),

  // Login Screen
  loginPath: ScreenWithForm(
    key: const Key(ScreenKeys.login),
    screenPath: const JetsRouteData(loginPath),
    screenConfig: getScreenConfig(ScreenKeys.login),
    formConfig: getFormConfig(FormKeys.login),
  ),

  // Register Screen
  registerPath: ScreenWithForm(
    key: const Key(ScreenKeys.register),
    screenPath: const JetsRouteData(registerPath),
    screenConfig: getScreenConfig(ScreenKeys.register),
    formConfig: getFormConfig(FormKeys.register),
  ),

  // User Adminstration Screen
  userAdminPath: ScreenWithForm(
    key: const Key(ScreenKeys.userAdmin),
    screenPath: const JetsRouteData(userAdminPath),
    screenConfig: getScreenConfig(ScreenKeys.userAdmin),
    formConfig: getFormConfig(FormKeys.userAdmin),
  ),

  // User Git Profile Screen
  userGitProfilePath: ScreenWithForm(
    key: const Key(ScreenKeys.userGitProfile),
    screenPath: const JetsRouteData(userGitProfilePath),
    screenConfig: getScreenConfig(ScreenKeys.userGitProfile),
    formConfig: getFormConfig(FormKeys.userGitProfile),
  ),

  // Domain Table Viewer
  domainTableViewerPath: ScreenOne(
      key: const Key(ScreenKeys.fileRegistryTable),
      screenPath: const JetsRouteData(domainTableViewerPath),
      screenConfig: getScreenConfig(ScreenKeys.fileRegistryTable),
      validatorDelegate: (formState, p2, p3, p4) => null,
      actionsDelegate: (context, formKey, formState, actionKey,
          {group = 0}) async {
        return null;
      },
      tableConfig: getTableConfig(DTKeys.inputTable)),

  // Query Tool
  queryToolPath: ScreenWithMultiForms(
    key: const Key(ScreenKeys.queryToolScreen),
    screenPath: const JetsRouteData(queryToolPath),
    screenConfig: getScreenConfig(ScreenKeys.queryToolScreen),
    formConfig: [
      getFormConfig(FormKeys.queryToolInputForm),
      getFormConfig(FormKeys.queryToolResultViewForm),
    ],
  ),

  // File Preview
  filePreviewPath: ScreenOne(
      key: const Key(ScreenKeys.filePreview),
      screenPath: const JetsRouteData(filePreviewPath),
      screenConfig: getScreenConfig(ScreenKeys.filePreview),
      validatorDelegate: (formState, p2, p3, p4) => null,
      actionsDelegate: (context, formKey, formState, actionKey,
          {group = 0}) async {
        return null;
      },
      tableConfig: getTableConfig(DTKeys.inputFileViewerTable)),

  // Pipeline Execution Status Details Viewer
  executionStatusDetailsPath: ScreenOne(
      key: const Key(ScreenKeys.execStatusDetailsTable),
      screenPath: const JetsRouteData(executionStatusDetailsPath),
      screenConfig: getScreenConfig(ScreenKeys.execStatusDetailsTable),
      validatorDelegate: (formState, p2, p3, p4) => null,
      actionsDelegate: (context, formKey, formState, actionKey,
          {group = 0}) async {
        return null;
      },
      tableConfig: getTableConfig(DTKeys.pipelineExecDetailsTable)),

  // Process Errors Viewer
  processErrorsPath: ScreenWithForm(
    key: const Key(ScreenKeys.processErrorsTable),
    screenPath: const JetsRouteData(processErrorsPath),
    screenConfig: getScreenConfig(ScreenKeys.processErrorsTable),
    formConfig: getFormConfig(FormKeys.viewProcessErrors),
  ),

  // Client Registry User Flow
  ufClientRegistryPath: UserFlowScreen(
    key: const Key(UserFlowKeys.clientRegistryUF),
    screenPath: const JetsRouteData(ufClientRegistryPath),
    screenConfig: getScreenConfig(ScreenKeys.ufClientRegistry),
    userFlowConfig: getUserFlowConfig(UserFlowKeys.clientRegistryUF),
  ),
  ufSourceConfigPath: UserFlowScreen(
    key: const Key(UserFlowKeys.sourceConfigUF),
    screenPath: const JetsRouteData(ufSourceConfigPath),
    screenConfig: getScreenConfig(ScreenKeys.ufSourceConfig),
    userFlowConfig: getUserFlowConfig(UserFlowKeys.sourceConfigUF),
  ),
  ufFileMappingPath: UserFlowScreen(
    key: const Key(UserFlowKeys.fileMappingUF),
    screenPath: const JetsRouteData(ufFileMappingPath),
    screenConfig: getScreenConfig(ScreenKeys.ufFileMapping),
    userFlowConfig: getUserFlowConfig(UserFlowKeys.fileMappingUF),
  ),
  ufMappingPath: UserFlowScreen(
    key: const Key(UserFlowKeys.mapFileUF),
    screenPath: const JetsRouteData(ufMappingPath),
    screenConfig: getScreenConfig(ScreenKeys.ufFileMapping),
    userFlowConfig: getUserFlowConfig(UserFlowKeys.mapFileUF),
  ),
  ufPipelineConfigPath: UserFlowScreen(
    key: const Key(UserFlowKeys.pipelineConfigUF),
    screenPath: const JetsRouteData(ufPipelineConfigPath),
    screenConfig: getScreenConfig(ScreenKeys.ufPipelineConfig),
    userFlowConfig: getUserFlowConfig(UserFlowKeys.pipelineConfigUF),
  ),
  ufLoadFilesPath: UserFlowScreen(
    key: const Key(UserFlowKeys.loadFilesUF),
    screenPath: const JetsRouteData(ufLoadFilesPath),
    screenConfig: getScreenConfig(ScreenKeys.ufLoadFiles),
    userFlowConfig: getUserFlowConfig(UserFlowKeys.loadFilesUF),
  ),
  ufStartPipelinePath: UserFlowScreen(
    key: const Key(UserFlowKeys.startPipelineUF),
    screenPath: const JetsRouteData(ufStartPipelinePath),
    screenConfig: getScreenConfig(ScreenKeys.ufStartPipeline),
    userFlowConfig: getUserFlowConfig(UserFlowKeys.startPipelineUF),
  ),
  ufPullWorkspacePath: UserFlowScreen(
    key: const Key(UserFlowKeys.workspacePullUF),
    screenPath: const JetsRouteData(ufPullWorkspacePath),
    screenConfig: getScreenConfig(ScreenKeys.ufPullWorkspace),
    userFlowConfig: getUserFlowConfig(UserFlowKeys.workspacePullUF),
  ),
  ufLoadConfigPath: UserFlowScreen(
    key: const Key(UserFlowKeys.loadConfigUF),
    screenPath: const JetsRouteData(ufLoadConfigPath),
    screenConfig: getScreenConfig(ScreenKeys.ufLoadConfig),
    userFlowConfig: getUserFlowConfig(UserFlowKeys.loadConfigUF),
  ),

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
              JetsRouterDelegate().user = UserModel();
              JetsRouterDelegate()(const JetsRouteData(loginPath));
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
