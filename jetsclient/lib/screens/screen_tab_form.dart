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
    this.formConfigWhenTabs,
  }) : super(builder: (BuildContext context, State<BaseScreen> baseState) {
          final state = baseState as ScreenWithTabsWithFormState;
          // print("*** BUILDING ScreenWithTabsWithForm, title is ${screenConfig.title}");
          return state.tabsStateHelper.tabsParams.isEmpty ||
                  formConfigWhenTabs == null
              ? Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                  if (screenConfig.title != null)
                    Flexible(
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
                    ),
                  Flexible(
                    flex: 8,
                    fit: FlexFit.tight,
                    child: formConfig.formTabsConfig.isNotEmpty
                        ? JetsFormWithTabs(
                            formPath: screenPath,
                            formState: state.formStateWhenNoTabs,
                            formKey: state.formKey,
                            formConfig: formConfig)
                        : JetsForm(
                            formPath: screenPath,
                            formState: state.formStateWhenNoTabs,
                            formKey: state.formKey,
                            formConfig: formConfig),
                  ),
                ])
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
                              (index) => Column(
                                      crossAxisAlignment:
                                          CrossAxisAlignment.start,
                                      children: [
                                        if (screenConfig.title != null)
                                          Flexible(
                                            flex: 1,
                                            fit: FlexFit.tight,
                                            child: Padding(
                                              padding:
                                                  const EdgeInsets.fromLTRB(
                                                      defaultPadding,
                                                      2 * defaultPadding,
                                                      0,
                                                      0),
                                              child: Text(
                                                screenConfig.title!,
                                                style: Theme.of(context)
                                                    .textTheme
                                                    .headlineMedium,
                                              ),
                                            ),
                                          ),
                                        Flexible(
                                          flex: 8,
                                          fit: FlexFit.tight,
                                          child: formConfigWhenTabs!
                                                  .formTabsConfig.isNotEmpty
                                              ? JetsFormWithTabs(
                                                  formPath: screenPath,
                                                  formState: state
                                                      .tabsStateHelper
                                                      .tabsParams[index]
                                                      .formState,
                                                  formKey: state.formKey,
                                                  formConfig:
                                                      formConfigWhenTabs)
                                              : JetsForm(
                                                  formPath: screenPath,
                                                  formState: state
                                                      .tabsStateHelper
                                                      .tabsParams[index]
                                                      .formState,
                                                  formKey: state.formKey,
                                                  formConfig:
                                                      formConfigWhenTabs),
                                        ),
                                      ]))))
                ]);
        });

  final FormConfig formConfig;
  final FormConfig? formConfigWhenTabs;

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
    // triggetRefreshListner();
    JetsRouterDelegate().addListener(triggetRefreshListner);
  }

  void triggetRefreshListner() {
    JetsRouterDelegate().currentConfiguration?.params.forEach((key, value) {
      formStateWhenNoTabs.setValue(0, key, value);
    });
    setState(() {});
  }

  @override
  void dispose() {
    JetsRouterDelegate().removeListener(triggetRefreshListner);
    super.dispose();
  }
}