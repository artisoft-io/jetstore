package delegate

import (
	"bufio"
	"log"
	"math/rand"
	"strings"
	"errors"
	"fmt"
	"io"

	// "math/rand"
	"os"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/csv"
	"github.com/artisoft-io/jetstore/jets/datatable/jcsv"
	"github.com/artisoft-io/jetstore/jets/run_reports/delegate"
	"github.com/xitongsys/parquet-go/writer"
	// "github.com/google/uuid"
	// "github.com/jackc/pgx/v4/pgxpool"
)

// The delegate that actually perform the test data generation
// Required Env variable:
// JETS_DSN_SECRET
// JETS_REGION
// JETS_BUCKET
// JETS_s3_INPUT_PREFIX
// JETS_S3_KMS_KEY_ARN
// WORKSPACE Workspace currently in use
// WORKSPACES_HOME Home dir of workspaces

type CommandArguments struct {
	AwsDsnSecret     string
	DbPoolSize       int
	UsingSshTunnel   bool
	AwsRegion        string
	Dsn              string
	NdcFilePath      string
	OutFileKey       string
	CsvTemplatePath  string
	NbrRawDataFile   int
	NbrMembers       int
	NbrRowPerMembers int
	NbrRowsPerChard  int
	NbrChards        int
}

// Support Functions
// --------------------------------------------------------------------------------------
func (ca *CommandArguments) GetTemplateInfo() (string, string, error) {
	fname := fmt.Sprintf("%s/%s/%s", os.Getenv("WORKSPACES_HOME"), os.Getenv("WORKSPACE"), ca.CsvTemplatePath)
	log.Printf("Reading template from: %s", fname)
	readFile, err := os.Open(fname)
	if err != nil {
		return "", "", err
	}
	defer readFile.Close()
	fileScanner := bufio.NewScanner(readFile)
	fileScanner.Split(bufio.ScanLines)

	// Header of the csv file
	var headers, rowTemplate string
	if fileScanner.Scan() {
		headers = fileScanner.Text()
	} else {
		return "", "", fmt.Errorf("error: file too short, no headers")
	}
	if fileScanner.Scan() {
		rowTemplate = fileScanner.Text()
	} else {
		return "", "", fmt.Errorf("error: file too short, no row template")
	}
	return headers, rowTemplate, nil
}

func (ca *CommandArguments) GetNdcList() (*[]string, error) {
	readFile, err := os.Open(ca.NdcFilePath)
	if err != nil {
		return nil, err
	}
	defer readFile.Close()

	// determine the csv separator
	// auto detect the separator based on the first line
	var sep_flag jcsv.Chartype
	buf := make([]byte, 2048)
	_, err = readFile.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("error while ready first few bytes of NdcFilePath %s: %v", ca.NdcFilePath, err)
	}
	sep_flag, err = jcsv.DetectDelimiter(buf)
	if err != nil {
		return nil, fmt.Errorf("while calling jcsv.DetectDelimiter: %v", err)
	}
	_, err = readFile.Seek(0, 0)
	if err != nil {
		return nil, fmt.Errorf("error while returning to beginning of NdcFilePath %s: %v", ca.NdcFilePath, err)
	}
	fmt.Println("Got csv separator", sep_flag)

	// Setup a csv reader
	csvReader := csv.NewReader(readFile)
	csvReader.Comma = rune(sep_flag)
	csvReader.ReuseRecord = true

	// Discard the headers
	_, err = csvReader.Read()
	if err == io.EOF {
		return nil, errors.New("NdcFilePath file is empty")
	} else if err != nil {
		return nil, fmt.Errorf("while reading NdcFilePath csv headers: %v", err)
	}

	// Read the NDC
	ndcList := make([]string, 0)
	for {
		record, err := csvReader.Read()
		switch {
		case err == io.EOF:
			// Done - expected exit route
			fmt.Println("Got", len(ndcList), "NDCs")
			return &ndcList, nil
		case err != nil:
			return nil, fmt.Errorf("while reading NdcFilePath csv file: %v", err)
		default:
			ndcList = append(ndcList, record[0])
		}
	}
}

