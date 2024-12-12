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
		spec:            spec,
		envSettings:     envSettings,
		isDebugMode:     isDebugMode,
		schemaProviders: make(map[string]SchemaProvider),
	}
}

type SchemaProvider interface {
	Initialize(dbpool *pgxpool.Pool, spec *SchemaProviderSpec,
		envSettings map[string]interface{}, isDebugMode bool) error
	Key() string
	SchemaName() string
	InputFormat() string
	Compression() string
	InputFormatDataJson() string
	IsPartFiles() bool
	Delimiter() rune
	UseLazyQuotes() bool
	VariableFieldsPerRecord() bool
	TrimColumns() bool
	Columns() []SchemaColumnSpec
	ColumnNames() []string
	ReadDateLayout() string
	WriteDateLayout() string
	FixedWidthFileHeaders() ([]string, string)
	FixedWidthEncodingInfo() *FixedWidthEncodingInfo
	Env() map[string]string
}

// columnNames is the list of file headers for fixed_width
// fwColumnPrefix is for fixed_width with multiple record type, prefix for making table columns (dkInfo)
type DefaultSchemaProvider struct {
	spec           *SchemaProviderSpec
	isDebugMode    bool
	columnNames    []string
	fwColumnPrefix string
	fwColumnInfo   *FixedWidthEncodingInfo
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
	// Ensure a default compression algo
	if sp.spec.Compression == "" {
		sp.spec.Compression = "none"
	}
	if spec.InputFormat == "fixed_width" {
		return sp.initializeFixedWidthInfo()
	}
	if len(sp.spec.Columns) > 0 {
		sp.columnNames = make([]string, 0, len(sp.spec.Columns))
		for i := range sp.spec.Columns {
			sp.columnNames = append(sp.columnNames, sp.spec.Columns[i].Name)
		}
	}
	return nil
}

func (sp *DefaultSchemaProvider) FixedWidthFileHeaders() ([]string, string) {
	if sp == nil {
		return nil, ""
	}
	return sp.columnNames, sp.fwColumnPrefix
}

func (sp *DefaultSchemaProvider) FixedWidthEncodingInfo() *FixedWidthEncodingInfo {
	if sp == nil {
		return nil
	}
	return sp.fwColumnInfo
}

func (sp *DefaultSchemaProvider) Key() string {
	if sp == nil {
		return ""
	}
	return sp.spec.Key
}

func (sp *DefaultSchemaProvider) Env() map[string]string {
	if sp == nil {
		return nil
	}
	return sp.spec.Env
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

func (sp *DefaultSchemaProvider) Compression() string {
	if sp == nil {
		return ""
	}
	return sp.spec.Compression
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

func (sp *DefaultSchemaProvider) UseLazyQuotes() bool {
	if sp == nil {
		return false
	}
	return sp.spec.UseLazyQuotes
}

func (sp *DefaultSchemaProvider) VariableFieldsPerRecord() bool {
	if sp == nil {
		return false
	}
	return sp.spec.VariableFieldsPerRecord
}

func (sp *DefaultSchemaProvider) TrimColumns() bool {
	if sp == nil {
		return false
	}
	return sp.spec.TrimColumns
}

func (sp *DefaultSchemaProvider) Columns() []SchemaColumnSpec {
	if sp == nil {
		return nil
	}
	return sp.spec.Columns
}

func (sp *DefaultSchemaProvider) ColumnNames() []string {
	if sp == nil {
		return nil
	}
	return sp.columnNames
}

func (sp *DefaultSchemaProvider) ReadDateLayout() string {
	if sp == nil {
		return ""
	}
	return sp.spec.ReadDateLayout
}

func (sp *DefaultSchemaProvider) WriteDateLayout() string {
	if sp == nil {
		return ""
	}
	return sp.spec.WriteDateLayout
}
