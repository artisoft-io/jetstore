import 'package:flutter/material.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/utils/form_config.dart';

class JetsFormButton extends StatefulWidget {
  const JetsFormButton({
    required super.key,
    required this.formActionConfig,
    required this.formKey,
    required this.formState,
    required this.actionsDelegate,
  });

  final FormActionConfig formActionConfig;
  final GlobalKey<FormState> formKey;
  final JetsFormState formState;
  final FormActionsDelegate actionsDelegate;

  @override
  State<JetsFormButton> createState() => _JetsFormButtonState();
}

class _JetsFormButtonState extends State<JetsFormButton> {
  @override
  void initState() {
    super.initState();
    if (widget.formActionConfig.enableOnlyWhenFormValid) {
      widget.formState.addListener(_handleStateChange);
    }
  }

  void _handleStateChange() {
    Future.delayed(Duration.zero, () {
      if (!mounted) return;
      setState(() {});
    });
  }

  @override
  void dispose() {
    if (widget.formActionConfig.enableOnlyWhenFormValid) {
      widget.formState.removeListener(_handleStateChange);
    }
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final themeData = Theme.of(context);
    return ElevatedButton(
        style: ElevatedButton.styleFrom(
          // Foreground color
          foregroundColor: themeData.colorScheme.onPrimary,
          backgroundColor: themeData.colorScheme.primary,
        ).copyWith(elevation: ButtonStyleButton.allOrNull(0.0)),
        onPressed: !widget.formActionConfig.enableOnlyWhenFormValid ||
                widget.formState.isFormValid()
            ? () => widget.actionsDelegate(context, widget.formKey,
                widget.formState, widget.formActionConfig.key)
            : null,
        child: Text(widget.formActionConfig.label));
  }
}
