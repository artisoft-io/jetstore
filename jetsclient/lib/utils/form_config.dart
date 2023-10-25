import 'package:flutter/material.dart';

import 'package:jetsclient/routes/jets_route_data.dart';
import 'package:jetsclient/screens/components/dropdown_shared_items.dart';
import 'package:jetsclient/screens/components/form.dart';
import 'package:jetsclient/screens/components/form_button.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/utils/data_table_config.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/screens/components/data_table.dart';
import 'package:jetsclient/screens/components/input_text_form_field.dart';
import 'package:jetsclient/screens/components/text_field.dart';
import 'package:jetsclient/screens/components/dropdown_form_field.dart';
import 'package:jetsclient/utils/data_table_config_impl.dart';

enum TextRestriction { none, allLower, allUpper, digitsOnly }

/// Form action delegate for [JetsForm] also used for dialogs presented from a
/// data table button
typedef FormActionsDelegate = Future<String?> Function(BuildContext context,
    GlobalKey<FormState> formKey, JetsFormState formState, String actionKey,
    {required int group});

/// Form Field Validator, this correspond to ValidatorDelegate with
/// the context and the formState curried by the JetsForm when calling makeFormField
typedef JetsFormFieldValidator = String? Function(
    int group, String key, dynamic v);

typedef JetsFormFieldRowBuilder1 = List<FormFieldConfig> Function(
    int index, List<String?> labels, JetsFormState formState);

typedef InputFieldType = List<List<FormFieldConfig>>;

typedef JetsFormFieldRowBuilder = InputFieldType Function(
    int index, List<String?>? inputFieldRow, JetsFormState formState);

// a do nothing function
Future<String?> pass(BuildContext context, GlobalKey<FormState> formKey,
    JetsFormState formState, String actionKey,
    {int group = 0}) async {
  return null;
}

/// Form Configuration
/// [title] is used mostly for dialog forms.
/// Simple case  is when [inputFields] are provided (most case)
/// Special case, when [inputFields] is empty, then [inputFieldRowBuilder]
/// shall be provided along with [inputFieldsQuery] and optionally
/// [dropdownItemsQueries], [metadataQueries], and [stateKeyPredicates].
///
/// [inputFieldsQuery] is to provide the list of items (e.g. data properties
///  of the input canonical model that must be mapped for the mapping screen.
/// This query the object_type_mapping_details table and returns 2 columns:
/// data_property (domain class data property)
/// and is_required (indicating that mapping must be specified)).
///
/// The returned data elements are provided to the [inputFieldRow] argument of the
/// [JetsFormFieldRowBuilder].
/// [dropdownItemsQueries] is to provide a cache of dropdown items for use
/// by [JetsDropdownWithSharedItemsFormField].
/// [dropdownItemsQueries] is a map, the key correspond to the
/// [FormFieldConfig] key build by the [inputFieldRowBuilder].
/// Similarily [metadataQueries] is a map, the key is known by the
/// [inputFieldRowBuilder], for example the key [FSK.savedStateCache] correspond
/// to the previously saved values.
///
/// [formWithDynamicRows] is to allow the user to delete rows or add new rows
/// to the input field list. Currently working in conjuction with a
/// [inputFieldRowBuilder].
///
/// Note that all queries are grouped into the map [queries] with a query key
/// used by [inputFieldsQuery], [dropdownItemsQueries], [metadataQueries],
/// and [stateKeyPredicates].
///
/// [initializationDelegate] when not null is invoked to initialize the form state(s)
/// or any data preparation that is needed.
/// This is invoked in [ScreenWithFormState.initState] and
/// [ScreenWithMultiFormsState.initState]
class FormConfig {
  FormConfig({
    required this.key,
    this.title,
    this.inputFields = const [],
    this.formTabsConfig = const [],
    this.inputFieldRowBuilder,
    required this.actions,
    this.queries,
    this.inputFieldsQuery,
    this.savedStateQuery,
    this.dropdownItemsQueries,
    this.metadataQueries,
    this.stateKeyPredicates,
    this.formWithDynamicRows,
    required this.formValidatorDelegate,
    required this.formActionsDelegate,
    this.useListView,
  });
  final String key;
  final String? title;
  // For form without tabs (classic forms)
  final InputFieldType inputFields;
  // Form form with tabs
  final List<FormTabConfig> formTabsConfig;
  final JetsFormFieldRowBuilder? inputFieldRowBuilder;
  final List<FormActionConfig> actions;
  final String? inputFieldsQuery;
  final String? savedStateQuery;
  final Map<String, String>? queries;
  final Map<String, String>? dropdownItemsQueries;
  final Map<String, String>? metadataQueries;
  final List<String>? stateKeyPredicates;
  final bool? formWithDynamicRows;
  final bool? useListView;

