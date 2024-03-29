import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/routes/jets_route_data.dart';
import 'package:jetsclient/components/form_button.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/models/form_config.dart';

class JetsForm extends StatefulWidget {
  const JetsForm({
    Key? key,
    required this.formPath,
    required this.formState,
    required this.formKey,
    required this.formConfig,
    this.isDialog = false,
  }) : super(key: key);

  final JetsFormState formState;
  final GlobalKey<FormState> formKey;
  final FormConfig formConfig;
  final JetsRouteData formPath;
  final bool isDialog;

  @override
  State<JetsForm> createState() => JetsFormWidgetState();
}

class JetsFormWidgetState extends State<JetsForm> {
  // alternate to widget.formConfig.inputFields when
  // widget.formConfig.inputFields.isEmpty() and are
  // build in [queryInputFieldItems]
  InputFieldType alternateInputFields = [];
  bool? get useListView => widget.formConfig.useListView;
  String? get errorMessage => widget.formState.getValue(0, FSK.serverError);

  InputFieldTypeV2 get inputFieldsV2 => widget.formConfig.inputFieldsV2;

  InputFieldType get inputFields => widget.formConfig.inputFields.isEmpty
      ? alternateInputFields
      : widget.formConfig.inputFields;

  @override
  void initState() {
    super.initState();
    widget.formState.activeFormWidgetState = this;
    widget.formState.isDialog = widget.isDialog;
    widget.formState.formKey = widget.formKey;
    if (inputFields.isEmpty) {
      queryInputFieldItems();
    }
  }

  void markAsDirty() {
    if (!mounted) return;
    setState(() {});
  }

