import 'package:jetsclient/screens/components/data_table.dart';
import 'package:jetsclient/utils/constants.dart';

/// Data Table Configuration class
/// [refreshOnKeyUpdateEvent] contains list of key that will trigger a table
/// refresh, used when underlying table is updated independently of this table.
class TableConfig {
  TableConfig(
      {required this.key,
      this.label = "",
      required this.apiPath,
      this.apiAction = "read",
      this.modelStateFormKey,
      required this.isCheckboxVisible,
      required this.isCheckboxSingleSelect,
      required this.actions,
      required this.columns,
      this.defaultToAllRows = false,
      required this.fromClauses,
      required this.whereClauses,
      this.distinctOnClauses = const [],
      this.refreshOnKeyUpdateEvent = const [],
      this.formStateConfig,
      required this.sortColumnName,
      this.sortColumnTableName = '',
      required this.sortAscending,
      required this.rowsPerPage,
      this.withClauses = const [],
      this.sqlQuery,
      this.requestColumnDef = false,
      this.noFooter = false,
      this.dataRowMinHeight,
      this.dataRowMaxHeight});
  final String key;
  final String label;
  final String apiPath;
  final String apiAction;
  final String? modelStateFormKey;
  final bool isCheckboxVisible;
  final bool isCheckboxSingleSelect;
  final List<ActionConfig> actions;
  final List<ColumnConfig> columns;
  final bool defaultToAllRows;
  final RawQuery? sqlQuery;
  final bool requestColumnDef;
  final List<WithClause> withClauses;
  final List<FromClause> fromClauses;
  final List<WhereClause> whereClauses;
  final List<String> distinctOnClauses;
  final List<String> refreshOnKeyUpdateEvent;
  final DataTableFormStateConfig? formStateConfig;
  final String sortColumnName;
  final String sortColumnTableName;
  final bool sortAscending;
  final int rowsPerPage;
  final bool noFooter;
  final double? dataRowMinHeight;
  final double? dataRowMaxHeight;
}

/// enum describing the type of actions that are available to data table
enum DataTableActionType {
  showDialog,
  showScreen,
  toggleCheckboxVisible,
  makeSelectedRowsEditable,
  saveDirtyRows,
  deleteSelectedRows,
  cancelModifications,
  refreshTable,
  doAction,
  doActionShowDialog
}

/// Table Action Configuration
/// case isVisibleWhenCheckboxVisible is null, action always visible
/// case isVisibleWhenCheckboxVisible == false, action visible when table check boxes are NOT visible
/// case isVisibleWhenCheckboxVisible == true, action visible when table check boxes ARE visble
///
/// case isEnabledWhenHavingSelectedRows is null, action always enable when visible (unless other conditions exist)
/// case isEnabledWhenHavingSelectedRows == false, action always enabled when table check boxes are visible
/// case isEnabledWhenHavingSelectedRows == true, action enabled when table HAVE selected row(s)
///
/// case isEnabledWhenWhereClauseSatisfied is null, action always enabled when visible (unless other conditions exists)
/// case isEnabledWhenWhereClauseSatisfied == false, action enabled when where clause fails (perhaps to have a 'show all rows' option)
/// case isEnabledWhenWhereClauseSatisfied == true, action enabled when where clause exists and is satisfied
///
/// case isEnabledWhenStateHasKeys is null, action always enabled when visible (unless other conditions exists)
/// case isEnabledWhenStateHasKeys != null, action enabled when dataTable state has all keys in isEnabledWhenStateHasKeys defined
///
/// [navigationParams] hold param information for:
///   - navigating to a screen (action type showScreen) with key corresponding
///     to the key to provide to navigator's param
///   - navigating to a dialog (action type showDialog) with key corresponding
///     to the key to provide dialog form state
///   - key correspond to the key to provide to navigator's param
/// The value associated to the [navigationParam]'s key correspond to a column
/// index to take the associated value of the selected row.
/// Note: if the value is a String (rather than an int), then use it as the value to pass to the navigator.
/// [stateFormNavigationParams] is similar to [navigationParams] with the difference
/// that the value are string corresponding to state form keys.
/// Therefore the navigation params' values are resolved by looking up the value
/// in the state form.
/// (see data table state method [actionDispatcher])
/// actionName is used for DataTableActionType.doAction, corresponding to the action
/// to invoke when the ActionConfig button is pressed
class ActionConfig {
  ActionConfig(
      {required this.actionType,
      required this.key,
      required this.label,
      this.isVisibleWhenCheckboxVisible,
      this.isEnabledWhenHavingSelectedRows,
      this.isEnabledWhenWhereClauseSatisfied,
      this.isEnabledWhenStateHasKeys,
      this.navigationParams,
      this.stateFormNavigationParams,
      required this.style,
      this.configForm,
      this.configScreenPath,
      this.actionName,
      this.stateGroup = 0});
  final DataTableActionType actionType;
  final String key;
  final String label;
  final bool? isVisibleWhenCheckboxVisible;
  final bool? isEnabledWhenHavingSelectedRows;
  final bool? isEnabledWhenWhereClauseSatisfied;
  final List<String>? isEnabledWhenStateHasKeys;
  final Map<String, dynamic>? navigationParams;
  final Map<String, String>? stateFormNavigationParams;
  final ActionStyle style;
  final String? configForm;
  final String? configScreenPath;
  final String? actionName;
  final int stateGroup;