  int groupCount() {
    var unique = <int>{};
    for (int i = 0; i < inputFields.length; i++) {
      for (int j = 0; j < inputFields[i].length; j++) {
        unique.add(inputFields[i][j].group);
      }
    }
    return unique.length;
  }

  final ValidatorDelegate formValidatorDelegate;
  final FormActionsDelegate formActionsDelegate;

  JetsFormState makeFormState({JetsFormState? parentFormState}) {
    return JetsFormState(
        initialGroupCount: groupCount(), parentFormState: parentFormState);
  }
}

class FormTabConfig {
  FormTabConfig({
    required this.label,
    required this.inputField,
  });
  final String label;
  final FormFieldConfig inputField;
}

abstract class FormFieldConfig {
  FormFieldConfig({
    required this.key,
    required this.group,
    required this.flex,
    required this.autovalidateMode,
  });
  final String key;

  /// group is not final to enable dynamic list of form field elements.
  /// The validation group needs to be re-assigned since the form field elements
  /// are arranged in a positional list.
  int group;
  final int flex;
  final AutovalidateMode autovalidateMode;

  /// make the form widget
  /// formFieldValidator and formValidator are both the same functon,
  /// the formFieldValidator has the context and formState curried
  /// by the form widget. This is need by the Jets Data Table since
  /// it's a FormField and must have arguments context and formState erased.
  Widget makeFormField({
    required JetsRouteData screenPath,
    required FormConfig formConfig,
    required JetsFormState formState,
  });
}

class PaddingConfig extends FormFieldConfig {
  PaddingConfig(
      {super.key = '',
      super.group = 0,
      super.flex = 1,
      super.autovalidateMode = AutovalidateMode.disabled,
      this.height = defaultPadding,
      this.width = defaultPadding});
  final double height;
  final double width;

  @override
  Widget makeFormField({
    required JetsRouteData screenPath,
    required FormConfig formConfig,
    required JetsFormState formState,
  }) {
    return SizedBox(
      height: height,
      width: width,
    );
  }
}

class TextFieldConfig extends FormFieldConfig {
  TextFieldConfig({
    super.key = '',
    super.group = 0,
    super.flex = 1,
    super.autovalidateMode = AutovalidateMode.disabled,
    required this.label,
    this.maxLines,
    this.leftMargin = 16.0,
    this.topMargin = 0.0,
    this.rightMargin = 16.0,
    this.bottomMargin = 0.0,
  });
  final String label;
  final int? maxLines;
  final double leftMargin;
  final double topMargin;
  final double rightMargin;
  final double bottomMargin;

  @override
  Widget makeFormField({
    required JetsRouteData screenPath,
    required FormConfig formConfig,
    required JetsFormState formState,
  }) {
    return JetsTextField(
      fieldConfig: this,
    );
  }
}

typedef ReadOnlyEvaluator = bool Function();

