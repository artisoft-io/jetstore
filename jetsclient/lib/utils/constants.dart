const defaultPadding = 16.0;

/// Screen ID Keys
/// standard keys to identify screen config key
class ScreenKeys {
  static const home = "homeScreen";
  static const login = "loginScreen";
  static const register = "registerScreen";
  static const pipelines = "pipelinesScreen";
  static const fileRegistry = "fileRegistryScreen";
  static const fileRegistryTable = "fileRegistryTableScreen";
}

/// Form ID Keys
/// standard keys to identify form config key
class FormKeys {
  static const login = "login";
  static const register = "register";
}

/// Form State Keys
/// standard keys for form elements
class FSK {
  static const userEmail = "email";
  static const userName = "name";
  static const userPassword = "password";
  static const userPasswordConfirm = "passwordConfirm";
  static const sessionId = "sessionId";
}

/// Form Action Keys
/// stardard keys to identify Form Action Config Key
class ActionKeys {
  static const login = "login";
  static const register = "register";
}

/// Data Table Config Keys
/// standard keys for data table config
class DTKeys {
  static const pipelineDemo = "pipelineTable";
  static const registryDemo = "registryTable";
  static const usersTable = "userTable";
  static const inputTable = "inputTable";
}
