import 'package:flutter/material.dart';
import 'package:jetsclient/routes/jets_route_data.dart';
import 'package:jetsclient/screens/components/form.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/screens/components/spinner_overlay.dart';
import 'package:jetsclient/utils/constants.dart';
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

/// Danger Zone alert dialog, requesting user confirmation to proceed
Future<String?> showDangerZoneDialog(
    BuildContext context, String message) async {
  final ThemeData td = Theme.of(context);
  return showDialog<String?>(
    context: context,
    barrierDismissible: false,
    builder: (context) => AlertDialog(
      title: const Text('Danger Zone'),
      content: Text(message),
      titleTextStyle: TextStyle(color: td.colorScheme.onErrorContainer),
      contentTextStyle: TextStyle(color: td.colorScheme.onErrorContainer),
      backgroundColor: td.colorScheme.errorContainer,
      actions: [
        TextButton(
          child: const Text('CANCEL'),
          onPressed: () => Navigator.of(context).pop('CANCEL'),
        ),
        TextButton(
          child: const Text('OK'),
          onPressed: () => Navigator.of(context).pop('OK'),
        ),
      ],
    ),
  );
}

/// Danger Zone alert dialog, requesting user confirmation to proceed
Future<String?> showConfirmationDialog(
    BuildContext context, String message) async {
  final ThemeData td = Theme.of(context);
  return showDialog<String?>(
    context: context,
    barrierDismissible: false,
    builder: (context) => AlertDialog(
      title: const Text('Please confirm'),
      content: Text(message),
      titleTextStyle: TextStyle(color: td.colorScheme.onPrimaryContainer),
      contentTextStyle: TextStyle(color: td.colorScheme.onPrimaryContainer),
      backgroundColor: td.colorScheme.primaryContainer,
      actions: [
        TextButton(
          child: const Text('CANCEL'),
          onPressed: () => Navigator.of(context).pop('CANCEL'),
        ),
        TextButton(
          child: const Text('OK'),
          onPressed: () => Navigator.of(context).pop('OK'),
        ),
      ],
    ),
  );
}

/// Enum to record the action result for data table actions
/// Used as the template type for DialogResultHandler when used
/// with data table actions
/// statusError is to indicate an error was return, formState.serverError
/// have a message to show the user
enum DTActionResult { canceled, ok, okDataTableDirty, statusError }

typedef DialogResultHandler<T> = void Function(
    BuildContext context, JetsFormState dialogFormState, T? t);

Future<void> showFormDialog<T>(
    {required GlobalKey<FormState> formKey,
    required JetsRouteData screenPath,
    required BuildContext context,
    required JetsFormState formState,
    required FormConfig formConfig,
    required DialogResultHandler<T> resultHandler}) async {
  resultHandler(
      context,
      formState,
      await showDialog<T>(
        context: context,
        builder: (context) => Dialog(
            child:
                Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Flexible(
            flex: 1,
            fit: FlexFit.tight,
            child: Padding(
              padding: const EdgeInsets.fromLTRB(
                  defaultPadding, 2 * defaultPadding, 0, 0),
              child: Text(
                formConfig.title ?? "",
                style: Theme.of(context).textTheme.headlineMedium,
              ),
            ),
          ),
          Flexible(
              flex: 8,
              fit: FlexFit.tight,
              child: JetsSpinnerOverlay(
                child: JetsForm(
                  formKey: formKey,
                  formPath: screenPath,
                  formState: formState,
                  formConfig: formConfig,
                  isDialog: true,
                ),
              )),
        ])),
      ));
}