// Package Main Functions
// --------------------------------------------------------------------------------------
func (ca *CommandArguments) ValidateArguments() []string {
	var errMsg []string
	// if ca.Dsn == "" && ca.AwsDsnSecret == "" {
	// 	ca.AwsDsnSecret = os.Getenv("JETS_DSN_SECRET")
	// 	if ca.Dsn == "" && ca.AwsDsnSecret == "" {
	// 		errMsg = append(errMsg, "Connection string must be provided using either -awsDsnSecret or -dsn.")
	// 	}
	// }
	if ca.AwsRegion == "" {
		ca.AwsRegion = os.Getenv("JETS_REGION")
	}
	// if ca.AwsDsnSecret != "" && ca.AwsRegion == "" {
	// 	errMsg = append(errMsg, "aws region (-awsRegion) must be provided when -awsDnsSecret is provided.")
	// }
	// Check we have required env var
	if os.Getenv("JETS_s3_INPUT_PREFIX") == "" {
		errMsg = append(errMsg, "Env var JETS_s3_INPUT_PREFIX must be provided (used when register domain table for file key prefix).")
	}
	if os.Getenv("JETS_BUCKET") == "" {
		errMsg = append(errMsg, "Env var JETS_BUCKET must be provided.")
	}

	if ca.NbrRawDataFile < 1 {
		errMsg = append(errMsg, "NbrRawDataFile must be at least 1.")
	}

	if ca.NbrMembers < 1 {
		errMsg = append(errMsg, "NbrMembers must be at least 1.")
	}

	if ca.NbrRowPerMembers < 1 {
		errMsg = append(errMsg, "NbrRowPerMembers must be at least 1.")
	}

	if ca.NbrRowsPerChard < 1 {
		errMsg = append(errMsg, "NbrRowsPerChard must be at least 1.")
	}

	if ca.NbrChards < 1 {
		errMsg = append(errMsg, "NbrChards must be at least 1.")
	}

	fmt.Println("Status Update Arguments:")
	fmt.Println("----------------")
	fmt.Println("Got argument: dsn, len", len(ca.Dsn))
	fmt.Println("Got argument: awsRegion", ca.AwsRegion)
	fmt.Println("Got argument: awsDsnSecret", ca.AwsDsnSecret)
	fmt.Println("Got argument: dbPoolSize", ca.DbPoolSize)
	fmt.Println("Got argument: usingSshTunnel", ca.UsingSshTunnel)
	fmt.Println("Got argument: ndcFilePath", ca.NdcFilePath)
	fmt.Println("Got argument: outFileKey", ca.OutFileKey)
	fmt.Println("Got argument: csvTemplatePath", ca.CsvTemplatePath)
	fmt.Println("Got argument: NbrRawDataFile", ca.NbrRawDataFile)
	fmt.Println("Got argument: nbrMembers", ca.NbrMembers)
	fmt.Println("Got argument: NbrRowPerMembers", ca.NbrRowPerMembers)
	fmt.Println("Got argument: NbrRowsPerChard", ca.NbrRowsPerChard)
	fmt.Println("Got argument: NbrChards", ca.NbrChards)
	fmt.Printf("ENV JETS_s3_INPUT_PREFIX: %s\n", os.Getenv("JETS_s3_INPUT_PREFIX"))
	fmt.Printf("ENV JETS_BUCKET: %s\n", os.Getenv("JETS_BUCKET"))
	fmt.Printf("ENV JETS_S3_KMS_KEY_ARN: %s\n", os.Getenv("JETS_S3_KMS_KEY_ARN"))
	fmt.Printf("ENV WORKSPACE: %s\n", os.Getenv("WORKSPACE"))
	fmt.Printf("ENV WORKSPACES_HOME: %s\n", os.Getenv("WORKSPACES_HOME"))

	return errMsg
}

