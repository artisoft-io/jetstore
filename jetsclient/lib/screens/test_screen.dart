import 'dart:convert';
import 'package:flutter/material.dart';

import 'package:jetsclient/utils/form_config.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/screens/components/form.dart';
import 'package:jetsclient/screens/components/app_bar.dart';
import 'package:jetsclient/screens/components/base_screen.dart';

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
    // print(
    //     "Form: dataTableDemoForm, validating group: $group, key $key for value $v ");
    assert((v is String?) || (v is List<String>?));
    switch (key) {
      case "dataTableDemoMainTable":
        var value = v as List<String>?;
        if (value == null || value.isEmpty) {
          return "Client Input row must be selected.";
        }
        return null;
      case "dataTableDemoSupportTable":
        return null;
      default:
        throw Exception(
            'ERROR: Invalid program configuration: No validator configured for form field $key');
    }
  }

  void _doIt() async {
    var valid = formKey.currentState!.validate();
    if (!valid) {
      return;
    }
    showAlertDialog(context, formState.encodeFullState("  "));
  }
}
