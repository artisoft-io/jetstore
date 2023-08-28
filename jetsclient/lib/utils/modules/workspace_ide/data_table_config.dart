import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/utils/data_table_config.dart';


final Map<String, TableConfig> _tableConfigurations = {
  // Workspace Registry
  DTKeys.workspaceRegistryTable: TableConfig(
    key: DTKeys.workspaceRegistryTable,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'workspace_registry')
    ],
    label: 'Workspace Registry',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'addWorkspace',
          label: 'Add/Update Workspace',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null,
          configForm: FormKeys.addWorkspace,
          navigationParams: {
            FSK.key: 0,
            FSK.wsName: 1,
            FSK.wsURI: 2,
            FSK.description: 3,
          }),
      ActionConfig(
          actionType: DataTableActionType.doAction,
          key: 'openWorkspace',
          label: 'Open Workspace',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          actionName: ActionKeys.openWorkspace),
      ActionConfig(
          actionType: DataTableActionType.doAction,
          key: 'compileWorkspace',
          label: 'Compile Workspace',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          actionName: ActionKeys.compileWorkspace),
      ActionConfig(
          actionType: DataTableActionType.doAction,
          key: 'deleteWorkspace',
          label: 'Delete Workspace',
          style: ActionStyle.danger,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          // actionName: ActionKeys.deleteWorkspace
          ),
    ],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.key,
        columnIdx: 0,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.wsName,
        columnIdx: 1,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.wsURI,
        columnIdx: 2,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.description,
        columnIdx: 3,
      ),
    ]),
    columns: [
      ColumnConfig(
          index: 0,
          name: "key",
          label: 'Key',
          tooltips: '',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 1,
          name: "workspace_name",
          label: 'Workspace Name',
          tooltips: 'Workspace identifier',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "workspace_uri",
          label: 'Workspace Repo',
          tooltips: 'Workspace Repository Location',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "description",
          label: 'Description',
          tooltips: 'Workspace Repository Location',
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "user_email",
          label: 'User Email',
          tooltips: 'User who made the last change',
          isNumeric: false),
      ColumnConfig(
          index: 5,
          name: "last_update",
          label: 'Last Update',
          tooltips: 'Last time the workspace was compiled',
          isNumeric: false),
    ],
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 10,
  ),

  // Workspace Changes
  DTKeys.workspaceChangesTable: TableConfig(
    key: DTKeys.workspaceChangesTable,
    fromClauses: [
      FromClause(schemaName: 'jetsapi', tableName: 'workspace_changes')
    ],
    label: 'Workspace Registry',
    apiPath: '/dataTable',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: false,
    whereClauses: [
      WhereClause(column: "workspace_name", formStateKey: FSK.wsName),
    ],
    refreshOnKeyUpdateEvent: ['state_modified'],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.doAction,
          key: 'revertChanges',
          label: 'Delete/Revert Changes',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: true,
          actionName: ActionKeys.deleteWorkspaceChanges),
      ActionConfig(
          actionType: DataTableActionType.doAction,
          key: 'revertAllChanges',
          label: 'Delete/Revert ALL Changes',
          style: ActionStyle.danger,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null,
          actionName: ActionKeys.deleteAllWorkspaceChanges),
    ],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.key,
        columnIdx: 0,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.wsOid,
        columnIdx: 2,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.wsFileName,
        columnIdx: 3,
      ),
    ]),
    columns: [
      ColumnConfig(
          index: 0,
          name: "key",
          label: 'Key',
          tooltips: '',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 1,
          name: "workspace_name",
          label: 'Workspace Name',
          tooltips: 'Workspace identifier',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "oid",
          label: 'Change OID',
          tooltips: 'Database Large Object ID',
          isNumeric: true),
      ColumnConfig(
          index: 3,
          name: "file_name",
          label: 'File Name',
          tooltips: 'Workspace File Name',
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "content_type",
          label: 'Content Type',
          tooltips: 'Workspace Content Type',
          isNumeric: false),
      ColumnConfig(
          index: 5,
          name: "status",
          label: 'Change Status',
          tooltips: '',
          isNumeric: false),
      ColumnConfig(
          index: 6,
          name: "user_email",
          label: 'User Email',
          tooltips: 'User who made the last change',
          isNumeric: false),
      ColumnConfig(
          index: 7,
          name: "last_update",
          label: 'Last Update',
          tooltips: 'Last time the workspace was compiled',
          isNumeric: false),
    ],
    sortColumnName: 'last_update',
    sortAscending: false,
    rowsPerPage: 20,
  ),

  // Workspace - Data Model Tables
  // domain_classes table
  DTKeys.wsDomainClassTable: TableConfig(
    key: DTKeys.wsDomainClassTable,
    fromClauses: [
      FromClause(schemaName: "\$SCHEMA", tableName: 'domain_classes'),
      FromClause(schemaName: "\$SCHEMA", tableName: 'workspace_control'),
    ],
    label: 'Data Model',
    apiPath: '/dataTable',
    apiAction: 'workspace_read',
    isCheckboxVisible: false,
    isCheckboxSingleSelect: false,
    whereClauses: [
      WhereClause(column: "source_file_key", joinWith: "workspace_control.key"),
    ],
    actions: [ ],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.key,
        columnIdx: 0,
      ),
    ]),
    columns: [
      ColumnConfig(
          index: 0,
          name: "key",
          table: "domain_classes",
          label: 'Key',
          tooltips: '',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 1,
          name: "name",
          table: "domain_classes",
          label: 'Class Name',
          tooltips: 'Domain Class Name',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "as_table",
          table: "domain_classes",
          label: 'Persisted as Table?',
          tooltips: 'Boolean (1:true, 0:false) indicating if this Domain Class is converted into a Table',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "source_file_name",
          table: "workspace_control",
          label: 'Source File Name',
          tooltips: 'File containing the Class definition',
          isNumeric: false),
    ],
    sortColumnName: 'name',
    sortColumnTableName: 'domain_classes',
    sortAscending: true,
    rowsPerPage: 20,
  ),

  // Workspace - Data Model Tables
  // data_properties table
  DTKeys.wsDataPropertyTable: TableConfig(
    key: DTKeys.wsDataPropertyTable,
    fromClauses: [
      FromClause(schemaName: "\$SCHEMA", tableName: 'data_properties'),
      FromClause(schemaName: "\$SCHEMA", tableName: 'domain_classes'),
    ],
    label: 'Data Model',
    apiPath: '/dataTable',
    apiAction: 'workspace_read',
    isCheckboxVisible: false,
    isCheckboxSingleSelect: false,
    whereClauses: [
      WhereClause(column: "domain_class_key", joinWith: "domain_classes.key"),
    ],
    actions: [ ],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.key,
        columnIdx: 0,
      ),
    ]),
    columns: [
      ColumnConfig(
          index: 0,
          name: "key",
          table: "data_properties",
          label: 'Key',
          tooltips: '',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 1,
          name: "name",
          table: "domain_classes",
          label: 'Class Name',
          tooltips: 'Domain Class Name',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "name",
          table: "data_properties",
          label: 'Property Name',
          tooltips: 'Domain Property Name',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "type",
          table: "data_properties",
          label: 'Property Type',
          tooltips: 'Range type of the property',
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "as_array",
          table: "data_properties",
          label: 'Is Array?',
          tooltips: 'Is the property multi value?',
          isNumeric: false),
    ],
    sortColumnName: 'name',
    sortColumnTableName: 'domain_classes',
    sortAscending: true,
    rowsPerPage: 20,
  ),

  // Workspace - Data Model Tables
  // domain_tables/domain_columns table
  DTKeys.wsDomainTableTable: TableConfig(
    key: DTKeys.wsDomainTableTable,
    fromClauses: [
      FromClause(schemaName: "\$SCHEMA", tableName: 'domain_tables'),
      FromClause(schemaName: "\$SCHEMA", tableName: 'domain_columns'),
      FromClause(schemaName: "\$SCHEMA", tableName: 'data_properties'),
      FromClause(schemaName: "\$SCHEMA", tableName: 'domain_classes'),
    ],
    label: 'Data Model',
    apiPath: '/dataTable',
    apiAction: 'workspace_read',
    isCheckboxVisible: false,
    isCheckboxSingleSelect: false,
    whereClauses: [
      WhereClause(table: "domain_columns", column: "domain_table_key", joinWith: "domain_tables.key"),
      WhereClause(table: "domain_columns", column: "data_property_key", joinWith: "data_properties.key"),
      WhereClause(table: "data_properties", column: "domain_class_key", joinWith: "domain_classes.key"),
    ],
    actions: [ ],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.key,
        columnIdx: 0,
      ),
    ]),
    columns: [
      ColumnConfig(
          index: 0,
          name: "key",
          table: "domain_tables",
          label: 'Key',
          tooltips: '',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 1,
          name: "name",
          table: "domain_tables",
          label: 'Table Name',
          tooltips: 'Domain Table Name',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "name",
          table: "domain_columns",
          label: 'Column Name',
          tooltips: 'Domain Column Name',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "type",
          table: "data_properties",
          label: 'Data Type',
          tooltips: 'Column data type',
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "as_array",
          table: "domain_columns",
          label: 'Is Array?',
          tooltips: 'Is the column multi value?',
          isNumeric: false),
      ColumnConfig(
          index: 5,
          name: "name",
          table: "domain_classes",
          label: 'Origin Class Name',
          tooltips: 'Origin Domain Class Name of the Column',
          isNumeric: false),
    ],
    sortColumnName: 'name',
    sortColumnTableName: 'domain_tables',
    sortAscending: true,
    rowsPerPage: 20,
  ),

  // Workspace - Jet Rules Tables
  // jet_rules table
  DTKeys.wsJetRulesTable: TableConfig(
    key: DTKeys.wsJetRulesTable,
    fromClauses: [
      FromClause(schemaName: "\$SCHEMA", tableName: 'jet_rules'),
      FromClause(schemaName: "\$SCHEMA", tableName: 'workspace_control'),
    ],
    label: 'Data Model',
    apiPath: '/dataTable',
    apiAction: 'workspace_read',
    isCheckboxVisible: false,
    isCheckboxSingleSelect: false,
    dataRowMinHeight: 64,
    dataRowMaxHeight: 90,
    whereClauses: [
      WhereClause(table: "jet_rules", column: "source_file_key", joinWith: "workspace_control.key"),
    ],
    actions: [ ],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.key,
        columnIdx: 0,
      ),
    ]),
    columns: [
      ColumnConfig(
          index: 0,
          name: "key",
          table: "jet_rules",
          label: 'Key',
          tooltips: '',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 1,
          name: "name",
          table: "jet_rules",
          label: 'Rule Name',
          tooltips: 'Jet Rule Name',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "salience",
          table: "jet_rules",
          label: 'Rule Salience',
          tooltips: 'Jet Rule Salience',
          isNumeric: true),
      ColumnConfig(
          index: 3,
          name: "authored_label",
          table: "jet_rules",
          label: 'Jet Rule',
          tooltips: 'Jet Rule as Written',
          maxLines: 5,
          columnWidth: 900,
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "source_file_name",
          table: "workspace_control",
          label: 'Source File Name',
          tooltips: 'File containing the Jet Rule definition',
          isNumeric: false),
    ],
    sortColumnName: 'name',
    sortColumnTableName: 'jet_rules',
    sortAscending: true,
    rowsPerPage: 20,
  ),

  // Workspace - Jet Rules Tables
  // rule_terms table
  DTKeys.wsRuleTermsTable: TableConfig(
    key: DTKeys.wsRuleTermsTable,
    fromClauses: [
      FromClause(schemaName: "\$SCHEMA", tableName: 'rule_terms'),
      FromClause(schemaName: "\$SCHEMA", tableName: 'jet_rules'),
      FromClause(schemaName: "\$SCHEMA", tableName: 'rete_nodes'),
    ],
    label: 'Rule Terms',
    apiPath: '/dataTable',
    apiAction: 'workspace_read',
    isCheckboxVisible: false,
    isCheckboxSingleSelect: false,
    whereClauses: [
      WhereClause(table: "rule_terms", column: "rule_key", joinWith: "jet_rules.key"),
      WhereClause(table: "rule_terms", column: "rete_node_key", joinWith: "rete_nodes.key"),
    ],
    actions: [ ],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.key,
        columnIdx: 0,
      ),
    ]),
    columns: [
      ColumnConfig(
          index: 0,
          name: "name",
          table: "jet_rules",
          label: 'Rule Name',
          tooltips: 'Jet Rule Name',
          isNumeric: false),
      ColumnConfig(
          index: 1,
          name: "vertex",
          table: "rete_nodes",
          label: 'Rule Vertex',
          tooltips: 'Jet Rule Vertex',
          isNumeric: true),
      ColumnConfig(
          index: 2,
          name: "parent_vertex",
          table: "rete_nodes",
          label: 'Parent Rule Vertex',
          tooltips: 'Jet Rule Parent Vertex',
          isNumeric: true),
      ColumnConfig(
          index: 3,
          name: "normalized_label",
          table: "rete_nodes",
          label: 'Jet Rule Term',
          tooltips: 'Jet Rule Term using normalized label',
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "consequent_seq",
          table: "rete_nodes",
          label: 'Consequent Seq',
          tooltips: '0: Antecedent, 1+: Consequent',
          isNumeric: true),
    ],
    sortColumnName: 'name',
    sortColumnTableName: 'jet_rules',
    sortAscending: true,
    rowsPerPage: 20,
  ),

  // Workspace - Jet Rules Tables
  // main_support_files table
  DTKeys.wsMainSupportFilesTable: TableConfig(
    key: DTKeys.wsMainSupportFilesTable,
    fromClauses: [
      FromClause(schemaName: "\$SCHEMA", tableName: 'main_support_files'),
      FromClause(schemaName: "\$SCHEMA", tableName: 'workspace_control', asTableName: 'main_file'),
      FromClause(schemaName: "\$SCHEMA", tableName: 'workspace_control', asTableName: 'support_file'),
    ],
    label: 'Rule Terms',
    apiPath: '/dataTable',
    apiAction: 'workspace_read',
    isCheckboxVisible: false,
    isCheckboxSingleSelect: false,
    whereClauses: [
      WhereClause(table: "main_support_files", column: "main_file_key", joinWith: "main_file.key"),
      WhereClause(table: "main_support_files", column: "support_file_key", joinWith: "support_file.key"),
    ],
    actions: [ ],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.key,
        columnIdx: 0,
      ),
    ]),
    columns: [
      ColumnConfig(
          index: 0,
          name: "source_file_name",
          table: "main_file",
          label: 'Main Rule Module',
          tooltips: 'Main Rule Module',
          isNumeric: false,
          isHidden: false),
      ColumnConfig(
          index: 1,
          name: "source_file_name",
          table: "support_file",
          label: 'Jet Rule File',
          tooltips: 'Jet Rule File',
          isNumeric: false),
    ],
    sortColumnName: 'source_file_name',
    sortColumnTableName: 'main_file',
    sortAscending: true,
    rowsPerPage: 20,
  ),

};

TableConfig? getWorkspaceTableConfig(String key) {
  var config = _tableConfigurations[key];
  return config;
}
