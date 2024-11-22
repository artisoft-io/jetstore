package main

// CGT lambda that register file keys from sqs events

import (
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/aws/aws-lambda-go/lambda"
)

var (
	pgtRequestUrl string = "https://login.cgt.us/cas/serviceValidate"
	pgtUrl        string = "https://fmapi.cgt.us/secure/receptor"
	hostUrl       string = "admin.fm.cgt.us"
	tgtRequestUrl string = "https://login.cgt.us/cas/v1/tickets"
	fmLayoutUrl   string = "https://fmapi.cgt.us/api/getLayoutInfo"
	userName      string = "jetstore"
	password      string
	tgtUrlRe      *regexp.Regexp
	pgtTicketRe   *regexp.Regexp
	jsInput       string = os.Getenv("JETS_s3_SCHEMA_TRIGGERS")
)

func init() {
	b, err := b64.StdEncoding.DecodeString("en1NfDQhSEc0MQ==")
	if err != nil {
		log.Panic(err)
	}
	password = string(b)
	tgtUrlRe = regexp.MustCompile(`action="(.*?)"`)
	pgtTicketRe = regexp.MustCompile(`<cas:proxyGrantingTicket>(.*?)</cas:`)
}

type CgtSqsEvent struct {
	Records []CgtSqsMessage `json:"Records"`
}
type CgtSqsMessage struct {
	MessageId      string `json:"MessageID"`
	SourceLocation string `json:"SourceLocation"`
	FileSize       int64  `json:"FileSize"`
	Timestamp      string `json:"Timestamp"`
	FileType       string `json:"FileType"`
	Layout         string `json:"Layout"`
	Environment    string `json:"Environment"`
	Type           string `json:"Type"`
}

type CgtLayoutResp struct {
	Data            []CgtLayout `json:"data"`
	ResponseCode    int         `json:"responseCode"`
	ResponseMessage string      `json:"responseMessage"`
}
type CgtLayout struct {
	Name       string      `json:"layout_name"`
	DateFormat string      `json:"date_format"`
	FileType   string      `json:"file_type"`
	RecordType string      `json:"input_record_type"`
	Delimiter  string      `json:"Delimiter"`
	HasHeader  int         `json:"has_header"`
	Columns    []CgtColumn `json:"field_info"`
}
type CgtColumn struct {
	Name   string `json:"field_name"`
	Length string `json:"field_length"`
}

func handler(event CgtSqsEvent) error {
	// Get the file mapper token
	pgtIou, err := getPgtIou()
	if err != nil {
		log.Println("Got error while generating PGT IOU token:", err)
		return err
	}

	// Now, process the records
	for _, record := range event.Records {
		err := processMessage(pgtIou, record)
		if err != nil {
			log.Println("Got error while processing record:", err)
			return err
		}
	}
	log.Println("done")
	return nil
}

func processMessage(pgtIou string, record CgtSqsMessage) error {
	// Get the layout info from FM
	log.Printf("Processing record for file %s, of type %s with layout %s, using PGT IOU %s",
		record.SourceLocation, record.FileType, record.Layout, pgtIou)
	// Get the layout info from File Mapper
	// https://fmapi.cgt.us/api/getLayoutInfo?layout=aetna_medical_delimited_176_v01&client=admin&pgtIou=PGTIOU-14277-kVW0FbjNs-2HL5I0txEbM2KFxr7K4Eeb3naacZucboUuc2QitXsj2VTWNoLVOYaDBgo-inst-cas-prod-front04-xed
	requestUrl := fmt.Sprintf("%s?layout=%s&client=admin&pgtIou=%s", fmLayoutUrl, record.Layout, pgtIou)
	layoutResp, err := getFileMapperLayout(requestUrl)
	if err != nil {
		return err
	}
	//** Print the response
	b, _ := json.MarshalIndent(*layoutResp, "", " ")
	log.Println(string(b))

	// Prepare the JetStore Schema Provider Event to put on s3
	layoutInfo := &layoutResp.Data[0]
	filePath := strings.Split(record.SourceLocation, "/")
	l := len(filePath)
	if l < 3 {
		return fmt.Errorf("error: sqs event SourceLocation path is too short: %s", record.SourceLocation)
	}
	bucket := filePath[0]
	fileKey := strings.Join(filePath[1:], "/")
	outPath := slices.Insert(filePath[1:l-1], 1, "blind")
	var inputFormat string
	switch strings.ToLower(layoutInfo.FileType) {
	case "delimited":
		if layoutInfo.HasHeader > 0 {
			inputFormat = "csv"
		} else {
			inputFormat = "headerless_csv"
		}
	case "fixedlength":
		inputFormat = "fixed_width"
	case "parquet":
		inputFormat = "parquet"
	}
	columns := make([]compute_pipes.SchemaColumnSpec, 0, len(layoutInfo.Columns))
	for _, c := range layoutInfo.Columns {
		column := compute_pipes.SchemaColumnSpec{
			Name: c.Name,
		}
		if len(c.Length) > 0 {
			column.Length, err = strconv.Atoi(c.Length)
			if err != nil {
				log.Println("invalid column lenght, ignoring")
				column.Length = 0
			}
		}
		columns = append(columns, column)
	}
	// Translate the File Mapper date format into the go reference date format
	layoutInfo.DateFormat = strings.Replace(layoutInfo.DateFormat, "yyyy", "2006", 1)
	layoutInfo.DateFormat = strings.Replace(layoutInfo.DateFormat, "MM", "01", 1)
	layoutInfo.DateFormat = strings.Replace(layoutInfo.DateFormat, "dd", "02", 1)

	schemaInfo := compute_pipes.SchemaProviderSpec{
		Client:      "CGT",
		Vendor:      "File_Manager",
		ObjectType:  "HealthcareData",
		Bucket:      bucket,
		FileKey:     fileKey,
		FileSize:    record.FileSize,
		FileDate:    record.Timestamp,
		SchemaName:  record.Layout,
		InputFormat: inputFormat,
		Compression: "none",
		Delimiter:   layoutInfo.Delimiter,
		DateFormat:  layoutInfo.DateFormat,
		TrimColumns: true,
		IsPartFiles: false,
		Columns:     columns,
		Env: map[string]string{
			"$CGT_OUT_PATH": strings.Join(outPath, "/"),
		},
	}
	// Write the schema trigger event to jetstore s3
	triggerObj, err := json.Marshal(schemaInfo)
	if err != nil {
		return err
	}
	triggerKey := fmt.Sprintf("%s/%s", jsInput, filePath[1])
	err = awsi.UploadBufToS3(triggerKey, triggerObj)
	return err
}

