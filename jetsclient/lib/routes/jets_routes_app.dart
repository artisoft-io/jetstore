import 'package:flutter/material.dart';
import 'package:adaptive_theme/adaptive_theme.dart';
import 'package:jetsclient/routes/export_routes.dart';
import 'package:jetsclient/screens/components/screen_one.dart';
import 'package:jetsclient/screens/login.dart';
import 'package:jetsclient/screens/registration.dart';
import 'package:jetsclient/models/user.dart';
import 'package:jetsclient/utils/screen_config.dart';

const PARAM_CHAR = ':';

const homePath = '/';
const jobListPath = '/dataPipelines';
const fileRegistryPath = '/fileRegistry';
const fileRegistryTablePath = '/fileRegistry/table/:table';
const loginPath = '/login';
const registerPath = '/register';
const jobDetailsPath = '/job/:id';
const pageNotFoundPath = '/404';

final Map<String, Widget> jetsRoutesMap = {
  homePath: ScreenOne(
      key: const Key('homeScreen'),
      tablePath: JetsRouteData(homePath),
      screenConfig: getScreenConfig('homeScreen')),
  jobListPath: ScreenOne(
      key: const Key('jobListScreen'),
      tablePath: JetsRouteData(jobListPath),
      screenConfig: getScreenConfig('jobListScreen')),
  fileRegistryPath: ScreenOne(
      key: const Key('fileRegistryScreen'),
      tablePath: JetsRouteData(fileRegistryPath),
      screenConfig: getScreenConfig('fileRegistryScreen')),
  fileRegistryTablePath: ScreenOne(
      key: const Key('fileRegistryTableScreen'),
      tablePath: JetsRouteData(fileRegistryTablePath),
      screenConfig: getScreenConfig('fileRegistryTableScreen')),
  loginPath: const LoginScreen(),
  registerPath: const RegistrationScreen(),
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
