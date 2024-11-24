import 'package:jetsclient/models/data_table_config.dart';

/// Load Files User Flow Tables
final Map<String, TableConfig> _tableConfigurations = {
};

TableConfig? getRegisterFileKeyTableConfig(String key) {
  var config = _tableConfigurations[key];
  return config;
}
