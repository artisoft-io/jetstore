package rdf

// MICRO-BENCHMARKS FOR THE GO RDF GRAPH (project rules_benchmark, task AB.6).
//
// They exist to separate three costs the engine pays on every graph read, so that a
// change to one of them can be measured on its own with benchstat:
//
//   - the iterator: three goroutines per BaseGraphIterator joined by unbuffered
//     channels, and a fourth per RdfSessionIterator merging the meta, asserted and
//     inferred graphs (base_graph_iterator.go, rdf_session_iterator.go);
//   - sync.Map: the three nested maps of BaseGraph (base_graph.go), each access an
//     `any` type assertion;
//   - the work itself: walking plain nested maps of the same triples.
//
// So every find benchmark has an /impl= dimension:
//
//	impl=chan      the engine's code path: FindSPO and its iterator, drained
//	impl=syncmap   a synchronous nested Range over the SAME sync.Maps, no goroutines
//	impl=plainmap  a synchronous walk over a plain map[*Node]map[*Node]map[*Node]int
//	               copy of the same indexes
//
// chan minus syncmap is the iterator's cost; syncmap minus plainmap is sync.Map's.
// The syncmap and plainmap walks are REFERENCES written here, not engine code: they
// follow FindSPO's choice of index and binding exactly, and they are what a
// synchronous iterator (G1) and plain maps (G2) would at best cost.
//
// The benchmarks:
//
//	BenchmarkBaseGraphInsert      /impl=syncmap|plainmap /triples=1000|10000
//	                              one op = a fresh graph filled with that many triples
//	BenchmarkRdfGraphInsert       /triples=1000|10000 /callbacks=0|16|128
//	                              three indexes plus the linear callback loop (F5)
//	BenchmarkRdfGraphFindSPO      /triples=1000|10000|100000
//	                              /pattern=spo|sp|s|so|p|po|o|none /impl=chan|syncmap|plainmap
//	                              one op = one find, fully drained
//	BenchmarkRdfSessionFind       /pattern=sp|s /impl=chan|syncmap
//	                              the same across meta, asserted and inferred graphs
//	BenchmarkRdfSessionGetObject  /where=asserted|inferred|miss
//	                              the early stop on a bound subject; also reports
//	                              leaked-goroutines/op (F3, I-1)
//	BenchmarkExist                /op=exist|exist_not /where=hit|miss  (F13)
//	BenchmarkContains             /op=contains|contains_sp /where=hit|miss
//	                              the iterator-free alternative to Exist (G7)
//	BenchmarkReifyResource        /kind=resource|text|int|double|date
//	                              re-interning an already interned node (F12)
//	BenchmarkLiteralInterning     /kind=text|int|double /where=root|session|new
//	BenchmarkTrueFalse            the allocation TRUE() and FALSE() make per call
//
// Graph shape, chosen so that every binding pattern returns a predictable number of
// triples: n/10 subjects, 10 predicates, each (subject, predicate) pair holding one of
// 100 text-literal objects. So per find: spo, sp and so return 1, s returns 10, po
// returns n/1000, o returns n/100, p returns n/10, none returns n. The bound keys
// rotate over 1,024 triples drawn from the graph with a fixed seed.
//
// Set-up (building graphs, drawing keys) is outside the timed loop; graphs are built
// once per size and shared by the sub-benchmarks, which only read them.
//
// Smoke check, which runs in seconds and times nothing worth reading:
//
//	go test -run '^$' -bench . -benchtime=100x ./jets/jetrules/rdf
//
// For measurement, run alone on a quiet machine, with -count and benchstat.

import (
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"
)

const (
	benchPredicates = 10
	benchObjects    = 100
	benchKeys       = 1024
)

var benchSizes = []int{1_000, 10_000, 100_000}

// benchGraph is an RdfGraph of n triples with the shape described above, a plain-map
// copy of its three indexes, and keys drawn from it.
type benchGraph struct {
	rm       *ResourceManager
	g        *RdfGraph
	subjects []*Node
	preds    []*Node
	objs     []*Node
	triples  [][3]*Node
	keys     [][3]*Node // benchKeys triples drawn from the graph
	plain    map[byte]plainIndex
}

type plainIndex = map[*Node]map[*Node]map[*Node]int

var (
	benchGraphsMu sync.Mutex
	benchGraphs   = map[int]*benchGraph{}
)

