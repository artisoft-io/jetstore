// Package graphdump writes an RDF session as a canonical dump: one triple per
// line, sorted, in a text form that is the same on the Go and the C++ engine, so
// that two runs can be compared by a line diff. It is the equivalence check of
// projects/rules_benchmark (§5.7 of its plan, task AB.4); jets/cmds/rules_bench_diff
// compares two dump trees (AB.5).
//
// # What is written
//
// The session's asserted and inferred triples. Both engines' Find returns the
// meta graph as well (Go: RdfSession.Find, jets/jetrules/rdf/rdf_session.go;
// C++: rdf_session.h find()), and nothing in compute_pipes.JetRdfSession tells
// the three graphs apart. So the meta graph is subtracted by value: it is read
// once per engine from a new, empty RDF session of the same engine, whose Find
// holds the meta graph and nothing else, and every triple equal to one of those
// is left out. The engine is reached through a JetRuleEngine() method that both
// adaptors' sessions have; a session without one is refused unless IncludeMeta
// is set. A rule that infers a triple equal to a meta triple is therefore not
// in the dump either, on either engine.
//
// Triples whose predicate is in Options.ExcludePredicates are left out before
// anything else is computed, so they do not affect the names below.
//
// # The line
//
// A line is subject, predicate and object, separated by a tab:
//
//	hc_usi:claim1	hc_usi:amountBilled	125.5^^double
//
// A term is written as:
//
//	resource           its name, bare: hc_usi:claim1
//	                   "…"^^resource when the bare name would be ambiguous
//	                   (empty, a tab or newline, a leading " or _:, a ^^, or null)
//	text               Go-quoted: "abc", "a\tb"
//	int, uint          42^^int, 42^^uint (both engines' int32/int64 are int)
//	double             the value the engine holds, as the shortest decimal that
//	                   reads back to it (strconv 'g', -1): 0.891082763671875^^double.
//	                   NaN^^double, +Inf^^double, -Inf^^double.
//	date               2024-01-31^^date (ISO-8601)
//	datetime           2024-01-31T13:05:09Z^^datetime (ISO-8601, UTC, whole seconds)
//	null               null
//	minted node        _:<label>, and _:<label>^^text for its text form (below)
//
// Doubles are written as held, not rounded: the Go engine quantises a double
// literal to 15 bits of mantissa (jets/jetrules/rete/README.md, "The Go and C++
// engines are independently maintained and they disagree", item 3) and the C++
// engine does not, so the same amount reads 12345.67 on C++ and 12345.5 on Go.
// That difference is kept in the dump and classified by the diff, which knows
// the rounding. The names of minted nodes, below, are computed on the quantised
// value, so a quantisation difference does not rename a node.
//
// # Minted nodes
//
// A minted node is one whose name an engine invents, so that two runs can name
// the same node differently:
//
//   - a blank node (multi-lookup rows, CreateBNode, …);
//   - a resource whose name is a random (version 4) UUID: create_uuid_resource,
//     and create_entity given an empty key. create_entity also writes the UUID
//     as the text of jets:key, so a text literal holding a version 4 UUID is the
//     same minted node, written _:<label>^^text. Version 3 and 5 UUIDs
//     (uuid_md5, uuid_sha1) are computed from their input and are not minted;
//   - a lookup subject: the object of a lookup cache triple. The engines name
//     both differently: Go caches (jets:cache:<table> jets:lookup_row
//     jets:lookup:<key>), C++ caches (<table> jets:lookup_row jets:<table>:<key>).
//     A C++ cache resource, recognised by that pattern, is renamed
//     jets:cache:<table>, the Go name, so that the cache itself compares equal.
//
// Each minted node is renamed from its structure, never from its name, by colour
// refinement (the one-dimensional Weisfeiler–Leman algorithm):
//
//  1. Every minted node starts with the same colour.
//  2. A round gives each node a new colour: the SHA-256 of its colour and the
//     sorted list of the triples it is in, each written with the node itself as
//     @, every other minted node as its current colour, and every other term as
//     above (doubles quantised). The predicate of a cache triple is written
//     jets:lookup_cache, because the engines use different ones (below).
//  3. The label is the first 12 hex digits of the colour after exactly three
//     rounds. A fixed count keeps a label local: it depends on the node's
//     neighbourhood up to three minted hops away and on nothing else, so a
//     difference elsewhere in the graph does not rename it.
//  4. Nodes that share a three-round colour are told apart by refining further
//     until the partition is stable, and get the suffix .1, .2, … in the order
//     of their stable colour. Nodes still tied are interchangeable when none of
//     them has a minted neighbour that is itself tied; they are numbered in the
//     order of their engine names, which cannot change the dump. Otherwise one of
//     them, the first by engine name, is singled out and the refinement runs
//     again (individualisation and refinement), until no tie is left.
//
// The order is therefore defined by structure: labels are content hashes, and
// the suffix order is the order of the refined colours. Two graphs equal up to
// the naming of minted nodes give identical dumps, with one known exception: two
// minted nodes that colour refinement cannot tell apart and that are not
// interchangeable (a regular structure, such as two cycles of equal length made
// only of minted nodes) are separated by engine name, which can differ between
// runs. None has been seen in a workspace.
//
// The labels are not numbered by first appearance in the sorted dump, the
// alternative §5.7 names: a number given by position changes for every node
// after an inserted one, so a single extra minted node would rename every node
// after it and make the whole dump differ.
//
// # Lookup bookkeeping
//
// The cache triples differ between the engines beyond their names: Go caches a
// multi-lookup with jets:lookup_multi_rows and C++ with jets:lookup_row. They are
// written as each engine holds them, and the diff classifies them.
package graphdump
