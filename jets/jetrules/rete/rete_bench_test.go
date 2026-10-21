package rete

// MICRO-BENCHMARKS FOR THE GO RETE SESSION (project rules_benchmark, task AB.6).
// The graph's own benchmarks, and the reasoning for the /impl= dimension, are in
// jets/jetrules/rdf/graph_bench_test.go.
//
//	BenchmarkBetaRowHash        /nodes=3|6|10 /impl=fnv|pointer
//	                            impl=fnv is BetaRow.Hash: an fnv hasher and a
//	                            MarshalBinary per node, which copies a string (F7).
//	                            impl=pointer is a REFERENCE written here, not engine
//	                            code: the nodes' pointers hashed with maphash, which is
//	                            what hashing by identity (G3) would cost. Row equality
//	                            is already by pointer (BetaRow.Eq).
//	BenchmarkBetaRowInitialize  /nodes=3|6|10
//	                            a candidate row as VisitReteGraph makes one: allocated,
//	                            filled from its parent row and a triple, and hashed
//	BenchmarkReteSessionInitialize /claims=10|100|1000
//	                            NewReteSession and Initialize on a member's asserted
//	                            data: a beta relation per vertex and callback
//	                            registration (F8), paid once per rule set
//	BenchmarkExecuteRules       /claims=10|100|1000
//	                            ExecuteRules alone on the network below; the session's
//	                            data and Initialize are outside the timer. Also reports
//	                            leaked-goroutines/op: the filter's exist stopped an
//	                            iterator early (rules_benchmark F3, I-1) until G1
//	                            and G7; 0 by construction since
//	BenchmarkVisitReteGraph     /vertices=6|100|450
//	                            one VisitReteGraph from the head of a network of that
//	                            many vertices, all children of the head, with nothing
//	                            pending: what a visit costs before it matches anything.
//	                            Before G6 each call allocated a stack of NbrVertices
//	                            ints (rules_benchmark F6, F103); CM's rule sets have 59
//	                            to 447 vertices. Read B/op and allocs/op
//
// Rows hold half named resources and half text literals of 20 to 40 characters,
// the shape of a mapped claim's keys and codes.
//
// THE NETWORK is hand-built, as loop_through_negation_test.go builds its own, so that
// the benchmark needs no compiled workspace. Two rules, five vertices, a join, a filter
// through exist and a negation:
//
//	v1..v3  (?c rdf:type Claim).(?c member ?m).[(?c exist amount)].(?m rdf:type Member)
//	          -> (?m hasClaim ?c)
//	v4..v5  (?m hasClaim ?c).not(?c excluded ?m) -> (?c linked ?m)
//
// Its data: claims in members of four, every claim with an amount, every tenth claim
// excluded from its member. Every call of BenchmarkExecuteRules checks the inferred
// counts once, untimed, before its loop (testing calls it once per b.N probe), so the
// benchmark cannot time a network that does nothing. $max_rule_exec is set well
// above what the largest size needs (rules_benchmark F9).
//
// Smoke check, which runs in seconds and times nothing worth reading:
//
//	go test -run '^$' -bench . -benchtime=100x ./jets/jetrules/rete
//
// For measurement, run alone on a quiet machine, with -count and benchstat.

