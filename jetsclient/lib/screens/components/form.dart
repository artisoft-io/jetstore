import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:jetsclient/http_client.dart';
import 'package:jetsclient/routes/jets_router_delegate.dart';
import 'package:jetsclient/routes/jets_routes_app.dart';
import 'package:jetsclient/routes/jets_route_data.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';
import 'package:jetsclient/utils/constants.dart';
import 'package:jetsclient/utils/form_config.dart';
import 'package:provider/provider.dart';

class JetsForm extends StatefulWidget {
  const JetsForm({
    Key? key,
    required this.formPath,
    required this.formState,
    required this.formKey,
    required this.formConfig,
    required this.validatorDelegate,
    required this.actionsDelegate,
  }) : super(key: key);

  final JetsFormState formState;
  final GlobalKey<FormState> formKey;
  final FormConfig formConfig;
  final ValidatorDelegate validatorDelegate;
  final FormActionsDelegate actionsDelegate;
  final JetsRouteData formPath;

  @override
  State<JetsForm> createState() => _JetsFormState();
}

class _JetsFormState extends State<JetsForm> {
  late final HttpClient httpClient;
  // alternate to widget.formConfig.inputFields when
  // widget.formConfig.inputFields.isEmpty() and are
  // build in [queryInputFieldItems]
  InputFieldType alternateInputFields = [];

  InputFieldType get inputFields => widget.formConfig.inputFields.isEmpty
      ? alternateInputFields
      : widget.formConfig.inputFields;

  @override
  void initState() {
    super.initState();
    httpClient = Provider.of<HttpClient>(context, listen: false);
    if (inputFields.isEmpty) {
      if (JetsRouterDelegate().user.isAuthenticated) {
        queryInputFieldItems();
      } else {
        // Get the first batch of data when navigated to screenPath
        JetsRouterDelegate().addListener(navListener);
      }
    }
  }

  void stateListener() async {
    queryInputFieldItems();
  }

  void navListener() async {
    if (JetsRouterDelegate().currentConfiguration?.path == homePath) {
      queryInputFieldItems();
    }
  }

  @override
  void dispose() {
    if (widget.formConfig.inputFields.isEmpty) {
      JetsRouterDelegate().addListener(navListener);
    }
    super.dispose();
  }

