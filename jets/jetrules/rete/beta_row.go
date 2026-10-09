package rete

import (
	"hash/fnv"
	"log"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// BetaRow class is a row in the BetaRelation table

const (
	kNone = iota
	kInserted
	kDeleted
	kProcessed
)

type BetaRowStatus = int

type BetaRow struct {
	NdVertex *NodeVertex
	Status   BetaRowStatus
	Data     []*rdf.Node
	h        uint64
	// inferred records the triples the row's consequents inferred, for a row of a
	// vertex flagged RecordsConsequents; nil otherwise. Retraction replays it.
	inferred *[]rdf.Triple
}

// startRecording gives the row an empty inferred-triple record.
func (row *BetaRow) startRecording() {
	row.inferred = &[]rdf.Triple{}
}

// isRecording is true while the row holds an inferred-triple record.
func (row *BetaRow) isRecording() bool {
	return row.inferred != nil
}

// takeRecorded hands the recorded triples over and leaves the record empty: the row
// still records, so retracting it again replays nothing.
func (row *BetaRow) takeRecorded() []rdf.Triple {
	if row.inferred == nil {
		return nil
	}
	recorded := *row.inferred
	*row.inferred = nil
	return recorded
}

func NewBetaRow(vertex *NodeVertex, size int) *BetaRow {
	return &BetaRow{
		NdVertex: vertex,
		Status:   kNone,
		Data:     make([]*rdf.Node, size),
	}
}

func (row *BetaRow) Initialize(initializer *BetaRowInitializer, parentRow *BetaRow, t3 *rdf.Triple) error {
	var value *rdf.Node
	for i, d := range initializer.InitData {
		pos := d & brcLowMask
		if d & brcParentNode != 0 {
			if len(parentRow.Data) == 0 {
				log.Panicf("oops len(parentRow.Data)==0 but (d & brcParentNode != 0) @ vertex %d, parent %d, triple %s", row.NdVertex.Vertex, parentRow.NdVertex.Vertex, rdf.ToString(t3))
			}
			if pos >= len(parentRow.Data) {
				log.Panicf("oops pos == %d >= len(parentRow.Data) == %d @ vertex %d, parent %d, triple %s", pos, len(parentRow.Data), row.NdVertex.Vertex, parentRow.NdVertex.Vertex, rdf.ToString(t3))
			}
			value = parentRow.Data[pos]
		} else {
			value = (*t3)[pos]
		}
		row.Data[i] = value
	}
	// Calculate the row hash
	row.h = row.Hash()
	return nil
}

func (row *BetaRow) IsInserted() bool {
	return row.Status == kInserted
}

func (row *BetaRow) IsDeleted() bool {
	return row.Status == kDeleted
}

func (row *BetaRow) IsProcessed() bool {
	return row.Status == kProcessed
}

func (lhs *BetaRow) Eq(rhs *BetaRow) bool {
	if len(lhs.Data) != len(rhs.Data) {
		return false
	}
	for i := range lhs.Data {
		// Should we do a deep ne??
		if lhs.Data[i] != rhs.Data[i] {
			return false
		}
	}
	return true
}

func (row *BetaRow) Hash() uint64 {
	if row == nil {
		return 0
	}
	h := fnv.New64a()
	var b []byte
	var err error
	for _, r := range (*row).Data {
		if r == nil {
			log.Fatalf("error BetaRow.Hash() with nil resource %s", r)
		}
		b, err = r.MarshalBinary()
		if err != nil {
			log.Fatalf("error while MarshalingBinary of resource %s: %v", r, err)
		}
		h.Write(b)
	}
	return h.Sum64()
}

func (row *BetaRow) Get(i int) *rdf.Node {
	if row == nil || i < 0 {
		return nil
	}
	if i < len(row.Data) {
		return row.Data[i]
	}
	log.Printf("*** BUG: BetaRow.Get index %d out of range (len=%d)\n", i, len(row.Data))
	return nil
}