func makeBenchTriples(rm *ResourceManager, n int) (subjects, preds, objs []*Node, triples [][3]*Node) {
	nsubj := n / benchPredicates
	subjects = make([]*Node, nsubj)
	for i := range subjects {
		subjects[i] = rm.NewResource(fmt.Sprintf("bench:s%d", i))
	}
	preds = make([]*Node, benchPredicates)
	for i := range preds {
		preds[i] = rm.NewResource(fmt.Sprintf("bench:p%d", i))
	}
	objs = make([]*Node, benchObjects)
	for i := range objs {
		objs[i] = rm.NewTextLiteral(fmt.Sprintf("value %d", i))
	}
	triples = make([][3]*Node, 0, n)
	for s := range subjects {
		for p := range preds {
			triples = append(triples, [3]*Node{subjects[s], preds[p], objs[(s+p)%benchObjects]})
		}
	}
	return
}

func drawKeys(triples [][3]*Node) [][3]*Node {
	r := rand.New(rand.NewSource(20261020))
	keys := make([][3]*Node, benchKeys)
	for i := range keys {
		keys[i] = triples[r.Intn(len(triples))]
	}
	return keys
}

func getBenchGraph(b *testing.B, n int) *benchGraph {
	b.Helper()
	benchGraphsMu.Lock()
	defer benchGraphsMu.Unlock()
	if bg := benchGraphs[n]; bg != nil {
		return bg
	}
	rm := NewResourceManager(nil)
	bg := &benchGraph{rm: rm, g: NewRdfGraph("ASSERTED")}
	bg.subjects, bg.preds, bg.objs, bg.triples = makeBenchTriples(rm, n)
	for _, t3 := range bg.triples {
		if _, err := bg.g.Insert(t3[0], t3[1], t3[2]); err != nil {
			b.Fatal(err)
		}
	}
	bg.keys = drawKeys(bg.triples)
	bg.plain = map[byte]plainIndex{
		's': toPlainIndex(bg.g.spoGraph),
		'p': toPlainIndex(bg.g.posGraph),
		'o': toPlainIndex(bg.g.ospGraph),
	}
	benchGraphs[n] = bg
	return bg
}

func toPlainIndex(g *BaseGraph) plainIndex {
	out := make(plainIndex)
	g.data.Range(func(u, vm any) bool {
		vmap := make(map[*Node]map[*Node]int)
		out[u.(*Node)] = vmap
		vm.(*sync.Map).Range(func(v, wm any) bool {
			wmap := make(map[*Node]int)
			vmap[v.(*Node)] = wmap
			wm.(*sync.Map).Range(func(w, c any) bool {
				wmap[w.(*Node)] = c.(int)
				return true
			})
			return true
		})
		return true
	})
	return out
}

// THE THREE IMPLEMENTATIONS OF A FIND. Each calls fn for every matching triple, in
// (s, p, o) order, and returns early when fn returns false.

// bindPattern is FindSPO's switch: which index, and which of u, v, w are bound.
func bindPattern(s, p, o *Node) (spin byte, u, v, w *Node) {
	switch {
	case s == nil && p == nil && o == nil:
		return 's', nil, nil, nil
	case s != nil && p == nil && o == nil:
		return 's', s, nil, nil
	case s != nil && p != nil && o == nil:
		return 's', s, p, nil
	case s != nil && p != nil && o != nil:
		return 's', s, p, o
	case s == nil && p != nil && o == nil:
		return 'p', p, nil, nil
	case s == nil && p != nil && o != nil:
		return 'p', p, o, nil
	case s == nil && p == nil && o != nil:
		return 'o', o, nil, nil
	default: // s != nil && p == nil && o != nil
		return 'o', o, s, nil
	}
}

func (bg *benchGraph) baseGraph(spin byte) *BaseGraph {
	switch spin {
	case 'p':
		return bg.g.posGraph
	case 'o':
		return bg.g.ospGraph
	}
	return bg.g.spoGraph
}

// findChan is the engine's path.
func findChan(g *RdfGraph, s, p, o *Node, fn func([3]*Node) bool) {
	itor := g.FindSPO(s, p, o)
	defer itor.Done()
	for t3 := range itor.Itor {
		if !fn(t3) {
			return
		}
	}
}

