import 'package:flutter/material.dart';
import 'package:jetsclient/components/jets_form_state.dart';

import 'package:jetsclient/models/form_config.dart';
import 'package:jetsclient/routes/jets_route_data.dart';
import 'package:jetsclient/routes/jets_routes_app.dart';
import 'package:jetsclient/screens/user_flow_screen.dart';
import 'package:jetsclient/utils/constants.dart';

typedef UserFlowActionDelegate = Future<String?> Function(
    UserFlowScreenState state,
    BuildContext context,
    GlobalKey<FormState> formKey,
    JetsFormState formState,
    String actionKey,
    {dynamic group});

/// Standard User Flow Actions for use in Form Config of UserFlowState
List<FormActionConfig> standardActions = [
  FormActionConfig(
      key: ActionKeys.ufPrevious,
      label: "Previous",
      buttonStyle: ActionStyle.ufPrimary,
      leftMargin: defaultPadding,
      rightMargin: betweenTheButtonsPadding),
  FormActionConfig(
      key: ActionKeys.ufContinueLater,
      label: "Cancel",
      buttonStyle: ActionStyle.ufPrimary,
      leftMargin: betweenTheButtonsPadding,
      rightMargin: defaultPadding),
  FormActionConfig(
      key: ActionKeys.ufNext,
      label: "Next",
      buttonStyle: ActionStyle.ufSecondary,
      leftMargin: betweenTheButtonsPadding,
      rightMargin: defaultPadding),
];

/// User Flow Configuration
/// The user flow configuration is greatly inspired from the
/// Amazon States Language spec (https://states-language.net/spec.html)
/// in particular the choice state (https://states-language.net/spec.html#choice-state)
/// A configuration has a number of states, each associated with a [FormConfig]
/// each state has conditions that determine the next state to transition to.
class UserFlowConfig {
  UserFlowConfig(
      {required this.startAtKey,
      required this.states,
      this.exitScreenPath});
  final String startAtKey;
  final Map<String, UserFlowState> states;

  /// The [JetsRouteData] to visit once the user flow has terminated
  final String? exitScreenPath;

  /// Returns a list of errors or empty list if valid
  List<String> validateConfiguration() {
    final errors = <String>[];
    if (states[startAtKey] == null) {
      errors.add(
          "Invalid UserFlowConfig startAt state $startAtKey does not exists");
    }
    states.forEach((key, state) {
      if (key != state.key) {
        errors.add("Invalid UserFlow entry, key not matching, Key $key");
      }
      if (!state.isEnd &&
          state.choices.isEmpty &&
          state.defaultNextState == null) {
        errors
            .add("Invalid UserFlow state for key $key, no next state possible");
      }
    });
    return errors;
  }
}

/// User Flow State
/// Describe a step in the user flow, using [formConfig] as the form
/// configuration. This class describe which is the next steps amongst
/// the list of [choices]. If the no choices return true on the evalChoices
/// function, the [defaultNextState] is chosen.
/// If [isEnd] is true, then this is the last step of the user flow and the
/// flow is terminated.
/// Validations:
///   - if [isEnd] is true, [choices] and [defaultNextState] must be empty
///   - if [isEnd] is false, [choices] or [defaultNextState] must be populated
///
class UserFlowState {
  UserFlowState({
    required this.key,
    this.description = '',
    required this.formConfig,
    required this.actionDelegate,
    this.stateAction,
    this.choices = const [],
    this.defaultNextState,
    this.isEnd = false,
  });
  final String key;
  final String description;
  final FormConfig formConfig;
  final String? stateAction;
  final bool isEnd;
  final List<UserFlowChoice> choices;
  final String? defaultNextState;
  final UserFlowActionDelegate actionDelegate;

  /// returns the next state key of the user flow
  /// return null when no choices are true and [defaultNextState] is null
  /// Returning null indicates an error, then the user flow is in error
  String? next({int group = 0, required JetsFormState formState}) {
    for (final choice in choices) {
      if (choice.evalChoice(group: group, formState: formState)) {
        return choice.nextState;
      }
    }
    return defaultNextState;
  }
}

