import 'package:flutter/material.dart';

import 'package:jetsclient/routes/export_routes.dart';
import 'package:jetsclient/utils/form_config.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/screens/components/form.dart';
import 'package:provider/provider.dart';
import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/screens/components/app_bar.dart';

class LoginScreen extends StatefulWidget {
  const LoginScreen({
    super.key,
    required this.screenPath,
  });

  final String formConfig = FormKeys.login;
  final JetsRouteData screenPath;

  @override
  State<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LoginScreen> {
  late final JetsFormState formData;
  final formKey = GlobalKey<FormState>();
  late final FormConfig formConfig;

  @override
  void initState() {
    super.initState();
    formConfig = getFormConfig(widget.formConfig);
    formData = formConfig.makeFormState();
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
    var result = await client.sendRequest(
        path: loginPath, encodedJsonBody: formData.encodeState(0));

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

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: appBar(context, 'Please Sign In'),
      body: JetsForm(
          formPath: widget.screenPath,
          formData: formData,
          formKey: formKey,
          formConfig: formConfig,
          validatorDelegate: validatorDelegate,
          actions: <String, VoidCallback>{
            ActionKeys.login: _doLogin,
            ActionKeys.register: _doRegister
          }),
    );
  }
}
