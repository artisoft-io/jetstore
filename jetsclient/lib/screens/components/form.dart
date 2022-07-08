import 'package:flutter/material.dart';
import 'package:jetsclient/utils/form_config.dart';

class JetsForm extends StatelessWidget {
  const JetsForm(
      {Key? key,
      required this.formData,
      required this.formKey,
      required this.formConfig,
      required this.validatorDelegate,
      required this.actions})
      : super(key: key);

  final List<Map<String, dynamic>> formData;
  final GlobalKey<FormState> formKey;
  final FormConfig formConfig;
  final String? Function(int group, String, String?) validatorDelegate;
  final Map<String, VoidCallback> actions;

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
                    children: fc.map((e) => e.makeFormField(state: formData, validator: validatorDelegate)).toList(),
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
