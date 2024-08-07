package rdf

import (
	"hash/fnv"
	"testing"
)

// This file contains base test cases for rdf.Node

func hashIt(key []byte) uint64 {
	h := fnv.New64a()
	h.Write(key)
	return h.Sum64()
}

func TestMarshalBinary(t *testing.T) {
	// Testing MarshalBinary for use in computing Node's hash value
	hashed := make(map[uint64]bool)

	// BlankNode
	node, _ := BN(0).MarshalBinary()
	hashed[hashIt(node)] = true
	if len(hashed) != 1 {
		t.Errorf("Unexpected number of distinct hashed values")
	}

	node, _ = BN(1).MarshalBinary()
	hashed[hashIt(node)] = true
	if len(hashed) != 2 {
		t.Errorf("Unexpected number of distinct hashed values")
	}

	node, _ = BN(1101123).MarshalBinary()
	hashed[hashIt(node)] = true
	if len(hashed) != 3 {
		t.Errorf("Unexpected number of distinct hashed values")
	}

	node, _ = BN(1).MarshalBinary()
	hashed[hashIt(node)] = true
	if len(hashed) != 3 {
		t.Errorf("Unexpected number of distinct hashed values")
	}

	node, _ = BN(1101123).MarshalBinary()
	hashed[hashIt(node)] = true
	if len(hashed) != 3 {
		t.Errorf("Unexpected number of distinct hashed values")
	}

	node, _ = BN(1101124).MarshalBinary()
	hashed[hashIt(node)] = true
	if len(hashed) != 4 {
		t.Errorf("Unexpected number of distinct hashed values")
	}

	// NamedResource
	node, _ = R("1").MarshalBinary()
	hashed[hashIt(node)] = true
	if len(hashed) != 5 {
		t.Errorf("Unexpected number of distinct hashed values")
	}

	node, _ = R("1101123").MarshalBinary()
	hashed[hashIt(node)] = true
	if len(hashed) != 6 {
		t.Errorf("Unexpected number of distinct hashed values")
	}

	node, _ = R("1101124").MarshalBinary()
	hashed[hashIt(node)] = true
	if len(hashed) != 7 {
		t.Errorf("Unexpected number of distinct hashed values")
	}

	node, _ = R("1101123").MarshalBinary()
	hashed[hashIt(node)] = true
	if len(hashed) != 7 {
		t.Errorf("Unexpected number of distinct hashed values")
	}

	node, _ = R("1101124").MarshalBinary()
	hashed[hashIt(node)] = true
	if len(hashed) != 7 {
		t.Errorf("Unexpected number of distinct hashed values")
	}

	// LDate
	d, err := D("2024/06/20")
	if err != nil {
		t.Error(err)
	}
	node, err = d.MarshalBinary()
	if err != nil {
		t.Error(err)
	}
	hashed[hashIt(node)] = true
	if len(hashed) != 8 {
		t.Errorf("Unexpected number of distinct hashed values")
	}

	d, err = D("2023/6/20")
	if err != nil {
		t.Error(err)
	}
	node, err = d.MarshalBinary()
	if err != nil {
		t.Error(err)
	}
	hashed[hashIt(node)] = true
	if len(hashed) != 9 {
		t.Errorf("Unexpected number of distinct hashed values")
	}

	d, err = D("2024/6/20")
	if err != nil {
		t.Error(err)
	}
	node, err = d.MarshalBinary()
	if err != nil {
		t.Error(err)
	}
	hashed[hashIt(node)] = true
	if len(hashed) != 9 {
		t.Errorf("Unexpected number of distinct hashed values")
	}

	// Int
	node, _ = I(1).MarshalBinary()
	hashed[hashIt(node)] = true
	if len(hashed) != 10 {
		t.Errorf("Unexpected number of distinct hashed values")
	}

	node, _ = I(1101123).MarshalBinary()
	hashed[hashIt(node)] = true
	if len(hashed) != 11 {
		t.Errorf("Unexpected number of distinct hashed values")
	}
	node, _ = I(1).MarshalBinary()
	hashed[hashIt(node)] = true
	if len(hashed) != 11 {
		t.Errorf("Unexpected number of distinct hashed values")
	}

	node, _ = I(1101123).MarshalBinary()
	hashed[hashIt(node)] = true
	if len(hashed) != 11 {
		t.Errorf("Unexpected number of distinct hashed values")
	}


	// Float64
	node, _ = F(1.05).MarshalBinary()
	hashed[hashIt(node)] = true
	if len(hashed) != 12 {
		t.Errorf("Unexpected number of distinct hashed values, got %d, expecting 14", len(hashed))
	}

	node, _ = F(1101123).MarshalBinary()
	hashed[hashIt(node)] = true
	if len(hashed) != 13 {
		t.Errorf("Unexpected number of distinct hashed values, got %d, expecting 15", len(hashed))
	}

	node, _ = F(1.05).MarshalBinary()
	hashed[hashIt(node)] = true
	if len(hashed) != 13 {
		t.Errorf("Unexpected number of distinct hashed values, got %d, expecting 15", len(hashed))
	}

	d, err = DT("2024/6/20T00:00:00.0")
	if err != nil {
		t.Error(err)
	}
	node, err = d.MarshalBinary()
	if err != nil {
		t.Error(err)
	}
	hashed[hashIt(node)] = true
	if len(hashed) != 14 {
		t.Errorf("Unexpected number of distinct hashed values, got %d, expecting 16", len(hashed))
	}
}
