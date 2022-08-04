import 'package:flutter/material.dart';
import 'package:jetsclient/routes/jets_route_data.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/routes/jets_routes_app.dart';
import 'package:jetsclient/screens/components/dialogs.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/http_client.dart';
import 'package:provider/provider.dart';

/// Validation and Actions delegates for the user-related forms
/// Login Form Validator
String? loginFormValidator(BuildContext context, JetsFormState formState,
    int group, String key, dynamic v) {
  // This form does not use data table, therefore v is String?
  assert(v is String?, "Login Form has unexpected data type");
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
      showAlertDialog(context,
          'Oops login form has no validator configured for form field $key');
  }
  return null;
}

/// Login Form Actions
void loginFormActions(BuildContext context, GlobalKey<FormState> formKey,
    JetsFormState formState, String actionKey) async {
  switch (actionKey) {
    case ActionKeys.login:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return;
      }
      // Use a JSON encoded string to send
      var client = context.read<HttpClient>();
      // Note: using the same keys for FormState (class FSK)
      // as the message structure (User class of api server)
      // May not be ideal and might need to have separate
      // keys.
      var result = await client.sendRequest(
          path: loginPath, encodedJsonBody: formState.encodeState(0));

      // if (!mounted) return; //* don't think we need this since we don't call setState() here
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
      break;
    case ActionKeys.register:
      JetsRouterDelegate()(JetsRouteData(registerPath));
      break;
    default:
      showAlertDialog(
          context, 'Oops unknown ActionKey for login form: $actionKey');
  }
}

/// Home Form Actions
void homeFormActions(BuildContext context, GlobalKey<FormState> formKey,
    JetsFormState formState, String actionKey) async {
  print("HOME FORM ACTION DELEGATE CALLED");
  switch (actionKey) {
    case "test.action":
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return;
      }
      print("Returning ${DTActionResult.okDataTableDirty}");
      Navigator.of(context).pop(DTActionResult.okDataTableDirty);
      break;
    case "test.cancel":
      print("OK got it!");
      Navigator.of(context).pop();
      break;
    default:
      showAlertDialog(
          context, 'Oops unknown ActionKey for login form: $actionKey');
  }
}

// Registration Form Validator
String? registrationFormValidator(BuildContext context, JetsFormState formState,
    int group, String key, dynamic v) {
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
      String? formValue = formState.getValue(group, FSK.userPassword);
      if (formValue != null && formValue == value) {
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
      showAlertDialog(context,
          'Oops registration form has no validator configured for form field $key');
  }
  return null;
}

/// Registration Form Actions
void registrationFormActions(BuildContext context, GlobalKey<FormState> formKey,
    JetsFormState formState, String actionKey) async {
  var valid = formKey.currentState!.validate();
  if (!valid) {
    return;
  }
  switch (actionKey) {
    case ActionKeys.register:
      // Use a JSON encoded string to send
      var client = context.read<HttpClient>();
      var result = await client.sendRequest(
          path: registerPath, encodedJsonBody: formState.encodeState(0));
      // if (!mounted) return; needed?
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
      break;
    default:
      showAlertDialog(
          context, 'Oops unknown ActionKey for login form: $actionKey');
  }
}
