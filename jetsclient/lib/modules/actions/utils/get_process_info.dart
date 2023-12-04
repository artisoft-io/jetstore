import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/modules/actions/delegate_helpers.dart';
import 'package:jetsclient/utils/constants.dart';

Future<Map<String, dynamic>?> getProcessInputRdfTypes(final BuildContext context,
    final JetsFormState formState, final String processName) async {
  var rawQuery = <String, dynamic>{
    'action': 'raw_query',
  };
  rawQuery['query'] =
      "SELECT key, input_rdf_types FROM jetsapi.process_config WHERE process_name = '$processName'";
  final rows = await queryJetsDataModel(
      context, formState, ServerEPs.dataTableEP, json.encode(rawQuery), silent: true);
  if (rows == null) {
    return null;
  }
  return {FSK.processConfigKey: rows[0][0], FSK.entityRdfType: rows[0][1]};
}
