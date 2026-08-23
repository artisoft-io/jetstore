import 'package:jetsclient/modules/screen_config_impl.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/screen_config.dart';

/// User Flow Screen Configurations
final Map<String, ScreenConfig> _screenConfigurations = {
  ScreenKeys.ufFileMapping: ScreenConfig(
      key: ScreenKeys.ufFileMapping,
      appBarLabel: 'JetStore Workspace',
      title: 'File Mapping Configuration',
      showLogout: true,
      leftBarLogo: 'assets/images/logo.png',
      menuEntries: defaultMenuEntries,
      adminMenuEntries: adminMenuEntries,
      toolbarMenuEntries: toolbarMenuEntries),
};


ScreenConfig? getFileMappingScreenConfig(String key) {
  var config = _screenConfigurations[key];
  return config;
}

/// The screen configurations the file mapping flows registers.
///
/// Exposed so `screen_reachability_corpus_test.dart` can enumerate the map
/// rather than probe a list of constants — see I-37, and the note in
/// `screen_config_corpus_test.dart` about why asking the map is the only
/// enumeration that cannot be wrong about its own contents.
Iterable<String> get fileMappingScreenConfigKeys => _screenConfigurations.keys;
