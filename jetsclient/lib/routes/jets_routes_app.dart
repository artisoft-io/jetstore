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
const clientAdminPath = '/clientAdmin';
const sourceConfigPath = '/sourceConfig';
const inputSourceMappingPath = '/inputSourceMapping';
const processInputPath = '/processInput';
const domainTableViewerPath = '/domainTableViewer/:table_name/:session_id';
const queryToolPath = '/queryTool';
const filePreviewPath = '/filePreviewPath/:file_key';
const executionStatusDetailsPath = '/executionStatusDetails/:session_id';
const processErrorsPath = '/processErrors/:session_id';

// Old Rule Config with triples
const processConfigPath = '/processConfig';
// Rule Configv2
const ruleConfigPath = '/ruleConfig';

const pipelineConfigPath = '/pipelineConfig';
const pipelineConfigEditFormPath =
    '/pipelineConfig/edit/:key/:client/:process_name/:process_config_key/:main_process_input_key/:merged_process_input_keys/:main_object_type/:main_source_type/:source_period_type/:automated/:description/:max_rete_sessions_saved/:injected_process_input_keys/:rule_config_json';
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
const ufClientRegistryPath = '/clientRegistryUF';
const ufSourceConfigPath = '/sourceConfigUF';
const ufFileMappingPath = '/fileMappingUF';
const ufPipelineConfigPath = '/pipelineConfigUF';
const ufLoadFilesPath = '/loadFilesUF';
const ufStartPipelinePath = '/startPipelineUF';

// const wsDomainClassesPath = '/workspaces/:workspace_name/domainClasses';
// const wsDomainClasseDetailsPath =
//     '/workspaces/:workspace_name/domainClasses/:class_name';
// const wsDomainTablesPath = '/workspaces/:workspace_name/domainTables';
// const wsDomainTableDetailsPath =
//     '/workspaces/:workspace_name/domainTables/:table_name';
// const wsJetRulesPath = '/workspaces/:workspace_name/jetRules';
// const wsJetRuleDetailsPath = '/workspaces/:workspace_name/jetRules/:rule_name';

final Map<String, Widget> jetsRoutesMap = {
  // Home Screen
  homePath: ScreenWithForm(
    key: const Key(ScreenKeys.home),
    screenPath: const JetsRouteData(homePath),
    screenConfig: getScreenConfig(ScreenKeys.home),
    formConfig: getFormConfig(FormKeys.home),
  ),

  // Client & Organization Admin
  clientAdminPath: ScreenWithForm(
    key: const Key(ScreenKeys.clientAdmin),
    screenPath: const JetsRouteData(clientAdminPath),
    screenConfig: getScreenConfig(ScreenKeys.clientAdmin),
    formConfig: getFormConfig(FormKeys.clientAdmin),
  ),

  // Source Config
  sourceConfigPath: ScreenWithForm(
    key: const Key(ScreenKeys.sourceConfig),
    screenPath: const JetsRouteData(sourceConfigPath),
    screenConfig: getScreenConfig(ScreenKeys.sourceConfig),
    formConfig: getFormConfig(FormKeys.sourceConfig),
  ),

  // Input Source Mapping
  inputSourceMappingPath: ScreenWithForm(
    key: const Key(ScreenKeys.inputSourceMapping),
    screenPath: const JetsRouteData(inputSourceMappingPath),
    screenConfig: getScreenConfig(ScreenKeys.inputSourceMapping),
    formConfig: getFormConfig(FormKeys.inputSourceMapping),
  ),

  // Process Input
  processInputPath: ScreenWithForm(
    key: const Key(ScreenKeys.processInput),
    screenPath: const JetsRouteData(processInputPath),
    screenConfig: getScreenConfig(ScreenKeys.processInput),
    formConfig: getFormConfig(FormKeys.processInput),
  ),

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

  // Pipeline Config
  pipelineConfigPath: ScreenWithForm(
    key: const Key(ScreenKeys.pipelineConfig),
    screenPath: const JetsRouteData(pipelineConfigPath),
    screenConfig: getScreenConfig(ScreenKeys.pipelineConfig),
    formConfig: getFormConfig(FormKeys.pipelineConfigForm),
  ),

  // Edit Form for Pipeline Config
  pipelineConfigEditFormPath: ScreenWithForm(
    key: const Key(ScreenKeys.pipelineConfigEdit),
    screenPath: const JetsRouteData(pipelineConfigEditFormPath),
    screenConfig: getScreenConfig(ScreenKeys.pipelineConfigEdit),
    formConfig: getFormConfig(FormKeys.pipelineConfigEditForm),
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
