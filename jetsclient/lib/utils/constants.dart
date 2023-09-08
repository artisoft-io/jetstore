import 'package:flutter/material.dart';

const defaultPadding = 16.0;
const betweenTheButtonsPadding = 8.0;

/// Button action style, used by both JetsDataTable and JetsForm
enum ActionStyle { primary, secondary, alternate, menuSelected, menuAlternate, danger }

ButtonStyle? buttonStyle(ActionStyle style, ThemeData td) {
  switch (style) {
    case ActionStyle.danger:
      return ElevatedButton.styleFrom(
        foregroundColor: td.colorScheme.onErrorContainer,
        backgroundColor: td.colorScheme.errorContainer,
      ).copyWith(elevation: ButtonStyleButton.allOrNull(0.0));

    case ActionStyle.secondary:
      return ElevatedButton.styleFrom(
        foregroundColor: td.colorScheme.onPrimaryContainer,
        backgroundColor: td.colorScheme.primaryContainer,
      ).copyWith(elevation: ButtonStyleButton.allOrNull(0.0));

    case ActionStyle.alternate:
      return ElevatedButton.styleFrom(
        foregroundColor: td.colorScheme.onPrimaryContainer,
        backgroundColor: Colors.orange.shade200,
      ).copyWith(elevation: ButtonStyleButton.allOrNull(0.0));

    case ActionStyle.menuSelected:
      return TextButton.styleFrom(
        foregroundColor: td.colorScheme.onSecondaryContainer,
        backgroundColor: td.colorScheme.primaryContainer,
        textStyle: const TextStyle(fontWeight: FontWeight.bold),
      ).copyWith(elevation: ButtonStyleButton.allOrNull(0.0));

    case ActionStyle.menuAlternate:
      return null;

    default: // primary
      return ElevatedButton.styleFrom(
        foregroundColor: td.colorScheme.onSecondaryContainer,
        backgroundColor: td.colorScheme.secondaryContainer,
      ).copyWith(elevation: ButtonStyleButton.allOrNull(0.0));
  }
}

/// Screen ID Keys
/// standard keys to identify screen config key
class ScreenKeys {
  static const home = "homeScreen";
  static const clientAdmin = "clientAdminScreen";
  static const sourceConfig = "sourceConfigScreen";
  static const domainTableViewer = "domainTableViewerScreen";
  static const inputSourceMapping = "inputSourceMappingScreen";
  static const processInput = "processInputScreen";
  static const processConfig = "processConfigScreen";
  static const ruleConfigv2 = "ruleConfigv2Screen";
  static const pipelineConfig = "pipelineConfigScreen";
  static const pipelineConfigEdit = "pipelineConfigEditScreen";

  // Query Tool
  static const queryToolScreen = "queryToolScreen";

  static const login = "loginScreen";
  static const register = "registerScreen";
  static const userAdmin = "userAdminScreen";

  static const fileRegistryTable = "fileRegistryTableScreen";
  static const filePreview = "filePreviewScreen";
  static const execStatusDetailsTable = "execStatusDetailsTable";
  static const processErrorsTable = "processErrorsTable";

  // Workspace IDE Screens
  static const workspaceRegistry = "workspaceRegistryScreen";
  static const workspaceHome = "workspaceHome";
}

/// Form ID Keys
/// standard keys to identify form config key
class FormKeys {
  // Home form
  static const home = "homeForm";
  // Client & Org Admin Forms
  static const clientAdmin = "clientAdminForm";
  static const addClient = "addClientDialog";
  static const addOrg = "addOrgDialog";
  // File Staging Area
  static const sourceConfig = "sourceConfigForm";
  static const addSourceConfig = "addSourceConfigDialog";
  static const loadRawRows = "loadRawRowsDialog";
  static const loadAllFiles = "loadAllFilesDialog";
  // Input Source Mapping Forms
  static const inputSourceMapping = "inputSourceMapping";
  static const processMapping = "processMappingDialog";
  // Process Input Forms
  static const processInput = "processInputForm";
  static const addProcessInput = "addProcessInputDialog";
  // Rule Process Config Forms
  static const processConfig = "processConfigForm";
  static const rulesConfig = "rulesConfigDialog";
  static const rulesConfigv2 = "rulesConfigv2SelectForm";
  static const rulesConfigv2Dialog = "rulesConfigv2Dialog";
  // Pipeline Config & Exec Forms
  static const pipelineConfigForm = "pipelineConfigForm";
  static const pipelineConfigEditForm = "pipelineConfigEditForm";
  static const startPipeline = "startPipelineDialog";
  // Process Errors Dialogs
  static const viewProcessErrors = "viewProcessErrorsForm";
  static const viewInputRecords = "viewInputRecordsDialog";
  static const viewReteTriples = "viewReteTriplesDialog";
  // Query Tool Forms
  static const queryToolInputForm = "queryToolInputForm";
  static const queryToolResultViewForm = "queryToolResultViewForm";
  // User Admin forms
  static const login = "login";
  static const register = "register";
  static const userAdmin = "userAdmin";

