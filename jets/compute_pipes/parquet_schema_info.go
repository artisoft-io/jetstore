package compute_pipes

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/decimal128"
	"github.com/apache/arrow/go/v17/arrow/decimal256"
	"github.com/apache/arrow/go/v17/arrow/memory"
)

type ParquetSchemaInfo struct {
	schema *arrow.Schema
	Fields []*FieldInfo `json:"fields,omitempty"`
}
type FieldInfo struct {
	Name             string `json:"name"`
	Type             string `json:"type"`
	Nullable         bool   `json:"nullable,omitzero"`
	DecimalPrecision int32  `json:"precision,omitzero"`
	DecimalScale     int32  `json:"scale,omitzero"`
}

type ArrayBuilder interface {
	Reserve(n int)
	Append(v any)
	AppendEmptyValue()
	AppendNull()
	NewArray() arrow.Array
	Release()
}

type ArrayRecord struct {
	schema *arrow.Schema
	arrays []arrow.Array
	Record arrow.Record
}

func NewArrayRecord(schema *arrow.Schema, builders []ArrayBuilder) *ArrayRecord {
	record := &ArrayRecord{
		schema: schema,
		arrays: make([]arrow.Array, 0, len(builders)),
	}
	for _, b := range builders {
		record.arrays = append(record.arrays, b.NewArray())
	}
	record.Record = array.NewRecord(schema, record.arrays, int64(record.arrays[0].Len()))
	return record
}

func (r *ArrayRecord) Release() {
	r.Record.Release()
	for _, a := range r.arrays {
		a.Release()
	}
}

func NewParquetSchemaInfo(schema *arrow.Schema) *ParquetSchemaInfo {
	fields := schema.Fields()
	fieldsInfo := make([]*FieldInfo, 0, len(fields))
	var precision, scale int32
	for _, field := range fields {
		switch vv := field.Type.(type) {
		case *arrow.Decimal128Type:
			precision = vv.Precision
			scale = vv.Scale
		case *arrow.Decimal256Type:
			precision = vv.Precision
			scale = vv.Scale
		}
		fieldsInfo = append(fieldsInfo, &FieldInfo{
			Name:             field.Name,
			Type:             field.Type.Name(),
			Nullable:         field.Nullable,
			DecimalPrecision: precision,
			DecimalScale:     scale,
		})
	}
	return &ParquetSchemaInfo{
		schema: schema,
		Fields: fieldsInfo,
	}
}

func NewEmptyParquetSchemaInfo() *ParquetSchemaInfo {
	return &ParquetSchemaInfo{}
}

func BuildParquetSchemaInfo(columns []string) *ParquetSchemaInfo {
	fieldsInfo := make([]*FieldInfo, 0, len(columns))
	for _, c := range columns {
		fieldsInfo = append(fieldsInfo, &FieldInfo{
			Name:     c,
			Type:     arrow.BinaryTypes.String.Name(),
			Nullable: true,
		})
	}
	return &ParquetSchemaInfo{
		Fields: fieldsInfo,
	}
}

func (psi *ParquetSchemaInfo) buildArrowSchema() {
	arrowFields := make([]arrow.Field, 0, len(psi.Fields))
	for _, fieldInfo := range psi.Fields {
		var fieldType arrow.DataType
		switch fieldInfo.Type {
		case arrow.FixedWidthTypes.Boolean.Name():
			fieldType = arrow.FixedWidthTypes.Boolean

		case arrow.PrimitiveTypes.Date32.Name(), "date":
			fieldType = arrow.PrimitiveTypes.Date32

		case arrow.PrimitiveTypes.Int32.Name():
			fieldType = arrow.PrimitiveTypes.Int32

		case arrow.PrimitiveTypes.Uint32.Name():
			fieldType = arrow.PrimitiveTypes.Uint32

		case arrow.PrimitiveTypes.Int64.Name():
			fieldType = arrow.PrimitiveTypes.Int64

		case arrow.PrimitiveTypes.Uint64.Name():
			fieldType = arrow.PrimitiveTypes.Uint64

		case arrow.PrimitiveTypes.Float32.Name():
			fieldType = arrow.PrimitiveTypes.Float32

		case arrow.PrimitiveTypes.Float64.Name():
			fieldType = arrow.PrimitiveTypes.Float64

		case arrow.BinaryTypes.String.Name(), "string":
			fieldType = arrow.BinaryTypes.String

		case arrow.BinaryTypes.Binary.Name():
			fieldType = arrow.BinaryTypes.Binary

		case "decimal", "DECIMAL", "decimal128", arrow.DECIMAL128.String():
			fieldType = &arrow.Decimal128Type{
				Precision: fieldInfo.DecimalPrecision,
				Scale:     fieldInfo.DecimalScale,
			}

		case "decimal256", arrow.DECIMAL256.String():
			fieldType = &arrow.Decimal256Type{
				Precision: fieldInfo.DecimalPrecision,
				Scale:     fieldInfo.DecimalScale,
			}

		default:
			log.Printf("WARNING: unsupported or invalid parquet type: %v -- using string\n", fieldInfo.Type)
			fieldType = arrow.BinaryTypes.String
		}
		arrowFields = append(arrowFields, arrow.Field{
			Name:     fieldInfo.Name,
			Type:     fieldType,
			Nullable: fieldInfo.Nullable,
		})
	}
	psi.schema = arrow.NewSchema(arrowFields, nil)
}

