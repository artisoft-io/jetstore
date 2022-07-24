import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import 'package:jetsclient/utils/form_config.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/routes/export_routes.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/screens/components/form.dart';
import 'package:jetsclient/screens/components/app_bar.dart';
import 'package:jetsclient/screens/components/base_screen.dart';

class RegistrationScreen extends BaseScreen {
  RegistrationScreen({
    required super.key,
    required super.screenPath,
    required super.screenConfig,
    required this.formConfig,
  }) : super(builder: (State<BaseScreen> baseState) {
          final state = baseState as _RegistrationScreenState;
          return JetsForm(
              formPath: screenPath,
              formState: state.formState,
              formKey: state.formKey,
              formConfig: formConfig,
              validatorDelegate: state.validatorDelegate,
              actions: <String, VoidCallback>{
                ActionKeys.register: state._doRegister
              });
        });

  final FormConfig formConfig;

  @override
  State<BaseScreen> createState() => _RegistrationScreenState();
}

class _RegistrationScreenState extends BaseScreenState {
  late final JetsFormState formState;
  final formKey = GlobalKey<FormState>();

  @override
  void initState() {
    super.initState();
    final w = widget as RegistrationScreen;
    formState = w.formConfig.makeFormState();
  }

  String? validatorDelegate(int group, String key, dynamic v) {
    // This form does not use data table, therefore v is String?
    assert(v is String?, "Registration Form has unexpected data type");
    String? value = v;
    switch (key) {
      case FSK.userName:
        if (value != null && value.characters.length > 1) {
          return null;
        }
        if (value != null && value.characters.length == 1) {
          return "Name is too short.";
        }
        return "Name must be provided.";
      case FSK.userEmail:
        if (value != null && value.characters.length > 3) {
          return null;
        }
        return "Email must be provided.";
      case FSK.userPassword:
        if (value != null && value.length >= 4) {
          var hasNum = value.contains(RegExp(r'[0-9]'));
          var hasUpper = value.contains(RegExp(r'[A-Z]'));
          var hasLower = value.contains(RegExp(r'[a-z]'));
          if (hasNum && hasUpper && hasLower) return null;
        }
        return "Password must have at least 4 charaters and contain at least one of: upper and lower case letter, and number.";
      case FSK.userPasswordConfirm:
        // Expecting [WidgetField]
        WidgetField? formValue = formState.getValue(group, FSK.userPassword);
        if (formValue != null &&
            formValue.length == 1 &&
            formValue[0] == value) {
          return null;
        }
        return "Passwords does not match.";
      //* REMOVE THIS DEMO CODE
      case 'emailType':
        if (value == null) {
          return "Please select an email type";
        }
        return null;
      default:
        throw Exception(
            'ERROR: Invalid Registration Form configuration: No validator configured for form field $key');
    }
  }

  void _doRegister() async {
    var valid = formKey.currentState!.validate();
    if (!valid) {
      return;
    }
    // Use a JSON encoded string to send
    var client = context.read<HttpClient>();
    var result = await client.sendRequest(
        path: registerPath, encodedJsonBody: formState.encodeState(0));
    if (!mounted) return;
    if (result.statusCode == 200 || result.statusCode == 201) {
      // update the [UserModel]
      JetsRouterDelegate().user.name = result.body[FSK.userName];
      JetsRouterDelegate().user.email = result.body[FSK.userEmail];
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
}
