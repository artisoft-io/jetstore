import 'package:flutter/material.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';

import 'package:jetsclient/utils/form_config.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/screens/components/form.dart';
import 'package:jetsclient/screens/components/base_screen.dart';
import 'package:jetsclient/utils/constants.dart';

class ScreenWithMultiForms extends BaseScreen {
  ScreenWithMultiForms({
    required super.key,
    required super.screenPath,
    required super.screenConfig,
    required this.formConfig,
  }) : super(builder: (BuildContext context, State<BaseScreen> baseState) {
          final state = baseState as ScreenWithMultiFormsState;
          // print("*** BUILDING * ScreenWithMultiForms: ${screenConfig.title}");
          var l = formConfig.length + 1;
          if (screenConfig.title == null) {
            l = formConfig.length;
          }
          return Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: List<Widget>.generate(l, (index) {
                if (index == 0 && screenConfig.title != null) {
                  return Flexible(
                    flex: 1,
                    fit: FlexFit.tight,
                    child: Padding(
                      padding: const EdgeInsets.fromLTRB(
                          defaultPadding, 2 * defaultPadding, 0, 0),
                      child: Text(
                        screenConfig.title!,
                        style: Theme.of(context).textTheme.headlineMedium,
                      ),
                    ),
                  );
                }
                final idx = screenConfig.title == null ? index : index - 1;
                return Flexible(
                  flex: 4 + idx * 8,
                  fit: FlexFit.tight,
                  child: JetsForm(
                      key: GlobalKey(),
                      formPath: screenPath,
                      formState: state.formState[idx],
                      formKey: GlobalKey<FormState>(),
                      formConfig: formConfig[idx]),
                );
              }));
        });

  final List<FormConfig> formConfig;

  @override
  State<BaseScreen> createState() => ScreenWithMultiFormsState();
}

class ScreenWithMultiFormsState extends BaseScreenState {
  late final List<JetsFormState> formState;
  // final formKey = GlobalKey<FormState>();
  late final FormConfig formConfig;

  ScreenWithMultiForms get _widget => super.widget as ScreenWithMultiForms;
  ValidatorDelegate get validatorDelegate =>
      _widget.formConfig[0].formValidatorDelegate;
  FormActionsDelegate get actionsDelegate =>
      _widget.formConfig[0].formActionsDelegate;

  @override
  void initState() {
    super.initState();
    formState = List<JetsFormState>.generate(_widget.formConfig.length,
        (index) => _widget.formConfig[index].makeFormState());

    // Initialize the Form State with the current navigation params
    final params = JetsRouterDelegate().currentConfiguration?.params;
    for (var i = 0; i < _widget.formConfig.length; i++) {
      params?.forEach((key, value) {
        //* TODO - Stop using group 0 as a special group with validation keys
        formState[i].setValue(0, key, value);
      });
      // reset the updated keys since these updates is to put default values
      // and is not from user interactions
      //* TODO - Stop using group 0 as a special group with validation keys
      formState[i].resetUpdatedKeys(0);
      formState[i].peersFormState = formState;
    }
  }
}