func (psi *ParquetSchemaInfo) Columns() []string {
	columns := make([]string, 0, len(psi.Fields))
	for _, fi := range psi.Fields {
		columns = append(columns, fi.Name)
	}
	return columns
}

func (psi *ParquetSchemaInfo) ArrowSchema() *arrow.Schema {
	if psi.schema == nil {
		psi.buildArrowSchema()
	}
	return psi.schema
}

func (psi *ParquetSchemaInfo) CreateBuilders(pool *memory.GoAllocator) ([]ArrayBuilder, error) {
	builders := make([]ArrayBuilder, 0, len(psi.Fields))
	for _, field := range psi.Fields {
		switch field.Type {

		case arrow.FixedWidthTypes.Boolean.Name():
			builders = append(builders, NewBooleanBuilder(pool))

		case arrow.PrimitiveTypes.Date32.Name():
			builders = append(builders, NewDateBuilder(pool))

		case arrow.PrimitiveTypes.Int32.Name():
			builders = append(builders, NewInt32Builder(pool))

		case arrow.PrimitiveTypes.Uint32.Name():
			builders = append(builders, NewUint32Builder(pool))

		case arrow.PrimitiveTypes.Int64.Name():
			builders = append(builders, NewInt64Builder(pool))

		case arrow.PrimitiveTypes.Uint64.Name():
			builders = append(builders, NewUint64Builder(pool))

		case arrow.PrimitiveTypes.Float32.Name():
			builders = append(builders, NewFloat32Builder(pool))

		case arrow.PrimitiveTypes.Float64.Name():
			builders = append(builders, NewFloat64Builder(pool))

		case arrow.BinaryTypes.String.Name():
			builders = append(builders, NewStringBuilder(pool))

		case arrow.BinaryTypes.Binary.Name():
			builders = append(builders, NewBinaryBuilder(pool))

		case "decimal", "DECIMAL", "decimal128", arrow.DECIMAL128.String():
			builders = append(builders, NewDecimal128Builder(pool,
				field.DecimalPrecision, field.DecimalScale))

		case "decimal256", arrow.DECIMAL256.String():
			builders = append(builders, NewDecimal256Builder(pool,
				field.DecimalPrecision, field.DecimalScale))

		default:
			log.Printf("WARNING: Create parquet column builders, unknown parquet type: %v -- using string builder\n", field.Type)
			builders = append(builders, NewStringBuilder(pool))
		}
	}
	return builders, nil
}

type BooleanBuilder struct {
	builder *array.BooleanBuilder
}

func NewBooleanBuilder(mem memory.Allocator) ArrayBuilder {
	return &BooleanBuilder{
		builder: array.NewBooleanBuilder(mem),
	}
}
func (b *BooleanBuilder) Reserve(n int) {
	b.builder.Reserve(n)
}
func (b *BooleanBuilder) Append(v any) {
	if v == nil {
		b.builder.AppendNull()
		return
	}
	b.builder.Append(v.(bool))
}
func (b *BooleanBuilder) AppendEmptyValue() {
	b.builder.AppendEmptyValue()
}
func (b *BooleanBuilder) AppendNull() {
	b.builder.AppendNull()
}
func (b *BooleanBuilder) NewArray() arrow.Array {
	return b.builder.NewArray()
}
func (b *BooleanBuilder) Release() {
	b.builder.Release()
}

type DateBuilder struct {
	builder *array.Date32Builder
}

