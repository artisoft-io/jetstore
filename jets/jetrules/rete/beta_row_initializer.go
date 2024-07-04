package rete

// Metadata component describing the schema of a BetaRow
// This is used to initialize a BetaRow

const (
	brcParentNode = 0x1000
	brcTriple     = 0x2000
	brcHiMask     = 0xF000
	brcLowMask    = 0x0FFF
)

type BetaRowInitializer struct {
	InitData []int
	Labels   []string
}

func NewBetaRowInitializer(data []int, labels []string) *BetaRowInitializer {
	return &BetaRowInitializer{
		InitData: data,
		Labels:   labels,
	}
}

func (b *BetaRowInitializer) RowSize() int {
	return len(b.InitData)
}

