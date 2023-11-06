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
    apiAction: 'workspace_read',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: true,
    whereClauses: [],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'addWorkspace',
          label: 'Add/Update',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null,
          configForm: FormKeys.addWorkspace,
          navigationParams: {
            FSK.key: 0,
            FSK.wsName: 1,
            FSK.wsBranch: 2,
            FSK.wsFeatureBranch: 3,
            FSK.wsURI: 4,
            FSK.description: 5,
          }),
      ActionConfig(
          actionType: DataTableActionType.doAction,
          key: 'openWorkspace',
          label: 'Open',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          actionEnableCriterias: [[
            ActionEnableCriteria(
                columnPos: 6,
                criteriaType: DataTableActionEnableCriteria.doesNotContain,
                value: 'removed'),
            ActionEnableCriteria(
                columnPos: 6,
                criteriaType: DataTableActionEnableCriteria.doesNotContain,
                value: 'in progress'),
          ]],
          actionName: ActionKeys.openWorkspace),
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'exportWorkspaceClientConfig',
          label: 'Export Client Config',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          actionEnableCriterias: [[
            ActionEnableCriteria(
                columnPos: 6,
                criteriaType: DataTableActionEnableCriteria.doesNotContain,
                value: 'removed'),
            ActionEnableCriteria(
                columnPos: 6,
                criteriaType: DataTableActionEnableCriteria.doesNotContain,
                value: 'in progress'),
          ]],
          configForm: FormKeys.exportWorkspaceClientConfig,
          navigationParams: {
            FSK.key: 0,
            FSK.wsName: 1,
            FSK.wsBranch: 2,
            FSK.wsFeatureBranch: 3,
            FSK.wsURI: 4,
          }),
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'unitTest',
          label: 'Unit Test',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          navigationParams: {
            FSK.dataTableAction: "workspace_insert_rows",
            FSK.dataTableFromTable: "unit_test",
            FSK.wsName: 1,
            FSK.wsBranch: 2,
            FSK.wsFeatureBranch: 3,
            FSK.wsURI: 4,
          },
          configForm: FormKeys.startPipeline),
      ActionConfig(
          actionType: DataTableActionType.doAction,
          key: 'loadWorkspaceConfig',
          label: 'Load Config',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          actionEnableCriterias: [[
            ActionEnableCriteria(
                columnPos: 6,
                criteriaType: DataTableActionEnableCriteria.doesNotContain,
                value: 'in progress'),
          ]],
          actionName: ActionKeys.loadWorkspaceConfig),
      ActionConfig(
          actionType: DataTableActionType.doAction,
          key: 'deleteWorkspace',
          label: 'Delete',
          style: ActionStyle.danger,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          actionEnableCriterias: [[
            ActionEnableCriteria(
                columnPos: 6,
                criteriaType: DataTableActionEnableCriteria.doesNotContain,
                value: 'active'),
            ActionEnableCriteria(
                columnPos: 6,
                criteriaType: DataTableActionEnableCriteria.doesNotContain,
                value: 'in progress'),
          ]],
          actionName: ActionKeys.deleteWorkspace),
    ],
    secondRowActions: [
      ActionConfig(
          actionType: DataTableActionType.doAction,
          key: 'compileWorkspace',
          label: 'Compile Workspace',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          actionEnableCriterias: [[
            ActionEnableCriteria(
                columnPos: 6,
                criteriaType: DataTableActionEnableCriteria.doesNotContain,
                value: 'in progress'),
          ]],
          actionName: ActionKeys.compileWorkspace),
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'commitWorkspace',
          label: 'Commit & Push Workspace',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          actionEnableCriterias: [[
            ActionEnableCriteria(
                columnPos: 6,
                criteriaType: DataTableActionEnableCriteria.contains,
                value: 'modified'),
          ]],
          configForm: FormKeys.commitWorkspace,
          navigationParams: {
            FSK.key: 0,
            FSK.wsName: 1,
            FSK.wsBranch: 2,
            FSK.wsFeatureBranch: 3,
            FSK.wsURI: 4,
          }),
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'pushOnlyWorkspace',
          label: 'Push Only',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          actionEnableCriterias: [[
            ActionEnableCriteria(
                columnPos: 6,
                criteriaType: DataTableActionEnableCriteria.doesNotContain,
                value: 'removed'),
          ]],
          configForm: FormKeys.pushOnlyWorkspace,
          navigationParams: {
            FSK.key: 0,
            FSK.wsName: 1,
            FSK.wsBranch: 2,
            FSK.wsFeatureBranch: 3,
            FSK.wsURI: 4,
          }),
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'pullWorkspace',
          label: 'Pull Workspace',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          actionEnableCriterias: [[
            ActionEnableCriteria(
                columnPos: 6,
                criteriaType: DataTableActionEnableCriteria.doesNotContain,
                value: 'removed'),
            ActionEnableCriteria(
                columnPos: 6,
                criteriaType: DataTableActionEnableCriteria.doesNotContain,
                value: 'in progress'),
          ]],
          configForm: FormKeys.pullWorkspace,
          navigationParams: {
            FSK.key: 0,
            FSK.wsName: 1,
            FSK.wsBranch: 2,
            FSK.wsFeatureBranch: 3,
            FSK.wsURI: 4,
          }),
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'doGitStatus',
          label: 'Git Status',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          configForm: FormKeys.doGitStatusWorkspace,
          navigationParams: {
            FSK.key: 0,
            FSK.wsName: 1,
            FSK.wsBranch: 2,
            FSK.wsFeatureBranch: 3,
            FSK.wsURI: 4,
          }),
      // ActionConfig(
      //     actionType: DataTableActionType.showDialog,
      //     key: 'doGitCommand',
      //     label: 'Git Command',
      //     style: ActionStyle.secondary,
      //     isVisibleWhenCheckboxVisible: true,
      //     isEnabledWhenHavingSelectedRows: true,
      //     configForm: FormKeys.doGitCommandWorkspace,
      //     navigationParams: {
      //       FSK.key: 0,
            // FSK.wsName: 1,
            // FSK.wsBranch: 2,
            // FSK.wsFeatureBranch: 3,
            // FSK.wsURI: 4,
      //     }),
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'viewGitLogWorkspace',
          label: 'View Last Log',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          configForm: FormKeys.viewGitLogWorkspace,
          navigationParams: {
            FSK.key: 0,
            FSK.wsName: 1,
            FSK.wsBranch: 2,
            FSK.wsFeatureBranch: 3,
            FSK.wsURI: 4,
            FSK.lastGitLog: 7,
          }),
      ActionConfig(
          actionType: DataTableActionType.refreshTable,
          key: 'refreshTable',
          label: 'Refresh',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null),
    ],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.key,
        columnIdx: 0,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.wsPreviousName,
        columnIdx: 1,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.wsName,
        columnIdx: 1,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.wsBranch,
        columnIdx: 2,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.wsFeatureBranch,
        columnIdx: 3,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.wsURI,
        columnIdx: 4,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.description,
        columnIdx: 5,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.status,
        columnIdx: 6,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.lastGitLog,
        columnIdx: 7,
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
          name: "workspace_branch",
          label: 'Workspace Branch',
          tooltips: 'Workspace branch from origin',
          isNumeric: false),
      ColumnConfig(
          index: 3,
          name: "feature_branch",
          label: 'Feature Branch',
          tooltips: 'Workspace feature branch to make update',
          isNumeric: false),
      ColumnConfig(
          index: 4,
          name: "workspace_uri",
          label: 'Workspace Repo',
          tooltips: 'Workspace Repository Location',
          isNumeric: false),
      ColumnConfig(
          index: 5,
          name: "description",
          label: 'Description',
          tooltips: 'Workspace Repository Location',
          isNumeric: false),
      ColumnConfig(
          index: 6,
          name: "status",
          label: 'Status',
          tooltips: 'Workspace status',
          isNumeric: false),
      ColumnConfig(
          index: 7,
          name: "last_git_log",
          label: 'Last Git Log',
          tooltips: '',
          isHidden: true,
          isNumeric: false),
      ColumnConfig(
          index: 8,
          name: "user_email",
          label: 'User Email',
          tooltips: 'User who made the last change',
          isNumeric: false),
      ColumnConfig(
          index: 9,
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
    actions: [],
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
          tooltips:
              'Boolean (1:true, 0:false) indicating if this Domain Class is converted into a Table',
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
    actions: [],
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
      WhereClause(
          table: "domain_columns",
          column: "domain_table_key",
          joinWith: "domain_tables.key"),
      WhereClause(
          table: "domain_columns",
          column: "data_property_key",
          joinWith: "data_properties.key"),
      WhereClause(
          table: "data_properties",
          column: "domain_class_key",
          joinWith: "domain_classes.key"),
    ],
    actions: [],
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

  // Workspace - Data Model Tables
  // data model files table
  DTKeys.wsDataModelFilesTable: TableConfig(
    key: DTKeys.wsDataModelFilesTable,
    fromClauses: [
      FromClause(schemaName: "\$SCHEMA", tableName: 'workspace_control'),
    ],
    label: 'Data Model Files',
    apiPath: '/dataTable',
    apiAction: 'workspace_read',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: false,
    whereClauses: [
      WhereClause(
          table: "workspace_control",
          column: "source_file_name",
          like: "data_model/%"),
    ],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'addWorkspaceFile',
          label: 'Add File',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null,
          navigationParams: {
            FSK.wsSection: "data_model/",
            FSK.wsDbSourceFileName: "data_model/",
          },
          stateFormNavigationParams: {
            FSK.wsName: FSK.wsName,
          },
          configForm: FormKeys.addWorkspaceFile),
      ActionConfig(
          actionType: DataTableActionType.doAction,
          key: 'deleteWorkspaceFiles',
          label: 'Delete',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          actionName: ActionKeys.deleteWorkspaceFiles),

    ],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.key,
        columnIdx: 0,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.wsDbSourceFileName,
        columnIdx: 1,
      ),
    ]),
    columns: [
      ColumnConfig(
          index: 0,
          name: "key",
          table: "workspace_control",
          label: 'Key',
          tooltips: '',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 1,
          name: "source_file_name",
          table: "workspace_control",
          label: 'File Name',
          tooltips: 'Workspace File Name',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "is_main",
          table: "workspace_control",
          label: 'Main Rule File?',
          tooltips: 'Indicate if file is a main rule file',
          isNumeric: false),
    ],
    sortColumnName: 'source_file_name',
    sortColumnTableName: 'workspace_control',
    sortAscending: true,
    rowsPerPage: 50,
  ),

  // Workspace - Jet Rules Table
  // jet rules files table
  DTKeys.wsJetRulesFilesTable: TableConfig(
    key: DTKeys.wsJetRulesFilesTable,
    fromClauses: [
      FromClause(schemaName: "\$SCHEMA", tableName: 'workspace_control'),
    ],
    label: 'Jet Rules Files',
    apiPath: '/dataTable',
    apiAction: 'workspace_read',
    isCheckboxVisible: true,
    isCheckboxSingleSelect: false,
    whereClauses: [
      WhereClause(
          table: "workspace_control",
          column: "source_file_name",
          like: "jet_rules/%"),
    ],
    actions: [
      ActionConfig(
          actionType: DataTableActionType.showDialog,
          key: 'addWorkspaceFile',
          label: 'Add File',
          style: ActionStyle.primary,
          isVisibleWhenCheckboxVisible: null,
          isEnabledWhenHavingSelectedRows: null,
          navigationParams: {
            FSK.wsSection: "jet_rules/",
            FSK.wsDbSourceFileName: "jet_rules/",
          },
          stateFormNavigationParams: {
            FSK.wsName: FSK.wsName,
          },
          configForm: FormKeys.addWorkspaceFile),
      ActionConfig(
          actionType: DataTableActionType.doAction,
          key: 'deleteWorkspaceFiles',
          label: 'Delete',
          style: ActionStyle.secondary,
          isVisibleWhenCheckboxVisible: true,
          isEnabledWhenHavingSelectedRows: true,
          actionName: ActionKeys.deleteWorkspaceFiles),

    ],
    formStateConfig: DataTableFormStateConfig(keyColumnIdx: 0, otherColumns: [
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.key,
        columnIdx: 0,
      ),
      DataTableFormStateOtherColumnConfig(
        stateKey: FSK.wsDbSourceFileName,
        columnIdx: 1,
      ),
    ]),
    columns: [
      ColumnConfig(
          index: 0,
          name: "key",
          table: "workspace_control",
          label: 'Key',
          tooltips: '',
          isNumeric: true,
          isHidden: true),
      ColumnConfig(
          index: 1,
          name: "source_file_name",
          table: "workspace_control",
          label: 'File Name',
          tooltips: 'Workspace File Name',
          isNumeric: false),
      ColumnConfig(
          index: 2,
          name: "is_main",
          table: "workspace_control",
          label: 'Main Rule File?',
          tooltips: 'Indicate if file is a main rule file',
          isNumeric: false),
    ],
    sortColumnName: 'source_file_name',
    sortColumnTableName: 'workspace_control',
    sortAscending: true,
    rowsPerPage: 50,
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
      WhereClause(
          table: "jet_rules",
          column: "source_file_key",
          joinWith: "workspace_control.key"),
    ],
    actions: [],
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
      WhereClause(
          table: "rule_terms", column: "rule_key", joinWith: "jet_rules.key"),
      WhereClause(
          table: "rule_terms",
          column: "rete_node_key",
          joinWith: "rete_nodes.key"),
    ],
    actions: [],
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
          // name: "normalized_label",
          name:
              "normalizedLabel", //* TODO Rename sqlite column to normalized_label
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
      FromClause(
          schemaName: "\$SCHEMA",
          tableName: 'workspace_control',
          asTableName: 'main_file'),
      FromClause(
          schemaName: "\$SCHEMA",
          tableName: 'workspace_control',
          asTableName: 'support_file'),
    ],
    label: 'Rule Terms',
    apiPath: '/dataTable',
    apiAction: 'workspace_read',
    isCheckboxVisible: false,
    isCheckboxSingleSelect: false,
    whereClauses: [
      WhereClause(
          table: "main_support_files",
          column: "main_file_key",
          joinWith: "main_file.key"),
      WhereClause(
          table: "main_support_files",
          column: "support_file_key",
          joinWith: "support_file.key"),
    ],
    actions: [],
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
