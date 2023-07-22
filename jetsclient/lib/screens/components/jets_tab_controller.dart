import 'package:flutter/material.dart';
import 'package:jetsclient/screens/components/jets_form_state.dart';

class JetsTabParams {
  JetsTabParams({
    required this.workspaceName,
    required this.label,
    required this.fileName,
    required this.fileContent,
    required this.formState,
  });
  final String workspaceName;
  final String label;
  final String fileName;
  final String fileContent;
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
