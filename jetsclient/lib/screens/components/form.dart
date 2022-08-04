import 'package:flutter/material.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/utils/form_config.dart';

import '../../routes/jets_route_data.dart';

class JetsForm extends StatelessWidget {
  const JetsForm({
    Key? key,
    required this.formPath,
    required this.formState,
    required this.formKey,
    required this.formConfig,
    required this.validatorDelegate,
    required this.actionsDelegate,
  }) : super(key: key);

  final JetsFormState formState;
  final GlobalKey<FormState> formKey;
  final FormConfig formConfig;
  final ValidatorDelegate validatorDelegate;
  final FormActionsDelegate actionsDelegate;
  final JetsRouteData formPath;

  @override
  Widget build(BuildContext context) {
    final themeData = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.fromLTRB(0, 8, 0, 0),
      child: FocusTraversalGroup(
        child: Form(
            key: formKey,
            child: ListView.builder(
                itemBuilder: (BuildContext context, int index) {
                  if (index < formConfig.inputFields.length) {
                    var fc = formConfig.inputFields[index];
                    return Row(
                      children: fc
                          .map((e) => e.makeFormField(
                                screenPath: formPath,
                                state: formState,
                                formFieldValidator: (group, key, v) =>
                                    validatorDelegate(
                                        context, formState, group, key, v),
                                formValidator: validatorDelegate,
                                formActionsDelegate: actionsDelegate,
                              ))
                          .toList(),
                    );
                  }
                  // case last: row of buttons
                  return Padding(
                    padding: const EdgeInsets.fromLTRB(10, 0, 0, 0),
                    child: Center(
                      child: Row(
                          children: List<Widget>.from(
                        formConfig.actions.map((e) => ElevatedButton(
                            style: ElevatedButton.styleFrom(
                              // Foreground color
                              foregroundColor: themeData.colorScheme.onPrimary,
                              backgroundColor: themeData.colorScheme.primary,
                            ).copyWith(
                                elevation: ButtonStyleButton.allOrNull(0.0)),
                            onPressed: () => actionsDelegate(
                                context, formKey, formState, e.key),
                            child: Text(e.label))),
                        growable: false,
                      )
                              .expand((element) => [
                                    const SizedBox(width: defaultPadding),
                                    element
                                  ])
                              .toList()),
                    ),
                  );
                },
                itemCount: formConfig.inputFields.length + 1)),
      ),
    );
  }
}
