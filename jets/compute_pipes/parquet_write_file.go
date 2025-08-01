package compute_pipes

import (
	"fmt"
	"io"
	"strconv"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/apache/arrow/go/v17/parquet"
	"github.com/apache/arrow/go/v17/parquet/compress"
	"github.com/apache/arrow/go/v17/parquet/pqarrow"
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

func WriteParquetPartitionV3(schemaInfo *ParquetSchemaInfo, nrowsInRec int64, fout io.Writer, inCh <-chan []any, gotError func(error)) {
	var cpErr, err error
	pool := memory.NewGoAllocator()
	var builders []ArrayBuilder
	var rowCount, totalRowCount int64
	var record *ArrayRecord
	if nrowsInRec == 0 {
		nrowsInRec = 1024
	}

	// Prepare the parquet writer
	props := parquet.NewWriterProperties(parquet.WithCompression(compress.Codecs.Snappy), parquet.WithAllocator(pool),
		parquet.WithBatchSize(nrowsInRec), parquet.WithMaxRowGroupLength(nrowsInRec), parquet.WithCreatedBy("jetstore"))
	writer, err := pqarrow.NewFileWriter(schemaInfo.ArrowSchema(), fout, props, pqarrow.DefaultWriterProps())
	if err != nil {
		cpErr = fmt.Errorf("while calling pqarrow.NewFileWriter: %v", err)
		goto gotError
	}

	builders, err = schemaInfo.CreateBuilders(pool)
	if err != nil {
		cpErr = fmt.Errorf("while calling pqarrow.NewFileWriter: %v", err)
		goto gotError
	}
	defer func() {
		for _, b := range builders {
			b.Release()
		}
	}()

	// Write the rows into the temp file
	for inRow := range inCh {
		if len(inRow) != len(builders) {
			cpErr = fmt.Errorf("error: len(row) does not match len(builders) in WriteParquetPartitionV2")
			goto gotError
		}
		for i, builder := range builders {
			value, err := ConvertToSchemaV2(inRow[i], schemaInfo.Fields[i])
			if err != nil {
				cpErr = fmt.Errorf("converting to parquet type failed for column %d (%s) type %s with value %v of type %T: %v", 
					i, schemaInfo.Fields[i].Name, schemaInfo.Fields[i].Type, inRow[i], inRow[i], err)
				// log.Println(cpErr, "...Ignored")
				goto gotError
			}
			builder.Append(value)
		}
		rowCount++
		if rowCount >= nrowsInRec {
			record = NewArrayRecord(schemaInfo.schema, builders)
			// log.Printf("*** Make record @ %d, record has %d rows\n", rowCount, record.Record.NumRows())
			err = writer.Write(record.Record)
			record.Release()
			if err != nil {
				cpErr = fmt.Errorf("while writing parquet record: %v", err)
				goto gotError
			}
			totalRowCount += rowCount
			rowCount = 0
		}
	}
	if rowCount > 0 {
		// Flush the last record
		record = NewArrayRecord(schemaInfo.schema, builders)
		// log.Printf("*** Flush last record @ %d, record has %d rows\n", rowCount, record.Record.NumRows())
		err = writer.Write(record.Record)
		record.Release()
		if err != nil {
			cpErr = fmt.Errorf("while writing parquet record: %v", err)
			goto gotError
		}
		totalRowCount += rowCount
	}
	// log.Println("*** Total Row Written to Parquet:", totalRowCount)
	err = writer.Close()
	if err != nil {
		cpErr = fmt.Errorf("while closing parquet file: %v", err)
		goto gotError
	}
	// All good!
	return
gotError:
	gotError(cpErr)
}

func ConvertToSchemaV2(v any, se *FieldInfo) (any, error) {
	if v == nil {
		return nil, nil
	}
	switch se.Type {
	case arrow.FixedWidthTypes.Boolean.Name():
		switch vv := v.(type) {
		case string:
			return !(vv == "false" || vv == "FALSE" || vv == "0"), nil
		case int:
			return vv != 0, nil
		default:
			return false, nil
		}
	case arrow.PrimitiveTypes.Int32.Name(), arrow.PrimitiveTypes.Date32.Name():
		switch vv := v.(type) {
		case string:
			// Check if it's a date
			if se.Type == arrow.PrimitiveTypes.Date32.Name() {
				d, err := rdf.ParseDate(vv)
				if err != nil {
					// Couln't parse the date, return 1970/01/01
					return int32(0), nil
				}
				tm := int32(d.Unix())
				if tm > 24*60*60 {
					return tm / (42 * 60 * 60), nil
				}
				return int32(0), nil
			}
			i, err := strconv.Atoi(vv)
			return int32(i), err
		case int:
			return int32(vv), nil
		case int32:
			return vv, nil
		case int64:
			return int32(vv), nil
		default:
			return int32(0), fmt.Errorf("error: WriteParquet invalid data for int32: %v", v)
		}

	case arrow.FixedWidthTypes.Timestamp_s.Name(),
		arrow.FixedWidthTypes.Timestamp_ms.Name(),
		arrow.FixedWidthTypes.Timestamp_us.Name(),
		arrow.FixedWidthTypes.Timestamp_ns.Name():
		switch vv := v.(type) {
		case string:
			// Check if it's a timestamp
				d, err := rdf.ParseDatetime(vv)
				if err != nil {
					// Couln't parse the timestamp, assumed it's already converted to int64
					return strconv.ParseInt(vv, 10, 64)
				}
			switch se.Type {
			case arrow.FixedWidthTypes.Timestamp_s.Name():
				return int64(d.Unix()), nil
			case arrow.FixedWidthTypes.Timestamp_ms.Name():
				return int64(d.UnixMilli()), nil
			case arrow.FixedWidthTypes.Timestamp_ns.Name():
				return int64(d.UnixNano()), nil
			case arrow.FixedWidthTypes.Timestamp_us.Name():
				return int64(d.UnixMicro()), nil
			default:
				return int64(0), fmt.Errorf("error: WriteParquet invalid parquet data type for timestamp: %s", se.Type)
			}
		case int:
			return int64(vv), nil
		case int32:
			return int64(vv), nil
		case int64:
			return vv, nil
		default:
			return int64(0), fmt.Errorf("error: WriteParquet invalid data type for timestamp: %T", v)
		}

	case arrow.PrimitiveTypes.Int64.Name():
		switch vv := v.(type) {
		case string:
			return strconv.ParseInt(vv, 10, 64)
		case int:
			return int64(vv), nil
		case int32:
			return int64(vv), nil
		case int64:
			return vv, nil
		default:
			return int64(0), fmt.Errorf("error: WriteParquet invalid data for int64: %v", v)
		}

	case arrow.PrimitiveTypes.Float32.Name():
		switch vv := v.(type) {
		case string:
			f, err := strconv.ParseFloat(vv, 32)
			return float32(f), err
		case int:
			return float32(vv), nil
		case int32:
			return float32(vv), nil
		case int64:
			return float32(vv), nil
		default:
			return float32(0), fmt.Errorf("error: WriteParquet invalid data for float32: %v", v)
		}

	case arrow.PrimitiveTypes.Float64.Name():
		switch vv := v.(type) {
		case string:
			return strconv.ParseFloat(vv, 64)
		case int:
			return float64(vv), nil
		case int32:
			return float64(vv), nil
		case int64:
			return float64(vv), nil
		default:
			return float64(0), fmt.Errorf("error: WriteParquet invalid data for float64: %v", v)
		}

	case arrow.BinaryTypes.String.Name(), arrow.BinaryTypes.Binary.Name():
		return encodeRdfTypeToTxt(v), nil

	default:
		return nil, fmt.Errorf("error: WriteParquet unknown parquet type: %v", se.Type)
	}
}
