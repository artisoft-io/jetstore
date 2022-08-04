const defaultPadding = 16.0;

/// Screen ID Keys
/// standard keys to identify screen config key
class ScreenKeys {
  static const home = "homeScreen";
  static const sourceConfig = "sourceConfigScreen";

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

  static const login = "login";
  static const register = "register";
}

/// Form State Keys
/// standard keys for form elements
/// These are universal keys used across forms and generally correspond
/// to keys expected in message sent to apiserver
class FSK {
  static const tableName = "tableName";
  static const fileKey = "fileKey";

  static const userEmail = "email";
  static const userName = "name";
  static const userPassword = "password";
  static const userPasswordConfirm = "passwordConfirm";
  static const sessionId = "sessionId";
}

/// Form Action Keys
/// stardard keys to identify Form Action Config Key
class ActionKeys {
  static const login = "loginAction";
  static const register = "registerAction";
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

  // DEMO keys
  static const pipelineDemo = "pipelineTable";
  static const registryDemo = "inputLoaderStatusTable"; // repurposed
  static const usersTable = "userTable";
  static const inputTable = "inputTable";
}