  /// returns true if action button is visible
  bool isVisible(JetsDataTableState widgetState) {
    if (isVisibleWhenCheckboxVisible != null) {
      return isVisibleWhenCheckboxVisible == widgetState.isTableEditable;
    }
    return true;
  }

  /// returns true if action button is enabled
  bool isEnabled(JetsDataTableState widgetState) {
    if (isEnabledWhenHavingSelectedRows != null) {
      return isEnabledWhenHavingSelectedRows ==
          widgetState.dataSource.hasSelectedRows();
    }
    if (isEnabledWhenWhereClauseSatisfied != null) {
      return isEnabledWhenWhereClauseSatisfied ==
          widgetState.dataSource.isWhereClauseSatisfiedOrDefaultToAllRows;
    }
    if (isEnabledWhenStateHasKeys != null) {
      return widgetState.dataSource
          .stateHasKeys(stateGroup, isEnabledWhenStateHasKeys!);
    }
    return true;
  }
}

class ColumnConfig {
  ColumnConfig({
    required this.index,
    this.table,
    required this.name,
    required this.label,
    required this.tooltips,
    required this.isNumeric,
    this.isHidden = false,
    this.maxLines = 0,
    this.columnWidth = 0,
  });
  final int index;
  final String? table;
  final String name;
  final String label;
  final String tooltips;
  final bool isNumeric;
  final bool isHidden;
  final int maxLines;
  final double columnWidth;
}

class WithClause {
  WithClause({
    required this.withName,
    required this.asStatement,
    this.stateVariables = const [],
  });
  final String withName;
  final String asStatement;
  // asStatement contains substituion like this {{stateVariableName}}
  // with is substituted with the variable value from the Form State object
  // It's an error to have a stateless form with a table containing WithClause
  // with state variables.
  final List<String> stateVariables;
}

class RawQuery {
  RawQuery({
    required this.sqlQuery,
    this.stateVariables = const [],
  });
  final String sqlQuery;
  // asStatement contains substituion like this {{stateVariableName}}
  // with is substituted with the variable value from the Form State object
  // It's an error to have a stateless form with a table containing WithClause
  // with state variables.
  final List<String> stateVariables;
}

class FromClause {
  FromClause({
    required this.schemaName,
    required this.tableName,
    this.asTableName = '',
  });
  final String schemaName;
  final String tableName;
  final String asTableName;
}

class FormStatePredicate {
  FormStatePredicate({required this.formStateKey, this.expectedValue});
  final String formStateKey;
  final String? expectedValue;
}

class WhereClause {
  WhereClause({
    this.table,
    required this.column,
    this.formStateKey,
    this.defaultValue = const [],
    this.joinWith,
    this.predicate,
    this.lookupColumnInFormState = false,
  });
  final String? table;
  final String column;
  final String? formStateKey;
  final List<String> defaultValue;
  final String? joinWith;
  final FormStatePredicate? predicate;
  final bool lookupColumnInFormState;
}

class DataTableFormStateConfig {
  DataTableFormStateConfig(
      {required this.keyColumnIdx, required this.otherColumns});
  final int keyColumnIdx;
  final List<DataTableFormStateOtherColumnConfig> otherColumns;
}

class DataTableFormStateOtherColumnConfig {
  DataTableFormStateOtherColumnConfig({
    required this.stateKey,
    required this.columnIdx,
  });
  final String stateKey;
  final int columnIdx;
}
