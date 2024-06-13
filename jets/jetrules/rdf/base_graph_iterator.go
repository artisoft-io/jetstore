package rdf

import (
	"fmt"
)

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
}
func (itor *BaseGraphIterator) Done() {
	select {
	case <-itor.done:
		// log.Printf("##!@@ Done ch was already closed!")
	default:
		close(itor.done)
		// log.Printf("##!@@ Done ch closed")
	}
}

func NewBaseGraphIterator(u, v *Node, g *UMapType) (*BaseGraphIterator, error) {
	// Some validation
	if u == nil && v != nil {
		return nil, fmt.Errorf("error: cannot have u to be nil when v is not nil")
	}
	bgItor := &BaseGraphIterator{
		uSource: make(chan Upair),
		vSource: make(chan Vpair),
		Itor:    make(chan [3]*Node),
		done:    make(chan struct{}),
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
				// Single value for u, v
				data := upair.data[v]
				if data != nil {
					select {
					case bgItor.vSource <- Vpair{u: u, v: v, data: data}:
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
			// Iterate over all elements
			for node := range vpair.data {
				select {
				case bgItor.Itor <- [3]*Node{vpair.u, vpair.v, node}:
				case <-bgItor.done:
					return
				}
			}
		}
		close(bgItor.Itor)
	}()
	return bgItor, nil
}
