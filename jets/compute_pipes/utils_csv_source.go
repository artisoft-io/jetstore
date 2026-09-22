package compute_pipes

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/artisoft-io/jetstore/jets/csv"
	"github.com/artisoft-io/jetstore/jets/utils"
	"github.com/golang/snappy"
	"github.com/google/uuid"
)

// Utilities for CSV Source Files, with spec via CsvSourceSpec.
// This correspond to specify a csv file in s3

type CsvSourceS3 struct {
	fileKey *FileKeyInfo
	spec    *CsvSourceSpec
	env     map[string]any
}

func NewCsvSourceS3(spec *CsvSourceSpec, env map[string]any) (*CsvSourceS3, error) {

	var fileKey *FileKeyInfo
	switch spec.Type {
	case "cpipes":
		if len(spec.ReadStepId) == 0 {
			return nil, fmt.Errorf("error: s3_csv_lookup of type cpipes must have read_step_id provided in cpipes spec")
		} else {
			spec.ReadStepId = utils.ReplaceEnvVars(spec.ReadStepId, env)
		}
		if len(spec.ProcessName) == 0 {
			spec.ProcessName = env["$PROCESS_NAME"].(string)
		} else {
			spec.ProcessName = utils.ReplaceEnvVars(spec.ProcessName, env)
		}
		if len(spec.SessionId) == 0 {
			spec.SessionId = env["$SESSIONID"].(string)
		} else {
			spec.SessionId = utils.ReplaceEnvVars(spec.SessionId, env)
		}
		if len(spec.JetsPartitionLabel) == 0 {
			spec.JetsPartitionLabel = env["$JETS_PARTITION_LABEL"].(string)
		} else {
			spec.JetsPartitionLabel = utils.ReplaceEnvVars(spec.JetsPartitionLabel, env)
		}
		if len(spec.Format) == 0 {
			spec.Format = "headerless_csv"
		}
		if len(spec.Compression) == 0 {
			spec.Compression = "snappy"
		}
		allFileKeys, err := GetS3FileKeys(spec.ProcessName, spec.SessionId,
			spec.ReadStepId, spec.JetsPartitionLabel, &InputChannelConfig{}, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to file keys for CsvSourceS3 of type cpipes: %v", err)
		}
		fileKeys := allFileKeys[0]
		if len(fileKeys) == 0 {
			if spec.MakeEmptyWhenNoFile {
				return &CsvSourceS3{
					fileKey: nil,
					spec:    spec,
					env:     env,
				}, nil
			}
			return nil, fmt.Errorf(
				"error: no file keys found for CsvSourceS3 of type cpipes, ReadStepId: %s, JetPartitionLabel: %s",
				spec.ReadStepId, spec.JetsPartitionLabel)
		}
		fileKey = fileKeys[0]

	// The `csv_file` arm: the object is named outright by the spec rather than
	// located from a previous cpipes step's stage output.
	//
	// This arm exists to make a static gate mean more rather than to make the
	// engine do more. `CsvSourceSpec/csv_file` has been a token of the Go
	// contract table (cpipes_contract_data.go), of the Pydantic model and of the
	// emitted json schema since the applicability matrix was extracted, and the
	// matrix names the switch above as its evidence that `type` is required --
	// while the switch had no arm for the token. So a document declaring
	// `csv_file` passed every static gate and then failed here, inside a running
	// worker, for what is a configuration error. Nothing about that failure was
	// detectable before the run.
	case "csv_file":
		if len(spec.CsvSourceFileKey) == 0 {
			return nil, fmt.Errorf(
				"error: CsvSourceS3 of type csv_file must have csv_source_file_key provided in the csv source spec")
		}
		csvFileKey := utils.ReplaceEnvVars(spec.CsvSourceFileKey, env)
		if len(csvFileKey) == 0 {
			return nil, fmt.Errorf(
				"error: CsvSourceS3 of type csv_file has csv_source_file_key '%s' resolving to an empty key",
				spec.CsvSourceFileKey)
		}
		// Defaults for an object named in full, as against a cpipes stage file:
		// a csv carrying its header row, uncompressed. The `cpipes` arm above
		// defaults the same two fields to headerless_csv and snappy for the same
		// reason in reverse -- that is what a stage file is.
		if len(spec.Format) == 0 {
			spec.Format = "csv"
		}
		if len(spec.Compression) == 0 {
			spec.Compression = "none"
		}
		// MakeEmptyWhenNoFile is applicable to this token in the contract, so it
		// has to mean something here. It costs a listing, which is why it is done
		// only when the flag asks for it: the documented default is to fail when
		// the file is absent, and the download reports that loudly on its own.
		if spec.MakeEmptyWhenNoFile {
			found, err := S3ObjectExists(csvFileKey)
			if err != nil {
				return nil, fmt.Errorf(
					"while looking for the file of a CsvSourceS3 of type csv_file: %v", err)
			}
			if !found {
				log.Printf("No file found for CsvSourceS3 of type csv_file with key %s, making an empty source",
					csvFileKey)
				return &CsvSourceS3{
					fileKey: nil,
					spec:    spec,
					env:     env,
				}, nil
			}
		}
		// FileKeyInfo's byte range applies only when `end > 0` (DownloadS3Object,
		// actions_s3_utils.go), so the zero value here is a full download.
		fileKey = &FileKeyInfo{key: csvFileKey}

	default:
		return nil, fmt.Errorf("error: unknown CsvSourceS3 type: %s", spec.Type)
	}
	log.Printf("Got file key %s from s3 as csv source", fileKey.key)
	return &CsvSourceS3{
		fileKey: fileKey,
		spec:    spec,
		env:     env,
	}, nil
}