/// Abstract class representing a condition for selecting the next step
abstract class UserFlowChoice {
  UserFlowChoice({
    required this.nextState,
  });

  /// State key for the next state to transition to
  /// if [evalChoice] returns true
  final String nextState;

  /// returns true if this [UserFlowChoice]
  /// [nextState] is the state to transition to
  bool evalChoice({
    int group = 0,
    required JetsFormState formState,
  });
}

enum Operator {
  equals,
  contains,
  // lessThan,
  // lessThanEq,
  // greaterThan,
  // greaterThanEq,
}

/// Expression to evaluate the next [UserFlowChoice] to transition to.
/// It evaluate the expression as a binary expression (lhs op rhs)
/// The [lhsStateKey] is a Form State key
/// The [rhsValue] is also a Form State Key when [isRhsStateKey] is true
/// otherwise [rhsValue] is a literal value.
class Expression extends UserFlowChoice {
  Expression(
      {required this.lhsStateKey,
      required this.op,
      required this.rhsValue,
      required this.isRhsStateKey,
      required super.nextState});
  final String lhsStateKey;
  final Operator op;
  final String rhsValue;
  final bool isRhsStateKey;
  @override
  bool evalChoice({
    int group = 0,
    required JetsFormState formState,
  }) {
    var lhs = formState.getValue(group, lhsStateKey);
    var rhs = isRhsStateKey ? formState.getValue(group, rhsValue) : rhsValue;
    if (lhs == null || rhs == null) return false;
    switch (op) {
      case Operator.equals:
        if (lhs is List<String> && lhs.isNotEmpty) {
          lhs = lhs[0];
        }
        if (rhs is List<String> && rhs.isNotEmpty) {
          rhs = rhs[0];
        }
        return lhs == rhs;

      case Operator.contains:
        if (lhs is List<String> && rhs is String) {
          return lhs.contains(rhs);
        }
        return false;

      default:
        return false;
    }
  }
}

class IsNullExpression extends UserFlowChoice {
  IsNullExpression({
    required this.lhsStateKey,
    required super.nextState,
  });
  final String lhsStateKey;
  @override
  bool evalChoice({
    int group = 0,
    required JetsFormState formState,
  }) {
    return formState.getValue(group, lhsStateKey) == null;
  }
}

/// Applies to String? or List<String?>? in formState
class IsNullOrEmptyExpression extends UserFlowChoice {
  IsNullOrEmptyExpression({
    required this.lhsStateKey,
    required super.nextState,
  });
  final String lhsStateKey;
  @override
  bool evalChoice({
    int group = 0,
    required JetsFormState formState,
  }) {
    final v = formState.getValue(group, lhsStateKey);
    // print("!!! IsNullOrEmptyExpression key: $lhsStateKey, v: $v");
    if (v is String? || v is List<String?>?) {
      return v == null || v.isEmpty;
    }
    print(
        "OOps IsNullOrEmptyExpression expression got $v or type ${v.runtimeType}");
    return false;
  }
}

class IsNotExpression extends UserFlowChoice {
  IsNotExpression({
    required this.expression,
    required super.nextState,
  });
  final UserFlowChoice expression;
  @override
  bool evalChoice({
    int group = 0,
    required JetsFormState formState,
  }) {
    final v = expression.evalChoice(group: group, formState: formState);
    // print("!!! IsNotExpression of: $v");
    return !v;
  }
}

/// Boolean expression in the form of an 'and' if [isConjunction] is true,
/// otherwise is a 'or'
class BooleanExpression extends UserFlowChoice {
  BooleanExpression({
    required this.items,
    required this.isConjunction,
    required super.nextState,
  });
  final bool isConjunction;
  final List<UserFlowChoice> items;
  @override
  bool evalChoice({
    int group = 0,
    required JetsFormState formState,
  }) {
    if (isConjunction) {
      for (final item in items) {
        if (!item.evalChoice(group: group, formState: formState)) {
          return false;
        }
      }
      return true;
    } else {
      for (final item in items) {
        if (item.evalChoice(group: group, formState: formState)) {
          return true;
        }
      }
    }
    return false;
  }
}
