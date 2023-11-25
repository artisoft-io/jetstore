import 'package:flutter/material.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/components/form_with_tabs.dart';

import 'package:jetsclient/models/form_config.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/components/form.dart';
import 'package:jetsclient/screens/base_screen.dart';
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
          // print("*** BUILDING ScreenWithTabsWithForm, title is ${screenConfig.title}");
          return state.tabsStateHelper.tabsParams.isEmpty
              // Case tabs are empty, showing default screen w/o tabs
              ? Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
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
                    flex: 8,
                    fit: FlexFit.tight,
                    child: formConfig.formTabsConfig.isNotEmpty
                        ? JetsFormWithTabs(
                            key: GlobalKey(),
                            formPath: screenPath,
                            formState: state.formStateWhenNoTabs,
                            formKey: state.formKey,
                            formConfig: formConfig)
                        : JetsForm(
                            key: GlobalKey(),
                            formPath: screenPath,
                            formState: state.formStateWhenNoTabs,
                            formKey: state.formKey,
                            formConfig: formConfig),
                  ),
                ])
              // Case with tabs, each tab has a formConfig, specified by
              // JetsTabParams.FormConfigKey
              : Column(children: [
                  TabBar(
                      controller: state.tabController,
                      tabs: List<Tab>.generate(
                          state.tabsStateHelper.tabsParams.length,
                          (index) => Tab(
                                  child: Row(
                                children: [
                                  Text(
                                      state.tabsStateHelper.tabsParams[index]
                                          .label,
                                      textAlign: TextAlign.left,
                                      style: Theme.of(context)
                                          .textTheme
                                          .bodyMedium,
                                      overflow: TextOverflow.ellipsis),
                                  IconButton(
                                      onPressed: () => state.removeTab(index),
                                      icon: Icon(Icons.close,
                                          color: Colors.blue.shade400)),
                                ],
                              )))),
                  // The actual tabs
                  Flexible(
                      flex: 8,
                      fit: FlexFit.tight,
                      child: TabBarView(
                          controller: state.tabController,
                          children: List<Widget>.generate(
                              state.tabsStateHelper.tabsParams.length,
                              (index) => state.tabsStateHelper.tabsParams[index]
                                      .formConfig.formTabsConfig.isNotEmpty
                                  ? JetsFormWithTabs(
                                      key: GlobalKey(),
                                      formPath: screenPath,
                                      formState: state.tabsStateHelper
                                          .tabsParams[index].formState,
                                      formKey: state.formKey,
                                      formConfig: state.tabsStateHelper
                                          .tabsParams[index].formConfig)
                                  : JetsForm(
                                      key: GlobalKey(),
                                      formPath: screenPath,
                                      formState: state.tabsStateHelper
                                          .tabsParams[index].formState,
                                      formKey: state.formKey,
                                      formConfig: state.tabsStateHelper
                                          .tabsParams[index].formConfig))))
                ]);
        });

  final FormConfig formConfig;

  @override
  State<BaseScreen> createState() => ScreenWithTabsWithFormState();
}

class ScreenWithTabsWithFormState extends BaseScreenState {
  final formKey = GlobalKey<FormState>();
  late final JetsFormState formStateWhenNoTabs;

  ScreenWithTabsWithForm get _widget => super.widget as ScreenWithTabsWithForm;
  ValidatorDelegate get validatorDelegate =>
      _widget.formConfig.formValidatorDelegate;
  FormActionsDelegate get actionsDelegate =>
      _widget.formConfig.formActionsDelegate;

  @override
  void initState() {
    super.initState();
    formStateWhenNoTabs = _widget.formConfig.makeFormState();

    // Put navigation params into form state
    JetsRouterDelegate().currentConfiguration?.params.forEach((key, value) {
      formStateWhenNoTabs.setValue(0, key, value);
    });
    // reset the updated keys since these updates is to put default values
    // and is not from user interactions
    //* TODO - Stop using group 0 as a special group with validation keys
    formStateWhenNoTabs.resetUpdatedKeys(0);

  }
}
