import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/routes/jets_route_data.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/routes/jets_routes_app.dart';
import 'package:jetsclient/screens/components/dialogs.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/screens/components/spinner_overlay.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/utils/form_config.dart';

/// Validation and Actions delegates for the user-related forms
/// Login Form Validator
String? loginFormValidator(
    JetsFormState formState, int group, String key, dynamic v) {
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
      print('Oops login form has no validator configured for form field $key');
  }
  return null;
}

/// Login Form Actions
Future<String?> loginFormActions(BuildContext context,
    GlobalKey<FormState> formKey, JetsFormState formState, String actionKey,
    {int group = 0}) async {
  switch (actionKey) {
    case ActionKeys.login:
      var valid = formKey.currentState!.validate();
      if (!valid) {
        return null;
      }
      // Use a JSON encoded string to send
      var client = HttpClientSingleton();
      var result = await client.sendRequest(
          path: ServerEPs.loginEP, encodedJsonBody: formState.encodeState(0));

      if (result.statusCode == 200) {
        // update the [UserModel]
        JetsRouterDelegate().user.name = result.body[FSK.userName];
        JetsRouterDelegate().user.email = result.body[FSK.userEmail];
        JetsRouterDelegate().user.isAdmin = result.body[FSK.isAdmin];
        final gitProfile = result.body['gitProfile'];
        if(gitProfile != null) {
          JetsRouterDelegate().user.gitName = gitProfile[FSK.gitName];
          JetsRouterDelegate().user.gitHandle = gitProfile[FSK.gitHandle];
          JetsRouterDelegate().user.gitEmail = gitProfile[FSK.gitEmail];
        }
        final devMode = result.body[FSK.devMode];
        JetsRouterDelegate().devMode = false;
        if (devMode != null) {
          JetsRouterDelegate().devMode = devMode == "true";
        }
        // Get list of clients
        var msg = <String, dynamic>{
          'action': 'raw_query',
          'query':
              'SELECT client FROM jetsapi.client_registry ORDER BY client ASC LIMIT 200'
        };
        var encodedMsg = json.encode(msg);
        result = await HttpClientSingleton().sendRequest(
            path: "/dataTable",
            token: JetsRouterDelegate().user.token,
            encodedJsonBody: encodedMsg);
        if (result.statusCode == 401) return "Not Authorized";
        if (result.statusCode == 200) {
          JetsRouterDelegate().clients = (result.body['rows'] as List)
              .map((e) => DropdownItemConfig(label: e[0]!, value: e[0]))
              .toList();
        }

        // Get the workspace_uri from the server WORKSPACE_URI env variable
        msg = <String, dynamic>{
          'action': 'get_workspace_uri',
        };
        encodedMsg = json.encode(msg);
        result = await HttpClientSingleton().sendRequest(
            path: "/dataTable",
            token: JetsRouterDelegate().user.token,
            encodedJsonBody: encodedMsg);
        if (result.statusCode == 401) return "Not Authorized";
        if (result.statusCode == 200) {
          globalWorkspaceUri = result.body['workspace_uri'] as String;
        }

        JetsRouterDelegate()(JetsRouteData(
            JetsRouterDelegate().user.isAdmin ? userAdminPath : homePath));
      } else if (result.statusCode == 401) {
        if (context.mounted) {
          showAlertDialog(context, 'Invalid email and/or password.');
        }
      } else if (result.statusCode == 422) {
        if (context.mounted) {
          showAlertDialog(context, result.body[FSK.error]);
        }
      } else {
        if (context.mounted) {
          showAlertDialog(context, 'Something went wrong. Please try again.');
        }
      }
      break;
    case ActionKeys.register:
      JetsRouterDelegate()(JetsRouteData(registerPath));
      break;
    default:
      showAlertDialog(
          context, 'Oops unknown ActionKey for login form: $actionKey');
  }
  return null;
}

// Registration Form Validator
String? registrationFormValidator(
    JetsFormState formState, int group, String key, dynamic v) {
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
      if (value != null && value.length >= 14) {
        var hasNum = value.contains(RegExp(r'[0-9]'));
        var hasUpper = value.contains(RegExp(r'[A-Z]'));
        var hasLower = value.contains(RegExp(r'[a-z]'));
        var hasSpecial = value.contains(RegExp(r'[!@#$%^&*()_+\-=\[\]{}|' ']'));
        if (hasNum && hasUpper && hasLower && hasSpecial) return null;
      }
      return "At least 14 charaters, one of: upper, lower char, number, and special char.";
    case FSK.userPasswordConfirm:
      // Expecting [WidgetField]
      String? formValue = formState.getValue(group, FSK.userPassword);
      if (formValue != null && formValue == value) {
        return null;
      }
      return "Passwords does not match.";
    default:
      print(
          'Oops registration form has no validator configured for form field $key');
  }
  return null;
}

