package compute_pipes

import (
	"testing"
)

func TestParseDomainKeyInfo01(t *testing.T) {
	spec, err := ParseDomainKeyInfo("Claim", "claim_id")
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.HashingOverride) > 0 {
		t.Error("unexpected")
	}
	info := spec.DomainKeys["Claim"]
	if info == nil {
		t.Fatal("unexpected")
	}
	if len(info.KeyExpr) != 1 {
		t.Error("unexpected")
	}
	if info.KeyExpr[0] != "claim_id" {
		t.Error("unexpected")
	}
}

func TestParseDomainKeyInfo02(t *testing.T) {
	spec, err := ParseDomainKeyInfo("Claim", []any{"claim_id1", "claim_id2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.HashingOverride) > 0 {
		t.Error("unexpected")
	}
	info := spec.DomainKeys["Claim"]
	if info == nil {
		t.Fatal("unexpected")
	}
	if len(info.KeyExpr) != 2 {
		t.Error("unexpected")
	}
	if info.KeyExpr[0] != "claim_id1" {
		t.Error("unexpected")
	}
	if info.KeyExpr[1] != "claim_id2" {
		t.Error("unexpected")
	}
}

func TestParseDomainKeyInfo03(t *testing.T) {
	spec, err := ParseDomainKeyInfo("XXX", map[string]any{
		"Claim": []any{"c1", "c2"},
		"jets:hashing_override": "none",
	})
	if err != nil {
		t.Fatal(err)
	}
	if spec.HashingOverride != "none" {
		t.Error("unexpected")
	}
	info := spec.DomainKeys["Claim"]
	if info == nil {
		t.Fatal("unexpected")
	}
	if len(info.KeyExpr) != 2 {
		t.Error("unexpected")
	}
	if info.KeyExpr[0] != "c1" {
		t.Error("unexpected")
	}
	if info.KeyExpr[1] != "c2" {
		t.Error("unexpected")
	}
}