func (ca *CommandArguments) CoordinateWork() error {
	// // open db connection
	// var err error
	// if ca.AwsDsnSecret != "" {
	// 	// Get the dsn from the aws secret
	// 	ca.Dsn, err = awsi.GetDsnFromSecret(ca.AwsDsnSecret, ca.UsingSshTunnel, ca.DbPoolSize)
	// 	if err != nil {
	// 		return fmt.Errorf("while getting dsn from aws secret: %v", err)
	// 	}
	// }
	// dbpool, err := pgxpool.Connect(context.Background(), ca.Dsn)
	// if err != nil {
	// 	return fmt.Errorf("while opening db connection: %v", err)
	// }
	// defer dbpool.Close()

	for i := 0; i < ca.NbrChards; i++ {
		err := ca.DoChard(i)
		if err != nil {
			return fmt.Errorf("while generation of chard %d: %v", i, err)
		}
	}

	return nil
}

func (ca *CommandArguments) DoChard(id int) error {
	// Set up the output writer
	// Setup a writer for error file (bad records)
	outFile, err := os.CreateTemp("", "testData")
	if err != nil {
		return fmt.Errorf("while creating temp file: %v", err)
	}
	defer os.Remove(outFile.Name()) // clean up

	// // Get the NDC list
	// ndcList, err := ca.GetNdcList()
	// if err != nil {
	// 	return fmt.Errorf("while getting the NDC list: %v", err)
	// }
	// nbrNdc := len(*ndcList)

	// Test data template info
	header, rowTemplate, err := ca.GetTemplateInfo()
	if err != nil {
		return fmt.Errorf("while getting csv template info: %v", err)
	}
	headers := strings.Split(header, ",")

	// Prepare the parquet schema -- saving rows as string
	parquetSchema := make([]string, len(headers))
	for i := range headers {
		parquetSchema[i] = fmt.Sprintf("name=%s, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY",
			headers[i])
	}

	// open the local temp file for the parquet writer
	fwName := outFile.Name()
	fw := delegate.NewLocalFile(fwName, outFile)

	// Create the parquet writer with the provided schema
	pw, err := writer.NewCSVWriter(parquetSchema, fw, 4)
	if err != nil {
		fw.Close()
		return fmt.Errorf("while opening local parquet csv writer %v", err)
	}

	// Write the rows into the temp file
	for i := 0; i < ca.NbrRowsPerChard; i++ {
		row := make([]interface{}, len(headers))
		mbrId := rand.Intn(ca.NbrMembers)
		mbrKey := fmt.Sprintf("MBR_ID000%d", mbrId)
		fileKey := fmt.Sprintf("FILE_KEY%d", rand.Intn(ca.NbrRawDataFile))
		recordKey := fmt.Sprintf("REC_ID%d_%d", mbrId, rand.Intn(ca.NbrRowPerMembers))
		data := strings.Split(fmt.Sprintf(rowTemplate, fileKey, mbrKey, recordKey), ",")
		for j := range data {
			row[j] = data[j]
		}
		if err = pw.Write(row); err != nil {
			fw.Close()
			return fmt.Errorf("while writing row to local parquet file: %v", err)
		}
	}

	if err = pw.WriteStop(); err != nil {
		fw.Close()
		return fmt.Errorf("while writing parquet stop (trailer): %v", err)
	}
	// fmt.Println("**&@@ WriteParquetPartition: DONE writing local parquet file for fileName:", *ctx.fileName)

	outFilePath := fmt.Sprintf("%s/%s/in-part%05d.parquet", os.Getenv("JETS_s3_INPUT_PREFIX"), ca.OutFileKey, id)
	fmt.Println("OutFile Path:", outFilePath)

	// All good, put the file in s3
	outFile.Seek(0, 0)

	fmt.Println("\nDone generating data, copying to s3...")
	err = awsi.UploadToS3(os.Getenv("JETS_BUCKET"), ca.AwsRegion, outFilePath, outFile)
	if err != nil {
		return fmt.Errorf("while writing csv headers to output file: %v", err)
	}

	return nil
}
