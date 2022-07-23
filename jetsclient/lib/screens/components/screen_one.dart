import 'package:flutter/material.dart';
import 'package:jetsclient/screens/components/data_table.dart';
import 'package:jetsclient/screens/components/base_screen.dart';
import 'package:jetsclient/utils/data_table_config.dart';

class ScreenOne extends BaseScreen {
  ScreenOne({
    required super.key,
    required super.screenPath,
    required super.screenConfig,
    required this.tableConfig,
  }) : super(builder: (State<BaseScreen> baseState) {
          return JetsDataTableWidget(
              key: Key(screenConfig.key),
              screenPath: screenPath,
              tableConfig: tableConfig);
        });
  final TableConfig tableConfig;

  @override
  State<BaseScreen> createState() => ScreenOneState();
}

class ScreenOneState extends BaseScreenState {
  //* Empty class for now, might be removed
}
