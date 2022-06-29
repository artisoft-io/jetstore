enum InputFieldType { text, numeric }

enum TextRestriction { none, allLower, allUpper, digitsOnly }

enum ButtonStyle { primary, secondary, other }

class FormInputFieldConfig {
  FormInputFieldConfig(
      {required this.key,
      required this.type,
      required this.label,
      required this.hint,
      required this.autofocus,
      required this.obscureText,
      required this.textRestriction,
      required this.maxLength});
  final String key;
  final InputFieldType type;
  final String label;
  final String hint;
  final bool autofocus;
  final bool obscureText;
  final TextRestriction textRestriction;
  // 0 for unbound
  final int maxLength;
}

class FormActionConfig {
  FormActionConfig(
      {required this.key, required this.label, required this.buttonStyle});
  final String key;
  final String label;
  final ButtonStyle buttonStyle;
}

class FormConfig {
  FormConfig(
      {required this.key,
      this.title,
      required this.inputFields,
      required this.actions});
  final String key;
  final String? title;
  final List<FormInputFieldConfig> inputFields;
  final List<FormActionConfig> actions;
}

final Map<String, FormConfig> _formConfigurations = {
  'login': FormConfig(
    key: 'login',
    title: 'Please Sign In',
    actions: [
      FormActionConfig(
          key: "login", label: "Sign in", buttonStyle: ButtonStyle.primary),
      FormActionConfig(
          key: "register",
          label: "Register",
          buttonStyle: ButtonStyle.secondary),
    ],
    inputFields: [
      FormInputFieldConfig(
          key: "email",
          type: InputFieldType.text,
          label: "Email",
          hint: "Your email address",
          autofocus: true,
          obscureText: false,
          textRestriction: TextRestriction.none,
          maxLength: 80),
      FormInputFieldConfig(
          key: "password",
          type: InputFieldType.text,
          label: "Password",
          hint: "Your password",
          autofocus: false,
          obscureText: true,
          textRestriction: TextRestriction.none,
          maxLength: 80),
    ],
  ),
  'register': FormConfig(
    key: 'register',
    title: 'Welcome, Please Register',
    actions: [
      FormActionConfig(
        key: "register", label: "Register", buttonStyle: ButtonStyle.primary),
    ],
    inputFields: [
      FormInputFieldConfig(
          key: "name",
          type: InputFieldType.text,
          label: "Name",
          hint: "Enter your name",
          autofocus: true,
          obscureText: false,
          textRestriction: TextRestriction.none,
          maxLength: 80),
      FormInputFieldConfig(
          key: "email",
          type: InputFieldType.text,
          label: "Email",
          hint: "Your email address",
          autofocus: false,
          obscureText: false,
          textRestriction: TextRestriction.allLower,
          maxLength: 80),
      FormInputFieldConfig(
          key: "password",
          type: InputFieldType.text,
          label: "Password",
          hint: "Your password",
          autofocus: false,
          obscureText: true,
          textRestriction: TextRestriction.none,
          maxLength: 80),
      FormInputFieldConfig(
          key: "passwordConfirmation",
          type: InputFieldType.text,
          label: "Password Confirmation",
          hint: "Re-enter your password",
          autofocus: false,
          obscureText: true,
          textRestriction: TextRestriction.none,
          maxLength: 80),
    ],
  ),
};

FormConfig getFormConfig(String key) {
  var config = _formConfigurations[key];
  if (config == null) {
    throw Exception(
        'ERROR: Invalid program configuration: form configuration $key not found');
  }
  return config;
}