import (
	"fmt"
	"hash/maphash"
	"runtime"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

var benchSink uint64

// reportLeakedGoroutines waits, outside the timer, for goroutines started by the
// timed loop to finish, and reports those still running per op. The same helper as
// jets/jetrules/rdf's, which a test file cannot export.
func reportLeakedGoroutines(b *testing.B, before int) {
	b.StopTimer()
	after := runtime.NumGoroutine()
	for range 50 {
		time.Sleep(4 * time.Millisecond)
		n := runtime.NumGoroutine()
		if n == after {
			break
		}
		after = n
	}
	b.ReportMetric(float64(after-before)/float64(b.N), "leaked-goroutines/op")
}

// benchRowNodes returns size nodes alternating named resources and text literals.
func benchRowNodes(rm *rdf.ResourceManager, size, seed int) []*rdf.Node {
	nodes := make([]*rdf.Node, size)
	for i := range nodes {
		if i%2 == 0 {
			nodes[i] = rm.NewResource(fmt.Sprintf("usi:claim/%08d-%02d", seed, i))
		} else {
			nodes[i] = rm.NewTextLiteral(fmt.Sprintf("procedure code %04d modifier %02d", seed%10000, i))
		}
	}
	return nodes
}

// pointerHash is the reference for G3: the row hashed by node identity.
func pointerHash(seed maphash.Seed, row *BetaRow) uint64 {
	var h uint64
	for _, n := range row.Data {
		h = h*31 + maphash.Comparable(seed, n)
	}
	return h
}

func BenchmarkBetaRowHash(b *testing.B) {
	for _, size := range []int{3, 6, 10} {
		for _, impl := range []string{"fnv", "pointer"} {
			b.Run(fmt.Sprintf("nodes=%d/impl=%s", size, impl), func(b *testing.B) {
				rm := rdf.NewResourceManager(nil)
				rows := make([]*BetaRow, 256)
				for i := range rows {
					rows[i] = &BetaRow{Data: benchRowNodes(rm, size, i)}
				}
				seed := maphash.MakeSeed()
				b.ReportAllocs()
				i := 0
				for b.Loop() {
					row := rows[i%len(rows)]
					if impl == "fnv" {
						benchSink += row.Hash()
					} else {
						benchSink += pointerHash(seed, row)
					}
					i++
				}
			})
		}
	}
}

func BenchmarkBetaRowInitialize(b *testing.B) {
	for _, size := range []int{3, 6, 10} {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			rm := rdf.NewResourceManager(nil)
			// All but the last position from the parent row, the last from the
			// triple's object: a child vertex binding one new variable.
			initData := make([]int, size)
			for i := range size - 1 {
				initData[i] = brcParentNode | i
			}
			initData[size-1] = brcTriple | 2
			initializer := NewBetaRowInitializer(initData, nil)
			vertex := NewNodeVertex(1, nil, false, 100, nil, "bench", nil, initializer)
			parents := make([]*BetaRow, 256)
			triples := make([]rdf.Triple, 256)
			for i := range parents {
				nodes := benchRowNodes(rm, size, i)
				parents[i] = &BetaRow{Data: nodes[:size-1]}
				triples[i] = rdf.Triple{nodes[0], rm.NewResource("usi:hasCode"), nodes[size-1]}
			}
			b.ReportAllocs()
			i := 0
			for b.Loop() {
				k := i % len(parents)
				row := NewBetaRow(vertex, initializer.RowSize())
				if err := row.Initialize(initializer, parents[k], &triples[k]); err != nil {
					b.Fatal(err)
				}
				benchSink += row.h
				i++
			}
		})
	}
}

// benchNetwork is the two-rule network above, its meta store and its resources.
type benchNetwork struct {
	rm                                  *rdf.ResourceManager
	ms                                  *ReteMetaStore
	claim, member, memberOf, amount     *rdf.Node
	hasClaim, excluded, linked, rdfType *rdf.Node
}

