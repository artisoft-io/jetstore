import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/routes/export_routes.dart';
import 'package:jetsclient/screens/components/form.dart';
import 'package:jetsclient/utils/form_config.dart';
import 'package:provider/provider.dart';
import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/models/user.dart';
import 'package:jetsclient/screens/components/app_bar.dart';

class LoginScreen extends StatefulWidget {
  const LoginScreen({super.key});

  final String formConfig = 'login';

  @override
  State<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LoginScreen> {
  final formData = <String, dynamic>{};
  final formKey = GlobalKey<FormState>();
  late final FormConfig formConfig;

  @override
  void initState() {
    super.initState();
    formConfig = getFormConfig(widget.formConfig);
  }

  String? validatorDelegate(String key, String? value) {
    switch (key) {
      case 'email':
        if (value != null && value.characters.length > 3) {
          return null;
        }
        return "Email must be provided.";
      case 'password':
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
    var user = UserModel();
    var result = await client.sendRequest(
      loginPath,
      json.encode(formData));

    if (!mounted) return;
    if (result.statusCode == 200) {
      // update the [UserModel]
      user.name = "";
      user.email = formData['email'] as String?;
      user.token = result.body as String;
      JetsRouterDelegate().user = user;
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
          formData: formData,
          formKey: formKey,
          formConfig: formConfig,
          validatorDelegate: validatorDelegate,
          actions: <String, VoidCallback>{'login': _doLogin, 'register': _doRegister}),
    );
  }
}