func main() {
	lambda.Start(handler)
}

func getFileMapperLayout(requestUrl string) (*CgtLayoutResp, error) {
	retry := 0
do_retry:
	resp, err := http.Get(requestUrl)
	if err != nil {
		if retry < 10 {
			log.Printf("File Mapper response with err %v, retrying\n", err)
			time.Sleep(1 * time.Second)
			retry++
			goto do_retry
		}
		return nil, fmt.Errorf("failed go get layout from File Mapper: %v", err)
	}
	if resp.StatusCode != 200 {
		if retry < 10 {
			log.Printf("File Mapper response status code is %d, retrying\n", resp.StatusCode)
			resp.Body.Close()
			time.Sleep(1 * time.Second)
			retry++
			goto do_retry
		}
		return nil, fmt.Errorf("failed go get layout from File Mapper, bad status code: %d", resp.StatusCode)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("while reading the response body from File Mapper: %v", err)
	}
	//** print the response
	log.Println("*** Response from FM:", string(body))

	var layoutResp CgtLayoutResp
	err = json.Unmarshal(body, &layoutResp)
	if err != nil {
		return nil, fmt.Errorf("while unmarshaling JSON from FM: %v", err)
	}
	return &layoutResp, err
}

func getPgtIou() (string, error) {
	// Generate the tgtUrl
	tgtUrl, err := casPostRequest(tgtRequestUrl, map[string]string{
		"username": userName, "password": password, "hostUrl": hostUrl}, 201, tgtUrlRe)
	if err != nil {
		return "", fmt.Errorf("while login request to %s: %v", tgtRequestUrl, err)
	}
	if len(tgtUrl) == 0 {
		return "", fmt.Errorf("error: could not get the TGT URL from the login endpoint %s", tgtRequestUrl)
	}
	// Generate the service ticket
	ticket, err := casPostRequest(tgtUrl, map[string]string{"service": "https://" + hostUrl}, 200, nil)
	if err != nil {
		return "", fmt.Errorf("while getting service ticket from %s: %v", tgtUrl, err)
	}
	if len(ticket) == 0 {
		return "", fmt.Errorf("error: could not get the service ticket from endpoint %s", tgtUrl)
	}
	// Generate the PGT IOU
	pgtIou, err := casPostRequest(pgtRequestUrl, map[string]string{
		"service": "https://" + hostUrl, "pgtUrl": pgtUrl, "ticket": ticket}, 200, pgtTicketRe)
	if err != nil {
		return "", fmt.Errorf("while getting pgtIou from %s: %v", tgtUrl, err)
	}
	if len(pgtIou) == 0 {
		return "", fmt.Errorf("error: could not get the pgt Iou from endpoint %s", pgtRequestUrl)
	}
	return pgtIou, nil
}

func casPostRequest(requestUrl string, body map[string]string, validStatusCode int, resultRe *regexp.Regexp) (string, error) {
  bodyData := url.Values{}
  for k, v := range body {
    bodyData.Set(k, v)
  }
	result, err := postRequestWithRetry(requestUrl, bodyData, 10, validStatusCode)
	if err != nil {
		return "", fmt.Errorf("while login request to %s: %v", requestUrl, err)
	}
	if resultRe == nil {
		return result, nil
	}
	matches := resultRe.FindStringSubmatch(result)
	if len(matches) > 1 {
		return matches[1], err
	}
	return "", fmt.Errorf("error: could not get the data from the endpoint %s", requestUrl)
}

func postRequestWithRetry(apiEndpoint string, body url.Values, maxRetry int, validStatusCode int) (string, error) {
	retry := 0
do_retry:
	result, err := postRequest(apiEndpoint, body, validStatusCode)
	if err != nil {
		if retry < maxRetry {
			time.Sleep(1 * time.Second)
			retry++
			goto do_retry
		}
		return "", fmt.Errorf("failed to post to %s: %v", apiEndpoint, err)
	}
	return result, nil
}

func postRequest(apiEndpoint string, body url.Values, validStatusCode int) (string, error) {
	client := &http.Client{}
	resp, err := client.PostForm(apiEndpoint, body)
	if err != nil {
		err = fmt.Errorf("while posting request to %s: %v", apiEndpoint, err)
		log.Println(err)
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != validStatusCode {
		err = fmt.Errorf("while posting request to %s: invalid returned status code: %d", apiEndpoint, resp.StatusCode)
		log.Println(err)
		return "", err
	}
	bodyResp, err := io.ReadAll(resp.Body)
	return string(bodyResp), err
}