// walkSync walks one BaseGraph's sync.Maps synchronously, binding as
// NewBaseGraphIterator does.
func walkSync(spin byte, data UMapType, u, v, w *Node, fn func([3]*Node) bool) bool {
	walkW := func(uu, vv *Node, wm *sync.Map) bool {
		if w != nil {
			if c, _ := wm.Load(w); c != nil && c.(int) > 0 {
				return fn(mapUVW2SPOArr(spin, uu, vv, w))
			}
			return true
		}
		cont := true
		wm.Range(func(ww, _ any) bool {
			cont = fn(mapUVW2SPOArr(spin, uu, vv, ww.(*Node)))
			return cont
		})
		return cont
	}
	walkV := func(uu *Node, vm *sync.Map) bool {
		if v != nil {
			if wm, _ := vm.Load(v); wm != nil {
				return walkW(uu, v, wm.(*sync.Map))
			}
			return true
		}
		cont := true
		vm.Range(func(vv, wm any) bool {
			cont = walkW(uu, vv.(*Node), wm.(*sync.Map))
			return cont
		})
		return cont
	}
	if u != nil {
		if vm, _ := data.Load(u); vm != nil {
			return walkV(u, vm.(*sync.Map))
		}
		return true
	}
	cont := true
	data.Range(func(uu, vm any) bool {
		cont = walkV(uu.(*Node), vm.(*sync.Map))
		return cont
	})
	return cont
}

func (bg *benchGraph) findSync(s, p, o *Node, fn func([3]*Node) bool) {
	spin, u, v, w := bindPattern(s, p, o)
	walkSync(spin, bg.baseGraph(spin).data, u, v, w, fn)
}

// walkPlain is walkSync over plain maps.
func walkPlain(spin byte, data plainIndex, u, v, w *Node, fn func([3]*Node) bool) bool {
	walkW := func(uu, vv *Node, wm map[*Node]int) bool {
		if w != nil {
			if wm[w] > 0 {
				return fn(mapUVW2SPOArr(spin, uu, vv, w))
			}
			return true
		}
		for ww := range wm {
			if !fn(mapUVW2SPOArr(spin, uu, vv, ww)) {
				return false
			}
		}
		return true
	}
	walkV := func(uu *Node, vm map[*Node]map[*Node]int) bool {
		if v != nil {
			if wm := vm[v]; wm != nil {
				return walkW(uu, v, wm)
			}
			return true
		}
		for vv, wm := range vm {
			if !walkW(uu, vv, wm) {
				return false
			}
		}
		return true
	}
	if u != nil {
		if vm := data[u]; vm != nil {
			return walkV(u, vm)
		}
		return true
	}
	for uu, vm := range data {
		if !walkV(uu, vm) {
			return false
		}
	}
	return true
}

func (bg *benchGraph) findPlain(s, p, o *Node, fn func([3]*Node) bool) {
	spin, u, v, w := bindPattern(s, p, o)
	walkPlain(spin, bg.plain[spin], u, v, w, fn)
}

// THE PATTERNS. Each maps a key triple to the (s, p, o) arguments of a find.
var benchPatterns = []struct {
	name string
	bind func(k [3]*Node) (s, p, o *Node)
}{
	{"spo", func(k [3]*Node) (*Node, *Node, *Node) { return k[0], k[1], k[2] }},
	{"sp", func(k [3]*Node) (*Node, *Node, *Node) { return k[0], k[1], nil }},
	{"s", func(k [3]*Node) (*Node, *Node, *Node) { return k[0], nil, nil }},
	{"so", func(k [3]*Node) (*Node, *Node, *Node) { return k[0], nil, k[2] }},
	{"p", func(k [3]*Node) (*Node, *Node, *Node) { return nil, k[1], nil }},
	{"po", func(k [3]*Node) (*Node, *Node, *Node) { return nil, k[1], k[2] }},
	{"o", func(k [3]*Node) (*Node, *Node, *Node) { return nil, nil, k[2] }},
	{"none", func(k [3]*Node) (*Node, *Node, *Node) { return nil, nil, nil }},
}

// benchSink and benchNodeSink keep the compiler from discarding a result, and
// benchNodeSink makes a returned node escape, as it does in the engine.
var (
	benchSink     int
	benchNodeSink *Node
)

// reportLeakedGoroutines waits, outside the timer, for goroutines started by the
// timed loop to finish, and reports those still running per op. A goroutine blocked
// forever on a channel (F3) is counted; one merely slow to exit is waited for.
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

// INSERT

