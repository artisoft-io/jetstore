---
title: "Limits, memory safety and the sanitizer build"
description: "The default vertex visit limit, the leaks and crash fixed in the engine and its bridge, and the sanitizer build."
kind: explanation
sidebar:
  order: 4
---

## Leaks, a crash, a default limit and the sanitizer gate — 2026-10-23

On the one engine. None of it changes what a session computes unless the session failed or ran
without a limit before.

- **A session with no `jets:max_vertex_visits` stops at 10,000 visits of a vertex** (decided
  2026-10-23: without one, a rule that never settles ran until the pipeline timed out, with no
  message). `ReteSession::kDefaultMaxVertexVisits`. The error is the one a set limit gives, with
  the limit and the setting named: `{"error":"Maximum vertex visits reached","limit":10000,
  "reason":"the default limit of 10000 visits of a rule's vertex, since jets:max_vertex_visits is
  not set; ...","data":{...}}`. A set limit gives the message it always gave. A value of 0 or less
  still turns the limit off; an integer of any width is read as the limit, where only an `int32`
  was, and a value that is not an integer gets the default with a warning. Production processes
  set 1,000,000. `VertexVisitLimitTest` in `meta_store_sharing_test.cc`.
- **The bridge's ISO date and datetime getters read text that nothing else can change**. The C wrapper's getters returned `c_str()` of a temporary, so the bridge read freed
  memory; a datetime's text is past the small-string buffer, so it read freed heap.
  `get_date_iso_string3` and `get_datetime_iso_string3` now write the text into a buffer the
  caller passes, and `GetDateIsoString` and `GetDatetimeIsoString` in `jets/bridge` pass one of
  64 bytes. The four older getters are kept, deprecated, for C callers: they copy the text into a
  fixed buffer of the calling thread, never freed, but **not safe from Go**. A first version of
  this fix had the bridge use that buffer, and the review found wrong values under concurrency:
  between the cgo call and `C.GoString` a goroutine can move, and another goroutine's call on the
  first thread overwrites the buffer, about one call in 8,000 at 32 goroutines.
  `cwrapper_test.cc`; `TestIsoGettersFromManyGoroutines` in `jets/bridge`.
- **A factory closes what it opened, and so does the lookup helper**.
  `~ReteMetaStoreFactory` calls `reset()` (it was commented out); `LookupSqlHelper` and each
  connection pool close their connections when they go, so a factory deleted any way, not only
  through `delete_jetstore_hdl`, leaks nothing; the helper is owned by the meta stores, so it goes
  after the last session using it. And `TypeOf::type_of` on an unknown data property, and a reset
  or bind error in a lookup, now give their connection back to the pool: each kept one, which
  nothing closed. `LookupDbOpenTest.AFactoryClosesWhatItOpened` counts SQLite's own memory.
- **`lookup.db` is opened only when a lookup table is declared, and read-only**. It was
  opened on every engine build with `sqlite3_open`'s default flags, which create a missing file.
  Every pooled connection is read-only too: their statements are all `SELECT`s. A missing
  `lookup.db` with a table declared is now an error that creates nothing.
- **The antecedent query keys are set once, when the meta store is built**.
  `ReteMetaStore::initialize` numbers each vertex's children by the index of the parent relation
  they query (`AlphaNode::antecedent_query_index`); each session's `BetaRelation::initialize` used
  to write the key into the shared node vertex, a data race while the value was the same. A
  session now adds the indexes in the same order and checks the key. `MetaStoreSharingTest`.
- **The sanitizer gate is one command**, from the repository root:

  ```bash
  cmake -S . -B build-san -DCMAKE_BUILD_TYPE=Release -DJETS_SANITIZE=ON \
    -DJETS_ABSEIL_SOURCE=/path/to/abseil-cpp-20260107.1 \
    && make -C build-san -j8 jets_test && (cd build-san/jets && ctest)
  ```

  `JETS_SANITIZE` compiles and links with AddressSanitizer, leak detection on, and
  UndefinedBehaviorSanitizer, every report fatal, less `null`, `nonnull-attribute` and
  `returns-nonnull-attribute`, under which GCC 13 rejects a constexpr comparison in abseil's
  `hash_policy_traits.h`. abseil must be built with the same flags, since under AddressSanitizer
  its headers change the flat tables' layout: `JETS_ABSEIL_SOURCE` builds it in the tree, or
  `CMAKE_PREFIX_PATH` can name one installed so. The three lookup tests' leaks, which the gate had
  to switch leak detection off for, are gone.
- **The `writes_graph` marks are tested against what the operators do**.
  `OperatorWritesGraph` (`operator_writes_graph_test.cc`) evaluates every operator of
  `operators.txt` on a set of operands, in a session with a lookup table, and an operator that grows
  the inferred graph must be marked, and a marked one must grow it. Today that is `lookup`,
  `multi_lookup`, `lookup_rand`, `multi_lookup_rand`, `range` and `create_entity`, the six the
  file marks. An operator that writes only for an operand the test does not try would pass
  unmarked.
- **The reason for keeping the two retraction defects is gone**, corrected in the 2026-10-08
  section of [Retraction and truth maintenance](/docs/developer/engine/rule-engine/retraction/).
