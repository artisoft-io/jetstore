import 'dart:convert';

import 'package:adaptive_theme/adaptive_theme.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/models/user.dart';

class LoginScreen extends StatefulWidget {
  const LoginScreen({super.key});

  @override
  State<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LoginScreen> {
  UserModel formData = UserModel();
  void _doLogin() async {
    // Use a JSON encoded string to send
    try {
      var client = context.read<HttpClient>();
      var user = context.read<UserModel>();
      var result = await client.httpClient.post(
          client.serverAdd.replace(path: '/login'),
          body: json.encode(formData.toJson()),
          headers: {'Content-Type': 'application/json'});
      // print('Response status: ${result.statusCode}');
      // print('Response body: ${result.body}');
      if (result.statusCode == 200) {
        // update the [UserModel]
        user.email = formData.email;
        // user.token = jsonDecode(utf8.decode(result.bodyBytes)) as String;
        user.token = jsonDecode(result.body) as String;
        // Inform the user and transition
        const snackBar = SnackBar(
          content: Text('Login Successful!'),
        );
        if (!mounted) return;
        ScaffoldMessenger.of(context).showSnackBar(snackBar);
        Navigator.pushReplacementNamed(context, '/');
      } else if (result.statusCode == 401 || result.statusCode == 422) {
        _showDialog('Invalid email and/or password.');
      } else {
        _showDialog('Something went wrong. Please try again.');
      }
    } on Exception catch (e) {
      print('Exception details\n$e');
      _showDialog('OOps error: $e');
    } catch (e) {
      print('Unknown exception $e of type ${e.runtimeType}.');
      _showDialog('OOps Unknown exception $e of type ${e.runtimeType}');
    }
  }

  void _doRegister() async {
    // Navigator.pushNamed(context, '/register').then((value) => _doLogin());
    Navigator.pushNamed(context, '/register');
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        automaticallyImplyLeading: false,
        title: const Text('Please Sign In'),
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
