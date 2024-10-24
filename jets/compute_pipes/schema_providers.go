package compute_pipes

import (
	"fmt"

	"github.com/jackc/pgx/v4/pgxpool"
)

// This package defines the schema manager & provider types

type SchemaManager struct {
	spec            []*SchemaProviderSpec
	envSettings     map[string]interface{}
	isDebugMode     bool
	schemaProviders map[string]SchemaProvider
}

func NewSchemaManager(spec []*SchemaProviderSpec,
	envSettings map[string]interface{}, isDebugMode bool) *SchemaManager {
	return &SchemaManager{
		spec:        spec,
		envSettings: envSettings,
		isDebugMode: isDebugMode,
    schemaProviders: make(map[string]SchemaProvider),
	}
}

type SchemaProvider interface {
	Initialize(dbpool *pgxpool.Pool, spec *SchemaProviderSpec,
		envSettings map[string]interface{}, isDebugMode bool) error
	Key() string
	Client() string
	Vendor() string
	ObjectType() string
	SchemaName() string
	InputFormat() string
	InputFormatDataJson() string
	IsPartFiles() bool
	Delimiter() rune
	Columns() []SchemaColumnSpec
}

type DefaultSchemaProvider struct {
	spec        *SchemaProviderSpec
	isDebugMode bool
}

func NewDefaultSchemaProvider() SchemaProvider {
	return &DefaultSchemaProvider{}
}


func (sm *SchemaManager) PrepareSchemaProviders(dbpool *pgxpool.Pool) error {
  if sm == nil || sm.spec == nil {
    return nil
  }
  for _, spec := range sm.spec {
    switch spec.Type {
    case "default":
      sp := NewDefaultSchemaProvider()
      sp.Initialize(dbpool, spec, sm.envSettings, sm.isDebugMode)
      sm.schemaProviders[sp.Key()] = sp
    default:
      return fmt.Errorf("error: unknown Schema Provider Type %s", spec.Type)
    }
  }
  return nil
}

func (sm *SchemaManager) GetSchemaProvider(key string) SchemaProvider {
  if sm == nil || sm.schemaProviders == nil {
    return nil
  }
  return sm.schemaProviders[key]
}

func (sp *DefaultSchemaProvider) Initialize(_ *pgxpool.Pool, spec *SchemaProviderSpec,
	envSettings map[string]interface{}, isDebugMode bool) error {
	sp.spec = spec
	sp.isDebugMode = isDebugMode
	return nil
}

func (sp *DefaultSchemaProvider) Key() string {
	if sp == nil {
		return ""
	}
	return sp.spec.Key
}

func (sp *DefaultSchemaProvider) Client() string {
	if sp == nil {
		return ""
	}
	return sp.spec.Client
}

func (sp *DefaultSchemaProvider) Vendor() string {
	if sp == nil {
		return ""
	}
	return sp.spec.Vendor
}

func (sp *DefaultSchemaProvider) ObjectType() string {
	if sp == nil {
		return ""
	}
	return sp.spec.ObjectType
}

func (sp *DefaultSchemaProvider) SchemaName() string {
	if sp == nil {
		return ""
	}
	return sp.spec.SchemaName
}

func (sp *DefaultSchemaProvider) InputFormat() string {
	if sp == nil {
		return ""
	}
	return sp.spec.InputFormat
}

func (sp *DefaultSchemaProvider) InputFormatDataJson() string {
	if sp == nil {
		return ""
	}
	return sp.spec.InputFormatDataJson
}

func (sp *DefaultSchemaProvider) IsPartFiles() bool {
	if sp == nil {
		return false
	}
	return sp.spec.IsPartFiles
}

func (sp *DefaultSchemaProvider) Delimiter() rune {
	if sp == nil {
		return 0
	}
	if sp.spec.Delimiter == "" {
		return '€'
	}
	return []rune(sp.spec.Delimiter)[0]
}

func (sp *DefaultSchemaProvider) Columns() []SchemaColumnSpec {
	if sp == nil {
		return nil
	}
	return sp.spec.Columns
}