func newBenchNetwork(b testing.TB) *benchNetwork {
	b.Helper()
	rm := rdf.NewResourceManager(nil)
	jr := rm.JetsResources
	metaGraph := rdf.NewMetaRdfGraph(rm)
	n := &benchNetwork{
		rm:       rm,
		claim:    rm.NewResource("bench:Claim"),
		member:   rm.NewResource("bench:Member"),
		memberOf: rm.NewResource("bench:member"),
		amount:   rm.NewResource("bench:amount"),
		hasClaim: rm.NewResource("bench:hasClaim"),
		excluded: rm.NewResource("bench:excluded"),
		linked:   rm.NewResource("bench:linked"),
		rdfType:  jr.Rdf__type,
	}
	hasAmount := NewExprBinaryOp(NewExprBindedVar(0, "?c"), NewExistOp(false), NewExprCst(n.amount))

	head := NewNodeVertex(0, nil, false, 100, nil, "head", nil, nil)
	nv1 := NewNodeVertex(1, head, false, 100, nil, "v1", []string{"r1"},
		NewBetaRowInitializer([]int{brcTriple | 0}, []string{"?c"}))
	nv2 := NewNodeVertex(2, nv1, false, 100, hasAmount, "v2", []string{"r1"},
		NewBetaRowInitializer([]int{brcParentNode | 0, brcTriple | 2}, []string{"?c", "?m"}))
	nv3 := NewNodeVertex(3, nv2, false, 100, nil, "v3", []string{"r1"},
		NewBetaRowInitializer([]int{brcParentNode | 0, brcParentNode | 1}, []string{"?c", "?m"}))
	nv4 := NewNodeVertex(4, head, false, 100, nil, "v4", []string{"r2"},
		NewBetaRowInitializer([]int{brcTriple | 0, brcTriple | 2}, []string{"?m", "?c"}))
	nv5 := NewNodeVertex(5, nv4, true, 100, nil, "v5", []string{"r2"},
		NewBetaRowInitializer([]int{brcParentNode | 0, brcParentNode | 1}, []string{"?m", "?c"}))
	nodeVertices := []*NodeVertex{head, nv1, nv2, nv3, nv4, nv5}

	alphaNodes := []*AlphaNode{
		NewRootAlphaNode(head),
		NewAlphaNode(&FVariable{variable: "?c"}, &FConstant{node: n.rdfType}, &FConstant{node: n.claim}, nv1, true, "(?c rdf:type Claim)"),
		NewAlphaNode(&FBinded{pos: 0}, &FConstant{node: n.memberOf}, &FVariable{variable: "?m"}, nv2, true, "(?c member ?m)"),
		NewAlphaNode(&FBinded{pos: 1}, &FConstant{node: n.rdfType}, &FConstant{node: n.member}, nv3, true, "(?m rdf:type Member)"),
		NewAlphaNode(&FVariable{variable: "?m"}, &FConstant{node: n.hasClaim}, &FVariable{variable: "?c"}, nv4, true, "(?m hasClaim ?c)"),
		NewAlphaNode(&FBinded{pos: 1}, &FConstant{node: n.excluded}, &FBinded{pos: 0}, nv5, true, "not(?c excluded ?m)"),
		NewAlphaNode(&FBinded{pos: 1}, &FConstant{node: n.hasClaim}, &FBinded{pos: 0}, nv3, false, "(?m hasClaim ?c)"),
		NewAlphaNode(&FBinded{pos: 1}, &FConstant{node: n.linked}, &FBinded{pos: 0}, nv5, false, "(?c linked ?m)"),
	}
	for _, an := range alphaNodes {
		switch {
		case an.IsAntecedent:
			an.NdVertex.ParentNodeVertex.AddChildAlphaNode(an)
		case an.IsConsequent:
			an.NdVertex.AddConsequentTerm(an, RetractionMode{})
		}
	}
	config := map[string]string{"$max_rule_exec": "100000000"}
	ms, err := NewReteMetaStore(rm, metaGraph, nil, alphaNodes, nodeVertices, &config, nil, nil)
	if err != nil {
		b.Fatal(err)
	}
	n.ms = ms
	return n
}

// newSession asserts claims claims, in members of four, into a new rdf session.
func (n *benchNetwork) newSession(claims int) *rdf.RdfSession {
	sess := rdf.NewRdfSession(n.rm, n.ms.MetaGraph)
	srm := sess.ResourceMgr
	var m *rdf.Node
	for i := range claims {
		if i%4 == 0 {
			m = srm.NewResource(fmt.Sprintf("member:%06d", i/4))
			sess.Insert(m, n.rdfType, n.member)
		}
		c := srm.NewResource(fmt.Sprintf("claim:%08d", i))
		sess.Insert(c, n.rdfType, n.claim)
		sess.Insert(c, n.memberOf, m)
		sess.Insert(c, n.amount, srm.NewIntLiteral(100+i))
		if i%10 == 0 {
			sess.Insert(c, n.excluded, m)
		}
	}
	return sess
}

