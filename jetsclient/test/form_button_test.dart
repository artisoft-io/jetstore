import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/models/form_config.dart';
import 'package:jetsclient/routes/jets_route_data.dart';
import 'package:jetsclient/utils/constants.dart';

/// Builds a button the way [JetsForm] builds a row of form fields: every field
/// wrapped in a Flexible carrying the field's flex.
Widget _asFormField(FormActionConfig config, JetsFormState formState) {
  return MaterialApp(
    home: Scaffold(
      body: Form(
        key: formState.formKey,
        child: Row(
          children: [
            Flexible(
              flex: config.flex,
              fit: FlexFit.tight,
              child: config.makeFormField(
                screenPath: const JetsRouteData('/test'),
                formConfig: FormConfig(
                  key: 'testForm',
                  actions: const [],
                  formValidatorDelegate: (p0, p1, p2, p3) => null,
                  formActionsDelegate: doNothingAction,
                ),
                formState: formState,
              ),
            ),
          ],
        ),
      ),
    ),
  );
}

void main() {
  late JetsFormState formState;

  setUp(() {
    formState = JetsFormState(initialGroupCount: 1);
    formState.formKey = GlobalKey<FormState>();
  });

  testWidgets(
      'action button renders as a form field without a ParentDataWidget conflict',
      (WidgetTester tester) async {
    final config = FormActionConfig(
        key: 'testAction', label: 'Go', buttonStyle: ActionStyle.primary);
    await tester.pumpWidget(_asFormField(config, formState));
    expect(tester.takeException(), isNull);
    expect(find.text('Go'), findsOneWidget);
  });

  testWidgets(
      'isEnabledEval disables the button and re-enables it on state change',
      (WidgetTester tester) async {
    final config = FormActionConfig(
      key: 'testAction',
      label: 'Start',
      buttonStyle: ActionStyle.primary,
      isEnabledEval: (state) => state.getValue(0, 'serverState') != 'running',
    );
    formState.setValue(0, 'serverState', 'running');
    await tester.pumpWidget(_asFormField(config, formState));

    ElevatedButton button() =>
        tester.widget<ElevatedButton>(find.byType(ElevatedButton));
    expect(button().onPressed, isNull, reason: 'disabled while running');

    formState.setValueAndNotify(0, 'serverState', 'stopped');
    // The button's state listener defers its setState by a zero-duration
    // Future, so the rebuild lands on a later frame.
    await tester.pumpAndSettle();
    expect(button().onPressed, isNotNull, reason: 're-enabled once stopped');
  });
}
