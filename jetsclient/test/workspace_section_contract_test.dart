/// The Workspace IDE's section contract, from the end that consumes it.
///
/// **This is the one surface no corpus in this suite can reach.** The five
/// generated corpora are all built by walking the client's own registries, so
/// they close over the configuration this app *declares*. The Workspace IDE's
/// section inventory is not declared here — it is data the apiserver sends, one
/// row per `wsfile.Section` — and the client's job is to render each section
/// either as a view of `workspace.db` or as the source files beneath it. A
/// corpus that enumerates registries cannot tell you that a section arrived with
/// no form to render it; it can only tell you which forms exist.
///
/// So this file is the mirror of `jets/datatable/wsfile/sections_test.go`. Both
/// carry the same declaration and the same checksum: change the Go table and the
/// Go test fails, update the Go constant alone and this test fails. What it
/// forces at that moment is the decision — **does this section's files compile
/// into `workspace.db`, and if so does this client render the view?**
///
/// **This suite only runs on the chrome platform** (I-5):
///
///     CHROME_EXECUTABLE=/usr/bin/google-chrome \
///       flutter test --platform chrome test/workspace_section_contract_test.dart
library;

import 'package:flutter_test/flutter_test.dart';
import 'package:jetsclient/modules/form_config_impl.dart';
import 'package:jetsclient/modules/workspace_ide/screen_delegates_helpers.dart';
import 'package:jetsclient/models/screen_config.dart';

import 'corpus_support.dart';

/// The section contract as the apiserver declares it, one `dir=compiledView`
/// per section in display order.
///
/// **Copied from `wsfile.SectionDeclaration()`, not derived from anything here.**
/// Deriving it from this app's registries is the mistake the whole file exists
/// to avoid: it would make the test agree with the client no matter what the
/// server sends. Recorded 2026-08-25 against `jetstore_ai` at `worktree-ui-c1`.
const serverSectionDeclaration = '''
data_model=data_model
jet_rules=jet_rules
lookups=lookups
pipes_config=
user_flows=
table_configs=
process_config=
reports=
''';

/// Duplicated in `jets/datatable/wsfile/sections_test.go` as
/// `expectedDeclarationChecksum`. Update the two together or neither.
const expectedDeclarationChecksum = 'fnv1a32:b3fc79eb';

/// Compiled views the server declares that this client deliberately does not
/// render.
///
/// **An empty set here would be the wrong shape**, because it would say the two
/// ends agree when what is true is that one of them has a view the other has not
/// built. `lookups` compiles into `lookup_tables` and `lookup_columns` and its
/// view is scheduled as C.3a **in React** — track X deletes this app, so a view
/// built here is discarded by construction (I-45). Naming it is what stops the
/// gap being indistinguishable from a bug.
const viewsNotBuiltInFlutter = <String>{'lookups'};

/// Parses [serverSectionDeclaration] into (directory, compiled view) pairs,
/// where an empty view means the section's files compile into nothing.
List<MapEntry<String, String>> parseDeclaration() => serverSectionDeclaration
    .trim()
    .split('\n')
    .map((line) {
      final at = line.indexOf('=');
      return MapEntry(line.substring(0, at), line.substring(at + 1));
    })
    .toList();

/// One `workspace_file_structure` payload, built from the declaration rather
/// than hand-written, so that adding a section to the Go table and updating the
/// declaration is the whole of extending this test.
List<dynamic> sectionPayload() => parseDeclaration()
    .map(
      (entry) => <String, dynamic>{
        'key': entry.key,
        'pageMatchKey': entry.key,
        'type': 'section',
        'size': 0.0,
        'label': entry.key,
        // The Go side omits the field entirely when there is no compiled view;
        // a section that has one sends it. Both shapes are exercised.
        if (entry.value.isNotEmpty) 'compiled_view': entry.value,
        'route_path': '/workspace/:workspace_name/home',
        'route_params': <String, dynamic>{'workspace_name': 'ws'},
        'children': <dynamic>[],
      },
    )
    .toList();

