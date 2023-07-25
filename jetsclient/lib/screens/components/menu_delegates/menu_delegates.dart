import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/routes/export_routes.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/screens/components/dialogs.dart';
import 'package:jetsclient/utils/screen_config.dart';

/// Menu Delegates are action functions to perform on the menu button
/// onPress method without navigating to a new form, typically use
/// a simple dialog for confirmation and issue a command to back end
Future<int> purgeDataAction(BuildContext context, MenuEntry menuEntry, State<StatefulWidget> state) async {
  var messenger = ScaffoldMessenger.of(context);
  // get user confirmation
  var uc = await showDangerZoneDialog(context,
      'Are you sure you want to purge ALL client data and rebuild the client tables?');
  // print('purgeData Action GOT: $uc');
  if (uc != 'OK') return 200;
  var encodedJsonBody = jsonEncode(<String, dynamic>{
    'action': 'reset_domain_tables',
    'table': '',
    'data': [],
  }, toEncodable: (_) => '');
  var result = await HttpClientSingleton().sendRequest(
      path: ServerEPs.purgeDataEP,
      token: JetsRouterDelegate().user.token,
      encodedJsonBody: encodedJsonBody);
  // if (!mounted) return; needed?
  if (result.statusCode == 200) {
    // Inform the user and transition
    const snackBar = SnackBar(
      content: Text('Purge Client Data Successful'),
    );
    messenger.showSnackBar(snackBar);
  } else {
    return result.statusCode;
  }
  return 200;
}

/// Rerun database init script
Future<int> rerunDbInitAction(
    BuildContext context, MenuEntry menuEntry, State<StatefulWidget> state) async {
  var messenger = ScaffoldMessenger.of(context);
  // get user confirmation
  var uc = await showDangerZoneDialog(context,
      'Are you sure you want to rerun the database init script? This will reset read only config tables and built-in test client config');
  // print('purgeData Action GOT: $uc');
  if (uc != 'OK') return 200;
  var encodedJsonBody = jsonEncode(<String, dynamic>{
    'action': 'rerun_db_init',
    'table': '',
    'data': [],
  }, toEncodable: (_) => '');
  var result = await HttpClientSingleton().sendRequest(
      path: ServerEPs.purgeDataEP,
      token: JetsRouterDelegate().user.token,
      encodedJsonBody: encodedJsonBody);
  // if (!mounted) return; needed?
  if (result.statusCode == 200) {
    // Inform the user and transition
    const snackBar = SnackBar(
      content: Text('Re-run Database Initialization Successful'),
    );
    messenger.showSnackBar(snackBar);
  } else {
    return result.statusCode;
  }
  return 200;
}
