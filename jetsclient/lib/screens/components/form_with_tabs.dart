import 'package:flutter/material.dart';
import 'package:jetsclient/routes/jets_route_data.dart';
import 'package:jetsclient/screens/components/form_button.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/utils/form_config.dart';

class JetsFormWithTabs extends StatefulWidget {
  const JetsFormWithTabs({
    Key? key,
    required this.formPath,
    required this.formState,
    required this.formKey,
    required this.formConfig,
    this.isDialog = false,
  }) : super(key: key);

  final JetsFormState formState;
  final GlobalKey<FormState> formKey;
  final FormConfig formConfig;
  final JetsRouteData formPath;
  final bool isDialog;

  @override
  State<JetsFormWithTabs> createState() => JetsFormWithTabsWidgetState();
}

class JetsFormWithTabsWidgetState extends State<JetsFormWithTabs> {
  List<FormTabConfig> get formTabs => widget.formConfig.formTabsConfig;

  @override
  void initState() {
    super.initState();
    widget.formState.isDialog = widget.isDialog;
    widget.formState.formKey = widget.formKey;
  }

  @override
  Widget build(BuildContext context) {
    final formTabsConfig = widget.formConfig.formTabsConfig;
    return FocusTraversalGroup(
      child: Form(
        key: widget.formKey,
        child: AutofillGroup(
          child: DefaultTabController(
            length: formTabsConfig.length,
            child: Column(
              children: [
                TabBar(
                    tabs: List<Tab>.generate(
                        formTabsConfig.length,
                        (index) => Tab(
                                child: Text(
                              formTabsConfig[index].label,
                              style: Theme.of(context).textTheme.headlineSmall,
                            )))),
                // The actual tabs
                Flexible(
                    flex: 8,
                    fit: FlexFit.tight,
                    child: TabBarView(
                        children: List<Widget>.generate(
                            formTabsConfig.length,
                            (index) => Padding(
                                padding: const EdgeInsets.fromLTRB(
                                    0.0, 48.0, 0.0, 0.0),
                                child: formTabsConfig[index]
                                    .inputField
                                    .makeFormField(
                                        screenPath: widget.formPath,
                                        formConfig: widget.formConfig,
                                        formState: widget.formState))))),
                // Row of buttons (actions)
                if (widget.formConfig.actions.isNotEmpty)
                  Center(
                    child: Padding(
                      padding:
                          const EdgeInsets.fromLTRB(0, defaultPadding, 0, 0),
                      child: Row(
                          children: widget.formConfig.actions
                              .map((e) => JetsFormButton(
                                  key: Key(e.key),
                                  formActionConfig: e,
                                  formKey: widget.formKey,
                                  formState: widget.formState,
                                  actionsDelegate:
                                      widget.formConfig.formActionsDelegate))
                              .toList()),
                    ),
                  )
              ],
            ),
          ),
        ),
      ),
    );
  }
}
