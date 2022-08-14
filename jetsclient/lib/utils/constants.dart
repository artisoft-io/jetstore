import 'package:flutter/material.dart';

const defaultPadding = 16.0;
const betweenTheButtonsPadding = 8.0;

/// Button action style, used by both JetsDataTable and JetsForm
enum ActionStyle { primary, secondary, alternate, danger }

ButtonStyle buttonStyle(ActionStyle style, ThemeData td) {
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
  static const sourceConfig = "sourceConfigScreen";
  static const domainTableViewer = "domainTableViewerScreen";
  static const processInput = "processInputScreen";
  static const processConfig = "processConfigScreen";

  static const login = "loginScreen";
  static const register = "registerScreen";

  //* DEMO
  static const pipelines = "pipelinesScreen";
  static const fileRegistry = "fileRegistryScreen";
  static const fileRegistryTable = "fileRegistryTableScreen";
}

/// Form ID Keys
/// standard keys to identify form config key
class FormKeys {
  static const home = "homeForm";
  static const sourceConfig = "sourceConfigForm";
  static const addClient = "addClientDialog";
  static const loadFile = "loadFileDialog";

  static const processInput = "processInputForm";
  static const addProcessInput = "addProcessInputDialog";
  static const processMapping = "processMappingDialog";
  static const processConfig = "processConfigForm";
  static const rulesConfig = "rulesConfigDialog";

  static const login = "login";
  static const register = "register";
}

/// Form State Keys
/// standard keys for form elements
/// These are universal keys used across forms and generally correspond
/// to keys expected in message sent to apiserver
class FSK {
  static const tableName = "table_name";
  static const fileKey = "file_key";

  static const userEmail = "user_email";
  static const userName = "name";
  static const userPassword = "password";
  static const userPasswordConfirm = "passwordConfirm";
  static const sessionId = "session_id";

  static const client = "client";
  static const details = "details";

  static const objectType = "object_type";
  static const sourceType = "source_type";
  static const groupingColumn = "grouping_column";
  static const entityRdfType = "entity_rdf_type";

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

  // reserve key to hold an error to display to user
  static const serverError = "server_error";
}

/// Form Action Keys
/// stardard keys to identify Form Action Config Key
class ActionKeys {
  static const login = "loginAction";
  static const register = "registerAction";
  static const clientOk = "client.ok";
  static const dialogCancel = "dialog.cancelAction";
  // for load file dialog
  static const loaderOk = "loader.ok";

  // for add process input dialog
  static const addProcessInputOk = "addProcessInputOk";
  // for process mapping dialog
  static const mapperOk = "mapper.ok";
  static const mapperDraft = "mapper.draft";
  
  // for process and rules config dialog
  static const ruleConfigOk = "ruleConfig.ok";
  static const ruleConfigAdd = "ruleConfig.add";
  static const ruleConfigDelete = "ruleConfig.delete";
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
  static const inputLoaderStatusTable = "inputLoaderStatusTable";
  static const pipelineExecStatusTable = "pipelineExecStatusTable";
  static const pipelineExecDetailsTable = "pipelineExecDetailsTable";

  // Source Config DT
  static const clientsTable = "clientsTable";
  static const sourceConfigsTable = "sourceConfigsTable";

  // Domain Table Viewer DT
  static const inputTable = "inputTable";

  // Process Input & Mapping DT
  static const processInputTable = "processInputTable";
  static const processMappingTable = "processMappingTable";

// Process and Rules Config DT
  static const processNameTable = "processNameTable";
  static const clientsNameTable = "clientsNameTable";
  static const processConfigTable = "processConfigTable";
  static const ruleConfigTable = "ruleConfigTable";

  // DEMO keys
  static const pipelineDemo = "pipelineTable";
  static const registryDemo = "inputLoaderStatusTable"; // repurposed
  static const usersTable = "userTable";
}

/// API Server endpoints
class ServerEPs {
  static const dataTableEP = "/dataTable";
}
