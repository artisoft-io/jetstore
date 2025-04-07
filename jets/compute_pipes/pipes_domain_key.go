package compute_pipes

import "fmt"

// This file contains the DomainKey, aka DomainKeyInfo to
// calculate a domain key as a composite key with pre-processing function

// DomainKeysSpec contains the overall information, with overriding hashing method.
type DomainKeysSpec struct {
	HashingOverride string                   `json:"hasing_override,omitempty"`
	DomainKeys      map[string]DomainKeyInfo `json:"domain_keys_info,omitempty"`
}

// DomainKeyInfo associates a domain hashed key made as a composide domain
// key with an optional prep-processing function on each of the column making the key.
// KeyExpr is the original function(column) expression.
// Columns: list of input column name making the domain key
// PreprocessFnc: list of pre-processing functions for the input column (one per column)
// ObjectType: Object type associated with the Domain Key
type DomainKeyInfo struct {
	KeyExpr       []string `json:"key_expr,omitempty"`
	Columns       []string `json:"columns,omitempty"`
	PreprocessFnc []string `json:"preprocess_fnc,omitempty"`
	ObjectType    string   `json:"object_type,omitempty"`
}

// Parse domain key configuration info from [domainKeys], supporting 3 use cases:
// in json format:
//
//		"key"
//	 ["key1", "key2"]
//
// {"ObjectType1": "key", "ObjectType1": ["key1", "key2"]}
func ParseDomainKeyInfo(mainObjectType string, domainKeys any) (*DomainKeysSpec, error) {
	return nil, fmt.Errorf("error: ParseDomainKeyInfo not implemented yet")
}

// [input] contains the values of [dk.Columns].
func (dk *DomainKeysSpec) ComputeDomainKey(objectType string, input []string) (string, error) {
	return "", fmt.Errorf("error: ComputeDomainKey not implemented yet")
}
