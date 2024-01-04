import 'package:jetsclient/models/user_flow_config.dart';
import 'package:jetsclient/modules/form_config_impl.dart';
import 'package:jetsclient/modules/user_flows/configure_files/form_action_delegates.dart';
import 'package:jetsclient/utils/constants.dart';

final Map<String, UserFlowConfig> _userFlowConfigurations = {
  //
  UserFlowKeys.sourceConfigUF:
      UserFlowConfig(startAtKey: "select_add_or_edit", states: {
    "select_add_or_edit": UserFlowState(
        key: "select_add_or_edit",
        description: 'Select between add or edit source_config',
        formConfig: getFormConfig(FormKeys.scAddOrEditSourceConfigUF),
        actionDelegate: configureFilesFormActions,
        choices: [
          Expression(
              lhsStateKey: FSK.scAddOrEditSourceConfigOption,
              op: Operator.equals,
              rhsValue: FSK.ufAddOption,
              isRhsStateKey: false,
              nextState: 'add_source_config'),
          Expression(
              lhsStateKey: FSK.scAddOrEditSourceConfigOption,
              op: Operator.equals,
              rhsValue: FSK.ufEditOption,
              isRhsStateKey: false,
              nextState: 'select_source_config'),
        ]),
    "add_source_config": UserFlowState(
        key: "add_source_config",
        description: 'Add Source Config',
        formConfig: getFormConfig(FormKeys.scAddSourceConfigUF),
        actionDelegate: configureFilesFormActions,
        stateAction: ActionKeys.scAddSourceConfigUF,
        defaultNextState: "select_file_type_option"),
    "select_source_config": UserFlowState(
        key: "select_source_config",
        description: 'Select an existing Source Config',
        formConfig: getFormConfig(FormKeys.scSelectSourceConfigUF),
        actionDelegate: configureFilesFormActions,
        stateAction: ActionKeys.scSelectSourceConfigUF,
        defaultNextState: "select_file_type_option"),
    "select_file_type_option": UserFlowState(
        key: "select_file_type_option",
        description: 'Select file type: csv, parquet, fixed-width, etc.',
        formConfig: getFormConfig(FormKeys.scSourceConfigTypeUF),
        actionDelegate: configureFilesFormActions,
        choices: [
          Expression(
              lhsStateKey: FSK.scFileTypeOption,
              op: Operator.equals,
              rhsValue: FSK.scCsvOption,
              isRhsStateKey: false,
              nextState: 'select_single_or_multi_part_file'),
          Expression(
              lhsStateKey: FSK.scFileTypeOption,
              op: Operator.equals,
              rhsValue: FSK.scHeaderlessCsvOption,
              isRhsStateKey: false,
              nextState: 'edit_csv_headers'),
          Expression(
              lhsStateKey: FSK.scFileTypeOption,
              op: Operator.equals,
              rhsValue: FSK.scXlsxOption,
              isRhsStateKey: false,
              nextState: 'edit_xlsx_options'),
          Expression(
              lhsStateKey: FSK.scFileTypeOption,
              op: Operator.equals,
              rhsValue: FSK.scHeaderlessXlsxOption,
              isRhsStateKey: false,
              nextState: 'edit_xlsx_options'),
          Expression(
              lhsStateKey: FSK.scFileTypeOption,
              op: Operator.equals,
              rhsValue: FSK.scFixedWidthOption,
              isRhsStateKey: false,
              nextState: 'edit_fixed_width_layout'),
          Expression(
              lhsStateKey: FSK.scFileTypeOption,
              op: Operator.equals,
              rhsValue: FSK.scParquetOption,
              isRhsStateKey: false,
              nextState: 'select_single_or_multi_part_file'),
          Expression(
              lhsStateKey: FSK.scFileTypeOption,
              op: Operator.equals,
              rhsValue: FSK.scParquetSelectOption,
              isRhsStateKey: false,
              nextState: 'edit_csv_headers'),
        ]),
    "edit_xlsx_options": UserFlowState(
        key: "edit_xlsx_options",
        description: 'Specify additional options for xlsx files',
        formConfig: getFormConfig(FormKeys.scEditXlsxOptionsUF),
        actionDelegate: configureFilesFormActions,
        stateAction: ActionKeys.scEditXlsxOptionsUF,
        choices: [
          Expression(
              lhsStateKey: FSK.scFileTypeOption,
              op: Operator.equals,
              rhsValue: FSK.scHeaderlessXlsxOption,
              isRhsStateKey: false,
              nextState: 'edit_csv_headers'),
        ],
        defaultNextState: "select_single_or_multi_part_file"),
    "edit_csv_headers": UserFlowState(
        key: "edit_csv_headers",
        description:
            'Specify Headers for Headerless CSV/XLSX or Parquet Select Options',
        formConfig: getFormConfig(FormKeys.scEditFileHeadersUF),
        actionDelegate: configureFilesFormActions,
        defaultNextState: "select_single_or_multi_part_file"),
    "edit_fixed_width_layout": UserFlowState(
        key: "edit_fixed_width_layout",
        description: 'Edit Source Config Fixed Width Layout',
        formConfig: getFormConfig(FormKeys.scEditFixedWidthLayoutUF),
        actionDelegate: configureFilesFormActions,
        defaultNextState: "select_single_or_multi_part_file"),
    "select_single_or_multi_part_file": UserFlowState(
        key: "select_single_or_multi_part_file",
        description: 'Select between single file or multi-part files',
        formConfig: getFormConfig(FormKeys.scSelectSingleOrMultiPartFileUF),
        actionDelegate: configureFilesFormActions,
        defaultNextState: "edit_domain_keys"),
    "edit_domain_keys": UserFlowState(
        key: "edit_domain_keys",
        description: 'Edit Source Config Domain Keys',
        formConfig: getFormConfig(FormKeys.scEditDomainKeysUF),
        actionDelegate: configureFilesFormActions,
        defaultNextState: "edit_code_value_mapping"),
    "edit_code_value_mapping": UserFlowState(
        key: "edit_code_value_mapping",
        description: 'Edit Code Value Mapping',
        formConfig: getFormConfig(FormKeys.scEditCodeValueMappingUF),
        actionDelegate: configureFilesFormActions,
        defaultNextState: "edit_automated_mode"),
    "edit_automated_mode": UserFlowState(
        key: "edit_automated_mode",
        description: 'Edit Automated Mode',
        formConfig: getFormConfig(FormKeys.scEditAutomatedModeUF),
        actionDelegate: configureFilesFormActions,
        defaultNextState: "confirm_state"),
    "confirm_state": UserFlowState(
        key: "confirm_state",
        description: 'Confirm changes',
        formConfig: getFormConfig(FormKeys.scSummaryUF),
        actionDelegate: configureFilesFormActions,
        stateAction: ActionKeys.addSourceConfigOk,
        isEnd: true),
  })
};

UserFlowConfig? getConfigureFilesUserFlowConfig(String key) {
  return _userFlowConfigurations[key];
}
