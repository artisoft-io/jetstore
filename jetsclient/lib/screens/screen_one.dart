import 'package:flutter/material.dart';
import 'package:jetsclient/components/data_table.dart';
import 'package:jetsclient/screens/base_screen.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/models/data_table_config.dart';
import 'package:jetsclient/models/form_config.dart';
import 'package:jetsclient/utils/constants.dart';

class ScreenOne extends BaseScreen {
  ScreenOne({
    required super.key,
    required super.screenPath,
    required super.screenConfig,
    required this.tableConfig,
    required this.validatorDelegate,
    required this.actionsDelegate,
  }) : super(builder: (BuildContext context, State<BaseScreen> baseState) {
          return Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (screenConfig.title != null)
                Flexible(
                  flex: 1,
                  fit: FlexFit.tight,
                  child: Padding(
                    padding: const EdgeInsets.fromLTRB(
                        defaultPadding, 2 * defaultPadding, 0, 0),
                    child: Text(
                      screenConfig.title!,
                      style: Theme.of(context).textTheme.headlineMedium,
                    ),
                  ),
                ),
                Flexible(
                  flex: 8,
                  fit: FlexFit.tight,
                  child: JetsDataTableWidget(
                      key: Key(screenConfig.key),
                      screenPath: screenPath,
                      tableConfig: tableConfig,
                      validatorDelegate: validatorDelegate,
                      actionsDelegate: actionsDelegate),
                ),
              ]);
        });
  final TableConfig tableConfig;
  final ValidatorDelegate validatorDelegate;
  final FormActionsDelegate actionsDelegate;

  @override
  State<BaseScreen> createState() => ScreenOneState();
}

class ScreenOneState extends BaseScreenState {
  //* Empty class for now, might be removed
}
