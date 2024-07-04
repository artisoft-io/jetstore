package rdf

// RdfGraph notifications when triples are inserted/deleted from graph using
// callback manager structure

// This interface is implemented in the rete package
type NotificationCallback interface {
	TripleInserted(s, p, o *Node)
	TripleDeleted(s, p, o *Node)
}

// Struct to manage a list of callbacks to invoke when
// triples are inserted/removed from RdfGraph
type CallbackManager struct {
	callbacks []NotificationCallback
}

func NewCallbackManager() *CallbackManager {
	return &CallbackManager{
		callbacks: make([]NotificationCallback, 0),
	}
}

func (cm *CallbackManager) AddCallback(nc NotificationCallback) {
	if nc == nil {
		return
	}
	cm.callbacks = append(cm.callbacks, nc)
}

func (cm *CallbackManager) ClearCallbacks() {
	cm.callbacks = make([]NotificationCallback, 0)
}

func (cm *CallbackManager) TripleInserted(s, p, o *Node) {
	for i := range cm.callbacks {
		cm.callbacks[i].TripleInserted(s, p, o)
	}
}

func (cm *CallbackManager) TripleDeleted(s, p, o *Node) {
	for i := range cm.callbacks {
		cm.callbacks[i].TripleDeleted(s, p, o)
	}
}