// check runs the rules once, untimed, and fails unless they inferred what they must.
func (n *benchNetwork) check(b *testing.B, claims int) {
	b.Helper()
	sess := n.newSession(claims)
	rs := NewReteSession(sess)
	rs.Initialize(n.ms)
	defer rs.Done()
	if err := rs.ExecuteRules(); err != nil {
		b.Fatal(err)
	}
	count := func(p *rdf.Node) int {
		c := 0
		itor := sess.InferredGraph.FindSPO(nil, p, nil)
		for range itor.All() {
			c++
		}
		return c
	}
	excluded := (claims + 9) / 10
	if got := count(n.hasClaim); got != claims {
		b.Fatalf("hasClaim: inferred %d, want %d", got, claims)
	}
	if got := count(n.linked); got != claims-excluded {
		b.Fatalf("linked: inferred %d, want %d", got, claims-excluded)
	}
}

var benchClaims = []int{10, 100, 1000}

func BenchmarkReteSessionInitialize(b *testing.B) {
	for _, claims := range benchClaims {
		b.Run(fmt.Sprintf("claims=%d", claims), func(b *testing.B) {
			n := newBenchNetwork(b)
			sess := n.newSession(claims)
			b.ReportAllocs()
			for b.Loop() {
				rs := NewReteSession(sess)
				rs.Initialize(n.ms)
				b.StopTimer()
				rs.Done()
				b.StartTimer()
			}
		})
	}
}

func BenchmarkExecuteRules(b *testing.B) {
	for _, claims := range benchClaims {
		b.Run(fmt.Sprintf("claims=%d", claims), func(b *testing.B) {
			n := newBenchNetwork(b)
			n.check(b, claims)
			before := runtime.NumGoroutine()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				rs := NewReteSession(n.newSession(claims))
				rs.Initialize(n.ms)
				b.StartTimer()
				if err := rs.ExecuteRules(); err != nil {
					b.Fatal(err)
				}
				b.StopTimer()
				rs.Done()
				b.StartTimer()
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*claims), "ns/claim")
			reportLeakedGoroutines(b, before)
		})
	}
}

// newWideNetwork returns a meta store whose vertices - 1 rule vertices are all
// children of the head, each matching (?x rdf:type bench:T<i>), with no consequent.
func newWideNetwork(b testing.TB, vertices int) (*rdf.ResourceManager, *ReteMetaStore) {
	b.Helper()
	rm := rdf.NewResourceManager(nil)
	rdfType := rm.JetsResources.Rdf__type
	head := NewNodeVertex(0, nil, false, 100, nil, "head", nil, nil)
	nodeVertices := []*NodeVertex{head}
	alphaNodes := []*AlphaNode{NewRootAlphaNode(head)}
	for i := 1; i < vertices; i++ {
		nv := NewNodeVertex(i, head, false, 100, nil, fmt.Sprintf("v%d", i), []string{"r"},
			NewBetaRowInitializer([]int{brcTriple | 0}, []string{"?x"}))
		nodeVertices = append(nodeVertices, nv)
		t := rm.NewResource(fmt.Sprintf("bench:T%d", i))
		an := NewAlphaNode(&FVariable{variable: "?x"}, &FConstant{node: rdfType}, &FConstant{node: t}, nv, true, "(?x rdf:type T)")
		head.AddChildAlphaNode(an)
		alphaNodes = append(alphaNodes, an)
	}
	config := map[string]string{"$max_rule_exec": "100000000"}
	ms, err := NewReteMetaStore(rm, rdf.NewMetaRdfGraph(rm), nil, alphaNodes, nodeVertices, &config, nil, nil)
	if err != nil {
		b.Fatal(err)
	}
	return rm, ms
}

func BenchmarkVisitReteGraph(b *testing.B) {
	for _, vertices := range []int{6, 100, 450} {
		b.Run(fmt.Sprintf("vertices=%d", vertices), func(b *testing.B) {
			rm, ms := newWideNetwork(b, vertices)
			rs := NewReteSession(rdf.NewRdfSession(rm, ms.MetaGraph))
			rs.Initialize(ms)
			defer rs.Done()
			if got := ms.NbrVertices(); got != vertices {
				b.Fatalf("NbrVertices: got %d, want %d", got, vertices)
			}
			// The first visit activates every vertex; the timed ones find nothing pending.
			if err := rs.VisitReteGraph(0, true); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			for b.Loop() {
				if err := rs.VisitReteGraph(0, true); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
