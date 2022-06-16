import 'dart:convert';

import 'package:adaptive_theme/adaptive_theme.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/routes/jets_route_data.dart';
import 'package:jetsclient/models/user.dart';

class RegistrationScreen extends StatefulWidget {
  const RegistrationScreen({super.key});

  @override
  State<RegistrationScreen> createState() => _RegistrationScreenState();
}

class _RegistrationScreenState extends State<RegistrationScreen> {
  var formData = UserModel();
  final _formKey = GlobalKey<FormState>();
  void _doRegister() async {
    var valid = _formKey.currentState!.validate();
    if (!valid) {
      return;
    }
    // Use a JSON encoded string to send
    try {
      var client = context.read<HttpClient>();
      var user = UserModel();
      var result = await client.httpClient.post(
          client.serverAdd.replace(path: '/register'),
          body: json.encode(formData.toJson()),
          headers: {'Content-Type': 'application/json'});

      if (result.statusCode == 200 || result.statusCode == 201) {
        // update the [UserModel]
        user.name = formData.name;
        user.email = formData.email;
        user.token = jsonDecode(result.body) as String;
        JetsRouterDelegate().user = user;

        // Inform the user and transition
        const snackBar = SnackBar(
          content: Text('Registration Successful, you are now signed in'),
        );
        if (!mounted) return;
        ScaffoldMessenger.of(context).showSnackBar(snackBar);
        JetsRouterDelegate()(JetsRouteData("/"));
      } else if (result.statusCode == 406 || result.statusCode == 422) {
        // http Not Acceptable / Unprocessable
        _showDialog('Invalid email or password.');
      } else if (result.statusCode == 409) {
        // http Conflict
        _showDialog('User already exist please signed in.');
      } else {
        _showDialog('Something went wrong. Please try again.');
      }
    } on Exception catch (e) {
      _showDialog('OOps error: $e');
    } catch (e) {
      _showDialog('OOps Unknown exception $e of type ${e.runtimeType}');
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        automaticallyImplyLeading: false,
        title: const Text('Registration'),
        actions: <Widget>[
          IconButton(
            icon: const Icon(Icons.dark_mode_sharp),
            tooltip: 'Toggle Theme',
            onPressed: () {
              AdaptiveTheme.of(context).toggleThemeMode();
            },
          ),
          TextButton(
            onPressed: () {
              AdaptiveTheme.of(context).toggleThemeMode();
            },
            child: const Text(''),
          )
        ],
      ),
      body: Form(
        key: _formKey,
        child: Scrollbar(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(16),
            child: Column(
              children: [
                ...[
                  TextFormField(
                    autofocus: true,
                    textInputAction: TextInputAction.next,
                    decoration: const InputDecoration(
                      hintText: 'Your name',
                    ),
                    onChanged: (value) {
                      formData.name = value;
                    },
                    validator: (value) {
                      if (value != null && value.runes.length > 1) {
                        return null;
                      }
                      if (value != null && value.runes.length == 1) {
                        return "Name is too short.";
                      }
                      return "Name must be provided.";
                    },
                  ),
                  TextFormField(
                    autofocus: true,
                    textInputAction: TextInputAction.next,
                    decoration: const InputDecoration(
                      hintText: 'Your email address',
                    ),
                    onChanged: (value) {
                      formData.email = value;
                    },
                    validator: (value) {
                      if (value != null && value.runes.length > 3) {
                        return null;
                      }
                      return "Email must be provided.";
                    },
                  ),
                  TextFormField(
                    decoration: const InputDecoration(
                      hintText: 'Enter your password',
                    ),
                    obscureText: true,
                    onChanged: (value) {
                      formData.password = value;
                    },
                    validator: (value) {
                      if (value != null && value.length >= 4) {
                        var hasNum = value.contains(RegExp(r'[0-9]'));
                        var hasUpper = value.contains(RegExp(r'[A-Z]'));
                        var hasLower = value.contains(RegExp(r'[a-z]'));
                        if (hasNum && hasUpper && hasLower) return null;
                      }
                      return "Password must have at least 4 charaters and contain at least one of: upper and lower case letter, and number.";
                    },
                  ),
                  TextFormField(
                    decoration: const InputDecoration(
                      hintText: 'Re-Enter your password',
                    ),
                    obscureText: true,
                    onChanged: (value) {
                      formData.password = value;
                    },
                    validator: (value) {
                      if (formData.password != null &&
                          formData.password!.isNotEmpty &&
                          formData.password == value) {
                        return null;
                      }
                      return "Passwords does not match.";
                    },
                  ),
                  TextButton(
                    onPressed: _doRegister,
                    child: const Text('Register'),
                  ),
                ].expand(
                  (widget) => [
                    widget,
                    const SizedBox(
                      height: 24,
                    )
                  ],
                )
              ],
            ),
          ),
        ),
      ),
    );
  }

  void _showDialog(String message) {
    showDialog<void>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(message),
        actions: [
          TextButton(
            child: const Text('OK'),
            onPressed: () => Navigator.of(context).pop(),
          ),
        ],
      ),
    );
  }
}
