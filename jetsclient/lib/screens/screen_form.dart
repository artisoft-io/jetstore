import 'package:flutter/material.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/screens/components/form_with_tabs.dart';

import 'package:jetsclient/utils/form_config.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/screens/components/form.dart';
import 'package:jetsclient/screens/components/base_screen.dart';
import 'package:jetsclient/utils/constants.dart';

class ScreenWithForm extends BaseScreen {
  ScreenWithForm({
    required super.key,
    required super.screenPath,
    required super.screenConfig,
    required this.formConfig,
  }) : super(builder: (BuildContext context, State<BaseScreen> baseState) {
          final state = baseState as ScreenWithFormState;
          return Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Flexible(
                  flex: 1,
                  fit: FlexFit.tight,
                  child: Padding(
                    padding: const EdgeInsets.fromLTRB(
                        defaultPadding, 2 * defaultPadding, 0, 0),
                    child: Text(
                      screenConfig.title,
                      style: Theme.of(context).textTheme.headlineMedium,
                    ),
                  ),
                ),
                Flexible(
                  flex: 8,
                  fit: FlexFit.tight,
                  child: formConfig.formTabsConfig.isNotEmpty
                      ? JetsFormWithTabs(
                          formPath: screenPath,
                          formState: state.formState,
                          formKey: state.formKey,
                          formConfig: formConfig)
                      : JetsForm(
                          formPath: screenPath,
                          formState: state.formState,
                          formKey: state.formKey,
                          formConfig: formConfig),
                ),
              ]);
        });

  final FormConfig formConfig;

  @override
  State<BaseScreen> createState() => ScreenWithFormState();
}

class ScreenWithFormState extends BaseScreenState {
  late final JetsFormState formState;
  final formKey = GlobalKey<FormState>();
  late final FormConfig formConfig;

  ScreenWithForm get _widget => super.widget as ScreenWithForm;
  ValidatorDelegate get validatorDelegate =>
      _widget.formConfig.formValidatorDelegate;
  FormActionsDelegate get actionsDelegate =>
      _widget.formConfig.formActionsDelegate;

  @override
  void initState() {
    super.initState();
    formState = _widget.formConfig.makeFormState();

    // Initialize the Form State with the current navigation params
    JetsRouterDelegate().currentConfiguration?.params.forEach((key, value) {
      formState.setValue(0, key, value);
    });
    // reset the updated keys since these updates is to put default values
    // and is not from user interactions
    formState.resetUpdatedKeys(0);
  }
}
