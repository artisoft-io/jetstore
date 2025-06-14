package delegate

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Run report and save it as parquet file locally and then copy it to s3

func (ca *CommandArguments) DoParquetReport(dbpool *pgxpool.Pool, tempDir string, s3FileName *string, name string, sqlStmt *string) error {
	// save report locally in parquet
	fmt.Println("STMT", name, "saving in parquet format")

	// open the parquet writer
	tempFileName := fmt.Sprintf("%s/csv.parquet", tempDir)
	// Write to file
	fw, err := os.Create(tempFileName)
	if err != nil {
		return err
	}
	defer func() {
		os.Remove(tempFileName)
	}()

	// reading from db
	rows, err := dbpool.Query(context.Background(), *sqlStmt)
	if err != nil {
		return fmt.Errorf("while called query: %v", err)
	}
	defer rows.Close()

	// output schema
	csvDatatypes := make([]*compute_pipes.FieldInfo, 0)
	fd := rows.FieldDescriptions()
	// keep a mapping between input col position to output col position (for droping arrays and unknown data type)
	outColFromInCol := make(map[int]int, len(fd))
	inColFromOutCol := make(map[int]int, len(fd))

	outPos := 0
	for inPos := range fd {
		oid := fd[inPos].DataTypeOID
		columName := string(fd[inPos].Name)
		// fmt.Println("*** ColumnName",columName,"oid",oid)
		// skipping arrays and unknown data type
		if !dbutils.IsArrayFromOID(oid) {
			switch datatype := dbutils.DataTypeFromOID(oid); datatype {
			case "string", "time":
				csvDatatypes = append(csvDatatypes, &compute_pipes.FieldInfo{
					Name:     columName,
					Type:     arrow.BinaryTypes.String.Name(),
					Nullable: true,
				})
				outColFromInCol[inPos] = outPos
				inColFromOutCol[outPos] = inPos
				outPos += 1
			case "date":
				csvDatatypes = append(csvDatatypes, &compute_pipes.FieldInfo{
					Name:     columName,
					Type:     arrow.PrimitiveTypes.Date32.Name(),
					Nullable: true,
				})
				outColFromInCol[inPos] = outPos
				inColFromOutCol[outPos] = inPos
				outPos += 1
			case "double":
				csvDatatypes = append(csvDatatypes, &compute_pipes.FieldInfo{
					Name:     columName,
					Type:     arrow.PrimitiveTypes.Float64.Name(),
					Nullable: true,
				})
				outColFromInCol[inPos] = outPos
				inColFromOutCol[outPos] = inPos
				outPos += 1
			case "timestamp", "long":
				csvDatatypes = append(csvDatatypes, &compute_pipes.FieldInfo{
					Name:     columName,
					Type:     arrow.PrimitiveTypes.Int64.Name(),
					Nullable: true,
				})
				outColFromInCol[inPos] = outPos
				inColFromOutCol[outPos] = inPos
				outPos += 1
			case "int":
				csvDatatypes = append(csvDatatypes, &compute_pipes.FieldInfo{
					Name:     columName,
					Type:     arrow.PrimitiveTypes.Int32.Name(),
					Nullable: true,
				})
				outColFromInCol[inPos] = outPos
				inColFromOutCol[outPos] = inPos
				outPos += 1
			default:
				log.Printf("Got unknown data type, report %s, column %s, datatype oid %d, skipping", name, columName, oid)
			}
		} else {
			log.Printf("Got an array data type, report %s, column %s, datatype oid %d, skipping", name, columName, oid)
		}
	}
	nbrInputColumns := len(fd)
	nbrOutputColumns := len(outColFromInCol)

	schemaInfo := compute_pipes.NewEmptyParquetSchemaInfo()
	schemaInfo.Fields = csvDatatypes
	inputCh := make(chan []any, 1)
	doneCh := make(chan struct{})

	var writeErr error
	gotError := func(err error) {
		writeErr = err
		close(doneCh)
	}
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		compute_pipes.WriteParquetPartitionV3(schemaInfo, fw, inputCh, gotError)
		wg.Done()
	}()

	var rowCount int64
	// Read from sql and write to parquet file
	for rows.Next() {
		dataRow := make([]any, nbrInputColumns)
		for inPos := range nbrInputColumns {
			outPos, ok := outColFromInCol[inPos]
			if ok {
				switch csvDatatypes[outPos].Type {
				case arrow.BinaryTypes.String.Name(), arrow.PrimitiveTypes.Date32.Name():
					dataRow[inPos] = &sql.NullString{}
				case arrow.PrimitiveTypes.Float64.Name():
					dataRow[inPos] = &sql.NullFloat64{}
				case arrow.PrimitiveTypes.Int64.Name():
					dataRow[inPos] = &sql.NullInt64{}
				case arrow.PrimitiveTypes.Int32.Name():
					dataRow[inPos] = &sql.NullInt32{}
				}
			} else {
				dataRow[inPos] = &sql.NullString{}
			}
		}
		// scan the row
		if err = rows.Scan(dataRow...); err != nil {
			writeErr = fmt.Errorf("while scanning the row: %v", err)
			goto doneReport
		}
		// make a flat row for writing
		flatRow := make([]any, nbrOutputColumns)
		for outPos := range nbrOutputColumns {
			inPos, ok := inColFromOutCol[outPos]
			if ok {
				switch csvDatatypes[outPos].Type {
				case arrow.BinaryTypes.String.Name(), arrow.PrimitiveTypes.Date32.Name():
					ns := dataRow[inPos].(*sql.NullString)
					if ns.Valid {
						flatRow[outPos] = ns.String
					} else {
						flatRow[outPos] = nil
					}
				case arrow.PrimitiveTypes.Float64.Name():
					ns := dataRow[inPos].(*sql.NullFloat64)
					if ns.Valid {
						flatRow[outPos] = ns.Float64
					} else {
						flatRow[outPos] = nil
					}
				case arrow.PrimitiveTypes.Int64.Name():
					ns := dataRow[inPos].(*sql.NullInt64)
					if ns.Valid {
						flatRow[outPos] = ns.Int64
					} else {
						flatRow[outPos] = nil
					}
				case arrow.PrimitiveTypes.Int32.Name():
					ns := dataRow[inPos].(*sql.NullInt32)
					if ns.Valid {
						flatRow[outPos] = ns.Int32
					} else {
						flatRow[outPos] = nil
					}
				}
			} else {
				writeErr = fmt.Errorf("unexpected error while scanning the row")
				goto doneReport
			}
		}
		select {
		case inputCh <- flatRow:
		case <-doneCh:
			log.Printf("DoParquetReport interrupted")
			goto doneReport
		}
		rowCount += 1
	}
doneReport:
	close(inputCh)
	wg.Wait()
	if writeErr != nil {
		log.Println("Got error while writing parquet file", writeErr)
		fw.Close()
		return writeErr
	}
	log.Println("Parquet Write Finished")
	fw.Close()

	// Copy file to s3 location
	fileHd, err := os.Open(tempFileName)
	if err != nil {
		return fmt.Errorf("while opening written file to copy to s3: %v", err)
	}
	defer fileHd.Close()
	if err = awsi.UploadToS3(ca.BucketName, ca.RegionName, *s3FileName, fileHd); err != nil {
		return fmt.Errorf("while copying to s3: %v", err)
	}
	fmt.Println("Report:", name, "rowsUploaded containing", rowCount, "rows")
	return nil
}