func BenchmarkBaseGraphInsert(b *testing.B) {
	for _, impl := range []string{"syncmap", "plainmap"} {
		for _, n := range benchSizes[:2] {
			b.Run(fmt.Sprintf("impl=%s/triples=%d", impl, n), func(b *testing.B) {
				_, _, _, triples := makeBenchTriples(NewResourceManager(nil), n)
				b.ReportAllocs()
				switch impl {
				case "syncmap":
					for b.Loop() {
						g := NewBaseGraph("ASSERTED", 's')
						for _, t3 := range triples {
							g.Insert(t3[0], t3[1], t3[2])
						}
						benchSink += g.Size()
					}
				case "plainmap":
					for b.Loop() {
						g := make(plainIndex)
						for _, t3 := range triples {
							vm := g[t3[0]]
							if vm == nil {
								vm = make(map[*Node]map[*Node]int)
								g[t3[0]] = vm
							}
							wm := vm[t3[1]]
							if wm == nil {
								wm = make(map[*Node]int)
								vm[t3[1]] = wm
							}
							wm[t3[2]]++
						}
						benchSink += len(g)
					}
				}
				b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*n), "ns/triple")
			})
		}
	}
}

// noopCallback stands for an alpha node's callback; it does no work, so what is
// measured is the dispatch loop alone.
type noopCallback struct{ n int }

func (c *noopCallback) TripleInserted(s, p, o *Node) { c.n++ }
func (c *noopCallback) TripleDeleted(s, p, o *Node)  { c.n-- }

func BenchmarkRdfGraphInsert(b *testing.B) {
	for _, n := range benchSizes[:2] {
		for _, ncb := range []int{0, 16, 128} {
			b.Run(fmt.Sprintf("triples=%d/callbacks=%d", n, ncb), func(b *testing.B) {
				_, _, _, triples := makeBenchTriples(NewResourceManager(nil), n)
				cbs := make([]*noopCallback, ncb)
				for i := range cbs {
					cbs[i] = &noopCallback{}
				}
				b.ReportAllocs()
				for b.Loop() {
					g := NewRdfGraph("INFERRED")
					for _, cb := range cbs {
						g.CallbackMgr.AddCallback(cb)
					}
					for _, t3 := range triples {
						if _, err := g.Insert(t3[0], t3[1], t3[2]); err != nil {
							b.Fatal(err)
						}
					}
					benchSink += g.Size()
				}
				b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*n), "ns/triple")
			})
		}
	}
}

// FIND

func BenchmarkRdfGraphFindSPO(b *testing.B) {
	for _, n := range benchSizes {
		for _, pat := range benchPatterns {
			for _, impl := range []string{"chan", "syncmap", "plainmap"} {
				b.Run(fmt.Sprintf("triples=%d/pattern=%s/impl=%s", n, pat.name, impl), func(b *testing.B) {
					bg := getBenchGraph(b, n)
					var find func(s, p, o *Node, fn func([3]*Node) bool)
					switch impl {
					case "chan":
						find = func(s, p, o *Node, fn func([3]*Node) bool) { findChan(bg.g, s, p, o, fn) }
					case "syncmap":
						find = bg.findSync
					case "plainmap":
						find = bg.findPlain
					}
					count := 0
					visit := func(t3 [3]*Node) bool {
						count++
						return true
					}
					b.ReportAllocs()
					i := 0
					for b.Loop() {
						s, p, o := pat.bind(bg.keys[i%benchKeys])
						find(s, p, o, visit)
						i++
					}
					benchSink += count
					b.ReportMetric(float64(count)/float64(b.N), "triples/op")
				})
			}
		}
	}
}

// benchSession is an RdfSession over a meta graph of 1,000 triples on subjects of its
// own, an asserted graph of 10,000 (benchGraph's shape) and an inferred graph holding
// one more predicate on every asserted subject.
type benchSession struct {
	sess          *RdfSession
	keys          [][3]*Node // asserted triples
	inferredKeys  [][3]*Node // inferred triples
	missSubjects  []*Node    // subjects in no graph
	inferredPred  *Node
	assertedTotal int
}

var (
	benchSessionOnce sync.Once
	benchSess        *benchSession
)

