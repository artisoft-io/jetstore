import 'package:flutter/material.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/utils/form_config.dart';

import '../../routes/jets_route_data.dart';

class JetsForm extends StatelessWidget {
  const JetsForm(
      {Key? key,
      required this.formPath,
      required this.formData,
      required this.formKey,
      required this.formConfig,
      required this.validatorDelegate,
      required this.actions})
      : super(key: key);

  final JetsFormState formData;
  final GlobalKey<FormState> formKey;
  final FormConfig formConfig;
  final String? Function(int group, String, dynamic) validatorDelegate;
  final Map<String, VoidCallback> actions;
  final JetsRouteData formPath;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(0, 8, 0, 0),
      child: Form(
          key: formKey,
          child: ListView.builder(
              itemBuilder: (BuildContext context, int index) {
                if (index < formConfig.inputFields.length) {
                  var fc = formConfig.inputFields[index];
                  return Row(
                    children: fc.map((e) => e.makeFormField(screenPath: formPath, state: formData, validator: validatorDelegate)).toList(),
                  );
                }
                // case last: row of buttons
                return Padding(
                  padding: const EdgeInsets.fromLTRB(10, 0, 0, 0),
                  child: Center(
                    child: Row(
                        children: List<Widget>.from(
                      formConfig.actions.map((e) => TextButton(
                          onPressed: actions[e.key], 
                          child: Text(e.label))),
                      growable: false,
                    )),
                  ),
                );
              },
              itemCount: formConfig.inputFields.length + 1)),
    );
  }
}
