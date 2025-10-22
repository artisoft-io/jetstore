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
	Env() map[string]any
	AdjustColumnWidth(width map[string]int) error
	BadRowsConfig() *BadRowsSpec
	Bucket() string
	ColumnNames() []string
	Columns() []SchemaColumnSpec
	Compression() string
	Delimiter() rune
	DetectEncoding() bool
	DomainClass() string
	DomainKeys() map[string]any
	Encoding() string
	EnforceRowMaxLength() bool
	EnforceRowMinLength() bool
	FixedWidthEncodingInfo() *FixedWidthEncodingInfo
	FixedWidthFileHeaders() ([]string, string)
	Format() string
	InputFormatDataJson() string
	IsPartFiles() bool
	NbrRowsInRecord() int64
	NoQuotes() bool
	OutputEncoding() string
	ParquetSchema() *ParquetSchemaInfo
	QuoteAllRecords() bool
	ReadBatchSize() int64
	ReadDateLayout() string
	SchemaName() string
	TrimColumns() bool
	UseLazyQuotes() bool
	UseLazyQuotesSpecial() bool
	VariableFieldsPerRecord() bool
	WriteDateLayout() string
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
	if spec.Format == "fixed_width" {
		return sp.initializeFixedWidthInfo()
	}
	switch {
	case len(sp.spec.Columns) > 0:
		sp.columnNames = make([]string, 0, len(sp.spec.Columns))
		for i := range sp.spec.Columns {
			sp.columnNames = append(sp.columnNames, sp.spec.Columns[i].Name)
		}
	case len(sp.spec.Headers) > 0:
		sp.columnNames = sp.spec.Headers
	}
	return nil
}

func (sp *DefaultSchemaProvider) FixedWidthFileHeaders() ([]string, string) {
	if sp == nil {
		return nil, ""
	}
	return sp.columnNames, sp.fwColumnPrefix
}

func (sp *DefaultSchemaProvider) AdjustColumnWidth(width map[string]int) error {
	if sp == nil || width == nil {
		return fmt.Errorf("error: schema provider or argument to AdjustColumnWidth is nil")
	}
	if len(sp.spec.Columns) == 0 {
		return fmt.Errorf("error: Cannot adjust column width of Schema Provider without column info")
	}
	for i := range sp.spec.Columns {
		c := &sp.spec.Columns[i]
		w, ok := width[c.Name]
		if ok {
			c.Length = w
		}
	}
	sp.spec.FixedWidthColumnsCsv = ""
	return sp.initializeFixedWidthInfo()
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

func (sp *DefaultSchemaProvider) Bucket() string {
	if sp == nil {
		return ""
	}
	return sp.spec.Bucket
}

func (sp *DefaultSchemaProvider) Env() map[string]any {
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

func (sp *DefaultSchemaProvider) Format() string {
	if sp == nil {
		return ""
	}
	return sp.spec.Format
}

func (sp *DefaultSchemaProvider) NbrRowsInRecord() int64 {
	if sp == nil {
		return 0
	}
	return sp.spec.NbrRowsInRecord
}

func (sp *DefaultSchemaProvider) ParquetSchema() *ParquetSchemaInfo {
	if sp == nil {
		return nil
	}
	return sp.spec.ParquetSchema
}

func (sp *DefaultSchemaProvider) ReadBatchSize() int64 {
	if sp == nil {
		return 0
	}
	return sp.spec.ReadBatchSize
}

func (sp *DefaultSchemaProvider) Encoding() string {
	if sp == nil {
		return ""
	}
	return sp.spec.Encoding
}

func (sp *DefaultSchemaProvider) OutputEncoding() string {
	if sp == nil {
		return ""
	}
	return sp.spec.OutputEncoding
}

func (sp *DefaultSchemaProvider) Compression() string {
	if sp == nil {
		return ""
	}
	return sp.spec.Compression
}

func (sp *DefaultSchemaProvider) DetectEncoding() bool {
	if sp == nil {
		return false
	}
	return sp.spec.DetectEncoding
}

func (sp *DefaultSchemaProvider) InputFormatDataJson() string {
	if sp == nil {
		return ""
	}
	return sp.spec.InputFormatDataJson
}

func (sp *DefaultSchemaProvider) DomainClass() string {
	if sp == nil {
		return ""
	}
	return sp.spec.DomainClass
}

func (sp *DefaultSchemaProvider) DomainKeys() map[string]any {
	if sp == nil {
		return nil
	}
	return sp.spec.DomainKeys
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
	return sp.spec.Delimiter
}

func (sp *DefaultSchemaProvider) UseLazyQuotes() bool {
	if sp == nil {
		return false
	}
	return sp.spec.UseLazyQuotes
}

func (sp *DefaultSchemaProvider) UseLazyQuotesSpecial() bool {
	if sp == nil {
		return false
	}
	return sp.spec.UseLazyQuotesSpecial
}

func (sp *DefaultSchemaProvider) VariableFieldsPerRecord() bool {
	if sp == nil {
		return false
	}
	return sp.spec.VariableFieldsPerRecord
}

func (sp *DefaultSchemaProvider) EnforceRowMinLength() bool {
	if sp == nil {
		return false
	}
	return sp.spec.EnforceRowMinLength
}

func (sp *DefaultSchemaProvider) EnforceRowMaxLength() bool {
	if sp == nil {
		return false
	}
	return sp.spec.EnforceRowMaxLength
}

func (sp *DefaultSchemaProvider) BadRowsConfig() *BadRowsSpec {
	if sp == nil {
		return nil
	}
	return sp.spec.BadRowsConfig
}

func (sp *DefaultSchemaProvider) QuoteAllRecords() bool {
	if sp == nil {
		return false
	}
	return sp.spec.QuoteAllRecords
}

func (sp *DefaultSchemaProvider) NoQuotes() bool {
	if sp == nil {
		return false
	}
	return sp.spec.NoQuotes
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