class FormInputFieldConfig extends FormFieldConfig {
  FormInputFieldConfig({
    required super.key,
    super.group = 0,
    super.flex = 1,
    super.autovalidateMode = AutovalidateMode.disabled,
    required this.label,
    required this.hint,
    required this.autofocus,
    this.obscureText = false,
    this.isReadOnly = false,
    this.isReadOnlyEval,
    required this.textRestriction,
    this.maxLines = 1,
    required this.maxLength,
    this.autofillHints,
    this.defaultValue,
    this.useDefaultFont = false,
  });
  final String label;
  final String hint;
  final bool autofocus;
  final bool obscureText;
  final bool isReadOnly;
  final ReadOnlyEvaluator? isReadOnlyEval;
  final TextRestriction textRestriction;
  final int maxLines;
  // 0 for unbound
  final int maxLength;
  final String? defaultValue;
  final List<String>? autofillHints;
  final bool useDefaultFont;

  @override
  Widget makeFormField({
    required JetsRouteData screenPath,
    required FormConfig formConfig,
    required JetsFormState formState,
  }) {
    return JetsTextFormField(
      key: UniqueKey(),
      formFieldConfig: this,
      onChanged: (p0) {
        formState.setValueAndNotify(group, key, p0.isNotEmpty ? p0 : null);
      },
      formValidator: ((group, key, v) =>
          formConfig.formValidatorDelegate(formState, group, key, v)),
      formState: formState,
    );
  }
}

class DropdownItemConfig {
  DropdownItemConfig({
    required this.label,
    this.value,
  });
  final String label;
  final String? value;
}

/// Dropdown Widget, [items] must be provided, with
/// perhaps a blank item to invite the user to make a selection.
/// If the [dropdownItemsQuery] is not null, it will be used
/// to query the server to obtain items that are appended to [items]
class FormDropdownFieldConfig extends FormFieldConfig {
  FormDropdownFieldConfig({
    required super.key,
    super.group = 0,
    super.flex = 1,
    super.autovalidateMode = AutovalidateMode.disabled,
    this.defaultItemPos = 0,
    this.dropdownItemsQuery,
    this.returnedModelCacheKey,
    this.stateKeyPredicates = const [],
    this.whereStateContains = const {},
    required this.items,
    this.isReadOnly = false,
    this.makeReadOnlyWhenHasSelectedValue = false,
  });
  final String? dropdownItemsQuery;
  final List<String> stateKeyPredicates;
  final Map<String, String> whereStateContains;
  // save the returned model from query and put it in the form state cache if not null
  final String? returnedModelCacheKey;
  final int defaultItemPos;
  final List<DropdownItemConfig> items;
  final bool isReadOnly;
  final bool makeReadOnlyWhenHasSelectedValue;
  bool dropdownItemLoaded = false;

  @override
  Widget makeFormField({
    required JetsRouteData screenPath,
    required FormConfig formConfig,
    required JetsFormState formState,
  }) {
    return JetsDropdownButtonFormField(
      key: UniqueKey(),
      screenPath: screenPath,
      formFieldConfig: this,
      onChanged: (p0) => formState.setValueAndNotify(group, key, p0),
      formValidator: ((group, key, v) =>
          formConfig.formValidatorDelegate(formState, group, key, v)),
      formState: formState,
    );
  }

  static final List<DropdownItemConfig> rdfDropdownItems = [
    DropdownItemConfig(label: 'Select...', value: ''),
    DropdownItemConfig(label: 'Boolean', value: 'bool'),
    DropdownItemConfig(label: 'Date', value: 'date'),
    DropdownItemConfig(label: 'Datetime', value: 'datetime'),
    DropdownItemConfig(label: 'Double', value: 'double'),
    DropdownItemConfig(label: 'Int', value: 'int'),
    DropdownItemConfig(label: 'Long', value: 'long'),
    DropdownItemConfig(label: 'Resource', value: 'resource'),
    DropdownItemConfig(label: 'Text', value: 'text'),
    DropdownItemConfig(label: 'Unsigned Int', value: 'uint'),
    DropdownItemConfig(label: 'Unsigned Long', value: 'ulong'),
  ];
}

