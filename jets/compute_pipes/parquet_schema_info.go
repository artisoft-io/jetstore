package compute_pipes

import (
	"github.com/apache/arrow/go/v17/arrow"
)

type ParquetSchemaInfo struct {
	Fields []*FieldInfo `json:"fields,omitempty"`
}
type FieldInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable,omitzero"`
}

func NewParquetSchemaInfo(schema *arrow.Schema) *ParquetSchemaInfo {
	fields := schema.Fields()
	fieldsInfo := make([]*FieldInfo, 0, len(fields))
	for _, field := range fields {
		fieldsInfo = append(fieldsInfo, &FieldInfo{
			Name:     field.Name,
			Type:     field.Type.Name(),
			Nullable: field.Nullable,
		})
	}
	return &ParquetSchemaInfo{Fields: fieldsInfo}
}

// return value is either nil or a string representing the input v
func ConvertWithSchemaV1(irow int, col arrow.Array, trimStrings bool, castToRdfTxtFnc CastToRdfTxtFnc) (any, error) {
	if col.IsValid(irow) {
		return nil, nil
	}
	var value string
	value = col.ValueStr(irow)
	// // Don't need the rest for now!!
	// switch col.DataType().Name() {

	// case arrow.FixedWidthTypes.Boolean.Name():
	// 	v, ok := col.(*array.Boolean)
	// 	if ok {
	// 		if v.Value(irow) {
	// 			value = "1"
	// 		} else {
	// 			value = "0"
	// 		}
	// 	} else {
	// 		return nil, fmt.Errorf("error: ConvertWithSchemaV1 expecting *array.Boolean got %T", v)
	// 	}

	// case arrow.PrimitiveTypes.Date32.Name():
	// 	v, ok := col.(*array.Date32)
	// 	if ok {
	// 		// return date(Jan 1 1970) + vv days
	// 		value = time.Unix(int64(v.Value(irow))*24*60*60, 0).Format("2006-01-02")
	// 	} else {
	// 		return nil, fmt.Errorf("error: ConvertWithSchemaV1 expecting *array.Date32 got %T", v)
	// 	}

	// case arrow.PrimitiveTypes.Int32.Name():
	// 	v, ok := col.(*array.Int32)
	// 	if ok {
	// 		value = strconv.Itoa(int(v.Value(irow)))
	// 	} else {
	// 		return nil, fmt.Errorf("error: ConvertWithSchemaV1 expecting *array.Int32 got %T", v)
	// 	}

	// case arrow.PrimitiveTypes.Uint32.Name():
	// 	v, ok := col.(*array.Uint32)
	// 	if ok {
	// 		value = strconv.Itoa(int(v.Value(irow)))
	// 	} else {
	// 		return nil, fmt.Errorf("error: ConvertWithSchemaV1 expecting *array.Uint32 got %T", v)
	// 	}

	// case arrow.PrimitiveTypes.Int64.Name():
	// 	v, ok := col.(*array.Int64)
	// 	if ok {
	// 		value = strconv.FormatInt(v.Value(irow), 10)
	// 	} else {
	// 		return nil, fmt.Errorf("error: ConvertWithSchemaV1 expecting *array.Int64 got %T", v)
	// 	}

	// case arrow.PrimitiveTypes.Uint64.Name():
	// 	v, ok := col.(*array.Uint64)
	// 	if ok {
	// 		value = strconv.FormatUint(v.Value(irow), 10)
	// 	} else {
	// 		return nil, fmt.Errorf("error: ConvertWithSchemaV1 expecting *array.Uint64 got %T", v)
	// 	}

	// case arrow.PrimitiveTypes.Float32.Name():
	// 	v, ok := col.(*array.Float32)
	// 	if ok {
	// 		value = strconv.FormatFloat(float64(v.Value(irow)), 'f', -1, 32)
	// 	} else {
	// 		return nil, fmt.Errorf("error: ConvertWithSchemaV1 expecting *array.Float32 got %T", v)
	// 	}

	// case arrow.PrimitiveTypes.Float64.Name():
	// 	v, ok := col.(*array.Float64)
	// 	if ok {
	// 		value = strconv.FormatFloat(v.Value(irow), 'f', -1, 32)
	// 	} else {
	// 		return nil, fmt.Errorf("error: ConvertWithSchemaV1 expecting *array.Float64 got %T", v)
	// 	}

	// case arrow.BinaryTypes.String.Name():
	// 	value = col.ValueStr(irow)

	// default:
	// 	return nil, fmt.Errorf("error: ConvertWithSchemaV0 unknown parquet type: %v", *se.Type)
	// }
	if castToRdfTxtFnc == nil {
		return value, nil
	}
	return castToRdfTxtFnc(value)
}
