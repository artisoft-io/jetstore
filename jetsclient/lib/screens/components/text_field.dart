import 'package:flutter/material.dart';
import 'package:jetsclient/utils/form_config.dart';

class JetsTextField extends StatelessWidget {
  const JetsTextField({
    Key? key,
    required this.fieldConfig,
    this.flex = 1,
  }) : super(key: key);
  final TextFieldConfig fieldConfig;
  final int flex;

  @override
  Widget build(BuildContext context) {
    final ThemeData themeData = Theme.of(context);
    return Expanded(
        flex: flex,
        child: Padding(
          padding: const EdgeInsets.fromLTRB(16.0, 0.0, 16.0, 0.0),
          child: Text(
            fieldConfig.label,
            style: themeData.textTheme.labelLarge?.copyWith(fontSize: 16),
          ),
        ));
  }
}
