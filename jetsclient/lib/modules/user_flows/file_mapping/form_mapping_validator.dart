import 'package:flutter/material.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/modules/actions/delegate_helpers.dart';

/// Process Input Form / Dialog Validator
String? mappingFormValidator(
    JetsFormState formState, int group, String key, dynamic v) {
  assert((v is String?) || (v is List<String>?),
      "Process Input Form has unexpected data type");
  var isRequired = formState.getValue(group, FSK.isRequiredFlag);
  // print(
  //     "%%% Validator Called for $group ($isRequired), $key, $v, state is ${formState.getValue(group, key)}");
  // Check if we have client, object_type, and source_type to populate table_name
  // add entity_rdf_type based on object_type
  var objectTypeRegistry =
      formState.getCacheValue(FSK.objectTypeRegistryCache) as List?;
  var client = formState.getValue(group, FSK.client);
  var sourceType = formState.getValue(group, FSK.sourceType);
  var entityRdfType = formState.getValue(group, FSK.entityRdfType);
  if (objectTypeRegistry != null &&
      client != null &&
      sourceType != null &&
      entityRdfType != null) {
    switch (sourceType) {
      case 'file':
        final org = formState.getValue(group, FSK.org);
        if (org != null) {
          var row = objectTypeRegistry.firstWhere((e) => e[1] == entityRdfType,
              orElse: () => [entityRdfType as String, entityRdfType]);
          if (row == null) {
            print(
                "processInputFormActions error: can't find object_type in objectTypeRegistry");
          } else {
            // add table_name to form state based on source_type of domain class (rdf:type)
            String tableName = makeTableName(client, org, row[0]);
            if (formState.getValue(0, FSK.tableName) != tableName) {
              // print("SET AND NOTIFY TABLENAME $tableName");
              formState.setValueAndNotify(0, FSK.tableName, tableName);
            }
          }
        }
        break;
      case 'domain_table':
        if (formState.getValue(group, FSK.tableName) != entityRdfType) {
          formState.setValueAndNotify(group, FSK.tableName, entityRdfType);
        }
        break;
      case 'alias_domain_table':
        // Do nothing, table_name is already in formState
        break;
      default:
        print(
            "processInputFormActions error: unknown source_type: $sourceType");
    }
  }

  // Check if we need to refresh the token - case of long running form
  if (JetsRouterDelegate().user.isTokenAged) {
    HttpClientSingleton().refreshToken();
  }

  switch (key) {
    // Add/Update Process Input Dialog Validations
    case FSK.client:
      String? value = v;
      if (value != null && value.characters.length > 1) {
        return null;
      }
      return "Client name must be provided.";
    case FSK.org:
      if (v != null) {
        return null;
      }
      if (sourceType == null || sourceType != 'file') {
        return null;
      }
      return "Organization must be selected.";

    case FSK.objectType:
      String? value = v;
      if (value != null && value.characters.length > 1) {
        return null;
      }
      return "Object Type name must be selected.";
    case FSK.sourceType:
      String? value = v;
      if (value != null && value.characters.length > 1) {
        return null;
      }
      return "Source Type name must be selected.";
    case FSK.entityRdfType:
      String? value = v;
      if (value != null && value.characters.length > 1) {
        return null;
      }
      return "Domain Class name must be selected.";
    case FSK.lookbackPeriods:
      String? value = v;
      if (value != null && value.characters.isNotEmpty) {
        return null;
      }
      return "Lookback period must be provided.";
    case FSK.tableName:
      String? value = v;
      if (value != null && value.characters.isNotEmpty) {
        return null;
      }
      if (sourceType == null || sourceType != 'alias_domain_table') {
        return null;
      }
      return "Table name must be provided.";

    // Process Mapping Dialog Validation
    case FSK.inputColumn:
      String? value = v;
      if (value != null && value.isNotEmpty) {
        // Check that the input column is among the file columns
        if(formState
            .getCacheValue(FSK.inputColumnsDropdownItemsCache)
            .where((item) => item == value)
            .toList().isEmpty) {
              formState.markFormKeyAsInvalid(group, key);
              return "Input Column is not valid.";
            }
        formState.markFormKeyAsValid(group, key);
        return null;
      }
      if (isRequired == null || isRequired == false) {
        formState.markFormKeyAsValid(group, key);
        return null;
      }
      final defaultValue =
          formState.getValue(group, FSK.mappingDefaultValue) as String?;
      if (defaultValue != null && defaultValue.isNotEmpty) {
        formState.markFormKeyAsValid(group, key);
        return null;
      }
      var errorMsg =
          formState.getValue(group, FSK.mappingErrorMessage) as String?;
      if (errorMsg != null && errorMsg.isNotEmpty) {
        formState.markFormKeyAsValid(group, key);
        return null;
      }
      formState.markFormKeyAsInvalid(group, key);
      return "Input Column must be selected or either a default or an error message must be provided.";

    case FSK.functionName:
      return null;

    case FSK.functionArgument:
      String? value = v;
      var functionName = formState.getValue(group, FSK.functionName) as String?;
      // print("Validating argument '$value' for function $functionName");
      if (functionName == null || functionName.isEmpty) {
        if (value != null && value.isNotEmpty) {
          formState.markFormKeyAsInvalid(group, key);
          return "Remove the argument when no function is selected";
        } else {
          formState.markFormKeyAsValid(group, key);
          return null;
        }
      }
      if (value != null && value.isNotEmpty) {
        formState.markFormKeyAsValid(group, key);
        return null;
      }
      var mappingFunctionDetails =
          formState.getCacheValue(FSK.mappingFunctionDetailsCache) as List?;
      assert(mappingFunctionDetails != null,
          "processInputFormActions error: mappingFunctionDetails is null");
      if (mappingFunctionDetails == null) {
        formState.markFormKeyAsValid(group, key);
        return null;
      }
      var row = mappingFunctionDetails.firstWhere(
        (e) => e[0] == functionName,
      );
      // check if function argument is required
      if (row[1] != "1") {
        formState.markFormKeyAsValid(group, key);
        return null;
      }
      formState.markFormKeyAsInvalid(group, key);
      return "Cleansing function argument is required";

    case FSK.mappingDefaultValue:
      String? value = v;
      if (value != null && value.isEmpty) {
        value = null;
      }
      var errorMsg =
          formState.getValue(group, FSK.mappingErrorMessage) as String?;
      if (errorMsg != null && errorMsg.isEmpty) {
        errorMsg = null;
      }
      if (value != null && errorMsg == null) {
        formState.markFormKeyAsValid(group, key);
        return null;
      }
      if (value == null && errorMsg != null) {
        formState.markFormKeyAsValid(group, key);
        return null;
      }
      if (value != null && errorMsg != null) {
        formState.markFormKeyAsInvalid(group, key);
        return "Cannot specify both a default value and an error message";
      }
      formState.markFormKeyAsValid(group, key);
      return null;

    case FSK.mappingErrorMessage:
      return null;

    default:
      print(
          'Oops process input form has no validator configured for form field $key, which has value $v');
  }
  return null;
}
