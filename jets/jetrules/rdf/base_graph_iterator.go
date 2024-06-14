package rdf

// import "log"

// BaseGraph Iterator implemented as channels

type Upair struct {
	u    *Node
	data VMapType
}
type Vpair struct {
	u, v *Node
	data WSetType
}

type BaseGraphIterator struct {
	uSource chan Upair
	vSource chan Vpair
	Itor    chan [3]*Node
	done    chan struct{}
	spin    byte
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
func NewBaseGraphIterator(spin byte, u, v, w *Node, g *UMapType) *BaseGraphIterator {
	bgItor := &BaseGraphIterator{
		uSource: make(chan Upair),
		vSource: make(chan Vpair),
		Itor:    make(chan [3]*Node),
		done:    make(chan struct{}),
		spin:    spin,
	}
	// Connect the sources to Itor
	// Iterate over the subjects (the u's)
	go func() {
		if u == nil {
			// Iterate over all elements
			for node, data := range *g {
				select {
				case bgItor.uSource <- Upair{u: node, data: data}:
				case <-bgItor.done:
					return
				}
			}
		} else {
			// Single value for u
			data := (*g)[u]
			if data != nil {
				select {
				case bgItor.uSource <- Upair{u: u, data: data}:
				case <-bgItor.done:
					return
				}
			}
		}
		close(bgItor.uSource)
	}()
	// For each subject, iterate over the predicates (the v's)
	go func() {
		for upair := range bgItor.uSource {
			if v == nil {
				// Iterate over all elements
				for node, data := range upair.data {
					select {
					case bgItor.vSource <- Vpair{u: upair.u, v: node, data: data}:
					case <-bgItor.done:
						return
					}
				}
			} else {
				// Single value for v
				data := upair.data[v]
				if data != nil {
					select {
					case bgItor.vSource <- Vpair{u: upair.u, v: v, data: data}:
					case <-bgItor.done:
						return
					}
				}
			}
		}
		close(bgItor.vSource)
	}()
	// For each subject, predicate pair, iterate over the objects (the w's)
	go func() {
		for vpair := range bgItor.vSource {
			if w == nil {
				// Iterate over all elements
				for node := range vpair.data {
					select {
					case bgItor.Itor <- mapUVW2SPOArr(spin, vpair.u, vpair.v, node):
					case <-bgItor.done:
						return
					}
				}
			} else {
				// Single value for w
				c := vpair.data[w]
				if c != 0 {
					select {
					case bgItor.Itor <- mapUVW2SPOArr(spin, vpair.u, vpair.v, w):
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