func NewDateBuilder(mem memory.Allocator) ArrayBuilder {
	return &DateBuilder{
		builder: array.NewDate32Builder(mem),
	}
}
func (b *DateBuilder) Reserve(n int) {
	b.builder.Reserve(n)
}
func (b *DateBuilder) Append(v any) {
	if v == nil {
		b.builder.AppendNull()
		return
	}
	switch vv := v.(type) {
	case int32:
		b.builder.Append(arrow.Date32(vv))
	case arrow.Date32:
		b.builder.Append(vv)
	case int:
		b.builder.Append(arrow.Date32(vv))
	default:
		log.Panicf("invalid data type in NewDateBuilder, unexpected type: %T", v)
	}
}
func (b *DateBuilder) AppendEmptyValue() {
	b.builder.AppendEmptyValue()
}
func (b *DateBuilder) AppendNull() {
	b.builder.AppendNull()
}
func (b *DateBuilder) NewArray() arrow.Array {
	return b.builder.NewArray()
}
func (b *DateBuilder) Release() {
	b.builder.Release()
}

type Int32Builder struct {
	builder *array.Int32Builder
}

func NewInt32Builder(mem memory.Allocator) ArrayBuilder {
	return &Int32Builder{
		builder: array.NewInt32Builder(mem),
	}
}
func (b *Int32Builder) Reserve(n int) {
	b.builder.Reserve(n)
}
func (b *Int32Builder) Append(v any) {
	if v == nil {
		b.builder.AppendNull()
		return
	}
	b.builder.Append(v.(int32))
}
func (b *Int32Builder) AppendEmptyValue() {
	b.builder.AppendEmptyValue()
}
func (b *Int32Builder) AppendNull() {
	b.builder.AppendNull()
}
func (b *Int32Builder) NewArray() arrow.Array {
	return b.builder.NewArray()
}
func (b *Int32Builder) Release() {
	b.builder.Release()
}

type Uint32Builder struct {
	builder *array.Uint32Builder
}

func NewUint32Builder(mem memory.Allocator) ArrayBuilder {
	return &Uint32Builder{
		builder: array.NewUint32Builder(mem),
	}
}
func (b *Uint32Builder) Reserve(n int) {
	b.builder.Reserve(n)
}
func (b *Uint32Builder) Append(v any) {
	if v == nil {
		b.builder.AppendNull()
		return
	}
	b.builder.Append(v.(uint32))
}
func (b *Uint32Builder) AppendEmptyValue() {
	b.builder.AppendEmptyValue()
}
func (b *Uint32Builder) AppendNull() {
	b.builder.AppendNull()
}
func (b *Uint32Builder) NewArray() arrow.Array {
	return b.builder.NewArray()
}
func (b *Uint32Builder) Release() {
	b.builder.Release()
}

type Int64Builder struct {
	builder *array.Int64Builder
}

func NewInt64Builder(mem memory.Allocator) ArrayBuilder {
	return &Int64Builder{
		builder: array.NewInt64Builder(mem),
	}
}
func (b *Int64Builder) Reserve(n int) {
	b.builder.Reserve(n)
}
func (b *Int64Builder) Append(v any) {
	if v == nil {
		b.builder.AppendNull()
		return
	}
	b.builder.Append(v.(int64))
}
func (b *Int64Builder) AppendEmptyValue() {
	b.builder.AppendEmptyValue()
}
func (b *Int64Builder) AppendNull() {
	b.builder.AppendNull()
}
func (b *Int64Builder) NewArray() arrow.Array {
	return b.builder.NewArray()
}
func (b *Int64Builder) Release() {
	b.builder.Release()
}

type Uint64Builder struct {
	builder *array.Uint64Builder
}

func NewUint64Builder(mem memory.Allocator) ArrayBuilder {
	return &Uint64Builder{
		builder: array.NewUint64Builder(mem),
	}
}
func (b *Uint64Builder) Reserve(n int) {
	b.builder.Reserve(n)
}
func (b *Uint64Builder) Append(v any) {
	if v == nil {
		b.builder.AppendNull()
		return
	}
	b.builder.Append(v.(uint64))
}
func (b *Uint64Builder) AppendEmptyValue() {
	b.builder.AppendEmptyValue()
}
func (b *Uint64Builder) AppendNull() {
	b.builder.AppendNull()
}
func (b *Uint64Builder) NewArray() arrow.Array {
	return b.builder.NewArray()
}
func (b *Uint64Builder) Release() {
	b.builder.Release()
}

