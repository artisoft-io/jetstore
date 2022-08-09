const defaultPadding = 16.0;

/// Screen ID Keys
/// standard keys to identify screen config key
class ScreenKeys {
  static const home = "homeScreen";
  static const sourceConfig = "sourceConfigScreen";
  static const domainTableViewer = "domainTableViewerScreen";

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
  static const processMapping = "processMappingDialog";

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
  static const groupingColumn = "grouping_column";

  // keys used for mapping
  // key for domain classes data properties
  static const processInputKey = "process_input_key";
  static const dataProperty = "data_property";
  static const inputColumn = "input_column";
  static const functionName = "function_name";
  static const functionArgument = "argument";
  static const mappingDefaultValue = "default_value";
  static const mappingErrorMessage = "error_message";

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
  static const mappingFunctionDetailsCache =
      "cache.mapping_function_details";

  // inputColumnsCache: cache value is a list<String?> of input columns
  // based on metadata query inputColumnsCache
  static const inputColumnsCache =
      "cache.input_columns";

  // savedStateCache: cache value is a list<String?>
  // based on query savedStateQuery
  static const savedStateCache = "cache.saved_state";

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
  // for process mapping dialog
  static const mapperOk = "mapper.ok";
  static const mapperDraft = "mapper.draft";
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

  // DEMO keys
  static const pipelineDemo = "pipelineTable";
  static const registryDemo = "inputLoaderStatusTable"; // repurposed
  static const usersTable = "userTable";
}

/// API Server endpoints
class ServerEPs {
  static const dataTableEP = "/dataTable";
}
