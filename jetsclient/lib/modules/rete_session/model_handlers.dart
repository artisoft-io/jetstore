import 'dart:convert';

import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/utils/constants.dart';

Map<String, dynamic>? reteSessionEntityKeyStateHandler(
    JetsFormState formState) {
  final entityType = formState.getValue(0, FSK.entityRdfType);
  if (entityType == null) return null;
  final model = formState.getValue(0, FSK.reteSessionEntityKeyByType);
  if (model == null) return null;
  final entityKeys = model[entityType[0]];
  if (entityKeys == null) return null;
  return <String, dynamic>{'rows': json.decode(entityKeys)};
}

Map<String, dynamic>? reteSessionEntityDetailsStateHandler(
    JetsFormState formState) {
  final entityKey = formState.getValue(0, FSK.entityKey);
  if (entityKey == null) return null;
  final model = formState.getValue(0, FSK.reteSessionEntityDetailsByKey);
  if (model == null) return null;
  final entityDetails = model[entityKey[0]];
  if (entityDetails == null) return null;
  return <String, dynamic>{'rows': json.decode(entityDetails)};
}
