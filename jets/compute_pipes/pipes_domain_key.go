package compute_pipes

import (
	"fmt"
	"github.com/artisoft-io/jetstore/jets/compute_pipes/pipesmodel"
	"reflect"
)

// This file contains the DomainKey, aka DomainKeyInfo to
// calculate a domain key as a composite key with pre-processing function.
// See ParsePreprocessingExpressions for available functions for preprocessing input column.

type DomainKeysSpec = pipesmodel.DomainKeysSpec

type DomainKeyInfo = pipesmodel.DomainKeyInfo

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
		// log.Println("*** Domain Key is single column", value)
		result.DomainKeys[mainObjectType] = &DomainKeyInfo{
			KeyExpr:    []string{value},
			ObjectType: mainObjectType,
		}
	case []any:
		// log.Println("*** Domain Key is a composite key", value)
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
		// log.Println("*** Domain Key is a struct of composite keys", value)
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