/// Registration Form Actions
Future<String?> registrationFormActions(BuildContext context,
    GlobalKey<FormState> formKey, JetsFormState formState, String actionKey,
    {int group = 0}) async {
  var valid = formKey.currentState!.validate();
  if (!valid) {
    return null;
  }
  switch (actionKey) {
    case ActionKeys.register:
      // Use a JSON encoded string to send
      var messenger = ScaffoldMessenger.of(context);
      var result = await HttpClientSingleton().sendRequest(
          path: ServerEPs.registerEP,
          encodedJsonBody: formState.encodeState(0));
      if (result.statusCode == 401) return "Not Authorized";
      if (result.statusCode == 200 || result.statusCode == 201) {
        // update the [UserModel]
        JetsRouterDelegate().user.name = result.body[FSK.userName];
        JetsRouterDelegate().user.email = result.body[FSK.userEmail];
        // Inform the user and transition
        const snackBar = SnackBar(
          content: Text('Registration Successful'),
        );
        if (context.mounted) {
          messenger.showSnackBar(snackBar);
        }
        JetsRouterDelegate()(JetsRouteData(loginPath));
      } else if (result.statusCode == 406 || result.statusCode == 422) {
        // http Not Acceptable / Unprocessable
        if (context.mounted) {
          showAlertDialog(context, 'Invalid email or password.');
        }
      } else if (result.statusCode == 409) {
        // http Conflict
        if (context.mounted) {
          showAlertDialog(context, 'User already exist please signed in.');
        }
      } else {
        if (context.mounted) {
          showAlertDialog(context, 'Something went wrong. Please try again.');
        }
      }
      break;
    default:
      showAlertDialog(
          context, 'Oops unknown ActionKey for login form: $actionKey');
  }
  return null;
}

// Git Profile Form Validator
String? gitProfileFormValidator(
    JetsFormState formState, int group, String key, dynamic v) {
  // This form does not use data table, therefore v is String?
  assert(v is String?, "Git Profile Form has unexpected data type");
  String? value = v;
  switch (key) {
    case FSK.gitName:
      if (value != null && value.characters.length > 1) {
        return null;
      }
      if (value != null && value.characters.length == 1) {
        return "Name is too short.";
      }
      return "Name must be provided.";
    case FSK.gitEmail:
      if (value != null && value.characters.length > 3) {
        return null;
      }
      return "Email must be provided.";
    case FSK.gitHandle:
      if (value != null && value.characters.length > 3) {
        return null;
      }
      return "Git handle (user name) must be provided.";
    case FSK.gitToken:
      if (value != null && value.length > 5) {
        return null;
      }
      return "Git token must be provided";
    case FSK.gitTokenConfirm:
      String? formValue = formState.getValue(group, FSK.gitToken);
      if (formValue != null && formValue == value) {
        return null;
      }
      return "Git tokens does not match.";
    default:
      print(
          'Oops Git Profile form has no validator configured for form field $key');
  }
  return null;
}

