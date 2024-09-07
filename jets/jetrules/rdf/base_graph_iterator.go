package rdf

import "sync"

// import "log"

// BaseGraph Iterator implemented as channels

type Upair struct {
	U    *Node
	Data VMapType
}
type Vpair struct {
	U, V *Node
	Data WSetType
}

type BaseGraphIterator struct {
	Usource chan Upair
	Vsource chan Vpair
	Itor    chan [3]*Node
	done    chan struct{}
	Spin    byte
}

func (itor *BaseGraphIterator) Done() {
	select {
	case <-itor.done:
		// log.Printf("##!@@ Done: BaseGraphIterator ch was already closed!")
	default:
		close(itor.done)
		// log.Printf("##!@@ Done: BaseGraphIterator ch closed")
	}
}

// Iterator over the BaseGraph, u, v, w can be nil
// The iterator returns the triple in (s, p, o) order based on the spin of the graph
func NewBaseGraphIterator(spin byte, u, v, w *Node, g UMapType) *BaseGraphIterator {
	bgItor := &BaseGraphIterator{
		Usource: make(chan Upair),
		Vsource: make(chan Vpair),
		Itor:    make(chan [3]*Node),
		done:    make(chan struct{}),
		Spin:    spin,
	}
	// Connect the sources to Itor
	// Iterate over the subjects (the u's)
	go func() {
		if u == nil {
			// Iterate over all elements
			g.Range(func(node, data any) bool {
				select {
				case bgItor.Usource <- Upair{U: node.(*Node), Data: data.(*sync.Map)}:
				case <-bgItor.done:
					return false
				}
				return true
			})
		} else {
			// Single value for u
			data, _ := g.Load(u)
			if data != nil {
				select {
				case bgItor.Usource <- Upair{U: u, Data: data.(*sync.Map)}:
				case <-bgItor.done:
					return
				}
			}
		}
		close(bgItor.Usource)
	}()
	// For each subject, iterate over the predicates (the v's)
	go func() {
		for upair := range bgItor.Usource {
			if v == nil {
					// Iterate over all elements
					upair.Data.Range(func(node, data any) bool {
					select {
					case bgItor.Vsource <- Vpair{U: upair.U, V: node.(*Node), Data: data.(*sync.Map)}:
					case <-bgItor.done:
						return false
					}
					return true
				})
			} else {
				// Single value for v
				data, _ := upair.Data.Load(v)
				if data != nil {
					select {
					case bgItor.Vsource <- Vpair{U: upair.U, V: v, Data: data.(*sync.Map)}:
					case <-bgItor.done:
						return
					}
				}
			}
		}
		close(bgItor.Vsource)
	}()
	// For each subject, predicate pair, iterate over the objects (the w's)
	go func() {
		for vpair := range bgItor.Vsource {
			if w == nil {
				// Iterate over all elements
				vpair.Data.Range(func(node, data any) bool {
					select {
					case bgItor.Itor <- mapUVW2SPOArr(spin, vpair.U, vpair.V, node.(*Node)):
					case <-bgItor.done:
						return false
					}
					return true
				})
			} else {
				// Single value for w
				c, _ := vpair.Data.Load(w)
				if c != nil && c.(int) > 0 {
					select {
					case bgItor.Itor <- mapUVW2SPOArr(spin, vpair.U, vpair.V, w):
					case <-bgItor.done:
						return
					}
				}
			}
		}
		close(bgItor.Itor)
	}()
	return bgItor
}
