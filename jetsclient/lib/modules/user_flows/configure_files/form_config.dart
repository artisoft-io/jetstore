import 'package:jetsclient/models/user_flow_config.dart';
import 'package:jetsclient/modules/user_flows/configure_files/form_action_delegates.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/form_config.dart';

/// Form Config for Source Config UF Module

final Map<String, FormConfig> _formConfigurations = {
  FormKeys.scAddOrEditSourceConfigUF: FormConfig(
    key: FormKeys.scAddOrEditSourceConfigUF,
    useListView: true,
    actions: standardActions,
    inputFields: [
      [
        PaddingConfig(height: 2 * defaultPadding),
      ],
      [
        FormDataTableFieldConfig(
            key: FSK.scAddOrEditSourceConfigOption,
            dataTableConfig: FSK.scAddOrEditSourceConfigOption),
      ],
    ],
    formValidatorDelegate: configureFilesFormValidator,
    formActionsDelegate:
        doNothingAction, // overriden by UserFlowState.actionDelegate
  ),
  FormKeys.scSelectSingleOrMultiPartFileUF: FormConfig(
    key: FormKeys.scSelectSingleOrMultiPartFileUF,
    useListView: true,
    actions: standardActions,
    inputFields: [
      [
        PaddingConfig(height: 2 * defaultPadding),
      ],
      [
        FormDataTableFieldConfig(
            key: FSK.scSingleOrMultiPartFileOption,
            dataTableConfig: FSK.scSingleOrMultiPartFileOption),
      ],
    ],
    formValidatorDelegate: configureFilesFormValidator,
    formActionsDelegate:
        doNothingAction, // overriden by UserFlowState.actionDelegate
  ),
  FormKeys.scAddSourceConfigUF: FormConfig(
    key: FormKeys.scAddSourceConfigUF,
    useListView: true,
    actions: standardActions,
    inputFields: [
      [
        PaddingConfig(height: 2 * defaultPadding),
      ],
      [
        FormDropdownFieldConfig(
            key: FSK.client,
            items: [
              DropdownItemConfig(label: 'Select a Client'),
            ],
            dropdownItemsQuery:
                "SELECT client FROM jetsapi.client_registry ORDER BY client ASC LIMIT 200"),
        FormDropdownFieldConfig(
            key: FSK.org,
            items: [
              DropdownItemConfig(label: 'Select an Organization'),
              DropdownItemConfig(label: 'No Organization', value: ''),
            ],
            dropdownItemsQuery:
                "SELECT org FROM jetsapi.client_org_registry WHERE client = '{client}' ORDER BY org ASC LIMIT 100",
            stateKeyPredicates: [FSK.client]),
      ],
      [
        PaddingConfig(height: 2 * defaultPadding),
      ],
      [
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
        PaddingConfig(height: 2 * defaultPadding),
      ],
    ],
    formValidatorDelegate: configureFilesFormValidator,
    formActionsDelegate:
        doNothingAction, // overriden by UserFlowState.actionDelegate
  ),
  FormKeys.scSelectSourceConfigUF: FormConfig(
    key: FormKeys.scSelectSourceConfigUF,
    actions: standardActions,
    inputFields: [
      [
        FormDataTableFieldConfig(
            key: FSK.scSourceConfigKey,
            tableHeight: double.infinity,
            dataTableConfig: FSK.scSourceConfigKey)
      ],
    ],
    formValidatorDelegate: configureFilesFormValidator,
    formActionsDelegate:
        doNothingAction, // overriden by UserFlowState.actionDelegate
  ),
  FormKeys.scSourceConfigTypeUF: FormConfig(
    key: FormKeys.scSourceConfigTypeUF,
    useListView: true,
    actions: standardActions,
    inputFields: [
      [
        PaddingConfig(height: 2 * defaultPadding),
      ],
      [
        FormDataTableFieldConfig(
            key: FSK.scFileTypeOption, dataTableConfig: FSK.scFileTypeOption),
      ],
    ],
    formValidatorDelegate: configureFilesFormValidator,
    formActionsDelegate:
        doNothingAction, // overriden by UserFlowState.actionDelegate
  ),
  FormKeys.scEditXlsxOptionsUF: FormConfig(
    key: FormKeys.scEditXlsxOptionsUF,
    useListView: true,
    actions: standardActions,
    inputFields: [
      [
        PaddingConfig(height: 2 * defaultPadding),
      ],
      [
        TextFieldConfig(
            label:
                "Enter the sheet name or position containing the data,\nthe first sheet is at position 0:",
            maxLines: 2,
            topMargin: 0,
            bottomMargin: 0),
      ],
      [
        FormInputFieldConfig(
            key: FSK.scCurrentSheet,
            label: "Sheet name or position containing the data (position starts at 0)",
            hint:
                "Sheet name or position containing the data (position starts at 0)",
            flex: 1,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLines: 1,
            maxLength: 256),
      ],
      [
        PaddingConfig(height: 2 * defaultPadding),
      ],
    ],
    formValidatorDelegate: configureFilesFormValidator,
    formActionsDelegate: doNothingAction,
  ),
  FormKeys.scEditFileHeadersUF: FormConfig(
    key: FormKeys.scEditFileHeadersUF,
    useListView: true,
    actions: standardActions,
    inputFields: [
      [
        PaddingConfig(height: 2 * defaultPadding),
      ],
      [
        TextFieldConfig(
            label:
                "Paste or enter the file headers as a json array:",
            maxLines: 2,
            topMargin: 0,
            bottomMargin: 0),
      ],
      [
        FormInputFieldConfig(
            key: FSK.inputColumnsJson,
            label: "Input file column names (json)",
            hint:
                "Input file column names, for csv headerless or parquet files",
            flex: 1,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLines: 13,
            maxLength: 51200),
      ],
      [
        PaddingConfig(height: 2 * defaultPadding),
      ],
    ],
    formValidatorDelegate: configureFilesFormValidator,
    formActionsDelegate: doNothingAction,
  ),
  FormKeys.scEditFixedWidthLayoutUF: FormConfig(
    key: FormKeys.scEditFixedWidthLayoutUF,
    useListView: true,
    actions: standardActions,
    inputFields: [
      [
        PaddingConfig(height: 2 * defaultPadding),
      ],
      [
        TextFieldConfig(
            label: "Paste or enter the fixed-width column layout as CSV:",
            maxLines: 1,
            topMargin: 0,
            bottomMargin: 0),
      ],
      [
        FormInputFieldConfig(
            key: FSK.inputColumnsPositionsCsv,
            label: "Column names and positions (csv)",
            hint: "Input file column names, only for fixed-width files",
            flex: 1,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLines: 13,
            maxLength: 51200),
      ],
      [
        PaddingConfig(height: 2 * defaultPadding),
      ],
    ],
    formValidatorDelegate: configureFilesFormValidator,
    formActionsDelegate: doNothingAction,
  ),
  FormKeys.scEditDomainKeysUF: FormConfig(
    key: FormKeys.scEditDomainKeysUF,
    useListView: true,
    actions: standardActions,
    inputFields: [
      [
        PaddingConfig(height: 2 * defaultPadding),
      ],
      [
        TextFieldConfig(
            label:
                "Paste or enter the Domain Keys definition as json (leave empty if there is no need to group lines together while executing rules):",
            maxLines: 2,
            topMargin: 0,
            bottomMargin: 0),
      ],
      [
        FormInputFieldConfig(
            key: FSK.domainKeysJson,
            label: "Domain Key(s) (json)",
            hint: "Column(s) making the key of the Master Item",
            flex: 1,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLines: 13,
            maxLength: 51200),
      ],
      [
        PaddingConfig(height: 2 * defaultPadding),
      ],
    ],
    formValidatorDelegate: configureFilesFormValidator,
    formActionsDelegate: doNothingAction,
  ),
  FormKeys.scEditCodeValueMappingUF: FormConfig(
    key: FormKeys.scEditCodeValueMappingUF,
    useListView: true,
    actions: standardActions,
    inputFields: [
      [
        PaddingConfig(height: 1 * defaultPadding),
      ],
      [
        TextFieldConfig(
            label: "Paste or enter the Code Values mapping as json or csv:",
            maxLines: 1,
            topMargin: 0,
            bottomMargin: 0),
      ],
      [
        FormInputFieldConfig(
            key: FSK.codeValuesMappingJson,
            label: "Code Values Mapping (csv or json)",
            hint: "Client-Specific Code Values Mapping to Canonical Codes",
            flex: 1,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLines: 13,
            maxLength: 51200),
      ],
      [
        PaddingConfig(height: 2 * defaultPadding),
      ],
    ],
    formValidatorDelegate: configureFilesFormValidator,
    formActionsDelegate: doNothingAction,
  ),
  FormKeys.scEditAutomatedModeUF: FormConfig(
    key: FormKeys.scEditAutomatedModeUF,
    useListView: true,
    actions: standardActions,
    inputFields: [
      [
        PaddingConfig(height: 2 * defaultPadding),
      ],
      [
        TextFieldConfig(
            label:
                "Select if the files will be loaded manually or automatically from S3:",
            maxLines: 1,
            topMargin: 0,
            bottomMargin: 0),
      ],
      [
        FormDropdownFieldConfig(
            key: FSK.automated,
            items: [
              DropdownItemConfig(label: 'Select Automation Status...'),
              DropdownItemConfig(label: 'Automated', value: '1'),
              DropdownItemConfig(label: 'Manual', value: '0'),
            ],
            flex: 1,
            defaultItemPos: 1),
      ],
      [
        PaddingConfig(height: 2 * defaultPadding),
      ],
    ],
    formValidatorDelegate: configureFilesFormValidator,
    formActionsDelegate: doNothingAction,
  ),
  // Summary Page
  FormKeys.scSummaryUF: FormConfig(
    key: FormKeys.scSummaryUF,
    title: "File Configuration Summary",
    actions: [
      FormActionConfig(
          key: ActionKeys.ufPrevious,
          label: "Previous",
          buttonStyle: ActionStyle.ufPrimary,
          leftMargin: defaultPadding,
          rightMargin: betweenTheButtonsPadding),
      FormActionConfig(
          key: ActionKeys.ufCancel,
          label: "Cancel",
          buttonStyle: ActionStyle.ufPrimary,
          leftMargin: betweenTheButtonsPadding,
          rightMargin: defaultPadding),
      FormActionConfig(
          key: ActionKeys.ufCompleted,
          label: "Save & Done",
          buttonStyle: ActionStyle.ufSecondary,
          leftMargin: betweenTheButtonsPadding,
          rightMargin: defaultPadding),
    ],
    useListView: true,
    inputFields: [
      [
        FormInputFieldConfig(
            key: FSK.client,
            label: "Client",
            hint: "",
            flex: 1,
            isReadOnly: true,
            autofocus: false,
            obscureText: false,
            maxLength: 20,
            textRestriction: TextRestriction.none,
            useDefaultFont: true),
        FormInputFieldConfig(
            key: FSK.org,
            label: "Vendor/Org",
            hint: "",
            flex: 1,
            isReadOnly: true,
            autofocus: false,
            obscureText: false,
            maxLength: 20,
            textRestriction: TextRestriction.none,
            useDefaultFont: true),
      ],
      [
        FormInputFieldConfig(
            key: FSK.objectType,
            label: "Object Type",
            hint: "Type of entity in the file",
            flex: 2,
            isReadOnly: true,
            autofocus: false,
            obscureText: false,
            maxLength: 40,
            textRestriction: TextRestriction.none,
            useDefaultFont: true),
        FormInputFieldConfig(
            key: FSK.tableName,
            label: "Staging Table Name",
            hint: "",
            flex: 3,
            isReadOnly: true,
            autofocus: false,
            obscureText: false,
            maxLength: 100,
            textRestriction: TextRestriction.none,
            useDefaultFont: true),
      ],
      [
        FormDropdownFieldConfig(
            key: FSK.scFileTypeOption,
            isReadOnly: true,
            items: [
              DropdownItemConfig(label: 'CSV', value: 'csv'),
              DropdownItemConfig(
                  label: 'Headerless CSV', value: 'headerless_csv'),
              DropdownItemConfig(
                  label: 'Headerless CSV (with a Schema Provider)', value: 'headerless_csv_with_schema_provider'),
              DropdownItemConfig(label: 'XLSX', value: 'xlsx'),
              DropdownItemConfig(
                  label: 'Headerless XLSX', value: 'headerless_xlsx'),
              DropdownItemConfig(
                  label: 'Fixed-Width Columns', value: 'fixed_width'),
              DropdownItemConfig(
                  label: 'Fixed-Width Columns (with a Schema Provider)', value: 'fixed_width_with_schema_provider'),
              DropdownItemConfig(label: 'Parquet', value: 'parquet'),
              DropdownItemConfig(
                  label: 'Parquet, Selected Columns', value: 'parquet_select'),
            ],
            flex: 1,
            defaultItemPos: 0),
        FormDropdownFieldConfig(
            key: FSK.scSingleOrMultiPartFileOption,
            isReadOnly: true,
            items: [
              DropdownItemConfig(
                  label: 'Single File', value: 'scSingleFileOption'),
              DropdownItemConfig(
                  label: 'Multi-Part Files', value: 'scMultiPartFileOption'),
            ],
            flex: 1,
            defaultItemPos: 0),
      ],
      [
        FormDropdownFieldConfig(
            key: FSK.automated,
            isReadOnly: true,
            items: [
              DropdownItemConfig(label: 'Select automation mode'),
              DropdownItemConfig(label: 'Automated', value: '1'),
              DropdownItemConfig(label: 'Manual', value: '0'),
            ],
            flex: 1,
            defaultItemPos: 0),
      ],
    ],
    formValidatorDelegate: configureFilesFormValidator,
    formActionsDelegate: doNothingAction,
  ),
};

FormConfig? getConfigureFileFormConfig(String key) {
  return _formConfigurations[key];
}