void main() {
  test('the section declaration is the one the apiserver produces', () {
    expect(
      checksum(serverSectionDeclaration),
      expectedDeclarationChecksum,
      reason:
          'The section contract changed. Reconcile this file with '
          'jets/datatable/wsfile/sections.go, and decide for each new or '
          'changed section whether its files compile into workspace.db and '
          'whether this client renders the view.',
    );
  });

  test('every compiled view is either rendered here or named as not built', () {
    for (final entry in parseDeclaration()) {
      if (entry.value.isEmpty) continue;
      final rendered = compiledViewForms.containsKey(entry.value);
      final named = viewsNotBuiltInFlutter.contains(entry.value);
      expect(
        rendered || named,
        isTrue,
        reason:
            'Section ${entry.key} declares compiled view "${entry.value}" and '
            'this client neither renders it nor names it in '
            'viewsNotBuiltInFlutter. Those are the only two honest answers.',
      );
      expect(
        rendered && named,
        isFalse,
        reason:
            'Compiled view "${entry.value}" is both rendered and listed as not '
            'built. One of the two is stale.',
      );
    }
  });

  test('a section with no compiled view is never listed as one', () {
    final declared = {
      for (final entry in parseDeclaration())
        if (entry.value.isNotEmpty) entry.value,
    };
    for (final view in compiledViewForms.keys) {
      expect(
        declared,
        contains(view),
        reason:
            'This client renders compiled view "$view", which the apiserver no '
            'longer declares. Nothing will ever send it.',
      );
    }
    for (final view in viewsNotBuiltInFlutter) {
      expect(
        declared,
        contains(view),
        reason:
            'Compiled view "$view" is listed as not built and is not declared '
            'by the apiserver either. Delete the entry.',
      );
    }
  });

  test('every form this client names for a view is registered', () {
    for (final key in compiledViewForms.values) {
      // getFormConfig throws rather than returning null for an unknown key, so
      // reaching the assertion at all is most of the test.
      expect(getFormConfig(key).key, isNotEmpty);
    }
  });

  test('mapMenuEntry resolves every section without throwing', () {
    // **The regression this pins.** Before 2026-08-25 the client composed
    // "workspace.<dir>.form" and looked it up; six of the eight sections
    // composed a key no registry held, and the throw happened inside an async
    // menu delegate where it read as a heading that did nothing.
    final entries = mapMenuEntry(sectionPayload());
    expect(entries.length, parseDeclaration().length);

    for (final entry in entries) {
      if (entry.formConfigKey == null) continue;
      expect(
        () => getFormConfig(entry.formConfigKey!),
        returnsNormally,
        reason:
            'Section ${entry.key} resolved to form key ${entry.formConfigKey}, '
            'which no registry holds. mapMenuEntry must only return keys '
            'compiledViewForms holds.',
      );
    }
  });

  test('a section renders its view, or its sources, and the two are distinct', () {
    final byKey = <String, MenuEntry>{
      for (final entry in mapMenuEntry(sectionPayload())) entry.key: entry,
    };

    for (final entry in parseDeclaration()) {
      final menuEntry = byKey[entry.key];
      expect(menuEntry, isNotNull, reason: 'section ${entry.key} is missing');

      final expectedKey = compiledViewForms[entry.value];
      expect(
        menuEntry!.formConfigKey,
        expectedKey,
        reason:
            'Section ${entry.key} declares "${entry.value}"; this client should '
            '${expectedKey == null ? 'render its sources' : 'render $expectedKey'}.',
      );
    }

    // The two sections that do have a view, named rather than counted, so a
    // regression that empties the map fails here as well as above.
    expect(byKey['data_model']!.formConfigKey, 'workspace.data_model.form');
    expect(byKey['jet_rules']!.formConfigKey, 'workspace.jet_rules.form');
    expect(byKey['lookups']!.formConfigKey, isNull);
    expect(byKey['reports']!.formConfigKey, isNull);
  });

  test('a payload that omits compiled_view resolves to the sources', () {
    // Belt and braces against the field being renamed on the Go side, where the
    // Dart compiler cannot see it: an unknown or absent view must not throw and
    // must not guess.
    final entries = mapMenuEntry(<dynamic>[
      <String, dynamic>{
        'key': 'data_model',
        'pageMatchKey': 'data_model',
        'type': 'section',
        'size': 0.0,
        'label': 'Data Model',
        'route_path': '/workspace/:workspace_name/home',
        'route_params': <String, dynamic>{'workspace_name': 'ws'},
        'children': <dynamic>[],
      },
      <String, dynamic>{
        'key': 'invented',
        'pageMatchKey': 'invented',
        'type': 'section',
        'size': 0.0,
        'label': 'Invented',
        'compiled_view': 'no_such_view',
        'route_path': '/workspace/:workspace_name/home',
        'route_params': <String, dynamic>{'workspace_name': 'ws'},
        'children': <dynamic>[],
      },
    ]);

    expect(entries[0].formConfigKey, isNull);
    expect(entries[1].formConfigKey, isNull);
  });
}
