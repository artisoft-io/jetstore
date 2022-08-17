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
          padding: EdgeInsets.fromLTRB(
              fieldConfig.leftMargin,
              fieldConfig.topMargin,
              fieldConfig.rightMargin,
              fieldConfig.bottomMargin),
          child: Text(
            fieldConfig.label,
            maxLines: fieldConfig.maxLines,
            style: themeData.textTheme.labelLarge?.copyWith(fontSize: 16),
          ),
        ));
  }
}
