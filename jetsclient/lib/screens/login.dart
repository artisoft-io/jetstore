import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/routes/export_routes.dart';
import 'package:provider/provider.dart';
import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/models/user.dart';
import 'package:jetsclient/screens/components/app_bar.dart';

class LoginScreen extends StatefulWidget {
  const LoginScreen({super.key});

  @override
  State<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LoginScreen> {
  UserModel formData = UserModel();
  void _doLogin() async {    
    // Use a JSON encoded string to send
    var client = context.read<HttpClient>();
    var user = UserModel();
    var result = await client.sendRequest(
      loginPath,
      json.encode(formData.toJson()));
    if (result.statusCode == 200) {
      // update the [UserModel]
      user.name = "";
      user.email = formData.email;
      user.token = result.body as String;
      JetsRouterDelegate().user = user;
      // Inform the user and transition
      const snackBar = SnackBar(
        content: Text('Login Successful!'),
      );
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(snackBar);
      JetsRouterDelegate()(JetsRouteData(homePath));
    } else if (result.statusCode == 401 || result.statusCode == 422) {
      _showDialog('Invalid email and/or password.');
    } else {
      _showDialog('Something went wrong. Please try again.');
    }
  }

  void _doRegister() async {
    JetsRouterDelegate()(JetsRouteData(registerPath));
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: appBar(context, 'Please Sign In'),
      body: Form(
        child: Scrollbar(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(16),
            child: Column(
              children: [
                ...[
                  TextFormField(
                    autofocus: true,
                    // textInputAction: TextInputAction.next,
                    decoration: const InputDecoration(
                      filled: true,
                      hintText: 'Your email address',
                      labelText: 'Email',
                    ),
                    onChanged: (value) {
                      formData.email = value;
                    },
                  ),
                  TextFormField(
                    autofocus: false,
                    // textInputAction: TextInputAction.next,
                    decoration: const InputDecoration(
                      filled: true,
                      labelText: 'Password',
                    ),
                    obscureText: true,
                    onChanged: (value) {
                      formData.password = value;
                    },
                  ),
                  Center(
                    child: Row(
                      children: <Widget>[
                        TextButton(
                            onPressed: _doLogin, 
                            child: const Text('Sign in')),
                        const SizedBox(
                          height: 24,
                        ),
                        TextButton(
                            onPressed: _doRegister,
                            child: const Text('Register')),
                      ],
                    ),
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
