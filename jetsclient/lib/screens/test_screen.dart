import 'package:flutter/material.dart';

import 'package:jetsclient/routes/export_routes.dart';
import 'package:jetsclient/utils/form_config.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/screens/components/form.dart';
import 'package:provider/provider.dart';
import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/screens/components/app_bar.dart';
import 'package:jetsclient/screens/components/base_screen.dart';

import 'package:jetsclient/screens/components/data_table.dart';
import 'package:jetsclient/utils/data_table_config.dart';

class TestScreen extends BaseScreen {
  TestScreen({
    required super.key,
    required super.screenPath,
    required super.screenConfig,
    required this.formConfig,
  }) : super(builder: (State<BaseScreen> baseState) {
          final state = baseState as _TestScreenState;
          return JetsForm(
              formPath: screenPath,
              formState: state.formState,
              formKey: state.formKey,
              formConfig: formConfig,
              validatorDelegate: state.validatorDelegate,
              actions: <String, VoidCallback>{
                "dataTableDemoAction1": state._doIt,
              });
        });

  final FormConfig formConfig;

  @override
  State<BaseScreen> createState() => _TestScreenState();
}

class _TestScreenState extends BaseScreenState {
  late final JetsFormState formState;
  final formKey = GlobalKey<FormState>();
  late final FormConfig formConfig;

  @override
  void initState() {
    super.initState();
    final w = widget as TestScreen;
    formState = w.formConfig.makeFormState();
  }

  String? validatorDelegate(int group, String key, dynamic v) {
    // String? value = v;
    print(
        "Form: dataTableDemoForm, validating group: $group, key $key for value $v ");
    // switch (key) {
    //   case FSK.userEmail:
    //     if (value != null && value.characters.length > 3) {
    //       return null;
    //     }
    //     return "Email must be provided.";
    //   case FSK.userPassword:
    //     if (value != null && value.length >= 4) {
    //       return null;
    //     }
    //     return "Password must be provided.";
    //   default:
    //     throw Exception(
    //         'ERROR: Invalid program configuration: No validator configured for form field $key');
    // }
    return null;
  }

  void _doIt() async {
    showAlertDialog(context, "Let's do it!");
  }

}
