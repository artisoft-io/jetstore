//go:build race

package rete

// rbmRaceEnabled is true in a -race build: see TestDefect_I5_SharedMetaStoreWrites.
const rbmRaceEnabled = true
