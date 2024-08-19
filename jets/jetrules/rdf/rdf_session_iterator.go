package rdf

// RdfSession Iterator, it's essentially a funnel from the iterator of the session's rdf graphs

type RdfSessionIterator struct {
	collector chan chan [3]*Node
	Itor      chan [3]*Node
	done      chan struct{}
}

func (itor *RdfSessionIterator) Done() {
	select {
	case <-itor.done:
		// log.Printf("##!@@ Done: RdfSessionIterator ch was already closed!")
	default:
		close(itor.done)
		// log.Printf("##!@@ Done: RdfSessionIterator ch closed")
	}
}

func NewRdfSessionIterator(metaItor, assertedItor, inferredItor *BaseGraphIterator) *RdfSessionIterator {
	if metaItor == nil || assertedItor == nil || inferredItor == nil {
		return nil
	}
	rsItor := &RdfSessionIterator{
		collector: make(chan chan [3]*Node, 3),
		Itor:      make(chan [3]*Node),
		done:      make(chan struct{}),
	}
	rsItor.collector <- metaItor.Itor
	rsItor.collector <- assertedItor.Itor
	rsItor.collector <- inferredItor.Itor
	close(rsItor.collector)
	go func() {
		defer func() {
			metaItor.Done()
			assertedItor.Done()
			inferredItor.Done()
		}()
		for gItor := range rsItor.collector {
			for t3 := range gItor {
				select {
				case rsItor.Itor <- t3:
				case <-rsItor.done:
					return
				}
			}
		}
		close(rsItor.Itor)
	}()
	return rsItor
}