  // Workspace IDE forms
  static const workspaceRegistry = "workspaceRegistry";
  static const workspaceHome = "workspaceHome";
  static const addWorkspace = "addWorkspace";
  // Forms for each section of the workspace, incl file editor
  // Note: The formConfig key is constructed in initializeWorkspaceFileEditor
  static const workspaceFileEditor = "workspace.file.form";
  static const wsDataModelForm = "workspace.data_model.form";
  static const wsJetRulesForm = "workspace.jet_rules.form";
  static const wsLookupsForm = "workspace.lookups.form";
  // ... more to come
}

/// Form State Keys
/// standard keys for form elements
/// These are universal keys used across forms and generally correspond
/// to keys expected in message sent to apiserver
class FSK {
  static const key = "key";
  static const label = "label";
  static const tableName = "table_name";
  static const fileKey = "file_key";

  static const userEmail = "user_email";
  static const userName = "name";
  static const userPassword = "password";
  static const userPasswordConfirm = "passwordConfirm";
  static const sessionId = "session_id";
  static const devMode = "dev_mode";
  static const isAdmin = "is_admin";
  static const isActive = "is_active";

  static const client = "client";
  static const org = "org";
  static const sourcePeriodKey = "source_period_key";
  static const fromSourcePeriodKey = "from_source_period_key";
  static const toSourcePeriodKey = "to_source_period_key";
  static const fromDayPeriod = "from_day_period";
  static const toDayPeriod = "to_day_period";
  static const lookbackPeriods = "lookback_periods";
  static const details = "details";

  static const objectType = "object_type";
  static const sourceType = "source_type";
  static const domainKeysJson = "domain_keys_json";
  static const inputColumnsJson = "input_columns_json";
  static const inputColumnsPositionsCsv = "input_columns_positions_csv";
  static const codeValuesMappingJson = "code_values_mapping_json";
  static const entityRdfType = "entity_rdf_type";
  static const status = "status";
  static const rawRows = "raw_rows";

  static const pipelineExectionStatusKey = "pipeline_execution_status_key";
  static const domainKey = "domain_key";
  static const domainKeyColumn =
      "domainKeyColumn"; // e.g. Eligibility:domain_key
  static const reteSessionTriples = "rete_session_triples";

  // Query Tool
  static const rawQuery = "raw_query";
  static const rawQueryReady = "raw_query.ready";
  static const rawDdlQueryReady = "raw_query.ddl.ready";
  static const queryReady = "query.ready";

  // keys used for mapping
  // key for domain classes data properties
  // processInputKey is a proxy to update process_input table's key
  // isRequiredFlag is a flag not corresponding to a specific model
  // data element.
  static const isRequiredFlag = "flag.is_required";
  static const processInputKey = "process_input.key";
  static const dataProperty = "data_property";
  static const inputColumn = "input_column";
  static const functionName = "function_name";
  static const functionArgument = "argument";
  static const mappingDefaultValue = "default_value";
  static const mappingErrorMessage = "error_message";

  // Process and Rule Config keys
  static const processConfigKey = "process_config_key";
  static const processName = "process_name";
  static const description = "description";
  static const subject = "subject";
  static const predicate = "predicate";
  static const object = "object";
  static const rdfType = "rdf_type";

  // Pipeline Config keys
  static const mainProcessInputKey = "main_process_input_key";
  static const mergedProcessInputKeys = "merged_process_input_keys";
  static const injectedProcessInputKeys = "injected_process_input_keys";
  static const mainObjectType = "main_object_type";
  static const mainSourceType = "main_source_type";
  static const mainTableName = "main_process_input.table_name";
  static const sourcePeriodType = "source_period_type";
  static const automated = "automated";
  static const maxReteSessionSaved = "max_rete_sessions_saved";
  static const ruleConfigJson = "rule_config_json";

