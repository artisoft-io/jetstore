package delegate

import (
	"bufio"
	// "context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/google/uuid"
	// "github.com/jackc/pgx/v4/pgxpool"
)

// The delegate that actually perform the test data generation
// Required Env variable:
// JETS_DSN_SECRET
// JETS_REGION
// JETS_BUCKET
// JETS_s3_INPUT_PREFIX

type CommandArguments struct {
	AwsDsnSecret string
	DbPoolSize int
	UsingSshTunnel bool
	AwsRegion string
	Dsn string
	NdcFilePath string
	OutFileKey string
	CsvTemplatePath string
	NbrBaseClaims int
	NbrMembers int
}

// Support Functions
// --------------------------------------------------------------------------------------
func (ca *CommandArguments) GetTemplateInfo() (string, string, error) {
	readFile, err := os.Open(ca.CsvTemplatePath)
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
	var sep_flag datatable.Chartype
	buf := make([]byte, 2048)
	_, err = readFile.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("error while ready first few bytes of NdcFilePath %s: %v", ca.NdcFilePath, err)
	}
	sep_flag, err = datatable.DetectDelimiter(buf)
	if err != nil {
		return nil, fmt.Errorf("while calling datatable.DetectDelimiter: %v",err)
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
			fmt.Println("Got",len(ndcList),"NDCs")
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

	if ca.NbrBaseClaims < 1 {
		errMsg = append(errMsg, "NbrBaseClaims must be at least 1.")		
	}

	if ca.NbrMembers < 1 {
		errMsg = append(errMsg, "NbrMembers must be at least 1.")		
	}

	fmt.Println("Status Update Arguments:")
	fmt.Println("----------------")
	fmt.Println("Got argument: dsn, len", len(ca.Dsn))
	fmt.Println("Got argument: awsRegion",ca.AwsRegion)
	fmt.Println("Got argument: awsDsnSecret",ca.AwsDsnSecret)
	fmt.Println("Got argument: dbPoolSize",ca.DbPoolSize)
	fmt.Println("Got argument: usingSshTunnel",ca.UsingSshTunnel)
	fmt.Println("Got argument: ndcFilePath",ca.NdcFilePath)
	fmt.Println("Got argument: outFileKey",ca.OutFileKey)
	fmt.Println("Got argument: csvTemplatePath",ca.CsvTemplatePath)
	fmt.Println("Got argument: nbrBaseClaims",ca.NbrBaseClaims)
	fmt.Println("Got argument: nbrMembers",ca.NbrMembers)
	fmt.Printf("ENV JETS_s3_INPUT_PREFIX: %s\n",os.Getenv("JETS_s3_INPUT_PREFIX"))
	fmt.Printf("ENV JETS_BUCKET: %s\n",os.Getenv("JETS_BUCKET"))

	return errMsg
}

func (ca *CommandArguments) CoordinateWork() error {
	// // open db connection
	// var err error
	// if ca.AwsDsnSecret != "" {
	// 	// Get the dsn from the aws secret
	// 	ca.Dsn, err = awsi.GetDsnFromSecret(ca.AwsDsnSecret, ca.AwsRegion, ca.UsingSshTunnel, ca.DbPoolSize)
	// 	if err != nil {
	// 		return fmt.Errorf("while getting dsn from aws secret: %v", err)
	// 	}
	// }
	// dbpool, err := pgxpool.Connect(context.Background(), ca.Dsn)
	// if err != nil {
	// 	return fmt.Errorf("while opening db connection: %v", err)
	// }
	// defer dbpool.Close()

	// Set up the output writer
	// Setup a writer for error file (bad records)
	outFile, err := os.CreateTemp("", "testData")
	if err != nil {
		return fmt.Errorf("while creating temp file: %v", err)
	}
	defer os.Remove(outFile.Name()) // clean up
	outWriter := bufio.NewWriter(outFile)

	// Get the NDC list
	ndcList, err := ca.GetNdcList()
	if err != nil {
		return fmt.Errorf("while getting the NDC list: %v", err)
	}
	nbrNdc := len(*ndcList)

	// Test data template info
	header, rowTemplate, err := ca.GetTemplateInfo()
	if err != nil {
		return fmt.Errorf("while getting csv template info: %v", err)
	}

	// Write the header
	_, err = outWriter.WriteString(header)
	if err != nil {
		return fmt.Errorf("while writing csv headers to output file: %v", err)
	}
	outWriter.WriteRune('\n')
	outFilePath := fmt.Sprintf("%s/%s", os.Getenv("JETS_s3_INPUT_PREFIX"), ca.OutFileKey)
	fmt.Println("OutFile Path:",outFilePath)

	// Generate test data
	for iMbr:=0; iMbr<ca.NbrMembers; iMbr++ {
		baseKey := uuid.New().String()
		fmt.Println(iMbr+1,"of",ca.NbrMembers,"baseKey",baseKey)
		for k1:=11; k1<13; k1++ {
			for k2:=21; k2<23; k2++ {
				for k3:=31; k3<34; k3++ {
					for iClm:=0; iClm<ca.NbrBaseClaims; iClm++ {
						ndc := (*ndcList)[rand.Intn(nbrNdc)]
						_, err = outWriter.WriteString(fmt.Sprintf(rowTemplate, baseKey, k1, k2, k3, ndc))
						if err != nil {
							return fmt.Errorf("while writing test claim row to output file: %v", err)
						}
						outWriter.WriteRune('\n')										
					}
				}
			}
		}
	}

	// All good, put the file in s3
	outWriter.Flush()
	outFile.Seek(0, 0)
	fmt.Println("\nDone generating data, copying to s3...")
	err = awsi.UploadToS3(os.Getenv("JETS_BUCKET"), ca.AwsRegion, outFilePath, outFile)
	if err != nil {
		return fmt.Errorf("while writing csv headers to output file: %v", err)
	}

	return nil
}