type Float32Builder struct {
	builder *array.Float32Builder
}

func NewFloat32Builder(mem memory.Allocator) ArrayBuilder {
	return &Float32Builder{
		builder: array.NewFloat32Builder(mem),
	}
}
func (b *Float32Builder) Reserve(n int) {
	b.builder.Reserve(n)
}
func (b *Float32Builder) Append(v any) {
	if v == nil {
		b.builder.AppendNull()
		return
	}
	b.builder.Append(v.(float32))
}
func (b *Float32Builder) AppendEmptyValue() {
	b.builder.AppendEmptyValue()
}
func (b *Float32Builder) AppendNull() {
	b.builder.AppendNull()
}
func (b *Float32Builder) NewArray() arrow.Array {
	return b.builder.NewArray()
}
func (b *Float32Builder) Release() {
	b.builder.Release()
}

type Float64Builder struct {
	builder *array.Float64Builder
}

func NewFloat64Builder(mem memory.Allocator) ArrayBuilder {
	return &Float64Builder{
		builder: array.NewFloat64Builder(mem),
	}
}
func (b *Float64Builder) Reserve(n int) {
	b.builder.Reserve(n)
}
func (b *Float64Builder) Append(v any) {
	if v == nil {
		b.builder.AppendNull()
		return
	}
	b.builder.Append(v.(float64))
}
func (b *Float64Builder) AppendEmptyValue() {
	b.builder.AppendEmptyValue()
}
func (b *Float64Builder) AppendNull() {
	b.builder.AppendNull()
}
func (b *Float64Builder) NewArray() arrow.Array {
	return b.builder.NewArray()
}
func (b *Float64Builder) Release() {
	b.builder.Release()
}

type TimestampBuilder struct {
	builder *array.TimestampBuilder
}

func NewTimestampBuilder(mem memory.Allocator) ArrayBuilder {
	return &TimestampBuilder{
		builder: array.NewTimestampBuilder(mem, &arrow.TimestampType{Unit: arrow.Millisecond, TimeZone: "UTC"}),
	}
}
func (b *TimestampBuilder) Reserve(n int) {
	b.builder.Reserve(n)
}
func (b *TimestampBuilder) Append(v any) {
	if v == nil {
		b.builder.AppendNull()
		return
	}
	switch vv := v.(type) {
	case int64:
		b.builder.Append(arrow.Timestamp(vv))
	case arrow.Timestamp:
		b.builder.Append(vv)
	case int:
		b.builder.Append(arrow.Timestamp(vv))
	default:
		log.Panicf("invalid data type in NewTimestampBuilder, unexpected type: %T", v)
	}

}
func (b *TimestampBuilder) AppendEmptyValue() {
	b.builder.AppendEmptyValue()
}
func (b *TimestampBuilder) AppendNull() {
	b.builder.AppendNull()
}
func (b *TimestampBuilder) NewArray() arrow.Array {
	return b.builder.NewArray()
}
func (b *TimestampBuilder) Release() {
	b.builder.Release()
}

type StringBuilder struct {
	builder *array.StringBuilder
}

func NewStringBuilder(mem memory.Allocator) ArrayBuilder {
	return &StringBuilder{
		builder: array.NewStringBuilder(mem),
	}
}
func (b *StringBuilder) Reserve(n int) {
	b.builder.Reserve(n)
}
func (b *StringBuilder) Append(v any) {
	if v == nil {
		b.builder.AppendNull()
		return
	}
	b.builder.Append(v.(string))
}
func (b *StringBuilder) AppendEmptyValue() {
	b.builder.AppendEmptyValue()
}
func (b *StringBuilder) AppendNull() {
	b.builder.AppendNull()
}
func (b *StringBuilder) NewArray() arrow.Array {
	return b.builder.NewArray()
}
func (b *StringBuilder) Release() {
	b.builder.Release()
}

type BinaryBuilder struct {
	builder *array.BinaryBuilder
}

func NewBinaryBuilder(mem memory.Allocator) ArrayBuilder {
	return &BinaryBuilder{
		builder: array.NewBinaryBuilder(mem, arrow.BinaryTypes.Binary),
	}
}
func (b *BinaryBuilder) Reserve(n int) {
	b.builder.Reserve(n)
}
func (b *BinaryBuilder) Append(v any) {
	if v == nil {
		b.builder.AppendNull()
		return
	}
	b.builder.Append([]byte(v.(string)))
}
func (b *BinaryBuilder) AppendEmptyValue() {
	b.builder.AppendEmptyValue()
}
func (b *BinaryBuilder) AppendNull() {
	b.builder.AppendNull()
}
func (b *BinaryBuilder) NewArray() arrow.Array {
	return b.builder.NewArray()
}
func (b *BinaryBuilder) Release() {
	b.builder.Release()
}

