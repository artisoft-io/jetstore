import 'package:flutter/material.dart';

import 'package:jetsclient/routes/export_routes.dart';
import 'package:jetsclient/utils/form_config.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/screens/components/form.dart';
import 'package:provider/provider.dart';
import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/screens/components/app_bar.dart';
import 'package:jetsclient/screens/components/base_screen.dart';

class LoginScreen extends BaseScreen {
  LoginScreen({
    required super.key,
    required super.screenPath,
    required super.screenConfig,
    required this.formConfig,
  }) : super(builder: (State<BaseScreen> baseState) {
          final state = baseState as _LoginScreenState;
          return JetsForm(
              formPath: screenPath,
              formState: state.formState,
              formKey: state.formKey,
              formConfig: formConfig,
              validatorDelegate: state.validatorDelegate,
              actions: <String, VoidCallback>{
                ActionKeys.login: state._doLogin,
                ActionKeys.register: state._doRegister
              });
        });

  final FormConfig formConfig;

  @override
  State<BaseScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends BaseScreenState {
  late final JetsFormState formState;
  final formKey = GlobalKey<FormState>();

  @override
  void initState() {
    super.initState();
    final w = widget as LoginScreen;
    formState = w.formConfig.makeFormState();
  }

  String? validatorDelegate(int group, String key, dynamic v) {
    String? value = v;
    switch (key) {
      case FSK.userEmail:
        if (value != null && value.characters.length > 3) {
          return null;
        }
        return "Email must be provided.";
      case FSK.userPassword:
        if (value != null && value.length >= 4) {
          return null;
        }
        return "Password must be provided.";
      default:
        throw Exception(
            'ERROR: Invalid program configuration: No validator configured for form field $key');
    }
  }

  void _doLogin() async {
    // Use a JSON encoded string to send
    var client = context.read<HttpClient>();
    // Note: using the same keys for FormState (class FSK)
    // as the message structure (User class of api server)
    // May not be ideal and might need to have separate
    // keys.
    var result = await client.sendRequest(
        path: loginPath, encodedJsonBody: formState.encodeState(0));

    if (!mounted) return;
    if (result.statusCode == 200) {
      // update the [UserModel]
      JetsRouterDelegate().user.name = result.body[FSK.userName];
      JetsRouterDelegate().user.email = result.body[FSK.userEmail];
      // Inform the user and transition
      const snackBar = SnackBar(
        content: Text('Login Successful!'),
      );
      ScaffoldMessenger.of(context).showSnackBar(snackBar);
      JetsRouterDelegate()(JetsRouteData(homePath));
    } else if (result.statusCode == 401 || result.statusCode == 422) {
      showAlertDialog(context, 'Invalid email and/or password.');
    } else {
      showAlertDialog(context, 'Something went wrong. Please try again.');
    }
  }

  void _doRegister() async {
    JetsRouterDelegate()(JetsRouteData(registerPath));
  }
}
