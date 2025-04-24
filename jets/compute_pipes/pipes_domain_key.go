package compute_pipes

import (
	"fmt"
	"reflect"
)

// This file contains the DomainKey, aka DomainKeyInfo to
// calculate a domain key as a composite key with pre-processing function.

// Available functions for preprocessing input column values used in domain keys

// DomainKeysSpec contains the overall information, with overriding hashing method.
// The hashing method is applicable to all object types.
// DomainKeys is a map keyed by the object type.
type DomainKeysSpec struct {
	HashingOverride string                   `json:"hasing_override,omitempty"`
	DomainKeys      map[string]*DomainKeyInfo `json:"domain_keys_info,omitempty"`
}

// DomainKeyInfo associates a domain hashed key made as a composide domain
// key with an optional prep-processing function on each of the column making the key.
// KeyExpr is the original function(column) expression.
// Columns: list of input column name making the domain key
// PreprocessFnc: list of pre-processing functions for the input column (one per column)
// ObjectType: Object type associated with the Domain Key
type DomainKeyInfo struct {
	KeyExpr       []string `json:"key_expr,omitempty"`
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
	if domainKeys == nil {
		domainKeys = "jets:key"
	}
	result := &DomainKeysSpec{
		DomainKeys: make(map[string]*DomainKeyInfo),
	}
	// Extract the domain keys structure from domainKeys
	switch value := domainKeys.(type) {
	case string:
		// fmt.Println("*** Domain Key is single column", value)
		result.DomainKeys[mainObjectType] = &DomainKeyInfo{
			KeyExpr:    []string{value},
			ObjectType: mainObjectType,
		}
	case []any:
		// fmt.Println("*** Domain Key is a composite key", value)
		ck := make([]string, 0, len(value))
		for i := range value {
			if reflect.TypeOf(value[i]).Kind() == reflect.String {
				ck = append(ck, value[i].(string))
			}
		}
		result.DomainKeys[mainObjectType] = &DomainKeyInfo{
			KeyExpr:    ck,
			ObjectType: mainObjectType,
		}
	case map[string]any:
		// fmt.Println("*** Domain Key is a struct of composite keys", value)
		for k, v := range value {
			switch vv := v.(type) {
			case string:
				if k == "jets:hashing_override" {
					result.HashingOverride = vv
				} else {
					result.DomainKeys[k] = &DomainKeyInfo{
						KeyExpr:    []string{vv},
						ObjectType: k,
					}
				}
			case []any:
				ck := make([]string, 0, len(vv))
				for i := range vv {
					if reflect.TypeOf(vv[i]).Kind() == reflect.String {
						ck = append(ck, vv[i].(string))
					}
				}
				result.DomainKeys[k] = &DomainKeyInfo{
					KeyExpr:    ck,
					ObjectType: k,
				}
			default:
				return nil, fmt.Errorf("error: domainKeysJson contains an element of unsupported type: %T", vv)
			}
		}
	default:
		return nil, fmt.Errorf("error: domainKeysJson contains an element of unsupported type: %T", value)
	}
	return result, nil
}
