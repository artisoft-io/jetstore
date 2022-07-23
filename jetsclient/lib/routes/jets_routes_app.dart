import 'package:flutter/material.dart';
import 'package:adaptive_theme/adaptive_theme.dart';
import 'package:jetsclient/routes/export_routes.dart';
import 'package:jetsclient/screens/components/screen_one.dart';
import 'package:jetsclient/screens/login.dart';
import 'package:jetsclient/screens/registration.dart';
import 'package:jetsclient/screens/test_screen.dart';
import 'package:jetsclient/models/user.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/utils/data_table_config.dart';
import 'package:jetsclient/utils/form_config.dart';
import 'package:jetsclient/utils/screen_config.dart';

const PARAM_CHAR = ':';

const homePath = '/';
const pipelinePath = '/dataPipelines';
const mappingConfigPath = '/mappingConfig';
const fileRegistryPath = '/fileRegistry';
const fileRegistryTablePath = '/fileRegistry/table/:table';
const loginPath = '/login';
const registerPath = '/register';
const jobDetailsPath = '/job/:id';
const pageNotFoundPath = '/404';

final Map<String, Widget> jetsRoutesMap = {
  // Home Screen
  homePath: ScreenOne(
      key: const Key(ScreenKeys.home),
      screenPath: JetsRouteData(homePath),
      screenConfig: getScreenConfig(ScreenKeys.home),
      //* DEMO
      tableConfig: getTableConfig(DTKeys.usersTable)),
  // Pipeline Screen
  pipelinePath: ScreenOne(
      key: const Key(ScreenKeys.pipelines),
      screenPath: JetsRouteData(pipelinePath),
      screenConfig: getScreenConfig(ScreenKeys.pipelines),
      tableConfig: getTableConfig(DTKeys.pipelineDemo)),
  //* TEST SCREEN
  mappingConfigPath: TestScreen(
      key: const Key('testScreen'),
      screenPath: JetsRouteData(mappingConfigPath),
      screenConfig: getScreenConfig("testScreen"),
      formConfig: getFormConfig("dataTableDemoForm")),
  // File Registry Screen
  fileRegistryPath: ScreenOne(
      key: const Key(ScreenKeys.fileRegistry),
      screenPath: JetsRouteData(fileRegistryPath),
      screenConfig: getScreenConfig(ScreenKeys.fileRegistry),
      tableConfig: getTableConfig(DTKeys.registryDemo)),
  // File Registry Table Screen
  fileRegistryTablePath: ScreenOne(
      key: const Key(ScreenKeys.fileRegistryTable),
      screenPath: JetsRouteData(fileRegistryTablePath),
      screenConfig: getScreenConfig(ScreenKeys.fileRegistryTable),
      tableConfig: getTableConfig(DTKeys.inputTable)),
  // Login Screen
  loginPath: LoginScreen(
      key: const Key(ScreenKeys.login),
      screenPath: JetsRouteData(loginPath),
      screenConfig: getScreenConfig(ScreenKeys.login),
      formConfig: getFormConfig(FormKeys.login)),
  // Register Screen
  registerPath: RegistrationScreen(
      key: const Key(ScreenKeys.register),
      screenPath: JetsRouteData(registerPath),
      screenConfig: getScreenConfig(ScreenKeys.register),
      formConfig: getFormConfig(FormKeys.register)),
  jobDetailsPath: const MessageScreen(message: "Detailed Welcome!"),
  pageNotFoundPath: const MessageScreen(message: "Opps 404!")
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
