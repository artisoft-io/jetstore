import 'package:jetsclient/components/jets_form_state.dart';
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
        PaddingConfig(height: 2*defaultPadding),
      ],
      [
        FormDataTableFieldConfig(
            key: FSK.scAddOrEditSourceConfigOption,
            dataTableConfig: FSK.scAddOrEditSourceConfigOption),
      ],
    ],
    formValidatorDelegate: configureFilesFormValidator,
    formActionsDelegate: doNothingAction, // overriden by UserFlowState.actionDelegate
  ),
  FormKeys.scAddSourceConfigUF: FormConfig(
    key: FormKeys.scAddSourceConfigUF,
    useListView: true,
    actions: standardActions,
    inputFields: [
      [
        PaddingConfig(height: 2*defaultPadding),
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
        PaddingConfig(height: 2*defaultPadding),
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
        PaddingConfig(height: 2*defaultPadding),
      ],
    ],
    formValidatorDelegate: configureFilesFormValidator,
    formActionsDelegate: doNothingAction, // overriden by UserFlowState.actionDelegate
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
    formActionsDelegate: doNothingAction, // overriden by UserFlowState.actionDelegate
  ),
  FormKeys.scCsvOrFixedSourceConfigUF: FormConfig(
    key: FormKeys.scCsvOrFixedSourceConfigUF,
    useListView: true,
    actions: standardActions,
    inputFields: [
      [
        PaddingConfig(height: 2*defaultPadding),
      ],
      [
        FormDataTableFieldConfig(
            key: FSK.scCsvOrFixedOption,
            dataTableConfig: FSK.scCsvOrFixedOption),
      ],
    ],
    formValidatorDelegate: configureFilesFormValidator,
    formActionsDelegate: doNothingAction, // overriden by UserFlowState.actionDelegate
  ),
  FormKeys.scEditCsvHeadersUF: FormConfig(
    key: FormKeys.scEditCsvHeadersUF,
    useListView: true,
    actions: standardActions,
    inputFields: [
      [
        PaddingConfig(height: 2*defaultPadding),
      ],
      [
        TextFieldConfig(
            label: "Paste or enter the CSV headers as a json array:",
            maxLines: 1,
            topMargin: 0,
            bottomMargin: 0),
      ],
      [
        FormInputFieldConfig(
            key: FSK.inputColumnsJson,
            label: "Input file column names (json)",
            hint: "Input file column names, only for headerless files",
            flex: 1,
            autofocus: false,
            obscureText: false,
            textRestriction: TextRestriction.none,
            maxLines: 13,
            maxLength: 51200),
      ],
      [
        PaddingConfig(height: 2*defaultPadding),
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
        PaddingConfig(height: 2*defaultPadding),
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
        PaddingConfig(height: 2*defaultPadding),
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
        PaddingConfig(height: 2*defaultPadding),
      ],
      [
        TextFieldConfig(
            label: "Paste or enter the Domain Keys definition as json (leave empty if there is no need to group lines together while executing rules):",
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
        PaddingConfig(height: 2*defaultPadding),
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
        PaddingConfig(height: 2*defaultPadding),
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
        PaddingConfig(height: 2*defaultPadding),
      ],
    ],
    formValidatorDelegate: configureFilesFormValidator,
    formActionsDelegate: doNothingAction,
  ),
  FormKeys.scEditAutomatedModeUF: FormConfig(
    key: FormKeys.scEditAutomatedModeUF,
    useListView: true,
    actions: [
      FormActionConfig(
          key: ActionKeys.ufPrevious,
          label: "Previous",
          buttonStyle: ActionStyle.ufPrimary,
          leftMargin: defaultPadding,
          rightMargin: betweenTheButtonsPadding),
      FormActionConfig(
          key: ActionKeys.ufContinueLater,
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
    inputFields: [
      [
        PaddingConfig(height: 2*defaultPadding),
      ],
      [
        TextFieldConfig(
            label: "Select if the files will be loaded manually or automatically from S3:",
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
            defaultItemPos: 0),
      ],
      [
        PaddingConfig(height: 2*defaultPadding),
      ],
    ],
    formValidatorDelegate: configureFilesFormValidator,
    formActionsDelegate: doNothingAction,
  ),
};

FormConfig? getConfigureFileFormConfig(String key) {
  return _formConfigurations[key];
}