  void queryInputFieldItems() async {
    // Check if we have a builder
    assert(widget.formConfig.inputFieldRowBuilder != null,
        "Jets Form with empty inputFields and no builder!");
    assert(widget.formConfig.inputFieldsQuery != null,
        "Jets Form with empty inputFields and no inputFieldsQuery!");

    var queryMap = <String, String>{
      FSK.inputFieldsCache: widget.formConfig.inputFieldsQuery!,
    };
    if (widget.formConfig.savedStateQuery != null) {
      queryMap.addAll({
        FSK.savedStateCache: widget.formConfig.savedStateQuery!,
      });
    }
    if (widget.formConfig.dropdownItemsQueries != null) {
      queryMap.addAll(widget.formConfig.dropdownItemsQueries!);
    }
    if (widget.formConfig.metadataQueries != null) {
      queryMap.addAll(widget.formConfig.metadataQueries!);
    }

    if (widget.formConfig.stateKeyPredicates != null) {
      for (var stateKey in widget.formConfig.stateKeyPredicates!) {
        var value = widget.formState.getValue(0, stateKey);

        assert(value != null,
            "queryInputFieldItems: Unexpected null stateKey $stateKey");
        if (value == null) return;
        assert((value is String) || (value is List<String>),
            "Error: unexpected type in formState passed to form");
        var tempMap = <String, String>{};
        for (var item in queryMap.entries) {
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
    var result = await httpClient.sendRequest(
        path: "/dataTable",
        token: JetsRouterDelegate().user.token,
        encodedJsonBody: encodedMsg);
    if (!mounted) return;
    if (result.statusCode == 200) {
      // Processing the server result: preparing caches in formState
      final data = result.body['result_map'] as Map<String, dynamic>?;
      if (data == null) return;

      // Prepare the saved state cache
      final savedStateModel = (data[FSK.savedStateCache] as List?)
          ?.map((e) => (e as List).cast<String?>())
          .toList();
      widget.formState
          .addCacheValue(FSK.inputColumnsDropdownItemsCache, savedStateModel);

      // Prepare the dropdown item list caches
      if (widget.formConfig.dropdownItemsQueries != null) {
        for (var key in widget.formConfig.dropdownItemsQueries!.keys) {
          final model = (data[key] as List)
              .map((e) => (e as List).cast<String?>())
              .toList();
          var dropdownItemList = [
            DropdownItemConfig(label: "Please select an item")
          ];
          dropdownItemList.addAll(
              model.map((e) => DropdownItemConfig(label: e[0]!, value: e[0]!)));
          widget.formState.addCacheValue(
              key,
              dropdownItemList
                  .map((e) => DropdownMenuItem<String>(
                      value: e.value, child: Text(e.label)))
                  .toList());
        }
      }

      // Prepare the metadata item list caches
      if (widget.formConfig.metadataQueries != null) {
        for (var key in widget.formConfig.metadataQueries!.keys) {
          final model = (data[key] as List)
              .map((e) => (e as List).cast<String?>())
              .toList();
          widget.formState.addCacheValue(key, model);
        }
      }

      // Construct the inputFields [FormFieldConfig] using the builder
      var inputFieldData = data[FSK.inputFieldsCache] as List;
      widget.formState.resizeFormState(inputFieldData.length);
      inputFieldData =
          inputFieldData.map((e) => (e as List).cast<String?>()).toList();
      alternateInputFields = InputFieldType.generate(
          inputFieldData.length,
          (index) => widget.formConfig.inputFieldRowBuilder!(
              index, inputFieldData[index], widget.formState));
      // Notify that we now have inputFields ready
      setState(() {});
    } else if (result.statusCode == 401) {
      const snackBar = SnackBar(
        content: Text('Session Expired, please login'),
      );
      ScaffoldMessenger.of(context).showSnackBar(snackBar);
    } else {
      const snackBar = SnackBar(
        content: Text('Error reading dropdown list items'),
      );
      ScaffoldMessenger.of(context).showSnackBar(snackBar);
    }
  }

  @override
  Widget build(BuildContext context) {
    final themeData = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.fromLTRB(0, 8, 0, 0),
      child: FocusTraversalGroup(
        child: Form(
            key: widget.formKey,
            child: ListView.builder(
                itemBuilder: (BuildContext context, int index) {
                  if (index < inputFields.length) {
                    var fc = inputFields[index];
                    return Row(
                      children: fc
                          .map((e) => e.makeFormField(
                                screenPath: widget.formPath,
                                state: widget.formState,
                                formFieldValidator: (group, key, v) =>
                                    widget.validatorDelegate(context,
                                        widget.formState, group, key, v),
                                formValidator: widget.validatorDelegate,
                                formActionsDelegate: widget.actionsDelegate,
                              ))
                          .toList(),
                    );
                  }
                  // case last: row of buttons
                  return Padding(
                    padding: const EdgeInsets.fromLTRB(10, 0, 0, 0),
                    child: Center(
                      child: Row(
                          children: List<Widget>.from(
                        widget.formConfig.actions.map((e) => ElevatedButton(
                            style: ElevatedButton.styleFrom(
                              // Foreground color
                              foregroundColor: themeData.colorScheme.onPrimary,
                              backgroundColor: themeData.colorScheme.primary,
                            ).copyWith(
                                elevation: ButtonStyleButton.allOrNull(0.0)),
                            onPressed: () => widget.actionsDelegate(context,
                                widget.formKey, widget.formState, e.key),
                            child: Text(e.label))),
                        growable: false,
                      )
                              .expand((element) => [
                                    const SizedBox(width: defaultPadding),
                                    element
                                  ])
                              .toList()),
                    ),
                  );
                },
                itemCount: inputFields.length + 1)),
      ),
    );
  }
}
