import 'package:flutter/material.dart';
import 'package:jetsclient/models/data_table_config.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/modules/actions/delegate_helpers.dart';

void updateHomeFilters(BuildContext context, JetsFormState formState) {
  // var state = formState.getState(0);
  var state = JetsRouterDelegate().homeFiltersState;

  // print('Entering updateHomeFilters, got state: $state');
  state[FSK.userEmail] = JetsRouterDelegate().user.email;

  List<WhereClause> homeFilters = [];
  List<WhereClause> dataRegistryFilters = [];
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
        dataRegistryFilters.add(WhereClause(
            table: 'input_registry',
            column: 'file_key',
            defaultValue: [fkSubstring]));
        break;
      case 'starts_with':
        homeFilters.add(WhereClause(
            table: 'pipeline_execution_status',
            column: 'main_input_file_key',
            like: '$fkSubstring%'));
        dataRegistryFilters.add(WhereClause(
            table: 'input_registry',
            column: 'file_key',
            like: '$fkSubstring%'));
        break;
      case 'ends_with':
        homeFilters.add(WhereClause(
            table: 'pipeline_execution_status',
            column: 'main_input_file_key',
            like: '%$fkSubstring'));
        dataRegistryFilters.add(WhereClause(
            table: 'input_registry',
            column: 'file_key',
            like: '%$fkSubstring'));
        break;
      case 'contains':
        homeFilters.add(WhereClause(
            table: 'pipeline_execution_status',
            column: 'main_input_file_key',
            like: '%$fkSubstring%'));
        dataRegistryFilters.add(WhereClause(
            table: 'input_registry',
            column: 'file_key',
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
    dataRegistryFilters.add(WhereClause(
        table: 'input_registry', column: 'last_update', ge: value));
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
    dataRegistryFilters.add(WhereClause(
        table: 'input_registry', column: 'last_update', le: value));
  }
  // print('*** Home Filters: $homeFilters');
  JetsRouterDelegate().homeFilters = homeFilters;
  JetsRouterDelegate().dataRegistryFilters = dataRegistryFilters;
}
