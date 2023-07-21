import 'package:flutter/material.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/screens/components/form_with_tabs.dart';

import 'package:jetsclient/utils/form_config.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/screens/components/form.dart';
import 'package:jetsclient/screens/components/base_screen.dart';
import 'package:jetsclient/utils/constants.dart';

/// Screen with tabs across the top, each tab contains a form
/// The BaseScreen contains the tab controller, which has the number
/// of tabs the screen has.
/// Each tab has a JetsForm or a JetsFormWithTabs and each tab has the same
/// screen layout (screenConfig) and share the same validation and action delegates
class ScreenWithTabsWithForm extends BaseScreen {
  ScreenWithTabsWithForm({
    required super.key,
    required super.screenPath,
    required super.screenConfig,
    required this.formConfig,
  }) : super(builder: (BuildContext context, State<BaseScreen> baseState) {
          final state = baseState as ScreenWithTabsWithFormState;
          print("*** BUILDING ScreenWithTabsWithForm: ${screenConfig.title}");
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
  State<BaseScreen> createState() => ScreenWithTabsWithFormState();
}

class ScreenWithTabsWithFormState extends BaseScreenState {
  late final JetsFormState formState;
  final formKey = GlobalKey<FormState>();

  ScreenWithTabsWithForm get _widget => super.widget as ScreenWithTabsWithForm;
  ValidatorDelegate get validatorDelegate =>
      _widget.formConfig.formValidatorDelegate;
  FormActionsDelegate get actionsDelegate =>
      _widget.formConfig.formActionsDelegate;

  @override
  void initState() {
    super.initState();
    formState = _widget.formConfig.makeFormState();

    print("*** initState for ScreenWithTabsWithForm");

    triggetRefreshListner();
    JetsRouterDelegate().addListener(triggetRefreshListner);
  }

  void triggetRefreshListner() {
    JetsRouterDelegate().currentConfiguration?.params.forEach((key, value) {
      formState.setValue(0, key, value);
    });
    // reset the updated keys since these updates is to put default values
    // and is not from user interactions
    //* TODO - Stop using group 0 as a special group with validation keys
    // formState.resetUpdatedKeys(0);
    setState(() {});
  }

  @override
  void dispose() {
    JetsRouterDelegate().removeListener(triggetRefreshListner);
    super.dispose();
  }

}
