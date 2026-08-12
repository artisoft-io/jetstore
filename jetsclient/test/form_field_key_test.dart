import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/models/form_config.dart';
import 'package:jetsclient/routes/jets_route_data.dart';

/// [FormFieldConfig.makeFormField] used to stamp every widget with a
/// `UniqueKey()`, which gave each field a fresh identity on every build. Flutter
/// then discarded the element and its State on each rebuild, so a field could
/// not hold anything across one — the reason a text field's controller had to be
/// re-seeded, and why an in-progress edit could be thrown away by an unrelated
/// notify.
///
/// The key is now derived from the field's identity, `group::key`, which is
/// stable across builds and distinct between fields. These tests pin both
/// halves of that: stability, and the absence of collisions that would throw a
/// duplicate-key error inside one form.
FormConfig _formConfig() => FormConfig(
      key: 'testForm',
      actions: const [],
      formValidatorDelegate: (p0, p1, p2, p3) => null,
      formActionsDelegate: doNothingAction,
    );

Key? _keyOf(FormFieldConfig config, JetsFormState formState) =>
    config
        .makeFormField(
          screenPath: const JetsRouteData('/test'),
          formConfig: _formConfig(),
          formState: formState,
        )
        .key;

FormInputFieldConfig _input({required String key, int group = 0}) =>
    FormInputFieldConfig(
      key: key,
      group: group,
      label: key,
      hint: key,
      autofocus: false,
      textRestriction: TextRestriction.none,
      maxLength: 100,
    );

void main() {
  late JetsFormState formState;

  setUp(() {
    formState = JetsFormState(initialGroupCount: 2);
    formState.formKey = GlobalKey<FormState>();
  });

  test('the same field config yields the same key on every build', () {
    final config = _input(key: 'inputColumn');
    expect(_keyOf(config, formState), _keyOf(config, formState));
  });

  test('two configs describing the same field agree on the key', () {
    expect(_keyOf(_input(key: 'inputColumn'), formState),
        _keyOf(_input(key: 'inputColumn'), formState));
  });

  test('different fields in the same group get different keys', () {
    expect(_keyOf(_input(key: 'inputColumn'), formState),
        isNot(_keyOf(_input(key: 'functionName'), formState)));
  });

  test('the same field in different groups gets different keys', () {
    // This is the dynamic-row case: a row builder emits the same field keys for
    // every row and separates them by group, so group must take part in the
    // identity or every row would collide.
    expect(_keyOf(_input(key: 'inputColumn', group: 0), formState),
        isNot(_keyOf(_input(key: 'inputColumn', group: 1), formState)));
  });

  testWidgets('a row of distinct fields builds without a duplicate-key error',
      (WidgetTester tester) async {
    final fields = [
      _input(key: 'inputColumn'),
      _input(key: 'functionName'),
      _input(key: 'functionArgument'),
    ];
    await tester.pumpWidget(MaterialApp(
      home: Scaffold(
        body: Form(
          key: formState.formKey,
          child: Row(
            children: fields
                .map((f) => Flexible(
                      flex: f.flex,
                      fit: FlexFit.tight,
                      child: f.makeFormField(
                        screenPath: const JetsRouteData('/test'),
                        formConfig: _formConfig(),
                        formState: formState,
                      ),
                    ))
                .toList(),
          ),
        ),
      ),
    ));
    expect(tester.takeException(), isNull);
  });
}