// *TODO Refactor this ReadFileToMetaGraph func
func (ctx *CsvSourceS3) ReadFileToMetaGraph(re JetRuleEngine, config *JetrulesSpec) error {
	rm := re.GetMetaResourceManager()

	// Create a local temp directory to hold the file
	inFolderPath, err := os.MkdirTemp("", "jetstore")
	if err != nil {
		return fmt.Errorf("failed to create local temp directory: %v", err)
	}
	defer os.RemoveAll(inFolderPath)

	// Fetch the file from s3, save it locally
	retry := 0
do_retry:
	localFileName, _, err := DownloadS3Object("", ctx.fileKey, inFolderPath, 1)
	if err != nil {
		if retry < 6 {
			time.Sleep(500 * time.Millisecond)
			retry++
			goto do_retry
		}
		return fmt.Errorf("failed to download file from s3 for s3_csv_lookup of type cpipes: %v", err)
	}
	defer os.Remove(localFileName)

	var fileHd *os.File
	var csvReader *csv.Reader
	var inputRowCount int64
	var inRow []string
	var predicates []RdfNode
	var rdfTypes []string

	fileHd, err = os.Open(localFileName)
	if err != nil {
		return fmt.Errorf("while opening temp file '%s' (readFile): %v", localFileName, err)
	}
	defer func() {
		fileHd.Close()
	}()

	source := ctx.spec
	sepFlag := ','
	if source.Delimiter != 0 {
		sepFlag = source.Delimiter
	}

	// Read the csv file and package the lookup table
	switch source.Compression {
	case "none":
		csvReader = csv.NewReader(fileHd)
	case "snappy":
		csvReader = csv.NewReader(snappy.NewReader(fileHd))
	default:
		return fmt.Errorf("error: unknown compression in readCsvLookup: %s", source.Compression)
	}
	csvReader.Comma = sepFlag
	if source.Format == "csv" {
		// get the header row (first row)
		headers, err := csvReader.Read()
		if err != nil {
			return fmt.Errorf("while in ReadFileToMetaGraph: %v", err)
		}
		rdfTypes = make([]string, 0, len(headers))
		dataPropertyMap, err := GetWorkspaceDataProperties()
		if err != nil {
			return fmt.Errorf("error get data properties from local workspace")
		}
		// Make the property resource (the predicate of the triple)
		predicates = make([]RdfNode, 0, len(headers))
		var dataType string
		for _, h := range headers {
			predicates = append(predicates, rm.NewResource(h))
			nd := dataPropertyMap[h]
			dataType = "text"
			if nd != nil {
				dataType = nd.Type
			}
			rdfTypes = append(rdfTypes, dataType)
		}
	} else {
		return fmt.Errorf("error: currently only supporting csv format in readCsvLookup, not supporting: %s", source.Format)
	}

	if err == io.EOF {
		// empty file
		return nil
	}
	// Check the should be impossible condition
	if err != nil {
		return fmt.Errorf("error while reading first input records in readCsvLookup: %v", err)
	}
	jr := re.JetResources()
	if jr == nil {
		return fmt.Errorf("error: bug nil JetsResources")
	}

	var object RdfNode
	var rdfClass RdfNode
	if len(ctx.spec.ClassName) > 0 {
		rdfClass = rm.NewResource(ctx.spec.ClassName)
	}

	for {
		// read and put the rows as rdf an entity (rdf type assertion must be in the data)
		err = nil
		inRow, err = csvReader.Read()
		if err == nil {
			subjectTxt := uuid.New().String()
			subject := rm.NewResource(subjectTxt)
			if rdfClass != nil {
				err = re.Insert(subject, jr.Rdf__type, rdfClass)
				if err != nil {
					return err
				}
			}
			for i, value := range inRow {
				// Parse the value to the rdfType
				object, err = ParseRdfNodeValue(rm, value, rdfTypes[i])
				if err != nil {
					log.Printf("WARNING: Cannot parse value to rdf type %s\n", rdfTypes[i])
					object = rm.RdfNull()
				}
				// Assert the triple
				err = re.Insert(subject, predicates[i], object)
				if err != nil {
					return err
				}
			}
			err = re.Insert(subject, jr.Jets__key, rm.NewTextLiteral(subjectTxt))
			if err != nil {
				return err
			}

		}

		switch {
		case err == io.EOF:
			// expected exit route
			return nil

		case err != nil:
			return fmt.Errorf("error while reading csv lookup table: %v", err)

		default:
			inputRowCount += 1
		}
	}
}
