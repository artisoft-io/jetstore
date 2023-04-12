import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/screens/components/dialogs.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/screens/screen_delegates/delegate_helpers.dart';

/// User Administration Form Actions
Future<String?> processErrorsActions(BuildContext context,
    GlobalKey<FormState> formKey, JetsFormState formState, String actionKey,
    {int group = 0}) async {
  switch (actionKey) {
    case ActionKeys.setupShowInputRecords:
      // Preparing to view input records from process_error table by domain_key
      // Need to:
      //  For each input source of the pipeline identified by pipeline_execution_status.key:
      //  - get the list of session_id that participated in pipeline
      //  - package list of domian_keys selected by user into an pg encoded list
      //  - get the table_name of the input tables
      //
      // NOTE:
      // Input to this Action:
      //  - JetsFormState group 0 has the keys from Process Errors Data Table
      //    - FSK.pipelineExectionStatusKey
      //    - FSK.objectType
      //    - FSK.processName
      //    - FSK.domainKey
      //
      // Output Groups to this Action:
      //  - JetsFormState group 0 is Main Input Source
      //  - JetsFormState group 1 to n is Merge Input Source
      // For each group,set keys (as List<String>):
      //  - FSK.sessionId, (part of where  clause of domain table view)
      //  - FSK.domainKey, (part of where  clause of domain table view)
      //  - FSK.tableName, (domain or staging table to view)
      var state = formState.getState(0);
      final peKeyList = state[FSK.pipelineExectionStatusKey];
      final objectTypeList = state[FSK.objectType];
      final domainKeys = state[FSK.domainKey] as List<String?>?;
      if (peKeyList == null || objectTypeList == null || domainKeys == null) {
        print("Error: null peKey, object_type, or domain_key!!");
        return "Error: null peKey, object_type, or domain_key!!";
      }
      final peKey = peKeyList[0].toString();
      final objectType = objectTypeList[0] as String;

      // -- Query session_id that participated to a pipeline execution
      // -- ----------------------------------------------------------
      // -- NOTE Assuming month_period is the source_period_type
      // -- For each input_registry of the pipeline_execution_status, get:
      // --    - input_registry.key
      // --    - input_registry.table_name
      // --    - process_input.lookback_periods
      // --    - input_registry.session_id (for case when lookback_period = 0)
      var q = """
        SELECT DISTINCT ir.key, ir.table_name, pi.lookback_periods, ir.session_id
        FROM
          jetsapi.process_input pi,
          jetsapi.input_registry ir,
          jetsapi.source_period sp,
          jetsapi.session_registry sr
        WHERE ir.key IN (SELECT main_input_registry_key 
                        FROM jetsapi.pipeline_execution_status WHERE key = {peKey} UNION 
                        SELECT unnest(merged_input_registry_keys) 
                        FROM jetsapi.pipeline_execution_status WHERE key = {peKey})
          AND ir.client = pi.client
          AND ir.org = pi.org
          AND ir.table_name = pi.table_name
        ORDER BY ir.key""";
      var rawQuery = <String, dynamic>{
        'action': 'raw_query',
      };
      rawQuery['query'] = q.replaceAll(RegExp('{peKey}'), peKey);
      final rows = await queryJetsDataModel(
          context, formState, ServerEPs.dataTableEP, json.encode(rawQuery));
      if (rows == null) {
        return "No rows returned";
      }
      // ir.key, ir.table_name, pi.lookback_periods, ir.session_id
      Map<String, String> queryMap = {};
      var ipos = 0;
      List<String> queryList = [];
      for (var items in rows) {
        // Prepare the query to get the session_id
        String? tableName = items[1];
        String? lookbackStr = items[2];
        String? sessionId = items[3];
        if (tableName == null || lookbackStr == null || sessionId == null) {
          return "Error: tableName, sessionId, or lookbackPeriods is null from db";
        }
        int lookbackPeriods = int.parse(lookbackStr);
        String q = "";
        if (lookbackPeriods > 0) {
          q = """
            SELECT DISTINCT sr.session_id
            FROM
              jetsapi.pipeline_execution_status pe,
              jetsapi.source_period sp,
              jetsapi.session_registry sr,
              "{TABLE_NAME}" mc
            WHERE pe.key = {PEKEY}
              AND pe.source_period_key = sp.key 
              AND sr.session_id = mc.session_id
              AND sr.month_period >= (sp.month_period - {LOOKBACK_PERIODS})
              AND sr.month_period <= sp.month_period
            ORDER BY sr.session_id""";
          q = q.replaceAll(RegExp('{TABLE_NAME}'), tableName);
          q = q.replaceAll(RegExp('{PEKEY}'), peKey);
          q = q.replaceAll(RegExp('{LOOKBACK_PERIODS}'), lookbackStr);
        } else {
          q = "SELECT '$sessionId' AS session_id";
        }
        var key = 'q${ipos.toString()}';
        queryMap[key] = q;
        queryList.add(key);
        ipos += 1;
      }

      // Action: raw_query_map
      // input is: map[query_key, query]
      // server returns in field 'result_map':
      //  map[query_key, model] where model is list[list[string?]]
      var rawQueryMap = <String, dynamic>{
        'action': 'raw_query_map',
      };
      rawQueryMap['query_map'] = queryMap;
      // ignore: use_build_context_synchronously
      final queryResultMap = await queryMapJetsDataModel(
          context, formState, ServerEPs.dataTableEP, json.encode(rawQueryMap));
      if (queryResultMap == null) {
        return "No rows returned.[2]";
      }

      // Put the query result (list of session_id) and table_name into formState, in same order
      // rows type is List<List<String?>>, each row = rows[i]
      // queryMap type is Map<String, List<List<String?>>> with keys in queryList: key = queryList[i]
      // Note that row = rows[i] match queryMap[key, result for row] with key = queryList[i]
      var igroup = 0;
      for (var key in queryList) {
        final row = rows[
            igroup]; // row columns: ir.key, ir.table_name, pi.lookback_periods, ir.session_id
        // We need to unwrap the session_id, we have [['sessionid']], where the inner list
        // has only one item (single column returned)
        formState.setValue(igroup, FSK.tableName, row[1]);
        var v = queryResultMap[key]
            ?.map(
              (e) => e[0],
            )
            .toList();
        formState.setValue(igroup, FSK.sessionId, v);
        formState.setValue(igroup, FSK.domainKey, domainKeys);
        formState.setValue(
            igroup, FSK.domainKeyColumn, '$objectType:domain_key');
        print("STATE group $igroup, sessionIds $v, domainKeys $domainKeys");
        // reset the updated keys since these updates is to put default values
        // and is not from user interactions
        formState.resetUpdatedKeys(igroup);
        igroup += 1;
      }
      break;

    case ActionKeys.setupShowReteTriples:
      // Preparing to view rdf graph as triples from the process_errors table
      // Need to:
      //  Fetch process_errors.rete_session_triples by process_errors.key
      var state = formState.getState(0);
      final keyList = state[FSK.key];
      if (keyList == null) {
        print("Error: null process_errors.key (FSK.key) in formState");
        return "Error: null process_errors.key (FSK.key) in formState";
      }
      final key = keyList[0].toString();
      var rawQuery = <String, dynamic>{
        'action': 'raw_query',
      };
      rawQuery['query'] =
          "SELECT rete_session_triples FROM jetsapi.process_errors WHERE key = $key";
      final rows = await queryJetsDataModel(
          context, formState, ServerEPs.dataTableEP, json.encode(rawQuery));
      if (rows == null) {
        return "No rows returned";
      }
      var triples = rows[0][0];
      if (triples != null) {
        formState.setValue(0, FSK.reteSessionTriples, json.decode(triples));
      }
      break;

    case ActionKeys.dialogCancel:
      Navigator.of(context).pop();
      break;

    default:
      showAlertDialog(context,
          'Oops unknown ActionKey for processErrorsActions: $actionKey');
  }
  return null;
}