type Decimal128Builder struct {
	builder   *array.Decimal128Builder
	Precision int32
	Scale     int32
}

func NewDecimal128Builder(mem memory.Allocator, precision, scale int32) ArrayBuilder {
	return &Decimal128Builder{
		builder: array.NewDecimal128Builder(mem, &arrow.Decimal128Type{
			Precision: precision,
			Scale:     scale,
		}),
		Precision: precision,
		Scale:     scale,
	}
}
func (b *Decimal128Builder) Reserve(n int) {
	b.builder.Reserve(n)
}
func (b *Decimal128Builder) Append(v any) {
	if v == nil {
		b.builder.AppendNull()
		return
	}
	switch vv := v.(type) {
	case int64:
		b.builder.Append(decimal128.FromI64(vv))
	case string:
		if len(vv) == 0 {
			b.builder.AppendNull()
			return
		}
		value, err := decimal128.FromString(vv, b.Precision, b.Scale)
		if err != nil {
			log.Println("WARNING: invalid string for decimal128")
		} else {
			b.builder.Append(value)
		}
	default:
		// unknown or unsupported data type, using string value
		value, err := decimal128.FromString(fmt.Sprintf("%s", v), b.Precision, b.Scale)
		if err != nil {
			log.Println("WARNING: invalid string for decimal128")
		} else {
			b.builder.Append(value)
		}
	}
}
func (b *Decimal128Builder) AppendEmptyValue() {
	b.builder.AppendEmptyValue()
}
func (b *Decimal128Builder) AppendNull() {
	b.builder.AppendNull()
}
func (b *Decimal128Builder) NewArray() arrow.Array {
	return b.builder.NewArray()
}
func (b *Decimal128Builder) Release() {
	b.builder.Release()
}

type Decimal256Builder struct {
	builder   *array.Decimal256Builder
	Precision int32
	Scale     int32
}

func NewDecimal256Builder(mem memory.Allocator, precision, scale int32) ArrayBuilder {
	return &Decimal256Builder{
		builder: array.NewDecimal256Builder(mem, &arrow.Decimal256Type{
			Precision: precision,
			Scale:     scale,
		}),
	}
}
func (b *Decimal256Builder) Reserve(n int) {
	b.builder.Reserve(n)
}
func (b *Decimal256Builder) Append(v any) {
	if v == nil {
		b.builder.AppendNull()
		return
	}
	switch vv := v.(type) {
	case int64:
		b.builder.Append(decimal256.FromI64(vv))
	case string:
		if len(vv) == 0 {
			b.builder.AppendNull()
			return
		}
		value, err := decimal256.FromString(vv, b.Precision, b.Scale)
		if err != nil {
			log.Println("WARNING: invalid string for decimal256")
		} else {
			b.builder.Append(value)
		}
	default:
		// unknown or unsupported data type, using string value
		value, err := decimal256.FromString(fmt.Sprintf("%s", v), b.Precision, b.Scale)
		if err != nil {
			log.Println("WARNING: invalid string for decimal256")
		} else {
			b.builder.Append(value)
		}
	}
}
func (b *Decimal256Builder) AppendEmptyValue() {
	b.builder.AppendEmptyValue()
}
func (b *Decimal256Builder) AppendNull() {
	b.builder.AppendNull()
}
func (b *Decimal256Builder) NewArray() arrow.Array {
	return b.builder.NewArray()
}
func (b *Decimal256Builder) Release() {
	b.builder.Release()
}