func getBenchSession(b *testing.B) *benchSession {
	b.Helper()
	benchSessionOnce.Do(func() {
		root := NewResourceManager(nil)
		meta := NewMetaRdfGraph(root)
		for i := range 100 {
			ms := root.NewResource(fmt.Sprintf("meta:s%d", i))
			for j := range benchPredicates {
				meta.Insert(ms, root.NewResource(fmt.Sprintf("meta:p%d", j)), root.NewIntLiteral(i*benchPredicates+j))
			}
		}
		sess := NewRdfSession(root, meta)
		rm := sess.ResourceMgr
		subjects, _, _, triples := makeBenchTriples(rm, 10_000)
		for _, t3 := range triples {
			sess.Insert(t3[0], t3[1], t3[2])
		}
		inferredPred := rm.NewResource("bench:inferred")
		inferred := make([][3]*Node, 0, len(subjects))
		for i, s := range subjects {
			o := rm.NewIntLiteral(i)
			sess.InsertInferred(s, inferredPred, o)
			inferred = append(inferred, [3]*Node{s, inferredPred, o})
		}
		miss := make([]*Node, benchKeys)
		for i := range miss {
			miss[i] = rm.NewResource(fmt.Sprintf("bench:missing%d", i))
		}
		benchSess = &benchSession{
			sess:          sess,
			keys:          drawKeys(triples),
			inferredKeys:  drawKeys(inferred),
			missSubjects:  miss,
			inferredPred:  inferredPred,
			assertedTotal: len(triples),
		}
	})
	return benchSess
}

// sessionFindSync is the session's find over sync.Maps without goroutines: the three
// graphs in the order NewRdfSessionIterator reads them.
func sessionFindSync(sess *RdfSession, s, p, o *Node, fn func([3]*Node) bool) {
	spin, u, v, w := bindPattern(s, p, o)
	for _, g := range []*RdfGraph{sess.MetaGraph, sess.AssertedGraph, sess.InferredGraph} {
		var base *BaseGraph
		switch spin {
		case 'p':
			base = g.posGraph
		case 'o':
			base = g.ospGraph
		default:
			base = g.spoGraph
		}
		if !walkSync(spin, base.data, u, v, w, fn) {
			return
		}
	}
}

func BenchmarkRdfSessionFind(b *testing.B) {
	for _, pat := range benchPatterns[1:3] { // sp, s
		for _, impl := range []string{"chan", "syncmap"} {
			b.Run(fmt.Sprintf("pattern=%s/impl=%s", pat.name, impl), func(b *testing.B) {
				bs := getBenchSession(b)
				count := 0
				visit := func(t3 [3]*Node) bool {
					count++
					return true
				}
				b.ReportAllocs()
				i := 0
				for b.Loop() {
					s, p, o := pat.bind(bs.keys[i%benchKeys])
					switch impl {
					case "chan":
						itor := bs.sess.FindSPO(s, p, o)
						for t3 := range itor.Itor {
							visit(t3)
						}
						itor.Done()
					case "syncmap":
						sessionFindSync(bs.sess, s, p, o, visit)
					}
					i++
				}
				benchSink += count
				b.ReportMetric(float64(count)/float64(b.N), "triples/op")
			})
		}
	}
}

// sessionKey returns the (s, p) a lookup binds, for where=asserted|inferred|miss.
func (bs *benchSession) sessionKey(where string, i int) (s, p *Node) {
	switch where {
	case "asserted":
		k := bs.keys[i%benchKeys]
		return k[0], k[1]
	case "inferred":
		k := bs.inferredKeys[i%benchKeys]
		return k[0], k[1]
	default:
		return bs.missSubjects[i%benchKeys], bs.inferredPred
	}
}

// BenchmarkRdfSessionGetObject is the early stop: GetObject reads one triple and
// calls Done on an iterator whose producers may still be running.
func BenchmarkRdfSessionGetObject(b *testing.B) {
	for _, where := range []string{"asserted", "inferred", "miss"} {
		b.Run("where="+where, func(b *testing.B) {
			bs := getBenchSession(b)
			found := 0
			before := runtime.NumGoroutine()
			b.ReportAllocs()
			i := 0
			for b.Loop() {
				s, p := bs.sessionKey(where, i)
				if bs.sess.GetObject(s, p) != nil {
					found++
				}
				i++
			}
			benchSink += found
			reportLeakedGoroutines(b, before)
		})
	}
}

// BenchmarkExist is the exist and exist_not operators as rules evaluate them:
// through GetObject (F13), with a new TRUE or FALSE node per call (F12).
func BenchmarkExist(b *testing.B) {
	for _, op := range []string{"exist", "exist_not"} {
		for _, where := range []string{"hit", "miss"} {
			b.Run(fmt.Sprintf("op=%s/where=%s", op, where), func(b *testing.B) {
				bs := getBenchSession(b)
				lookup := "asserted"
				if where == "miss" {
					lookup = "miss"
				}
				before := runtime.NumGoroutine()
				b.ReportAllocs()
				i := 0
				for b.Loop() {
					s, p := bs.sessionKey(lookup, i)
					var r *Node
					if op == "exist" {
						r = s.Exist(bs.sess, p)
					} else {
						r = s.ExistNot(bs.sess, p)
					}
					if r.Bool() {
						benchSink++
					}
					i++
				}
				reportLeakedGoroutines(b, before)
			})
		}
	}
}

