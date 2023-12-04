import 'package:flutter/material.dart';
import 'package:jetsclient/components/jets_form_state.dart';
import 'package:jetsclient/models/form_config.dart';

class JetsTabParams {
  JetsTabParams({
    required this.workspaceName,
    required this.label,
    required this.pageMatchKey,
    required this.formConfig,
    required this.formState,
  });
  final String workspaceName;
  final String label;
  final String pageMatchKey;
  final FormConfig formConfig;
  final JetsFormState formState;
}

class JetsTabsStateHelper extends ChangeNotifier {
  final List<JetsTabParams> tabsParams = [];

  void addTab({required JetsTabParams tabParams}) {
    tabsParams.add(tabParams);
    // print("*** tab added to JetsTabsStateHelper, notifying...");
    notifyListeners();
  }

  void removeTab({required int index}) {
    tabsParams.removeAt(index);
    // print("*** tab removed @ $index to JetsTabsStateHelper, notifying...");
    notifyListeners();
  }
}