// return value is either nil or a string representing the input v
func ConvertWithSchemaV1(irow int, col arrow.Array, trimStrings bool, fieldInfo *FieldInfo, castToRdfTxtFnc CastToRdfTxtFnc) (any, error) {
	var value string
	// value = col.ValueStr(irow)
	switch col.DataType().Name() {

	case arrow.FixedWidthTypes.Boolean.Name():
		v, ok := col.(*array.Boolean)
		if ok {
			if v.Value(irow) {
				value = "1"
			} else {
				value = "0"
			}
		} else {
			return nil, fmt.Errorf("error: ConvertWithSchemaV1 expecting *array.Boolean got %T", v)
		}

	case arrow.PrimitiveTypes.Date32.Name():
		v, ok := col.(*array.Date32)
		if ok {
			// return date(Jan 1 1970) + vv days
			value = time.Unix(int64(v.Value(irow))*24*60*60, 0).Format("2006-01-02")
		} else {
			return nil, fmt.Errorf("error: ConvertWithSchemaV1 expecting *array.Date32 got %T", v)
		}

	case arrow.PrimitiveTypes.Int32.Name():
		v, ok := col.(*array.Int32)
		if ok {
			value = strconv.Itoa(int(v.Value(irow)))
		} else {
			return nil, fmt.Errorf("error: ConvertWithSchemaV1 expecting *array.Int32 got %T", v)
		}

	case arrow.PrimitiveTypes.Uint32.Name():
		v, ok := col.(*array.Uint32)
		if ok {
			value = strconv.Itoa(int(v.Value(irow)))
		} else {
			return nil, fmt.Errorf("error: ConvertWithSchemaV1 expecting *array.Uint32 got %T", v)
		}

	case arrow.PrimitiveTypes.Int64.Name():
		v, ok := col.(*array.Int64)
		if ok {
			value = strconv.FormatInt(v.Value(irow), 10)
		} else {
			return nil, fmt.Errorf("error: ConvertWithSchemaV1 expecting *array.Int64 got %T", v)
		}

	case arrow.PrimitiveTypes.Uint64.Name():
		v, ok := col.(*array.Uint64)
		if ok {
			value = strconv.FormatUint(v.Value(irow), 10)
		} else {
			return nil, fmt.Errorf("error: ConvertWithSchemaV1 expecting *array.Uint64 got %T", v)
		}

	case arrow.PrimitiveTypes.Float32.Name():
		v, ok := col.(*array.Float32)
		if ok {
			value = strconv.FormatFloat(float64(v.Value(irow)), 'g', -1, 32)
		} else {
			return nil, fmt.Errorf("error: ConvertWithSchemaV1 expecting *array.Float32 got %T", v)
		}

	case arrow.PrimitiveTypes.Float64.Name():
		v, ok := col.(*array.Float64)
		if ok {
			value = strconv.FormatFloat(v.Value(irow), 'g', -1, 64)
			//***CHECK
			_, e1 := strconv.ParseFloat(value, 64)
			if e1 != nil {
				return nil, fmt.Errorf("parse float check: orgiginal float64: %v, value str: %s, conversion err from str:%v", v.Value(irow), value, e1)
			}
		} else {
			return nil, fmt.Errorf("error: ConvertWithSchemaV1 expecting *array.Float64 got %T", v)
		}

	case arrow.BinaryTypes.String.Name():
		v, ok := col.(*array.String)
		if ok {
			value = v.Value(irow)
			if trimStrings {
				value = strings.TrimSpace(value)
			}
		} else {
			return nil, fmt.Errorf("error: ConvertWithSchemaV1 expecting *array.String got %T", v)
		}

	case arrow.BinaryTypes.Binary.Name():
		v, ok := col.(*array.Binary)
		if ok {
			value = string(v.Value(irow))
			if trimStrings {
				value = strings.TrimSpace(value)
			}
		} else {
			return nil, fmt.Errorf("error: ConvertWithSchemaV1 expecting *array.Binary got %T", v)
		}

	case "decimal", "decimal128":
		v, ok := col.(*array.Decimal128)
		if ok && fieldInfo != nil {
			value = v.Value(irow).ToString(fieldInfo.DecimalScale)
		} else {
			return nil, fmt.Errorf("error: ConvertWithSchemaV1 expecting *array.Binary got %T or missing schema info (fieldInfo)", v)
		}

	case "decimal256":
		v, ok := col.(*array.Decimal256)
		if ok && fieldInfo != nil {
			value = v.Value(irow).ToString(fieldInfo.DecimalScale)
		} else {
			return nil, fmt.Errorf("error: ConvertWithSchemaV1 expecting *array.Binary got %T or missing schema info (fieldInfo)", v)
		}

	default:
		log.Printf("WARNING: unknown or unsupported type in ConvertWithSchemaV1: %s\n", col.DataType().Name())
		value = col.ValueStr(irow)
	}
	if castToRdfTxtFnc == nil {
		if len(value) == 0 {
			return nil, nil
		}
		return value, nil
	}
	return castToRdfTxtFnc(value)
}
