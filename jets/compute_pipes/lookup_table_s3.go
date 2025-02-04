package compute_pipes

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/csv"
	"github.com/golang/snappy"
	"github.com/jackc/pgx/v4/pgxpool"
)

// lookup table from s3 files, loaded into memory

// data is the mapping of the looup key -> values
// columnsMap is the mapping of the return column name -> position in the returned row (values)
type LookupTableS3 struct {
	spec         *LookupSpec
	isEmptyTable bool
	data         map[string]*[]interface{}
	columnsMap   map[string]int
}

func NewLookupTableS3(_ *pgxpool.Pool, spec *LookupSpec, env map[string]interface{}, isVerbose bool) (LookupTable, error) {
	if spec == nil || spec.CsvSource == nil {
		return nil, fmt.Errorf("error: lookup table of type s3_csv_lookup must have csv_source configured")
	}
	tbl := &LookupTableS3{
		spec:       spec,
		data:       make(map[string]*[]interface{}),
		columnsMap: make(map[string]int),
	}

	csvSource, err := NewCsvSourceS3(spec.CsvSource, env)
	if err != nil {
		return nil, fmt.Errorf("while calling NewCsvSourceS3 (NewLookupTableS3): %v", err)
	}
	// Check if this is an empty source (no file found and spec indicates not to error out)
	if csvSource.fileKey == nil {
		tbl.isEmptyTable = true
		return tbl, nil
	}

	// Create a local temp directory to hold the file
	inFolderPath, err := os.MkdirTemp("", "jetstore")
	if err != nil {
		return nil, fmt.Errorf("failed to create local temp directory: %v", err)
	}
	defer os.Remove(inFolderPath)

	// Fetch the file from s3, save it locally
	retry := 0
do_retry:
	inFilePath, _, err := DownloadS3Object("", csvSource.fileKey, inFolderPath, 1)
	if err != nil {
		if retry < 6 {
			time.Sleep(500 * time.Millisecond)
			retry++
			goto do_retry
		}
		return nil, fmt.Errorf("failed to download file from s3 for s3_csv_lookup of type cpipes: %v", err)
	}
	defer os.Remove(inFilePath)

	// Read the file and load the lookup table into memory
	nrows, err := tbl.readCsvLookup(inFilePath)
	if err != nil {
		err = fmt.Errorf("while loading s3_csv_lookup with key %s: %v", tbl.spec.Key, err)
		return nil, err
	}
	log.Printf("Lookup table of type s3_csv_lookup with key %s is loaded with %d rows", tbl.spec.Key, nrows)
	return tbl, nil
}

func (tbl *LookupTableS3) Lookup(key *string) (*[]interface{}, error) {
	if key == nil {
		return nil, fmt.Errorf("error: cannot do a lookup with a null key for lookup table %s", tbl.spec.Key)
	}
	if tbl.isEmptyTable {
		return nil, nil
	}
	return tbl.data[*key], nil
}

func (tbl *LookupTableS3) LookupValue(row *[]interface{}, columnName string) (interface{}, error) {
	if tbl.isEmptyTable {
		return nil, nil
	}
	pos, ok := tbl.columnsMap[columnName]
	if !ok {
		return nil, fmt.Errorf("error: column named %s is not a column returned by the lookup table %s",
			columnName, tbl.spec.Key)
	}
	return (*row)[pos], nil
}

func (tbl *LookupTableS3) ColumnMap() map[string]int {
	return tbl.columnsMap
}

// Return true only if there was no files found on s3
func (tbl *LookupTableS3) IsEmptyTable() bool {
	return tbl.isEmptyTable
}

func (tbl *LookupTableS3) readCsvLookup(localFileName string) (int64, error) {
	var fileHd *os.File
	var csvReader *csv.Reader
	var err error
	fileHd, err = os.Open(localFileName)
	if err != nil {
		return 0, fmt.Errorf("while opening temp file '%s' (readCsvLookup): %v", localFileName, err)
	}
	defer func() {
		fileHd.Close()
	}()

	// Keep a mapping of the returned column names to their position in the returned row
	for i, valueColumn := range tbl.spec.LookupValues {
		tbl.columnsMap[valueColumn] = i
	}

	source := tbl.spec.CsvSource
	sepFlag := ','
	if source.Delimiter != 0 {
		sepFlag = source.Delimiter
	}

	// Read the csv file and package the lookup table
	switch source.Compression {
	case "none":
		csvReader = csv.NewReader(fileHd)
		csvReader.Comma = sepFlag
	case "snappy":
		csvReader = csv.NewReader(snappy.NewReader(fileHd))
		csvReader.Comma = sepFlag
	default:
		return 0, fmt.Errorf("error: unknown compression in readCsvLookup: %s", source.Compression)
	}

	if source.Format == "csv" {
		// Make a lookup of the current column spec
		overrides := make(map[string]*TableColumnSpec)
		for i := range tbl.spec.Columns {
			tblSpec := &tbl.spec.Columns[i]
			overrides[tblSpec.Name] = tblSpec
		}
		// get the header row (first row)
		headers, err := csvReader.Read()
		if err != nil {
			if err == io.EOF {
				// empty file
				return 0, nil
			}
			return 0, fmt.Errorf("while in reading the header row in readCsvLookup: %v", err)
		}
		columns := make([]TableColumnSpec, 0, len(headers))
		for _, h := range headers {
			override := overrides[h]
			rdfType := "text"
			isArray := false
			if override != nil {
				rdfType = override.RdfType
				isArray = override.IsArray
			}
			columns = append(columns, TableColumnSpec{
				Name:    h,
				RdfType: rdfType,
				IsArray: isArray,
			})
		}
		// set the column spec
		tbl.spec.Columns = columns
	}

	// keep track of the column name and their pos in the returned csv row
	csvColumnsPos := make(map[string]int)
	for i := range tbl.spec.Columns {
		csvColumnsPos[tbl.spec.Columns[i].Name] = i
	}
	
	// Read the file
	var inputRowCount int64
	var inRow []string
	keys := make([]string, len(tbl.spec.LookupKey))
	for {
		// read and put the lookup rows into tbl method receiver
		err = nil
		inRow, err = csvReader.Read()
		if err == nil {
			// If a key component is null, the corresponding key component will be the empty string
			for i, key := range tbl.spec.LookupKey {
				pos, ok := csvColumnsPos[key]
				if !ok {
					return 0, fmt.Errorf("error: key column '%s' is not in the csv lookup table %s", key, tbl.spec.Key)
				}
				keys[i] = inRow[pos]
			}
			lookupKey := strings.Join(keys, "")

			// the associated values
			lookupValues := make([]interface{}, len(tbl.spec.LookupValues))
			for i, name := range tbl.spec.LookupValues {
				pos, ok := csvColumnsPos[name]
				if !ok {
					return 0, fmt.Errorf("error: lookup value column '%s' is not in the csv lookup table %s", name, tbl.spec.Key)
				}
				lookupValues[i], err = CastToRdfType(inRow[pos], tbl.spec.Columns[csvColumnsPos[name]].RdfType)
				if err != nil {
					return 0, fmt.Errorf("while loading csv lookup table, error in casting to rdf type: %v", err)
				}
			}

			// save the lookup row
			tbl.data[lookupKey] = &lookupValues
		}

		switch {
		case err == io.EOF:
			// expected exit route
			return inputRowCount, nil

		case err != nil:
			return 0, fmt.Errorf("error while reading csv lookup table: %v", err)

		default:
			inputRowCount += 1
		}
	}
}