  // Pipeline Exec keys
  static const pipelineConfigKey = "pipeline_config_key";
  static const mainInputRegistryKey = "main_input_registry_key";
  static const mainInputFileKey = "main_input_file_key";
  static const mergedInputRegistryKeys = "merged_input_registry_keys";

  // Source Period keys
  static const year = "year";
  static const month = "month";
  static const day = "day";
  static const dayPeriod = "day_period";

  // Form Keys for Workspace IDE
  static const wsName = "workspace_name";
  static const wsURI = "workspace_uri";
  static const wsFileName = "file_name";
  static const wsFileEditorContent = "file_content";
  static const wsOid = "oid";
  // matching menuItem and current page (virtual page)
  static const pageMatchKey = "pageMatchKey";


  // reserved keys for cache

  // inputFieldsCache: cache value is a list<String?>
  // based on query inputFieldsQuery
  static const inputFieldsCache = "cache.input_fields";

  // inputColumnsDropdownItemsCache: value is DropdownButtonFormField
  static const inputColumnsDropdownItemsCache =
      "cache.dropdown_items.input_column";

  // mappingFunctionsDropdownItemsCache: value is DropdownButtonFormField
  static const mappingFunctionsDropdownItemsCache =
      "cache.dropdown_items.mapping_function";

  // mappingFunctionDetailsCache: cache value is a list<String?>
  // based on metadata query mappingFunctionDetailsCache
  static const mappingFunctionDetailsCache = "cache.mapping_function_details";

  // inputColumnsCache: cache value is a list<String?> of input columns
  // based on metadata query inputColumnsCache
  static const inputColumnsCache = "cache.input_columns";

  // savedStateCache: cache value is a list<String?>
  // based on query savedStateQuery
  static const savedStateCache = "cache.saved_state";

  // objectTypeRegistryCache: cache value is a list<list<String?>> (model)
  // from table object_type_registry based on query objectTypeRegistryQuery
  // provides mapping between object_type and entity_rdf_type
  static const objectTypeRegistryCache = "cache.object_type_registry";

  // entityRdfTypeRegistryCache: cache value is a List<String?>
  // provides list of entity_rdf_type
  static const entityRdfTypeRegistryCache = "cache.dropdown_items.entity_rdf_type";

  // processConfigCache: cache value is a list<list<String?>> (model)
  // from table process_config provides [key, process_name]
  // The query is in FSK.processName drowpdown initaization query
  static const processConfigCache = "cache.process_config";

  // reserve key to hold an error to display to user
  static const serverError = "server_error";

  // key for error message from server
  static const error = "error";
}

/// Form Action Keys
/// stardard keys to identify Form Action Config Key
class ActionKeys {
  static const login = "loginAction";
  static const register = "registerAction";
  static const dialogCancel = "dialog.cancelAction";
  static const toggleUserActive = "toggleUserActive";
  static const deleteUser = "deleteUser";

  // for Client & Org Admin dialog
  static const clientOk = "client.ok";
  static const orgOk = "org.ok";
  static const deleteClient = "deleteClientAction";
  static const deleteOrg = "deleteOrgAction";
  static const exportClientConfig = "exportClientConfig";

  // for Source Config dialog
  static const addSourceConfigOk = "addSourceConfig.ok";
  static const dropTable = "dropTable";

  // for load file
  static const loaderOk = "loader.ok";
  static const loadAllFilesOk = "loadAllFiles.ok";
  static const loaderMultiOk = "loaderMulti.ok";

  // to sync file key with web storage (s3)
  static const syncFileKey = "syncFileKey";

  // Query Tool Actions
  static const queryToolOk = "queryTool.ok";
  static const queryToolDdlOk = "queryTool.ddl.ok";


  // for add process input dialog
  static const addProcessInputOk = "addProcessInputOk";
  // for process mapping dialog
  static const mapperOk = "mapper.ok";
  static const mapperDraft = "mapper.draft";
  static const loadRawRowsOk = "loadRawRows.Ok";
  // to download process mapping rows
  static const downloadMapping = "downloadMapping";