/// Git Profile Form Actions
Future<String?> gitProfileFormActions(BuildContext context,
    GlobalKey<FormState> formKey, JetsFormState formState, String actionKey,
    {int group = 0}) async {
  var valid = formKey.currentState!.validate();
  if (!valid) {
    return null;
  }
  switch (actionKey) {
    case ActionKeys.submitGitProfileOk:
      // Use a JSON encoded string to send
      var messenger = ScaffoldMessenger.of(context);

      final state = formState.getState(0);
      state['user_email'] = JetsRouterDelegate().user.email;
      // print('Update User Git Profile state: $state');
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'insert_rows',
        'fromClauses': [
          <String, String>{'table': 'update/user_git_profile'}
        ],
        'data': [state],
      }, toEncodable: (_) => '');

      if (context.mounted) {
        JetsSpinnerOverlay.of(context).show();

        var result = await HttpClientSingleton().sendRequest(
            path: ServerEPs.dataTableEP,
            token: JetsRouterDelegate().user.token,
            encodedJsonBody: encodedJsonBody);
        if (result.statusCode == 401) return null;
        if (result.statusCode == 200) {
          // update the [UserModel]
          JetsRouterDelegate().user.gitName =
              formState.getValue(group, FSK.gitName);
          JetsRouterDelegate().user.gitEmail =
              formState.getValue(group, FSK.gitEmail);
          JetsRouterDelegate().user.gitHandle =
              formState.getValue(group, FSK.gitHandle);
          // Inform the user and transition
          const snackBar = SnackBar(
            content: Text('Git Profile Updated Successful'),
          );
          if (context.mounted) {
            messenger.showSnackBar(snackBar);
          }
          JetsRouterDelegate()(JetsRouteData(homePath));
        } else {
          if (context.mounted) {
            showAlertDialog(context, 'Something went wrong. Please try again.');
          }
        }

        if (context.mounted) {
          JetsSpinnerOverlay.of(context).hide();
        }
      }
      break;
    default:
      showAlertDialog(
          context, 'Oops unknown ActionKey for git profile form: $actionKey');
  }
  return null;
}

/// User Administration Form Actions
Future<String?> userAdminFormActions(BuildContext context,
    GlobalKey<FormState> formKey, JetsFormState formState, String actionKey,
    {int group = 0}) async {
  var messenger = ScaffoldMessenger.of(context);
  switch (actionKey) {
    case ActionKeys.toggleUserActive:
      // Use a JSON encoded string to send
      var data = [];
      var emails = formState.getValue(0, DTKeys.usersTable) as List<dynamic>;
      var areActive = formState.getValue(0, FSK.isActive) as List<dynamic>;
      var isActive = '1';
      if (areActive[0] == '1') {
        isActive = '0';
      }
      for (int i = 0; i < emails.length; i++) {
        data.add(<String, dynamic>{
          FSK.userEmail: emails[i],
          FSK.isActive: isActive,
        });
      }
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'insert_rows',
        'fromClauses': [
          <String, String>{'table': 'update/users'}
        ],
        'data': data,
      }, toEncodable: (_) => '');
      var result = await HttpClientSingleton().sendRequest(
          path: ServerEPs.dataTableEP,
          token: JetsRouterDelegate().user.token,
          encodedJsonBody: encodedJsonBody);
      // handling server reply
      if (result.statusCode == 401) return "Not Authorized";
      if (result.statusCode == 200) {
        // Inform the user and transition
        const snackBar = SnackBar(
          content: Text('Update Successful'),
        );
        if (context.mounted) {
          messenger.showSnackBar(snackBar);
        }
        formState.invokeCallbacks();
      } else {
        if (context.mounted) {
          showAlertDialog(context, 'Something went wrong. Please try again.');
        }
      }
      break;
    case ActionKeys.deleteUser:
      // Get confirmation to delete user
      var uc = await showDangerZoneDialog(
          context, 'Are you sure you want to delete the selected user(s)?');
      if (uc != 'OK') return null;
      // Use a JSON encoded string to send
      var data = [];
      var emails = formState.getValue(0, DTKeys.usersTable) as List<dynamic>;
      for (int i = 0; i < emails.length; i++) {
        data.add(<String, dynamic>{
          FSK.userEmail: emails[i],
        });
      }
      var encodedJsonBody = jsonEncode(<String, dynamic>{
        'action': 'insert_rows',
        'fromClauses': [
          <String, String>{'table': 'delete/users'}
        ],
        'data': data,
      }, toEncodable: (_) => '');
      var result = await HttpClientSingleton().sendRequest(
          path: ServerEPs.dataTableEP,
          token: JetsRouterDelegate().user.token,
          encodedJsonBody: encodedJsonBody);
      // handling server reply
      if (result.statusCode == 401) return "Not Authorized";
      if (result.statusCode == 200) {
        // Inform the user and transition
        const snackBar = SnackBar(
          content: Text('Delete User(s) Successful'),
        );
        if (context.mounted) {
          messenger.showSnackBar(snackBar);
        }
        formState.invokeCallbacks();
      } else {
        if (context.mounted) {
          showAlertDialog(context, 'Something went wrong. Please try again.');
        }
      }
      break;
    default:
      showAlertDialog(
          context, 'Oops unknown ActionKey for userAdmin form: $actionKey');
  }
  return null;
}
