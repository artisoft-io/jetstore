// lib/config.dart
class AppConfig {
  static const String buttonsConfigJson = String.fromEnvironment(
    'BUTTON_CFG_JSON',
    defaultValue: '[]', // Optional default value
  );
}
