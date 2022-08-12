import 'package:flutter/material.dart';

import 'package:jetsclient/routes/jets_route_data.dart';
import 'package:jetsclient/screens/components/dropdown_shared_items.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/utils/data_table_config.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/screens/components/data_table.dart';
import 'package:jetsclient/screens/components/input_text_form_field.dart';
import 'package:jetsclient/screens/components/text_field.dart';
import 'package:jetsclient/screens/components/dropdown_form_field.dart';

enum TextRestriction { none, allLower, allUpper, digitsOnly }

enum ButtonStyle { primary, secondary, other }

/// Form action delegate for dialog presented from a data table button
typedef FormActionsDelegate = void Function(BuildContext context,
    GlobalKey<FormState> formKey, JetsFormState formState, String actionKey);

/// Form Field Validator, this correspond to ValidatorDelegate with
/// the context and the formState curried by the JetsForm when calling makeFormField
typedef JetsFormFieldValidator = String? Function(
    int group, String key, dynamic v);

typedef JetsFormFieldRowBuilder1 = List<FormFieldConfig> Function(
    int index, List<String?> labels, JetsFormState formState);

typedef JetsFormFieldRowBuilder = List<List<FormFieldConfig>> Function(
    int index, List<String?> inputFieldRow, JetsFormState formState);

typedef InputFieldType = List<List<FormFieldConfig>>;

/// Form Configuration
/// Simple case  is when [inputFields] are provided (most case)
/// Special case, when [inputFields] is empty, then [inputFieldRowBuilder]
/// shall be provided along with [inputFieldsQuery] and optionally
/// [dropdownItemsQueries], [metadataQueries], and [stateKeyPredicates].
/// [inputFieldsQuery] is to provide the list of data properties of the
/// input canonical model that must be mapped.
/// This query the object_type_mapping_details table and returns 2 columns:
/// data_property (domain class data property)
/// and is_required (indicating that mapping must be specified).
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
/// Note that all queries are grouped into the map [queries] with a query key
/// used by [inputFieldsQuery], [dropdownItemsQueries], [metadataQueries],
/// and [stateKeyPredicates].
class FormConfig {
  FormConfig({
    required this.key,
    this.inputFields = const [],
    this.inputFieldRowBuilder,
    required this.actions,
    this.queries,
    this.inputFieldsQuery,
    this.savedStateQuery,
    this.dropdownItemsQueries,
    this.metadataQueries,
    this.stateKeyPredicates,
  });
  final String key;
  final InputFieldType inputFields;
  final JetsFormFieldRowBuilder? inputFieldRowBuilder;
  final List<FormActionConfig> actions;
  final String? inputFieldsQuery;
  final String? savedStateQuery;
  final Map<String, String>? queries;
  final Map<String, String>? dropdownItemsQueries;
  final Map<String, String>? metadataQueries;
  final List<String>? stateKeyPredicates;

  int groupCount() {
    var unique = <int>{};
    for (int i = 0; i < inputFields.length; i++) {
      for (int j = 0; j < inputFields[i].length; j++) {
        unique.add(inputFields[i][j].group);
      }
    }
    return unique.length;
  }

  JetsFormState makeFormState({JetsFormState? parentFormState}) {
    return JetsFormState(
        initialGroupCount: groupCount(), parentFormState: parentFormState);
  }
}

abstract class FormFieldConfig {
  FormFieldConfig({
    required this.key,
    required this.group,
    required this.flex,
    required this.autovalidateMode,
  });
  final String key;
  final int group;
  final int flex;
  final AutovalidateMode autovalidateMode;

  /// make the form widget
  /// formFieldValidator and formValidator are both the same functon,
  /// the formFieldValidator has the context and formState curried
  /// by the form widget. This is need by the Jets Data Table since
  /// it's a FormField and must have arguments context and formState erased.
  Widget makeFormField({
    required JetsRouteData screenPath,
    required JetsFormState state,
    required JetsFormFieldValidator formFieldValidator,
    required ValidatorDelegate formValidator,
    required FormActionsDelegate formActionsDelegate,
  });
}