// BenchmarkContains is the iterator-free lookups Exist could use (G7).
func BenchmarkContains(b *testing.B) {
	for _, op := range []string{"contains", "contains_sp"} {
		for _, where := range []string{"hit", "miss"} {
			b.Run(fmt.Sprintf("op=%s/where=%s", op, where), func(b *testing.B) {
				bs := getBenchSession(b)
				b.ReportAllocs()
				i := 0
				for b.Loop() {
					k := bs.keys[i%benchKeys]
					s, p, o := k[0], k[1], k[2]
					if where == "miss" {
						s = bs.missSubjects[i%benchKeys]
					}
					var ok bool
					if op == "contains" {
						ok = bs.sess.Contains(s, p, o)
					} else {
						ok = bs.sess.ContainsSP(s, p)
					}
					if ok {
						benchSink++
					}
					i++
				}
			})
		}
	}
}

// INTERNING

// benchNodes returns benchKeys uninterned nodes of a kind, as an expression's result
// would be before ReifyResource.
func benchNodes(kind string) []*Node {
	nodes := make([]*Node, benchKeys)
	for i := range nodes {
		switch kind {
		case "resource":
			nodes[i] = R(fmt.Sprintf("bench:r%d", i))
		case "text":
			nodes[i] = S(fmt.Sprintf("text %d", i))
		case "int":
			nodes[i] = I(i)
		case "double":
			nodes[i] = F(float64(i) + 0.25)
		case "date":
			d := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i)
			nodes[i] = &Node{Value: LDate{Date: &d}}
		}
	}
	return nodes
}

// BenchmarkReifyResource re-interns nodes that are already interned in the session's
// manager: the steady state of an expression evaluated on every row.
func BenchmarkReifyResource(b *testing.B) {
	for _, kind := range []string{"resource", "text", "int", "double", "date"} {
		b.Run("kind="+kind, func(b *testing.B) {
			rm := NewResourceManager(NewResourceManager(nil))
			nodes := benchNodes(kind)
			for _, n := range nodes {
				rm.ReifyResource(n)
			}
			b.ReportAllocs()
			i := 0
			for b.Loop() {
				if rm.ReifyResource(nodes[i%benchKeys]) != nil {
					benchSink++
				}
				i++
			}
		})
	}
}

// BenchmarkLiteralInterning is NewTextLiteral, NewIntLiteral and NewDoubleLiteral on
// a session's manager: where=root finds the literal in the locked root manager,
// where=session in the session's own, where=new creates it.
func BenchmarkLiteralInterning(b *testing.B) {
	for _, kind := range []string{"text", "int", "double"} {
		for _, where := range []string{"root", "session", "new"} {
			b.Run(fmt.Sprintf("kind=%s/where=%s", kind, where), func(b *testing.B) {
				texts := make([]string, benchKeys)
				for i := range texts {
					texts[i] = fmt.Sprintf("text %d", i)
				}
				intern := func(rm *ResourceManager, i int) *Node {
					switch kind {
					case "text":
						return rm.NewTextLiteral(texts[i%benchKeys])
					case "int":
						return rm.NewIntLiteral(i % benchKeys)
					default:
						return rm.NewDoubleLiteral(float64(i%benchKeys) + 0.25)
					}
				}
				root := NewResourceManager(nil)
				if where == "root" {
					for i := range benchKeys {
						intern(root, i)
					}
				}
				rm := NewResourceManager(root)
				if where == "session" {
					for i := range benchKeys {
						intern(rm, i)
					}
				}
				b.ReportAllocs()
				if where == "new" {
					// A fresh manager every benchKeys literals, outside the timer, so
					// every timed call creates its literal.
					for i := 0; i < b.N; i++ {
						if i%benchKeys == 0 {
							b.StopTimer()
							rm = NewResourceManager(root)
							b.StartTimer()
						}
						if intern(rm, i) != nil {
							benchSink++
						}
					}
					return
				}
				i := 0
				for b.Loop() {
					if intern(rm, i) != nil {
						benchSink++
					}
					i++
				}
			})
		}
	}
}

func BenchmarkTrueFalse(b *testing.B) {
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		benchNodeSink = B(i%2 == 0)
		i++
	}
}
