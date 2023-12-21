import 'package:flutter_typeahead/flutter_typeahead.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/modules/actions/delegate_helpers.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/form_config.dart';

class JetsTypeaheadFormField extends StatefulWidget {
  const JetsTypeaheadFormField({
    required super.key,
    required this.formFieldConfig,
    required this.onChanged,
    required this.formValidator,
    required this.formState,
  });
  final FormTypeaheadFieldConfig formFieldConfig;
  final void Function(String) onChanged;
  final JetsFormFieldValidator formValidator;
  final JetsFormState formState;

  @override
  State<JetsTypeaheadFormField> createState() => _JetsTypeaheadFormFieldState();
}

class _JetsTypeaheadFormFieldState extends State<JetsTypeaheadFormField> {
  late final TextEditingController _controller;

  late FocusNode _node;
  bool _focused = false;
  FormTypeaheadFieldConfig get formConfig => widget.formFieldConfig;
  FormInputFieldConfig get _config => widget.formFieldConfig.inputFieldConfig;

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
    var value = unpack(widget.formState.getValue(_config.group, _config.key));
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
        child: TypeAheadField<String>(
          controller: _controller,
          builder: (context, controller, focusNode) => TextFormField(
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
            validator: (p0) =>
                widget.formValidator(_config.group, _config.key, p0),
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

          // builder: (context, controller, focusNode) => TextField(
          //   controller: controller,
          //   focusNode: focusNode,
          //   autofocus: true,
          //   style: DefaultTextStyle.of(context)
          //       .style
          //       .copyWith(fontStyle: FontStyle.italic),
          //   decoration: InputDecoration(
          //     border: const OutlineInputBorder(),
          //     hintText: hintText,
          //   ),
          // ),

          decorationBuilder: (context, child) => Material(
            type: MaterialType.card,
            elevation: 4,
            borderRadius: BorderRadius.circular(10),
            child: child,
          ),
          itemBuilder: (context, item) => Text(item),
          // debounceDuration: debounceDuration,
          // hideOnSelect: settings.hideOnSelect.value,
          // hideOnUnfocus: settings.hideOnUnfocus.value,
          // hideWithKeyboard: settings.hideOnUnfocus.value,
          // retainOnLoading: settings.retainOnLoading.value,
          onSelected: (item) => _controller.clear(),
          suggestionsCallback: suggestionsCallback,
          // itemSeparatorBuilder: itemSeparatorBuilder,
          // listBuilder:
          //     settings.gridLayout.value ? gridLayoutBuilder : null,
        )
        // child: TextFormField(
        //   autofocus: _config.autofocus,
        //   controller: _controller,
        //   focusNode: _node,
        //   showCursor: _focused,
        //   obscureText: _config.obscureText,
        //   readOnly: _config.isReadOnlyEval != null
        //       ? _config.isReadOnlyEval!(widget.formState)
        //       : _config.isReadOnly,
        //   maxLines: _config.maxLines,
        //   maxLength: _config.maxLength,
        //   maxLengthEnforcement: MaxLengthEnforcement.enforced,
        //   decoration: InputDecoration(
        //     hintText: _config.hint,
        //     labelText: _config.label,
        //   ),
        //   onChanged: widget.onChanged,
        //   validator: (p0) => widget.formValidator(_config.group, _config.key, p0),
        //   autovalidateMode: _config.autovalidateMode,
        //   autofillHints: _config.autofillHints,
        //   style: _config.useDefaultFont
        //       ? null
        //       : const TextStyle(
        //           fontFamily: 'Victor Mono',
        //           fontWeight: FontWeight.w500,
        //           fontFeatures: [FontFeature.tabularFigures()],
        //         ),
        // ),
        );
  }

  bool doesMatch(String item, String pattern) {
    final itemLower = item.toLowerCase().split(' ').join('');
    final patternLower = pattern.toLowerCase().split(' ').join('');
    final result = itemLower.contains(patternLower);
    print("doesMatch item $item pattern $pattern result $result");
    return result;
  }

  Future<List<String>> suggestionsCallback(String pattern) async =>
      Future<List<String>>.delayed(
        const Duration(milliseconds: 50),
        () => widget.formState
            .getCacheValue(formConfig.typeaheadMenuItemCacheKey)
            .where((item) => doesMatch(item, pattern))
            .toList(),
      );
}
