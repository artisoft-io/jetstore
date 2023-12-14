package delegate

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/xitongsys/parquet-go/writer"
)

// Run report and save it as parquet file locally and then copy it to s3

func (ca *CommandArguments)DoParquetReport(dbpool *pgxpool.Pool, s3FileName *string, name string, sqlStmt *string) error {
		// save report locally in parquet
		fmt.Println("STMT", name, "saving in parquet format")
		// Create temp directory for the local parquet file
		tempDir, err := os.MkdirTemp("", "jetstore")
		if err != nil {
			return fmt.Errorf("while creating temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)
	
		// open the parquet writer
		tempFileName := fmt.Sprintf("%s/csv.parquet", tempDir)
		fw, err := NewLocalFileWriter(tempFileName)
		if err != nil {
			return fmt.Errorf("while opening parquet file for write: %v", err)
		}

		// reading from db
		rows, err := dbpool.Query(context.Background(), *sqlStmt)
		if err != nil {
			return fmt.Errorf("while called query: %v", err)
		}
		defer rows.Close()
		
		// output schema
		csvSchema := make([]string, 0)
		csvDatatypes := make([]string, 0)
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
				case "string", "date", "time":
					csvSchema = append(csvSchema, fmt.Sprintf("name=%s, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY", columName))
					csvDatatypes = append(csvDatatypes, datatype)
					outColFromInCol[inPos] = outPos
					inColFromOutCol[outPos] = inPos
					outPos += 1
				case "double":
					csvSchema = append(csvSchema, fmt.Sprintf("name=%s, type=DOUBLE", columName))
					csvDatatypes = append(csvDatatypes, datatype)
					outColFromInCol[inPos] = outPos
					inColFromOutCol[outPos] = inPos
					outPos += 1
				case "timestamp", "long":
					csvSchema = append(csvSchema, fmt.Sprintf("name=%s, type=INT64", columName))
					csvDatatypes = append(csvDatatypes, datatype)
					outColFromInCol[inPos] = outPos
					inColFromOutCol[outPos] = inPos
					outPos += 1
				case "int":
					csvSchema = append(csvSchema, fmt.Sprintf("name=%s, type=INT32", columName))
					csvDatatypes = append(csvDatatypes, datatype)
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

		// Create the parquet writer now that we have the schema ready
		pw, err := writer.NewCSVWriter(csvSchema, fw, 4)
		if err != nil {
			fw.Close()
			return fmt.Errorf("while opening parquet csv writer: %v", err)
		}
		var rowCount int64

		// Read from sql and write to parquet file
		for rows.Next() {
			dataRow := make([]interface{}, nbrInputColumns)
			for inPos := 0; inPos < nbrInputColumns; inPos++ {
				outPos, ok := outColFromInCol[inPos]
				if ok {
					switch csvDatatypes[outPos] {
					case "string", "date", "time":
						dataRow[inPos] = &sql.NullString{}
					case "double":
						dataRow[inPos] = &sql.NullFloat64{}
					case "timestamp", "long":
						dataRow[inPos] = &sql.NullInt64{}
					case "int":
						dataRow[inPos] = &sql.NullInt32{}	
					}
				} else {
					dataRow[inPos] = &sql.NullString{}
				}
			}
			// scan the row
			if err = rows.Scan(dataRow...); err != nil {
			fw.Close()
			return fmt.Errorf("while scanning the row: %v", err)
			}
			// make a flat row for writing
			flatRow := make([]interface{}, nbrOutputColumns)
			for outPos := 0; outPos < nbrOutputColumns; outPos++ {
				inPos, ok := inColFromOutCol[outPos]
				if ok {
					switch csvDatatypes[outPos] {
					case "string", "date", "time":
						ns := dataRow[inPos].(*sql.NullString)
						if ns.Valid {
							flatRow[outPos] = ns.String
						} else {
							flatRow[outPos] = ""
						}
					case "double":
						ns := dataRow[inPos].(*sql.NullFloat64)
						if ns.Valid {
							flatRow[outPos] = ns.Float64
						} else {
							flatRow[outPos] = float64(0)
						}
					case "timestamp", "long":
						ns := dataRow[inPos].(*sql.NullInt64)
						if ns.Valid {
							flatRow[outPos] = ns.Int64
						} else {
							flatRow[outPos] = int64(0)
						}
					case "int":
						ns := dataRow[inPos].(*sql.NullInt32)
						if ns.Valid {
							flatRow[outPos] = ns.Int32
						} else {
							flatRow[outPos] = int32(0)
						}
					}
				} else {
					fw.Close()
					return fmt.Errorf("unexpected error while scanning the row")
				}
			}
			if err = pw.Write(flatRow); err != nil {
				fw.Close()
				return fmt.Errorf("while writing row to parquet file: %v", err)
			}
			rowCount += 1
		}
		if err = pw.WriteStop(); err != nil {
			fw.Close()
			return fmt.Errorf("while writing parquet stop (trailer): %v", err)
		}
		log.Println("Parquet Write Finished")
		fw.Close()

		// Copy file to s3 location
		fileHd, err := os.Open(tempFileName)
		if err != nil {
			return fmt.Errorf("while opening written file to copy to s3: %v", err)
		}
		if err = awsi.UploadToS3(ca.BucketName, ca.RegionName, *s3FileName, fileHd); err != nil {
			return fmt.Errorf("while copying to s3: %v", err)
		}
		fmt.Println("Report:", name, "rowsUploaded containing", rowCount, "rows")

	return nil
}
