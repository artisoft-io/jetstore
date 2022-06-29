import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/routes/export_routes.dart';
import 'package:jetsclient/screens/components/form.dart';
import 'package:provider/provider.dart';
import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/models/user.dart';
import 'package:jetsclient/screens/components/app_bar.dart';
import 'package:jetsclient/utils/form_config.dart';

class RegistrationScreen extends StatefulWidget {
  const RegistrationScreen({super.key});

  final String formConfig = 'register';

  @override
  State<RegistrationScreen> createState() => _RegistrationScreenState();
}

class _RegistrationScreenState extends State<RegistrationScreen> {
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
      case 'name':
        if (value != null && value.characters.length > 1) {
          return null;
        }
        if (value != null && value.characters.length == 1) {
          return "Name is too short.";
        }
        return "Name must be provided.";
      case 'email':
        if (value != null && value.characters.length > 3) {
          return null;
        }
        return "Email must be provided.";
      case 'password':
        if (value != null && value.length >= 4) {
          var hasNum = value.contains(RegExp(r'[0-9]'));
          var hasUpper = value.contains(RegExp(r'[A-Z]'));
          var hasLower = value.contains(RegExp(r'[a-z]'));
          if (hasNum && hasUpper && hasLower) return null;
        }
        return "Password must have at least 4 charaters and contain at least one of: upper and lower case letter, and number.";
      case 'passwordConfirmation':
        if (formData['password'] != null &&
            formData['password']!.isNotEmpty &&
            formData['password'] == value) {
          return null;
        }
        return "Passwords does not match.";
      default:
        throw Exception(
            'ERROR: Invalid program configuration: No validator configured for form field $key');
    }
  }

  void _doRegister() async {
    var valid = formKey.currentState!.validate();
    if (!valid) {
      return;
    }
    // Use a JSON encoded string to send
    var client = context.read<HttpClient>();
    var user = UserModel();
    var result = await client.sendRequest(registerPath, json.encode(formData));
    if (!mounted) return;
    if (result.statusCode == 200 || result.statusCode == 201) {
      // update the [UserModel]
      user.name = formData['name'] as String?;
      user.email = formData['email'] as String?;
      user.token = result.body as String;
      JetsRouterDelegate().user = user;
      // Inform the user and transition
      const snackBar = SnackBar(
        content: Text('Registration Successful, you are now signed in'),
      );
      ScaffoldMessenger.of(context).showSnackBar(snackBar);
      JetsRouterDelegate()(JetsRouteData("/"));
    } else if (result.statusCode == 406 || result.statusCode == 422) {
      // http Not Acceptable / Unprocessable
      showAlertDialog(context, 'Invalid email or password.');
    } else if (result.statusCode == 409) {
      // http Conflict
      showAlertDialog(context, 'User already exist please signed in.');
    } else {
      showAlertDialog(context, 'Something went wrong. Please try again.');
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: appBar(context, 'Registration'),
      body: JetsForm(
          formData: formData,
          formKey: formKey,
          formConfig: formConfig,
          validatorDelegate: validatorDelegate,
          actions: <String, VoidCallback>{'register': _doRegister}),
    );
  }
}
