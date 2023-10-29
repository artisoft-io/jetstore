import 'package:flutter/material.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/utils/constants.dart';
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
  late ActionStyle _buttonStyle;

  JetsFormState get formState => widget.formState;
  FormActionConfig get config => widget.formActionConfig;

  @override
  void initState() {
    super.initState();
    _buttonStyle = config.buttonStyle;
    formState.setValue(config.group, config.key, _buttonStyle);
    formState.addListener(_handleStateChange);
  }

  String? get label {
    if (widget.formActionConfig.label.isNotEmpty) {
      return widget.formActionConfig.label;
    }
    return widget.formActionConfig.labelByStyle[_buttonStyle];
  }

  void _handleStateChange() {
    Future.delayed(Duration.zero, () {
      if (!mounted) return;
      setState(() {
        _buttonStyle = formState.getValue(config.group, config.key);
      });
    });
  }

  @override
  void dispose() {
    formState.removeListener(_handleStateChange);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final themeData = Theme.of(context);
    var isOn = false;
    final capability = widget.formActionConfig.capability;
    if (JetsRouterDelegate().user.isAdmin ||
        capability == null ||
        JetsRouterDelegate().user.hasCapability(capability)) {
      if (widget.formActionConfig.enableOnlyWhenFormValid) {
        isOn = widget.formState.isFormValid();
      } else if (widget.formActionConfig.enableOnlyWhenFormNotValid) {
        isOn = !widget.formState.isFormValid();
      } else {
        isOn = !widget.formActionConfig.enableOnlyWhenFormValid ||
            widget.formState.isFormValid();
      }
    }
    return Expanded(
      flex: widget.formActionConfig.flex,
      child: Padding(
        padding: EdgeInsets.fromLTRB(
            widget.formActionConfig.leftMargin,
            widget.formActionConfig.topMargin,
            widget.formActionConfig.rightMargin,
            widget.formActionConfig.bottomMargin),
        child: ElevatedButton(
            style: buttonStyle(_buttonStyle, themeData),
            onPressed: isOn
                ? () => widget.actionsDelegate(
                    context, widget.formKey, formState, config.key,
                    group: config.group)
                : null,
            child: Text(label ?? '')),
      ),
    );
  }
}
