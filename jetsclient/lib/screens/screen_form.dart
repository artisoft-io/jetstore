import 'package:flutter/material.dart';

import 'package:jetsclient/utils/form_config.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/screens/components/form.dart';
import 'package:jetsclient/screens/components/base_screen.dart';

class ScreenWithForm extends BaseScreen {
  ScreenWithForm({
    required super.key,
    required super.screenPath,
    required super.screenConfig,
    required this.formConfig,
    required this.formValidatorDelegate,
    required this.formActionsDelegate,
  }) : super(builder: (State<BaseScreen> baseState) {
          final state = baseState as ScreenWithFormState;
          return JetsForm(
              formPath: screenPath,
              formState: state.formState,
              formKey: state.formKey,
              formConfig: formConfig,
              validatorDelegate: state.validatorDelegate,
              actionsDelegate: state.actionsDelegate);
        });

  final FormConfig formConfig;
  final ValidatorDelegate formValidatorDelegate;
  final FormActionsDelegate formActionsDelegate;

  @override
  State<BaseScreen> createState() => ScreenWithFormState();
}

class ScreenWithFormState extends BaseScreenState {
  late final JetsFormState formState;
  final formKey = GlobalKey<FormState>();
  late final FormConfig formConfig;

  ScreenWithForm get _widget => super.widget as ScreenWithForm;
  ValidatorDelegate get validatorDelegate => _widget.formValidatorDelegate;
  FormActionsDelegate get actionsDelegate => _widget.formActionsDelegate;

  @override
  void initState() {
    super.initState();
    formState = _widget.formConfig.makeFormState();
  }
}
