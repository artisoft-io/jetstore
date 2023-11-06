import 'dart:ui';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/utils/form_config.dart';

class JetsTextFormField extends StatefulWidget {
  const JetsTextFormField({
    required super.key,
    required this.formFieldConfig,
    required this.onChanged,
    required this.formValidator,
    required this.formState,
  });
  final FormInputFieldConfig formFieldConfig;
  final void Function(String) onChanged;
  // Note: formValidator is require as this control needs to be part of a form
  //       so to have formFieldConfig. We need to externalize the widget
  //       config (as done for data table) to to be able to use the widget
  //       without a form. Same applies to dropdown widget.
  final JetsFormFieldValidator formValidator;
  final JetsFormState formState;

  @override
  State<JetsTextFormField> createState() => _JetsTextFormFieldState();
}

class _JetsTextFormFieldState extends State<JetsTextFormField> {
  late final TextEditingController _controller;

  late FocusNode _node;
  bool _focused = false;
  late final FormInputFieldConfig _config;

  void _handleFocusChange() {
    if (_node.hasFocus != _focused) {
      setState(() {
        _focused = _node.hasFocus;
      });
    }
  }

  void _controllerListener() {
    if (_config.textRestriction == TextRestriction.allLower) {
      final String text = _controller.text.toLowerCase();
      if (text != _controller.text) {
        _controller.value = _controller.value.copyWith(
          text: text,
          selection:
              TextSelection(baseOffset: text.length, extentOffset: text.length),
          composing: TextRange.empty,
        );
      }
    } else if (_config.textRestriction == TextRestriction.allUpper) {
      final String text = _controller.text.toUpperCase();
      if (text != _controller.text) {
        _controller.value = _controller.value.copyWith(
          text: text,
          selection:
              TextSelection(baseOffset: text.length, extentOffset: text.length),
          composing: TextRange.empty,
        );
      }
    } else if (_config.textRestriction == TextRestriction.digitsOnly) {
      final buf = StringBuffer();
      final re = RegExp(r'[^0-9]');
      for (var c in _controller.text.characters) {
        if (!c.contains(re)) {
          buf.write(c);
        }
      }
      final String text = buf.toString();
      if (text != _controller.text) {
        _controller.value = _controller.value.copyWith(
          text: text,
          selection:
              TextSelection(baseOffset: text.length, extentOffset: text.length),
          composing: TextRange.empty,
        );
      }
    }
    // debugPrint("Controller listener called for ${_config.key}");
  }

  @override
  void initState() {
    super.initState();
    _config = widget.formFieldConfig;
    var value = widget.formState.getValue(_config.group, _config.key);
    if (value == null) {
      if (_config.key == FSK.wsURI) {
        value = globalWorkspaceUri;
      } else {
        value = _config.defaultValue;
      }
      widget.formState.setValue(_config.group, _config.key, value);
    }
    _controller = TextEditingController(text: value);
    _controller.addListener(_controllerListener);
    _node = FocusNode(debugLabel: _config.key);
    _node.addListener(_handleFocusChange);
  }

  @override
  void dispose() {
    // print("*** InputText dispose called for ${_config.key}");
    _controller.removeListener(_controllerListener);
    _controller.dispose();
    _node.removeListener(_handleFocusChange);
    _node.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16.0, 0.0, 16.0, 0.0),
      child: TextFormField(
        autofocus: _config.autofocus,
        controller: _controller,
        focusNode: _node,
        showCursor: _focused,
        obscureText: _config.obscureText,
        readOnly: _config.isReadOnlyEval != null
            ? _config.isReadOnlyEval!(widget.formState)
            : _config.isReadOnly,
        maxLines: _config.maxLines,
        maxLength: _config.maxLength,
        maxLengthEnforcement: MaxLengthEnforcement.enforced,
        decoration: InputDecoration(
          hintText: _config.hint,
          labelText: _config.label,
        ),
        onChanged: widget.onChanged,
        validator: (p0) => widget.formValidator(_config.group, _config.key, p0),
        autovalidateMode: _config.autovalidateMode,
        autofillHints: _config.autofillHints,
        style: _config.useDefaultFont
            ? null
            : const TextStyle(
                fontFamily: 'Victor Mono',
                fontWeight: FontWeight.w500,
                fontFeatures: [FontFeature.tabularFigures()],
              ),
      ),
    );
  }
}
