import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:http/http.dart' as http;
import 'package:provider/provider.dart';
import 'package:jetsclient/models/user.dart';

class LoginScreen extends StatefulWidget {
  const LoginScreen({super.key});

  @override
  State<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LoginScreen> {
  UserModel formData = UserModel();

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Sign in Form'),
      ),
      body: Form(
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
                      filled: true,
                      hintText: 'Your email address',
                      labelText: 'Email',
                    ),
                    onChanged: (value) {
                      formData.email = value;
                    },
                  ),
                  TextFormField(
                    decoration: const InputDecoration(
                      filled: true,
                      labelText: 'Password',
                    ),
                    obscureText: true,
                    onChanged: (value) {
                      formData.password = value;
                    },
                  ),
                  TextButton(
                    child: const Text('Sign in'),
                    onPressed: () async {
                      // Use a JSON encoded string to send
                      try {
                        var httpClient = context.read<http.Client>();
                        var result = await httpClient.post(
                            Uri.parse('http://localhost:8080/login'),
                            body: json.encode(formData.toJson()),
                            headers: {'Content-Type': 'application/json'});

                        print('Response status: ${result.statusCode}');
                        print('Response body: ${result.body}');
                        if (result.statusCode == 200) {
                          //* update the [UserModel]
                          //
                          // We are using context.read() here because the callback
                          // is executed whenever the user taps the button. In other
                          // words, it is executed outside the build method.
                          // Get a hold of the [UserModel] before making the async
                          // call
                          var user = context.read<UserModel>();
                          user.email = formData.email;
                          // user.token = jsonDecode(utf8.decode(result.bodyBytes)) as String;
                          user.token = jsonDecode(result.body) as String;
                          _showDialog('Successfully signed in ${user.email} with token ${user.token}');
                        } else if (result.statusCode == 401) {
                          _showDialog('Unable to sign in.');
                        } else {
                          _showDialog(
                              'Something went wrong. Please try again.');
                        }
                      } on Exception catch (e) {
                        print('Exception details\n$e');
                        _showDialog('OOps error: $e');
                      } catch (e) {
                        print('Unknown exception $e of type ${e.runtimeType}.');
                        _showDialog(
                            'OOps Unknown exception $e of type ${e.runtimeType}');
                      }
                    },
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