class TextFieldConfig extends FormFieldConfig {
  TextFieldConfig({
    super.key = '',
    super.group = 0,
    super.flex = 1,
    super.autovalidateMode = AutovalidateMode.disabled,
    required this.label,
    this.leftMargin = 16.0,
    this.topMargin = 0.0,
    this.rightMargin = 16.0,
    this.bottomMargin = 0.0,
  });
  final String label;
  final double leftMargin;
  final double topMargin;
  final double rightMargin;
  final double bottomMargin;

  @override
  Widget makeFormField({
    required JetsRouteData screenPath,
    required JetsFormState state,
    required JetsFormFieldValidator formFieldValidator,
    required ValidatorDelegate formValidator,
    required FormActionsDelegate formActionsDelegate,
  }) {
    return JetsTextField(
      fieldConfig: this,
      flex: flex,
    );
  }
}

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
    required this.textRestriction,
    required this.maxLength,
  });
  final String label;
  final String hint;
  final bool autofocus;
  final bool obscureText;
  final TextRestriction textRestriction;
  // 0 for unbound
  final int maxLength;

  @override
  Widget makeFormField({
    required JetsRouteData screenPath,
    required JetsFormState state,
    required JetsFormFieldValidator formFieldValidator,
    required ValidatorDelegate formValidator,
    required FormActionsDelegate formActionsDelegate,
  }) {
    return JetsTextFormField(
      key: Key(key),
      formFieldConfig: this,
      onChanged: (p0) =>
          state.setValueAndNotify(group, key, p0.isNotEmpty ? p0 : null),
      formValidator: formFieldValidator,
      formState: state,
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
    required this.items,
  });
  final String? dropdownItemsQuery;
  final List<String> stateKeyPredicates;
  // save the returned model from query and put it in the form state cache if not null
  final String? returnedModelCacheKey;
  final int defaultItemPos;
  final List<DropdownItemConfig> items;
  bool dropdownItemLoaded = false;

  @override
  Widget makeFormField({
    required JetsRouteData screenPath,
    required JetsFormState state,
    required JetsFormFieldValidator formFieldValidator,
    required ValidatorDelegate formValidator,
    required FormActionsDelegate formActionsDelegate,
  }) {
    return JetsDropdownButtonFormField(
      key: Key(key),
      screenPath: screenPath,
      formFieldConfig: this,
      onChanged: (p0) => state.setValueAndNotify(group, key, p0),
      formValidator: formFieldValidator,
      formState: state,
    );
  }
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
    required JetsFormState state,
    required JetsFormFieldValidator formFieldValidator,
    required ValidatorDelegate formValidator,
    required FormActionsDelegate formActionsDelegate,
  }) {
    return JetsDropdownWithSharedItemsFormField(
      key: Key(key),
      screenPath: screenPath,
      formFieldConfig: this,
      onChanged: (p0) => state.setValueAndNotify(group, key, p0),
      formValidator: formFieldValidator,
      formState: state,
      selectedValue: state.getValue(group, key),
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
    required JetsFormState state,
    required JetsFormFieldValidator formFieldValidator,
    required ValidatorDelegate formValidator,
    required FormActionsDelegate formActionsDelegate,
  }) {
    return Expanded(
      child: SizedBox(
        width: tableWidth,
        height: tableHeight,
        child: JetsDataTableWidget(
          key: Key(key),
          screenPath: screenPath,
          formFieldConfig: this,
          tableConfig: getTableConfig(dataTableConfig),
          formState: state,
          formFieldValidator: formFieldValidator,
          dialogValidatorDelegate: formValidator,
          actionsDelegate: formActionsDelegate,
        ),
      ),
    );
  }
}

class FormActionConfig {
  FormActionConfig({
    required this.key,
    required this.label,
    required this.buttonStyle,
    this.enableOnlyWhenFormValid = false,
  });
  final String key;
  final String label;
  final ButtonStyle buttonStyle;
  final bool enableOnlyWhenFormValid;
}

