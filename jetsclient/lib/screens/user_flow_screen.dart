import 'package:flutter/material.dart';
import 'package:jetsclient/models/user_flow_config.dart';
import 'package:jetsclient/modules/actions/user_flow_actions.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/components/form_with_tabs.dart';

import 'package:jetsclient/models/form_config.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/components/form.dart';
import 'package:jetsclient/screens/base_screen.dart';
import 'package:jetsclient/utils/constants.dart';

class UserFlowScreen extends BaseScreen {
  UserFlowScreen({
    required super.key,
    required super.screenPath,
    required super.screenConfig,
    required this.userFlowConfig,
  }) : super(builder: (BuildContext context, State<BaseScreen> baseState) {
          final state = baseState as UserFlowScreenState;
          // print("*** BUILDING UserFlowScreen: ${screenConfig.title}");
          final formConfig = state.formConfig;
          // Curried Form Action Delegate to put the UF State
          formConfig.formActionsDelegate = (BuildContext context,
                  GlobalKey<FormState> formKey,
                  JetsFormState formState,
                  String actionKey,
                  {group = 0}) =>
              userFlowStateActions(
                  state, context, formKey, formState, actionKey,
                  group: group);
          return Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (screenConfig.title != null)
                  Flexible(
                    flex: 1,
                    fit: FlexFit.tight,
                    child: Padding(
                      padding: const EdgeInsets.fromLTRB(
                          defaultPadding, 0, 0, 0),
                      child: Text(
                        screenConfig.title!,
                        style: Theme.of(context).textTheme.headlineMedium,
                      ),
                    ),
                  ),
                Flexible(
                  flex: 10,
                  fit: FlexFit.tight,
                  child: formConfig.formTabsConfig.isNotEmpty
                      ? JetsFormWithTabs(
                          key: GlobalKey(),
                          formPath: screenPath,
                          formState: state.formState,
                          formKey: state.formKey,
                          formConfig: formConfig)
                      : JetsForm(
                          key: GlobalKey(),
                          formPath: screenPath,
                          formState: state.formState,
                          formKey: state.formKey,
                          formConfig: formConfig),
                ),
              ]);
        });

  final UserFlowConfig userFlowConfig;

  @override
  State<BaseScreen> createState() => UserFlowScreenState();
}

class UserFlowScreenState extends BaseScreenState {
  late final JetsFormState formState;
  late UserFlowState currentUserFlowState;
  final formKey = GlobalKey<FormState>();
  late FormConfig formConfig;

  UserFlowScreen get _widget => super.widget as UserFlowScreen;
  UserFlowConfig get userFlowConfig => _widget.userFlowConfig;

  @override
  void initState() {
    super.initState();
    final ufState = userFlowConfig.states[userFlowConfig.startAtKey];
    if (ufState == null) {
      print(
          "*** ERROR userFlowConfig for ${userFlowConfig.startAtKey} not found!");
    } else {
      // print(
      //     "^^^ Setting currentUserFlowState to ${ufState.key}: ${ufState.description}");
      currentUserFlowState = ufState;
      formConfig = currentUserFlowState.formConfig;
      formState = formConfig.makeFormState();
      // Initialize the Form State with the current navigation params
      JetsRouterDelegate().currentConfiguration?.params.forEach((key, value) {
        formState.setValue(0, key, value);
      });
      formState.setValue(0, 'user_email', JetsRouterDelegate().user.email);

      // reset the updated keys since these updates is to put default values
      // and is not from user interactions
      //* TODO - Stop using group 0 as a special group with validation keys
      formState.resetUpdatedKeys(0);

      // Keep the list of visited page for supporting previous and next buttons
      formState.setValue(0, FSK.ufCurrentPage, 0);
      final visitedPages = <String>[
        userFlowConfig.startAtKey
      ];
      formState.setValue(0, FSK.ufVisitedPages, visitedPages);
    }
  }

  void setCurrentUserFlowState(UserFlowState ufState, FormConfig fConfig) {
    setState(() {
      currentUserFlowState = ufState;
      formConfig = fConfig;
    });
  }
}
