import 'package:flutter/material.dart';
import 'package:jetsclient/routes/jets_route_data.dart';
import 'package:jetsclient/screens/components/form.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/utils/form_config.dart';

/// our basic alert dialog, may need some upgrade
void showAlertDialog(BuildContext context, String message) {
  showDialog<void>(
    context: context,
    builder: (context) => AlertDialog(
      title: Text(message),
      actions: [
        TextButton(
          child: const Text('OK'),
          onPressed: () => Navigator.of(context).pop(),
        ),
      ],
    ),
  );
}

/// Enum to record the action result for data table actions
/// Used as the template type for DialogResultHandler when used
/// with data table actions
enum DTActionResult { canceled, ok, okDataTableDirty }

typedef DialogResultHandler<T> = void Function(BuildContext context, T? t);

Future<void> showFormDialog<T>(
    {required GlobalKey<FormState> formKey,
    required JetsRouteData screenPath,
    required BuildContext context,
    required JetsFormState formState,
    required FormConfig formConfig,
    required ValidatorDelegate validatorDelegate,
    required FormActionsDelegate actionsDelegate,
    required DialogResultHandler<T> resultHandler}) async {
  resultHandler(
      context,
      await showDialog<T>(
        context: context,
        builder: (context) => Dialog(
          child: JetsForm(
            formKey: formKey,
            formPath: screenPath,
            formState: formState,
            formConfig: formConfig,
            validatorDelegate: validatorDelegate,
            actionsDelegate: actionsDelegate,
          ),
        ),
      ));
}