  void queryInputFieldItems() async {
    // Check if we have a builder
    // assert(widget.formConfig.inputFieldRowBuilder != null,
    //     "Jets Form with empty inputFields and no builder!");
    // assert(widget.formConfig.inputFieldsQuery != null,
    //     "Jets Form with empty inputFields and no inputFieldsQuery!");
    if (widget.formConfig.inputFieldRowBuilder == null ||
        widget.formConfig.inputFieldsQuery == null) {
      return;
    }

    // Check if user is logged in
    if (!JetsRouterDelegate().user.isAuthenticated) {
      // print(
      //     "*** Form.queryInputFieldItems CANCELLED for ${widget.formConfig.key} not auth");
      return;
    }

    var queryMap = widget.formConfig.queries;
    assert(queryMap != null,
        "queryInputFieldItems: Expecting to find queries in form config");
    if (queryMap == null) return;
    // apply the parameter substitutions in the queries
    if (widget.formConfig.stateKeyPredicates != null) {
      for (var stateKey in widget.formConfig.stateKeyPredicates!) {
        var value = widget.formState.getValue(0, stateKey);
        if (value == null) {
          print(
              "ERROR QueryMap substitution: unexpected null from formState for key $stateKey");
          return;
        }
        assert((value is String) || (value is List<String>),
            "Error: unexpected type in formState passed to form");
        var tempMap = <String, String>{};
        for (var item in queryMap!.entries) {
          var query = "";
          if (value is String) {
            query = item.value.replaceAll(RegExp('{$stateKey}'), value);
          } else {
            query = query.replaceAll(RegExp('{$stateKey}'), value[0]);
          }
          tempMap[item.key] = query;
        }
        queryMap = tempMap;
      }
    }

    // print("queryInputFieldItems: queryMap is $queryMap");

    // Action: raw_query_map
    // input is: map[query_key, query]
    // server returns in field 'result_map':
    //  map[query_key, model] where model is list[list[string?]]
    var msg = <String, dynamic>{
      'action': 'raw_query_map',
    };
    msg['query_map'] = queryMap;
    var encodedMsg = json.encode(msg);
    var result = await HttpClientSingleton().sendRequest(
        path: "/dataTable",
        token: JetsRouterDelegate().user.token,
        encodedJsonBody: encodedMsg);
    if (!mounted) return;
    if (result.statusCode == 200) {
      // Processing the server result: preparing caches in formState
      final rawData = result.body['result_map'] as Map<String, dynamic>?;
      if (rawData == null) return;
      final data = <String, List<List<String?>>>{};
      for (var item in rawData.entries) {
        data[item.key] = (item.value as List)
            .map((e) => (e as List).cast<String?>())
            .toList();
      }

      //* THIS IS SPECIFIC TO PROCESS MAPPING ADD CONFIG
      // Let's make sure the input table exist otherwise there are no
      // input column to map to
      final ic = data["inputColumnsQuery"];
      if (ic != null && ic.isEmpty) {
        widget.formState.setValue(0, FSK.serverError,
            "Data has not been loaded to the staging table. Please load the data to configure the mapping.");
        // if(context.mounted) {
        //   Navigator.of(context).pop(DTActionResult.statusError);
        // }
      }

      // Prepare the saved state cache
      final savedStateModel = data[widget.formConfig.savedStateQuery];
      if (savedStateModel != null && savedStateModel.isNotEmpty) {
        widget.formState.addCacheValue(FSK.savedStateCache, savedStateModel);
      }

      // Prepare the dropdown item list caches
      // var label0 = "Select an item";
      var label0 = "Select cleansing function";
      if (widget.formConfig.dropdownItemsQueries != null) {
        for (var item in widget.formConfig.dropdownItemsQueries!.entries) {
          final model = data[item.value];
          assert(model != null,
              "queryInputFieldItems: Form is missconfigured, dropdown query is missing");
          var dropdownItemList = [DropdownItemConfig(label: label0)];
          dropdownItemList.addAll(model!
              .map((e) => DropdownItemConfig(label: e[0]!, value: e[0]!)));
          widget.formState.addCacheValue(
              item.key,
              dropdownItemList
                  .map((e) => DropdownMenuItem<String>(
                      value: e.value, child: Text(e.label)))
                  .toList());
        }
      }
      if (widget.formConfig.typeaheadItemsQueries != null) {
        for (var item in widget.formConfig.typeaheadItemsQueries!.entries) {
          final model = data[item.value];
          assert(model != null,
              "queryInputFieldItems: Form is missconfigured, typeahead query is missing");
          var dropdownItemList = [DropdownItemConfig(label: label0)];
          dropdownItemList.addAll(model!
              .map((e) => DropdownItemConfig(label: e[0]!, value: e[0]!)));
          widget.formState
              .addCacheValue(item.key, model.map((e) => e[0]!).toList());
        }
      }

      // Prepare the metadata item list caches
      if (widget.formConfig.metadataQueries != null) {
        for (var item in widget.formConfig.metadataQueries!.entries) {
          final model = data[item.value];
          assert(model != null,
              "queryInputFieldItems: Form is missconfigured, metadata query is missing");
          widget.formState.addCacheValue(item.key, model);
        }
      }

      // Construct the inputFields [FormFieldConfig] using the builder
      var inputFieldData = data[widget.formConfig.inputFieldsQuery];
      assert(inputFieldData != null,
          "queryInputFieldItems: Form is missconfigured, inputFieldQuery is missing");
      if (inputFieldData == null) return;
      var sz = inputFieldData.length;
      if (widget.formConfig.formWithDynamicRows == true) {
        sz += 1;
      }
      // print("GOT result of inputFieldQuery size: ${inputFieldData.length}, resiziformState to $sz");
      widget.formState.resizeFormState(sz);
      for (var index = 0; index < inputFieldData.length; index++) {
        alternateInputFields.addAll(widget.formConfig.inputFieldRowBuilder!(
            index, inputFieldData[index], widget.formState));
      }
      // Check if we add one extra row to add items dynamically
      if (widget.formConfig.formWithDynamicRows == true) {
        alternateInputFields.addAll(widget.formConfig.inputFieldRowBuilder!(
            inputFieldData.length, null, widget.formState));
      }
      // Notify that we now have inputFields ready
      setState(() {});
    } else if (result.statusCode == 401) {
      // const snackBar = SnackBar(
      //   content: Text('Session Expired, please login'),
      // );
      // ScaffoldMessenger.of(context).showSnackBar(snackBar);
    } else {
      const snackBar = SnackBar(
        content: Text('Error reading dropdown list items'),
      );
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(snackBar);
      }
    }
  }

  Widget buildFormV1(BuildContext context) {
    return inputFields.length > 5 ||
            (useListView != null && useListView == true)
        ? Column(
            children: [
              Expanded(
                child: ListView.builder(
                    itemBuilder: (BuildContext context, int index) {
                      if (index == 0 && errorMessage != null) {
                        return Padding(
                          padding: const EdgeInsets.all(8.0),
                          child: Text(errorMessage!,
                              overflow: TextOverflow.ellipsis,
                              style: Theme.of(context).textTheme.headlineSmall),
                        );
                      }
                      var fc = inputFields[index];
                      return Row(
                        children: fc
                            .map((e) => Flexible(
                                flex: e.flex,
                                fit: FlexFit.tight,
                                child: e.makeFormField(
                                    screenPath: widget.formPath,
                                    formConfig: widget.formConfig,
                                    formState: widget.formState)))
                            .toList(),
                      );
                    },
                    itemCount: errorMessage != null
                        ? inputFields.length + 1
                        : inputFields.length),
              ),
              Center(
                  child: Padding(
                padding: const EdgeInsets.fromLTRB(
                    0, defaultPadding, 0, defaultPadding),
                child: Row(
                    children: widget.formConfig.actions
                        .map((e) => JetsFormButton(
                            key: Key(e.key),
                            formActionConfig: e,
                            formKey: widget.formKey,
                            formState: widget.formState,
                            actionsDelegate:
                                widget.formConfig.formActionsDelegate))
                        .toList()),
              ))
            ],
          )
        : Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: List<Widget>.generate(
                errorMessage != null
                    ? (widget.formConfig.actions.isNotEmpty
                        ? inputFields.length + 2
                        : inputFields.length + 1)
                    : (widget.formConfig.actions.isNotEmpty
                        ? inputFields.length + 1
                        : inputFields.length), (index) {
              if (index == 0 && errorMessage != null) {
                return Padding(
                  padding: const EdgeInsets.all(8.0),
                  child: Text(errorMessage!,
                      overflow: TextOverflow.ellipsis,
                      style: Theme.of(context).textTheme.headlineSmall),
                );
              }
              return index < inputFields.length
                  // widgets of the form's row
                  ? Flexible(
                      flex: 10,
                      fit: FlexFit.tight,
                      child: Row(
                        children: inputFields[index]
                            .map((e) => Flexible(
                                flex: e.flex,
                                fit: FlexFit.tight,
                                child: e.makeFormField(
                                    screenPath: widget.formPath,
                                    formConfig: widget.formConfig,
                                    formState: widget.formState)))
                            .toList(),
                      ),
                    )
                  // last row of form action button
                  : Align(
                      alignment: Alignment.centerRight,
                      child: Padding(
                        padding: const EdgeInsets.fromLTRB(
                            0, defaultPadding, 0, defaultPadding),
                        child: Row(
                            children: widget.formConfig.actions
                                .map((e) => JetsFormButton(
                                    key: Key(e.key),
                                    formActionConfig: e,
                                    formKey: widget.formKey,
                                    formState: widget.formState,
                                    actionsDelegate:
                                        widget.formConfig.formActionsDelegate))
                                .toList()),
                      ),
                    );
            }),
          );
  }

  Widget buildFormV2(BuildContext context) {
    return inputFieldsV2.length > 5 ||
            (useListView != null && useListView == true)
        ? Column(
            children: [
              Expanded(
                child: ListView.builder(
                    itemBuilder: (BuildContext context, int index) {
                      if (index == 0 && errorMessage != null) {
                        return Padding(
                          padding: const EdgeInsets.all(8.0),
                          child: Text(errorMessage!,
                              overflow: TextOverflow.ellipsis,
                              style: Theme.of(context).textTheme.headlineSmall),
                        );
                      }
                      var fc = inputFieldsV2[index];
                      return Row(
                        children: fc.rowConfig
                            .map((e) => Flexible(
                                flex: e.flex,
                                fit: FlexFit.tight,
                                child: e.makeFormField(
                                    screenPath: widget.formPath,
                                    formConfig: widget.formConfig,
                                    formState: widget.formState)))
                            .toList(),
                      );
                    },
                    itemCount: errorMessage != null
                        ? inputFieldsV2.length + 1
                        : inputFieldsV2.length),
              ),
              Center(
                  child: Padding(
                padding: const EdgeInsets.fromLTRB(
                    0, defaultPadding, 0, defaultPadding),
                child: Row(
                    children: widget.formConfig.actions
                        .map((e) => JetsFormButton(
                            key: Key(e.key),
                            formActionConfig: e,
                            formKey: widget.formKey,
                            formState: widget.formState,
                            actionsDelegate:
                                widget.formConfig.formActionsDelegate))
                        .toList()),
              ))
            ],
          )
        : Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: List<Widget>.generate(
                errorMessage != null
                    ? (widget.formConfig.actions.isNotEmpty
                        ? inputFieldsV2.length + 2
                        : inputFieldsV2.length + 1)
                    : (widget.formConfig.actions.isNotEmpty
                        ? inputFieldsV2.length + 1
                        : inputFieldsV2.length), (index) {
              if (index == 0 && errorMessage != null) {
                return Padding(
                  padding: const EdgeInsets.all(8.0),
                  child: Text(errorMessage!,
                      overflow: TextOverflow.ellipsis,
                      style: Theme.of(context).textTheme.headlineSmall),
                );
              }
              return index < inputFieldsV2.length
                  // widgets of the form's row
                  ? Flexible(
                      flex: inputFieldsV2[index].flex,
                      fit: FlexFit.tight,
                      child: Row(
                        children: inputFieldsV2[index]
                            .rowConfig
                            .map((e) => Flexible(
                                flex: e.flex,
                                fit: FlexFit.tight,
                                child: e.makeFormField(
                                    screenPath: widget.formPath,
                                    formConfig: widget.formConfig,
                                    formState: widget.formState)))
                            .toList(),
                      ),
                    )
                  // last row of form action button
                  : Align(
                      alignment: Alignment.centerRight,
                      child: Padding(
                        padding: const EdgeInsets.fromLTRB(
                            0, defaultPadding, 0, defaultPadding),
                        child: Row(
                            children: widget.formConfig.actions
                                .map((e) => JetsFormButton(
                                    key: Key(e.key),
                                    formActionConfig: e,
                                    formKey: widget.formKey,
                                    formState: widget.formState,
                                    actionsDelegate:
                                        widget.formConfig.formActionsDelegate))
                                .toList()),
                      ),
                    );
            }),
          );
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
        padding: const EdgeInsets.fromLTRB(0, 8, 0, 0),
        child: FocusTraversalGroup(
          child: Form(
            key: widget.formKey,
            child: AutofillGroup(
                // When inputFields.length > 5 or useListView==true then use ListView
                // otherwise expand the controls to occupy the viewport
                child: inputFieldsV2.isNotEmpty
                    ? buildFormV2(context)
                    : buildFormV1(context)),
          ),
        ));
  }
}