/// Dropdown Widget with Shared [items], [items] is provided via
/// the [dropdownMenuItemCacheKey] form state cache key.
/// [defaultItem] is specified here a the actual value (which can be null)
/// of the dropdown.
class FormDropdownWithSharedItemsFieldConfig extends FormFieldConfig {
  FormDropdownWithSharedItemsFieldConfig({
    required super.key,
    super.group = 0,
    super.flex = 1,
    super.autovalidateMode = AutovalidateMode.disabled,
    required this.dropdownMenuItemCacheKey,
    this.defaultItem,
  });
  final String dropdownMenuItemCacheKey;
  final String? defaultItem;

  @override
  Widget makeFormField({
    required JetsRouteData screenPath,
    required FormConfig formConfig,
    required JetsFormState formState,
  }) {
    return JetsDropdownWithSharedItemsFormField(
      key: UniqueKey(),
      screenPath: screenPath,
      formFieldConfig: this,
      onChanged: (p0) => formState.setValueAndNotify(group, key, p0),
      formValidator: ((group, key, v) =>
          formConfig.formValidatorDelegate(formState, group, key, v)),
      formState: formState,
      selectedValue: formState.getValue(group, key),
    );
  }
}

class FormDataTableFieldConfig extends FormFieldConfig {
  FormDataTableFieldConfig({
    required super.key,
    super.group = 0,
    super.flex = 1,
    super.autovalidateMode = AutovalidateMode.disabled,
    this.tableWidth = double.infinity,
    this.tableHeight = 400,
    required this.dataTableConfig,
  });
  final double tableWidth;
  final double tableHeight;
  final String dataTableConfig;

  @override
  Widget makeFormField({
    required JetsRouteData screenPath,
    required FormConfig formConfig,
    required JetsFormState formState,
  }) {
    return SizedBox(
        width: tableWidth,
        height: tableHeight,
        child: JetsDataTableWidget(
            key: UniqueKey(),
            screenPath: screenPath,
            formFieldConfig: this,
            tableConfig: getTableConfig(dataTableConfig),
            formState: formState,
            validatorDelegate: formConfig.formValidatorDelegate,
            actionsDelegate: formConfig.formActionsDelegate));
  }
}

/// This class can be used in 2 ways:
///   1. as the form actions in the last row of the form
///   2. as a form field within the form
/// In both cases it is an action button with the action implementation
/// in the form action delegate.
/// Note that this class is somewhat similar to [ActionConfig] which is the
/// configuration for buttons of [JetsDataTable] while
/// [FormActionConfig] is for the configuration for buttons of [JetsForm]
/// [label] is the fixed label to use, when it is empty, a value is
/// looked up in [labelByStyle] using the [buttonStyle].
/// An empty label is used when not found.
class FormActionConfig extends FormFieldConfig {
  FormActionConfig({
    required super.key,
    super.group = 0,
    super.flex = 1,
    super.autovalidateMode = AutovalidateMode.disabled,
    this.label = '',
    required this.buttonStyle,
    this.labelByStyle = const <ActionStyle, String>{},
    this.enableOnlyWhenFormValid = false,
    this.enableOnlyWhenFormNotValid = false,
    this.leftMargin = 0.0,
    this.topMargin = 0.0,
    this.rightMargin = 0.0,
    this.bottomMargin = 0.0,
  });
  final String label;
  final Map<ActionStyle, String> labelByStyle;
  final ActionStyle buttonStyle;
  final bool enableOnlyWhenFormValid;
  final bool enableOnlyWhenFormNotValid;
  final double leftMargin;
  final double topMargin;
  final double rightMargin;
  final double bottomMargin;

  @override
  Widget makeFormField({
    required JetsRouteData screenPath,
    required FormConfig formConfig,
    required JetsFormState formState,
  }) {
    return JetsFormButton(
        key: UniqueKey(),
        formActionConfig: this,
        formKey: formState.formKey!,
        formState: formState,
        actionsDelegate: formConfig.formActionsDelegate);
  }
}