  // for process and rules config dialog
  static const ruleConfigOk = "ruleConfig.ok";
  static const ruleConfigv2Ok = "ruleConfigv2.ok";
  static const ruleConfigAdd = "ruleConfig.add";
  static const ruleConfigDelete = "ruleConfig.delete";

  // for add / edit pipeline config dialog
  static const pipelineConfigOk = "pipelineConfig.ok";

  // for pipeline execution dialogs
  static const startPipelineOk = "startPipeline.ok";

  // for process_error data table
  static const setupShowInputRecords = "setupShowInputRecords";
  static const setupShowReteTriples = "setupShowReteTriples";

  // Workspace IDE ActionKeys
  static const addWorkspaceOk = "addWorkspaceOk";
  static const compileWorkspace = "compileWorkspace";
  static const openWorkspace = "openWorkspace";
  static const wsSaveFileOk = "wsSaveFileOk";
  static const deleteWorkspaceChanges = "deleteWorkspaceChanges";
  static const deleteAllWorkspaceChanges = "deleteAllWorkspaceChanges";
}

/// Form Action Keys
/// stardard keys to identify Form Action Config Key
class StatusKeys {
  static const submitted = "submitted";
  static const processing = "processing";
  static const error = "error";
  static const failed = "failed";
}

/// Data Table Config Keys
/// standard keys for data table config
class DTKeys {
  // Home Screen DT
  static const inputRegistryTable = "inputRegistryTable";
  static const inputLoaderStatusTable = "inputLoaderStatusTable";
  static const pipelineExecStatusTable = "pipelineExecStatusTable";
  static const pipelineExecDetailsTable = "pipelineExecDetailsTable";
  static const processErrorsTable = "processErrorsTable";
  static const reteSessionTriplesTable = "reteSessionTriplesTable";
  static const inputRecordsFromProcessErrorTable =
      "inputRecordsFromProcessErrorTable";

  // Client & Organization Admin DT
  static const clientAdminTable = "clientAdminTable";
  static const clientTable = "clientTable";
  static const orgNameTable = "orgNameTable";

  // File Staging Area / Source Config DT
  static const sourceConfigTable = "sourceConfigTable";
  static const fileKeyStagingTable = "fileKeyStagingTable";
  static const fileKeyStagingMultiLoadTable = "fileKeyStagingMultiLoadTable";

  // Domain Table Viewer DT
  static const inputTable = "inputTable";

  // QueryTool ResultSetTable
  static const queryToolResultSetTable = "queryToolResultSetTable";

  // Input File Viewer DT
  static const inputFileViewerTable = "inputFileViewerTable";

  // Input Source Mapping DT
  static const inputSourceMapping = "inputSourceMapping";

  // Process Input Configuration DT
  static const processInputTable = "processInputTable";
  static const processMappingTable = "processMappingTable";

  // Rules Config DT
  static const ruleConfigTable = "ruleConfigTable";
  static const ruleConfigv2Table = "ruleConfigv2Table";
  static const clientsAndProcessesTableView = "clientsAndProcessesTableView";

  // Pipeline Config DT
  static const pipelineConfigTable = "pipelineConfigTable";
  static const mainProcessInputTable = "mainProcessInputTable";
  static const mergeProcessInputTable = "mergeProcessInputTable";
  static const injectedProcessInputTable = "injectedProcessInputTable";

  // User administration DT
  static const usersTable = "userTable";

  // Workspace IDE DT
  static const workspaceRegistryTable = "workspaceRegistryTable";
  static const workspaceChangesTable = "workspaceChangesTable";
  
  // Workspace - Data Model Tables
  static const wsDomainTableTable = "wsDomainTableTable";
  static const wsDomainClassTable = "wsDomainClassTable";
  static const wsDataPropertyTable = "wsDataPropertyTable";

  // Workspace - Jet Rules Tables
  static const wsJetRulesTable = "wsJetRulesTable";
  static const wsRuleTermsTable = "wsRuleTermsTable";
  static const wsMainSupportFilesTable = "wsMainSupportFilesTable";

  static const wsLookupsTable = "wsLookupsTable";
}

/// API Server endpoints
class ServerEPs {
  static const dataTableEP = "/dataTable";
  static const purgeDataEP = "/purgeData";
  static const registerFileKeyEP = "/registerFileKey";
  static const loginEP = "/login";
  static const registerEP = "/register";
}
