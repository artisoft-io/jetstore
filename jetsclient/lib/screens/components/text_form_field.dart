import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:jetsclient/utils/form_config.dart';

class JetsTextFormField extends StatefulWidget {
  const JetsTextFormField(
      {Key? key,
      required this.inputFieldConfig,
      required this.onChanged,
      required this.validatorDelegate})
      : super(key: key);
  final FormInputFieldConfig inputFieldConfig;
  final void Function(String) onChanged;
  final String? Function(String, String?) validatorDelegate;

  @override
  State<JetsTextFormField> createState() => _JetsTextFormFieldState();
}

class _JetsTextFormFieldState extends State<JetsTextFormField> {
  final TextEditingController _controller = TextEditingController();

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

  @override
  void initState() {
    super.initState();
    _controller.addListener(() {
      if (_config.textRestriction == TextRestriction.allLower) {
        final String text = _controller.text.toLowerCase();
        if (text != _controller.text) {
          _controller.value = _controller.value.copyWith(
            text: text,
            selection: TextSelection(
                baseOffset: text.length, extentOffset: text.length),
            composing: TextRange.empty,
          );
        }
      } else if (_config.textRestriction == TextRestriction.allUpper) {
        final String text = _controller.text.toUpperCase();
        if (text != _controller.text) {
          _controller.value = _controller.value.copyWith(
            text: text,
            selection: TextSelection(
                baseOffset: text.length, extentOffset: text.length),
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
        if(text != _controller.text) {
          _controller.value = _controller.value.copyWith(
            text: text,
            selection:
              TextSelection(baseOffset: text.length, extentOffset: text.length),
            composing: TextRange.empty,
          );
        }
      }
      // debugPrint("Controller listener called for ${_config.key}");
    });
    _config = widget.inputFieldConfig;
    _node = FocusNode(debugLabel: _config.key);
    _node.addListener(_handleFocusChange);
  }

  @override
  void dispose() {
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
        maxLength: _config.maxLength,
        maxLengthEnforcement: MaxLengthEnforcement.enforced,
        decoration: InputDecoration(
          hintText: _config.hint,
          labelText: _config.label,
        ),
        onChanged: widget.onChanged,
        validator: (String? value) =>
            widget.validatorDelegate(_config.key, value),
      ),
    );
  }
}