final Map<String, FormConfig> _formConfigurations = {
  // Home Form (actionless)
  FormKeys.home: FormConfig(
    key: FormKeys.home,
    actions: [
      // Action-less form
    ],
    inputFields: [
      [
        FormDataTableFieldConfig(
            key: DTKeys.inputLoaderStatusTable,
            dataTableConfig: DTKeys.inputLoaderStatusTable)
      ],
      [
        FormDataTableFieldConfig(
            key: DTKeys.pipelineExecStatusTable,
            dataTableConfig: DTKeys.pipelineExecStatusTable)
      ],
      [
        FormDataTableFieldConfig(
            key: DTKeys.pipelineExecDetailsTable,
            dataTableConfig: DTKeys.pipelineExecDetailsTable)
      ],
    ],
  ),
  // Source Config (actionless)
  FormKeys.sourceConfig: FormConfig(
    key: FormKeys.sourceConfig,
    actions: [
      // Action-less form
    ],
    inputFields: [
      [
        FormDataTableFieldConfig(
            key: DTKeys.clientsTable,
            tableHeight: 400,
            dataTableConfig: DTKeys.clientsTable)
      ],
      [
        FormDataTableFieldConfig(
            key: DTKeys.sourceConfigsTable,
            tableHeight: 400,
            dataTableConfig: DTKeys.sourceConfigsTable)
      ],
    ],
  ),

  // Login Form
  FormKeys.login: FormConfig(
    key: FormKeys.login,
    actions: [
      FormActionConfig(
          key: ActionKeys.login,
          label: "Sign in",
          buttonStyle: ButtonStyle.primary),
      FormActionConfig(
          key: ActionKeys.register,
          label: "Register",
          buttonStyle: ButtonStyle.secondary),
    ],
    inputFields: [
      [
        FormInputFieldConfig(
            key: FSK.userEmail,
            label: "Email",
            hint: "Your email address",
            autofocus: true,
            obscureText: false,
            textRestriction: TextRestriction.allLower,
            maxLength: 80)
      ],
      [
        FormInputFieldConfig(
            key: FSK.userPassword,
            label: "Password",
            hint: "Your password",
            autofocus: false,
            obscureText: true,
            textRestriction: TextRestriction.none,
            maxLength: 80)
      ],
    ],
  ),
  // User Registration Form
  FormKeys.register: FormConfig(
    key: FormKeys.register,
    actions: [
      FormActionConfig(
          key: ActionKeys.register,
          label: "Register",
          buttonStyle: ButtonStyle.primary),
    ],
    inputFields: [
      [
        TextFieldConfig(label: 'This is the First Item'),
        FormInputFieldConfig(
            key: FSK.userName,
            label: "Name",
            hint: "Enter your name",
            flex: 2,
            autofocus: true,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 80), // ],
        // [
        FormInputFieldConfig(
            key: FSK.userEmail,
            label: "Email",
            hint: "Your email address",
            flex: 2,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.allLower,
            maxLength: 80),
        //* REMOVE THIS DEMO CODE
        FormDropdownFieldConfig(key: 'emailType', items: [
          DropdownItemConfig(label: 'Work', value: 'work'),
          DropdownItemConfig(label: 'Home', value: 'home'),
        ]),
      ],
      [
        TextFieldConfig(label: 'This is the Second Item'),
        FormInputFieldConfig(
            key: FSK.userPassword,
            label: "Password",
            hint: "Your password",
            flex: 2,
            autofocus: false,
            obscureText: true,
            textRestriction: TextRestriction.none,
            maxLength: 80), // ],
        // [
        FormInputFieldConfig(
            key: FSK.userPasswordConfirm,
            label: "Password Confirmation",
            hint: "Re-enter your password",
            flex: 2,
            autofocus: false,
            obscureText: true,
            textRestriction: TextRestriction.none,
            maxLength: 80),
        //* REMOVE THIS DEMO CODE
        FormDropdownFieldConfig(key: 'emailType', items: [
          DropdownItemConfig(label: ''),
          DropdownItemConfig(label: 'Work', value: 'work'),
          DropdownItemConfig(label: 'Home', value: 'home'),
        ]),
      ],
    ],
  ),
  // Add Client Dialog
  FormKeys.addClient: FormConfig(
    key: FormKeys.addClient,
    actions: [
      FormActionConfig(
          key: ActionKeys.clientOk,
          label: "Insert",
          buttonStyle: ButtonStyle.primary),
      FormActionConfig(
          key: ActionKeys.dialogCancel,
          label: "Cancel",
          buttonStyle: ButtonStyle.secondary),
    ],
    inputFields: [
      [
        FormInputFieldConfig(
            key: FSK.client,
            label: "Client Name",
            hint: "Client name as a short name",
            flex: 1,
            autofocus: true,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 20),
      ],
      [
        FormInputFieldConfig(
            key: FSK.details,
            label: "Details",
            hint: "Optional notes",
            flex: 1,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 80),
      ],
    ],
  ),
  // loadFile - Dialog to load file by client and file type
  FormKeys.loadFile: FormConfig(
    key: FormKeys.loadFile,
    actions: [
      FormActionConfig(
          key: ActionKeys.loaderOk,
          label: "Load",
          buttonStyle: ButtonStyle.primary),
      FormActionConfig(
          key: ActionKeys.dialogCancel,
          label: "Cancel",
          buttonStyle: ButtonStyle.secondary),
    ],
    inputFields: [
      [
        FormDropdownFieldConfig(
            key: FSK.client,
            items: [
              DropdownItemConfig(label: 'Select a client'),
            ],
            dropdownItemsQuery:
                "SELECT client FROM jetsapi.client_registry ORDER BY client ASC LIMIT 50"),
        FormDropdownFieldConfig(
            key: FSK.objectType,
            items: [
              DropdownItemConfig(label: 'Select a file type'),
            ],
            dropdownItemsQuery:
                "SELECT object_type FROM jetsapi.object_type_registry ORDER BY object_type ASC LIMIT 50"),
      ],
      [
        FormDropdownFieldConfig(
            key: FSK.fileKey,
            items: [
              DropdownItemConfig(label: 'Select a file key'),
            ],
            dropdownItemsQuery:
                "SELECT file_key FROM jetsapi.file_key_staging WHERE client = '{client}' AND object_type = '{object_type}' ORDER BY file_key ASC LIMIT 100",
            stateKeyPredicates: [FSK.client, FSK.objectType]),

        FormInputFieldConfig(
            key: FSK.groupingColumn,
            label: "Grouping Column",
            hint: "Column containing Member Key (optional)",
            flex: 1,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 60), // ],
      ],
    ],
  ),
  // Process Input Form (table as actionless form)
  FormKeys.processInput: FormConfig(
    key: FormKeys.processInput,
    actions: [
      // Action-less form
    ],
    inputFields: [
      [
        FormDataTableFieldConfig(
            key: DTKeys.processInputTable,
            dataTableConfig: DTKeys.processInputTable)
      ],
      [
        FormDataTableFieldConfig(
            key: DTKeys.processMappingTable,
            dataTableConfig: DTKeys.processMappingTable)
      ],
    ],
  ),
  // addProcessInput - Dialog to add process input
  FormKeys.addProcessInput: FormConfig(
    key: FormKeys.addProcessInput,
    actions: [
      FormActionConfig(
          key: ActionKeys.addProcessInputOk,
          label: "Add",
          buttonStyle: ButtonStyle.primary),
      FormActionConfig(
          key: ActionKeys.dialogCancel,
          label: "Cancel",
          buttonStyle: ButtonStyle.secondary),
    ],
    inputFields: [
      [
        FormDropdownFieldConfig(
            key: FSK.client,
            items: [
              DropdownItemConfig(label: 'Select a Client'),
            ],
            dropdownItemsQuery:
                "SELECT client FROM jetsapi.client_registry ORDER BY client ASC LIMIT 50"),
        FormDropdownFieldConfig(
            key: FSK.objectType,
            returnedModelCacheKey: FSK.objectTypeRegistryCache,
            items: [
              DropdownItemConfig(label: 'Select an Object Type'),
            ],
            dropdownItemsQuery:
                "SELECT object_type, entity_rdf_type FROM jetsapi.object_type_registry ORDER BY object_type ASC LIMIT 50"),
      ],
      [
        FormDropdownFieldConfig(
            key: FSK.sourceType,
            items: [
              DropdownItemConfig(label: 'File', value: 'file'),
              DropdownItemConfig(label: 'Domain Table', value: 'domain_table'),
            ],
            defaultItemPos: 0),
        FormDropdownFieldConfig(
            key: FSK.groupingColumn,
            items: [
              DropdownItemConfig(label: 'Select a Grouping Column'),
            ],
            dropdownItemsQuery:
                "SELECT column_name FROM information_schema.columns WHERE table_schema = 'public' AND table_name = '{table_name}' AND column_name NOT IN ('file_key','last_update','session_id','shard_id')",
            stateKeyPredicates: [FSK.tableName]),
      ],
    ],
  ),
  // processMapping - Dialog to mapping intake file structure to canonical model
  FormKeys.processMapping: FormConfig(
    key: FormKeys.processMapping,
    actions: [
      FormActionConfig(
          key: ActionKeys.mapperOk,
          label: "Save",
          enableOnlyWhenFormValid: true,
          buttonStyle: ButtonStyle.primary),
      FormActionConfig(
          key: ActionKeys.mapperDraft,
          label: "Save as Draft",
          buttonStyle: ButtonStyle.primary),
      FormActionConfig(
          key: ActionKeys.dialogCancel,
          label: "Cancel",
          buttonStyle: ButtonStyle.secondary),
    ],
    queries: {
      "inputFieldsQuery":
          "SELECT md.data_property, md.is_required, pm.input_column, pm.function_name, pm.argument, pm.default_value, pm.error_message FROM jetsapi.object_type_mapping_details md, jetsapi.process_mapping pm WHERE md.object_type = '{object_type}' AND table_name = '{table_name}' AND pm.data_property = md.data_property ORDER BY md.data_property ASC LIMIT 300",
      "inputColumnsQuery":
          "SELECT column_name FROM information_schema.columns WHERE table_schema = 'public' AND table_name = '{table_name}' AND column_name NOT IN ('file_key','last_update','session_id','shard_id')",
      "mappingFunctionsQuery":
          "SELECT function_name, is_argument_required FROM jetsapi.mapping_function_registry ORDER BY function_name ASC LIMIT 50",
    },
    inputFieldsQuery: "inputFieldsQuery",
    savedStateQuery: "inputFieldsQuery",
    dropdownItemsQueries: {
      FSK.inputColumnsDropdownItemsCache: "inputColumnsQuery",
      FSK.mappingFunctionsDropdownItemsCache: "mappingFunctionsQuery",
    },
    metadataQueries: {
      FSK.mappingFunctionDetailsCache: "mappingFunctionsQuery",
      FSK.inputColumnsCache: "inputColumnsQuery",
    },
    stateKeyPredicates: [FSK.objectType, FSK.tableName],
    inputFieldRowBuilder: (index, inputFieldRow, formState) {
      // savedState is List<String?>? with values as per savedStateQuery
      final savedState = formState.getCacheValue(FSK.savedStateCache) as List?;
      final isRequired = inputFieldRow[1]! == '1';
      final isRequiredIndicator = isRequired ? '*' : '';
      final savedInputColumn = savedState?[index][2];
      final inputColumnList =
          formState.getCacheValue(FSK.inputColumnsCache) as List;
      final inputColumnDefault =
          inputColumnList.contains(inputFieldRow[0]) ? inputFieldRow[0] : null;
      if (isRequired) formState.setValue(index, FSK.isRequiredFlag, "1");
      // set the default values to the formState
      formState.setValue(index, FSK.dataProperty, inputFieldRow[0]);
      formState.setValue(
          index, FSK.inputColumn, savedInputColumn ?? inputColumnDefault);
      formState.setValue(index, FSK.functionName, savedState?[index][3]);
      formState.setValue(index, FSK.functionArgument, savedState?[index][4]);
      formState.setValue(index, FSK.mappingDefaultValue, savedState?[index][5]);
      formState.setValue(index, FSK.mappingErrorMessage, savedState?[index][6]);
      // print("Form BUILDER savedState row ${savedState![index]}");
      return [
        [
          // data_property
          TextFieldConfig(
              label: "$index: ${inputFieldRow[0]}$isRequiredIndicator",
              group: index,
              flex: 1,
              topMargin: 20.0)
        ],
        [
          // input_column
          FormDropdownWithSharedItemsFieldConfig(
            key: FSK.inputColumn,
            group: index,
            flex: 2,
            autovalidateMode: AutovalidateMode.always,
            dropdownMenuItemCacheKey: FSK.inputColumnsDropdownItemsCache,
            defaultItem: savedInputColumn ?? inputColumnDefault,
          ),
          // function_name
          FormDropdownWithSharedItemsFieldConfig(
            key: FSK.functionName,
            group: index,
            flex: 1,
            dropdownMenuItemCacheKey: FSK.mappingFunctionsDropdownItemsCache,
            defaultItem: savedState?[index][3],
          ),
          // argument
          FormInputFieldConfig(
            key: FSK.functionArgument,
            label: "Function Argument",
            hint:
                "Cleansing function argument, it is either required or ignored",
            group: index,
            flex: 1,
            autovalidateMode: AutovalidateMode.always,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 512,
          ),
          // default_value
          FormInputFieldConfig(
            key: FSK.mappingDefaultValue,
            label: "Default Value",
            hint:
                "Default value to use if input value is not provided or cleansing function returns null",
            group: index,
            flex: 1,
            autovalidateMode: AutovalidateMode.always,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 512,
          ),
          // error_message
          FormInputFieldConfig(
            key: FSK.mappingErrorMessage,
            label: "Error Message",
            hint:
                "Error message to raise if input value is not provided or cleansing function returns null and there is no default value",
            group: index,
            flex: 1,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 125,
          ),
        ],
      ];
    },
  ),

  //* DEMO FORM
  "dataTableDemoForm": FormConfig(
    key: "dataTableDemoForm",
    actions: [
      FormActionConfig(
          key: "dataTableDemoAction1",
          label: "Do it!",
          buttonStyle: ButtonStyle.primary),
    ],
    inputFields: [
      [
        FormInputFieldConfig(
            key: "object_type",
            label: "S3 Folder",
            hint: "Folder where the files are dropped",
            flex: 2,
            autofocus: true,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 80), // ],
        FormDropdownFieldConfig(
            key: FSK.client,
            items: [
              DropdownItemConfig(label: ''),
            ],
            dropdownItemsQuery:
                "SELECT client FROM jetsapi.client_registry ORDER BY client ASC LIMIT 50"),
      ],
      [
        FormInputFieldConfig(
            key: "table_name",
            label: "Client Table Name",
            hint: "Table where client file is loaded into",
            flex: 2,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 60), // ],
        FormInputFieldConfig(
            key: "grouping_column",
            label: "Grouping Column",
            hint: "Column containing Member Key",
            flex: 1,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLength: 60), // ],
      ],
      [
        FormDataTableFieldConfig(
            key: "dataTableDemoMainTable",
            dataTableConfig: "dataTableDemoMainTableConfig")
      ],
      [
        FormDataTableFieldConfig(
            key: "dataTableDemoSupportTable",
            dataTableConfig: "dataTableDemoSupportTableConfig")
      ],
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
