import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/models/data_table_config.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/utils/download.dart';
import 'package:jetsclient/modules/actions/delegate_helpers.dart';

Future<String?> downloadMapping(
    BuildContext context, JetsFormState formState) async {
  var state = formState.getState(0);
  var client = unpack(state[FSK.client]);
  var org = unpack(state[FSK.org]);
  var objectType = unpack(state[FSK.objectType]);
  // Build the query
  var query = <String, dynamic>{
    "action": "read",
    "fromClauses": [
      {"schema": "jetsapi", "table": "source_config"},
      {"schema": "jetsapi", "table": "process_mapping"}
    ],
    "whereClauses": [
      {
        "table": "source_config",
        "column": "client",
        "values": [client]
      },
      {
        "table": "source_config",
        "column": "org",
        "values": [org]
      },
      {
        "table": "source_config",
        "column": "object_type",
        "values": [objectType]
      },
      {
        "table": "source_config",
        "column": "table_name",
        "joinWith": "process_mapping.table_name"
      }
    ],
    "offset": 0,
    "limit": 1000,
    "columns": [
      {"column": "client"},
      {"column": "org"},
      {"column": "object_type"},
      {"column": "data_property"},
      {"column": "input_column"},
      {"column": "function_name"},
      {"column": "argument"},
      {"column": "default_value"},
      {"column": "error_message"}
    ],
    "sortColumn": "data_property",
    "sortAscending": true
  };
  var result = await HttpClientSingleton().sendRequest(
      path: ServerEPs.dataTableEP,
      token: JetsRouterDelegate().user.token,
      encodedJsonBody: json.encode(query));
  Map<String, dynamic>? data;
  if (result.statusCode == 401) return null;
  if (result.statusCode == 200) {
    data = result.body;
  } else {
    const snackBar = SnackBar(
      content: Text('Unknown Error reading data from table'),
    );
    if (context.mounted) {
      ScaffoldMessenger.of(context).showSnackBar(snackBar);
    }
    return null;
  }
  final rows = data!['rows'] as List;
  List<List<String?>> model =
      rows.map((e) => (e as List).cast<String?>()).toList();
  // Prepare the csv buffer
  var buffer = StringBuffer();
  buffer.writeln(
      '"client","org","object_type","data_property","input_column","function_name","argument","default_value","error_message"');
  for (var row in model) {
    var isFirst = true;
    for (var column in row) {
      if (!isFirst) {
        buffer.write(',');
      }
      isFirst = false;
      if (column != null) {
        buffer.write('"$column"');
      }
    }
    buffer.writeln();
  }
  // Download the result!
  download(utf8.encode(buffer.toString()), downloadName: 'mapping.csv');
  return null;
}

void updateHomeFilters(BuildContext context, JetsFormState formState) {
  // var state = formState.getState(0);
  var state = JetsRouterDelegate().homeFiltersState;

  // print('Entering updateHomeFilters, got state: $state');
  state[FSK.userEmail] = JetsRouterDelegate().user.email;

  List<WhereClause> homeFilters = [];
  var processNameList = unpackToList(state[FSK.processName]);
  if (processNameList != null) {
    homeFilters.add(WhereClause(
        table: 'pipeline_execution_status',
        column: 'process_name',
        defaultValue: processNameList));
  }

  var statusList = unpackToList(state[FSK.status]);
  if (statusList != null) {
    homeFilters.add(WhereClause(
        table: 'pipeline_execution_status',
        column: 'status',
        defaultValue: statusList));
  }

  var fkMatchType = unpack(state[FSK.hfFileKeyMatchType]);
  var fkSubstring = unpack(state[FSK.hfFileKeySubstring]);
  if (fkMatchType != null && fkSubstring != null && fkSubstring.isNotEmpty) {
    switch (fkMatchType) {
      case 'equals_value':
        homeFilters.add(WhereClause(
            table: 'pipeline_execution_status',
            column: 'main_input_file_key',
            defaultValue: [fkSubstring]));
        break;
      case 'starts_with':
        homeFilters.add(WhereClause(
            table: 'pipeline_execution_status',
            column: 'main_input_file_key',
            like: '$fkSubstring%'));
        break;
      case 'ends_with':
        homeFilters.add(WhereClause(
            table: 'pipeline_execution_status',
            column: 'main_input_file_key',
            like: '%$fkSubstring'));
        break;
      case 'contains':
        homeFilters.add(WhereClause(
            table: 'pipeline_execution_status',
            column: 'main_input_file_key',
            like: '%$fkSubstring%'));
        break;
      default:
    }
    state[FSK.hfFileKeySubstring] = unpack(state[FSK.hfFileKeySubstring]);
  }

  var hfStartTime = unpack(state[FSK.hfStartTime]);
  var hfStartOffset = unpack(state[FSK.hfStartOffset]);
  if ((hfStartTime != null && hfStartTime.isNotEmpty) ||
      (hfStartOffset != null && hfStartOffset.isNotEmpty)) {
    var value = '';
    if (hfStartTime != null && hfStartTime.isNotEmpty) {
      value = "timestamp '$hfStartTime'";
    } else {
      value = 'now()';
    }
    if (hfStartOffset != null && hfStartOffset.isNotEmpty) {
      value += "-interval '$hfStartOffset'";
    }
    homeFilters.add(WhereClause(
        table: 'pipeline_execution_status', column: 'start_time', ge: value));
  }

  var hfEndTime = unpack(state[FSK.hfEndTime]);
  var hfEndOffset = unpack(state[FSK.hfEndOffset]);
  if ((hfEndTime != null && hfEndTime.isNotEmpty) ||
      (hfEndOffset != null && hfEndOffset.isNotEmpty)) {
    var value = '';
    if (hfEndTime != null && hfEndTime.isNotEmpty) {
      value = "timestamp '$hfEndTime'";
    } else {
      value = 'now()';
    }
    if (hfEndOffset != null && hfEndOffset.isNotEmpty) {
      value += "-interval '$hfEndOffset'";
    }
    homeFilters.add(WhereClause(
        table: 'pipeline_execution_status', column: 'start_time', le: value));
  }
  // print('*** Home Filters: $homeFilters');
  JetsRouterDelegate().homeFilters = homeFilters;
}

Future<String?> addProcessInput(
    BuildContext context, JetsFormState formState) async {
  final state = formState.getState(0);
  state[FSK.userEmail] = JetsRouterDelegate().user.email;
  state[FSK.client] = unpack(state[FSK.client]);
  state[FSK.org] = unpack(state[FSK.org]);
  state[FSK.sourceType] = unpack(state[FSK.sourceType]);
  state[FSK.objectType] = unpack(state[FSK.objectType]);
  state[FSK.entityRdfType] = unpack(state[FSK.entityRdfType]);
  state[FSK.tableName] = unpack(state[FSK.tableName]);
  var query = 'process_input'; // case add
  if (formState.getValue(0, FSK.key) != null) {
    query = 'update2/process_input';
  }
  var sourceType = state[FSK.sourceType] as String?;
  if (sourceType == null) {
    print("Oops bailing out of addProcessInputOk, source_type is null!");
    return null;
  }
  if (sourceType != 'file') {
    formState.setValue(0, FSK.org, '');
  }
  var encodedJsonBody = jsonEncode(<String, dynamic>{
    'action': 'insert_rows',
    'fromClauses': [
      <String, String>{'table': query}
    ],
    'data': [formState.getState(0)],
  }, toEncodable: (_) => '');
  return postInsertRows(context, formState, encodedJsonBody);
}
