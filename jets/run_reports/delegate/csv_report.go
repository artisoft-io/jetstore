package delegate

import (
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Run report and save it as csv file locally and then copy it to s3
// This function is used when a custom kms key is specified since aws_s3 does not support custom kms key

func (ca *CommandArguments)DoCsvReport(dbpool *pgxpool.Pool, tempDir string, s3FileName *string, name string, sqlStmt *string) error {
		// save report locally in csv format
		fmt.Println("STMT", name, "saving in csv format locally then copied to s3")
	
		// open the file writer
		fw, err := os.CreateTemp("", "csv_rpt")
		if err != nil {
			return fmt.Errorf("while creating temp file for write: %v", err)
		}
		defer func (){
			if fw != nil {
				fw.Close()
				os.Remove(fw.Name())
			}
		}()

		// reading from db
		rows, err := dbpool.Query(context.Background(), *sqlStmt)
		if err != nil {
			return fmt.Errorf("while called query: %v", err)
		}
		defer rows.Close()
		
		// output schema: column name and data type
		csvColumnNames := make([]string, 0)
		// csvDatatypes := make([]string, 0)
		fd := rows.FieldDescriptions()
		// keep a mapping between input col position to output col position (for droping arrays and unknown data type)
		outColFromInCol := make(map[int]int, len(fd))
		inColFromOutCol := make(map[int]int, len(fd))

		outPos := 0
		for inPos := range fd {
			oid := fd[inPos].DataTypeOID
			columName := string(fd[inPos].Name)
			fmt.Println("*** ColumnName",columName,"oid",oid)
			// skipping arrays and unknown data type (for now anyways...)
			if !dbutils.IsArrayFromOID(oid) {
				switch datatype := dbutils.DataTypeFromOID(oid); datatype {
				case "string", "date", "time", "timestamp", "int", "long", "double":
					csvColumnNames = append(csvColumnNames, columName)
					// csvDatatypes = append(csvDatatypes, datatype)
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

		// Open a csv writer
		csvWriter := csv.NewWriter(fw)
		// Write the header
		csvWriter.Write(csvColumnNames)

		var rowCount int64

		// Read from sql and write to temp file
		for rows.Next() {
			dataRow := make([]interface{}, nbrInputColumns)
			for inPos := 0; inPos < nbrInputColumns; inPos++ {
				// outPos, ok := outColFromInCol[inPos]
				_, ok := outColFromInCol[inPos]
				if ok {
					// switch csvDatatypes[outPos] {
					// case "string", "date", "time":
					// 	dataRow[inPos] = &sql.NullString{}
					// case "double":
					// 	dataRow[inPos] = &sql.NullFloat64{}
					// case "timestamp", "long":
					// 	dataRow[inPos] = &sql.NullInt64{}
					// case "int":
					// 	dataRow[inPos] = &sql.NullInt32{}	
					// }
					dataRow[inPos] = &sql.NullString{}	
				// } else {
				// 	dataRow[inPos] = &sql.NullString{}
				}
			}
			// scan the row
			if err = rows.Scan(dataRow...); err != nil {
			return fmt.Errorf("while scanning the row: %v", err)
			}
			// make a flat row for writing
			flatRow := make([]string, nbrOutputColumns)
			for outPos := 0; outPos < nbrOutputColumns; outPos++ {
				inPos, ok := inColFromOutCol[outPos]
				if ok {
					if dataRow[inPos] != nil {
						v := dataRow[inPos].(*sql.NullString)
						if v.Valid {
							flatRow[outPos] = v.String
						}
					}
				} else {
					return fmt.Errorf("unexpected error while scanning the row")
				}
			}
			// flatRow := make([]interface{}, nbrOutputColumns)
			// for outPos := 0; outPos < nbrOutputColumns; outPos++ {
			// 	inPos, ok := inColFromOutCol[outPos]
			// 	if ok {
			// 		switch csvDatatypes[outPos] {
			// 		case "string", "date", "time":
			// 			ns := dataRow[inPos].(*sql.NullString)
			// 			if ns.Valid {
			// 				flatRow[outPos] = ns.String
			// 			} else {
			// 				flatRow[outPos] = ""
			// 			}
			// 		case "double":
			// 			ns := dataRow[inPos].(*sql.NullFloat64)
			// 			if ns.Valid {
			// 				flatRow[outPos] = ns.Float64
			// 			} else {
			// 				flatRow[outPos] = float64(0)
			// 			}
			// 		case "timestamp", "long":
			// 			ns := dataRow[inPos].(*sql.NullInt64)
			// 			if ns.Valid {
			// 				flatRow[outPos] = ns.Int64
			// 			} else {
			// 				flatRow[outPos] = int64(0)
			// 			}
			// 		case "int":
			// 			ns := dataRow[inPos].(*sql.NullInt32)
			// 			if ns.Valid {
			// 				flatRow[outPos] = ns.Int32
			// 			} else {
			// 				flatRow[outPos] = int32(0)
			// 			}
			// 		}
			// 	} else {
			// 		return fmt.Errorf("unexpected error while scanning the row")
			// 	}
			// }
			if err = csvWriter.Write(flatRow); err != nil {
				return fmt.Errorf("while writing row to local csv file: %v", err)
			}
			rowCount += 1
		}
		csvWriter.Flush()
		log.Println("Local CSV Write Finished")
		fw.Seek(0, 0)

		// Copy file to s3 location
		if err != nil {
			return fmt.Errorf("while opening written file to copy to s3: %v", err)
		}
		if err = awsi.UploadToS3(ca.BucketName, ca.RegionName, *s3FileName, fw); err != nil {
			return fmt.Errorf("while copying to s3: %v", err)
		}
		fmt.Println("Report:", name, "rowsUploaded containing", rowCount, "rows")

	return nil
}
