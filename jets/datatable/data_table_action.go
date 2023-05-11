package datatable

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"io"

	// "io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	// "time"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/artisoft-io/jetstore/jets/user"

	// lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Environment and settings needed
type Context struct {
	Dbpool         *pgxpool.Pool
	DevMode        bool
	UsingSshTunnel bool
	UnitTestDir    *string
	NbrShards      int
	AdminEmail     *string
}

func NewContext(dbpool *pgxpool.Pool, devMode bool, usingSshTunnel bool,
	unitTestDir *string, nbrShards int,
	adminEmail *string) *Context {
	return &Context{
		Dbpool:         dbpool,
		DevMode:        devMode,
		UsingSshTunnel: usingSshTunnel,
		UnitTestDir:    unitTestDir,
		NbrShards:      nbrShards,
		AdminEmail:     adminEmail,
	}
}

// sql access builder
type DataTableAction struct {
	Action        string                   `json:"action"`
	RawQuery      string                   `json:"query"`
	RawQueryMap   map[string]string        `json:"query_map"`
	Columns       []Column                 `json:"columns"`
	FromClauses   []FromClause             `json:"fromClauses"`
	WhereClauses  []WhereClause            `json:"whereClauses"`
	SortColumn    string                   `json:"sortColumn"`
	SortAscending bool                     `json:"sortAscending"`
	Offset        int                      `json:"offset"`
	Limit         int                      `json:"limit"`
	Data          []map[string]interface{} `json:"data"`
}
type Column struct {
	Table string    `json:"table"`
	Column string   `json:"column"`
}
type FromClause struct {
	Schema  string `json:"schema"`
	Table   string `json:"table"`
}
type WhereClause struct {
	Table string    `json:"table"`
	Column string   `json:"column"`
	Values []string `json:"values"`
	JoinWith string `json:"joinWith"`
}
// DataTableColumnDef used when returning the column definition
// obtained from db catalog
type DataTableColumnDef struct {
	Index     int    `json:"index"`
	Name      string `json:"name"`
	Label     string `json:"label"`
	Tooltips  string `json:"tooltips"`
	IsNumeric bool   `json:"isnumeric"`
}

func (dtq *DataTableAction) makeSelectColumns() string {
	if len(dtq.Columns) == 0 {
		return "SELECT *"
	}
	var buf strings.Builder
	buf.WriteString("SELECT ")
	isFirst := true
	for i := range dtq.Columns {
		if !isFirst {
			buf.WriteString(", ")
		}
		isFirst = false
		if dtq.Columns[i].Table != "" {
			buf.WriteString(pgx.Identifier{dtq.Columns[i].Table, dtq.Columns[i].Column}.Sanitize())
		} else {
			buf.WriteString(pgx.Identifier{dtq.Columns[i].Column}.Sanitize())
		}
	}
	return buf.String()
}

func (dtq *DataTableAction) makeFromClauses() string {
	var buf strings.Builder
	isFirst := true
	for i := range dtq.FromClauses {
		if !isFirst {
			buf.WriteString(", ")
		}
		isFirst = false
		if dtq.FromClauses[i].Schema != "" {
			buf.WriteString(pgx.Identifier{dtq.FromClauses[i].Schema, dtq.FromClauses[i].Table}.Sanitize())
		} else {
			buf.WriteString(pgx.Identifier{dtq.FromClauses[i].Table}.Sanitize())
		}
	}
	return buf.String()
}

// var pgArrayRe *regexp.Regexp
// func init() {
// 	pgArrayRe = regexp.MustCompile(`\d+`)

// }
func (dtq *DataTableAction) makeWhereClause() string {
	if len(dtq.WhereClauses) == 0 {
		return ""
	}
	var buf strings.Builder
	buf.WriteString(" WHERE ")
	isFirst := true
	for i := range dtq.WhereClauses {
		if !isFirst {
			buf.WriteString(" AND ")
		}
		isFirst = false
		if dtq.WhereClauses[i].Table != "" {
			buf.WriteString(pgx.Identifier{dtq.WhereClauses[i].Table, dtq.WhereClauses[i].Column}.Sanitize())
		} else {
			buf.WriteString(pgx.Identifier{dtq.WhereClauses[i].Column}.Sanitize())
		}
		nvalues := len(dtq.WhereClauses[i].Values)
		// Check if value contains an pg array encoded into a string
		if nvalues == 1 {
			if dtq.WhereClauses[i].Values[0] == "{}" {
				dtq.WhereClauses[i].Values[0] = "NULL"
			} else {
				v := dtq.WhereClauses[i].Values[0]
				if strings.HasPrefix(v, "{") && strings.HasSuffix(v, "}") {
					dtq.WhereClauses[i].Values = strings.Split(v[1:len(v)-1], ",")
					nvalues = len(dtq.WhereClauses[i].Values)
				}
			}
		}
		switch {
		case len(dtq.WhereClauses[i].JoinWith) > 0:
			buf.WriteString(" = ")
			buf.WriteString(dtq.WhereClauses[i].JoinWith)
		case nvalues > 1:
			buf.WriteString(" IN (")
			isFirstValue := true
			for j := range dtq.WhereClauses[i].Values {
				if !isFirstValue {
					buf.WriteString(", ")
				}
				isFirstValue = false
				value := dtq.WhereClauses[i].Values[j]
				if value == "NULL" {
					buf.WriteString(" NULL ")
				} else {
					buf.WriteString("'")
					buf.WriteString(value)
					buf.WriteString("'")
				}
			}
			buf.WriteString(") ")
		default:
			value := dtq.WhereClauses[i].Values[0]
			if value == "NULL" {
				buf.WriteString(" is NULL ")
			} else {
				buf.WriteString(" = '")
				buf.WriteString(value)
				buf.WriteString("'")
			}
		}
	}
	return buf.String()
}

// Simple definition of sql statement for insert
type SqlInsertDefinition struct {
	Stmt       string
	ColumnKeys []string
	AdminOnly  bool
}

func isNumeric(dtype string) bool {
	switch dtype {
	case "int", "long", "uint", "ulong", "double":
		return true
	default:
		return false
	}
}

// var tableSchemaCache *lru.Cache
// func init() {
// 	var err error
// 	tableSchemaCache, err = lru.NewWithEvict(128, func(key, value interface{}) {log.Printf("Cache evicting item with key %v", key)})
// 	if err != nil {
// 		log.Fatal("error: cannot create cache")
// 	}
// }
// func (dataTableAction *DataTableAction) getKey() string {
// 	return dataTableAction.Schema+"_"+dataTableAction.FromClauses[0].Table
// }

// ExecRawQuery ------------------------------------------------------
// These are queries to load reference data for widget, e.g. dropdown list of items
func (ctx *Context) ExecRawQuery(dataTableAction *DataTableAction) (results *map[string]interface{}, httpStatus int, err error) {
	// // Check if we're in dev mode and the query is delegated to a proxy implementation
	// if ctx.DevMode && len(*ctx.unitTestDir) > 0 {
	// 	// We're in dev mode, see if we override the table being queried
	// 	switch {
	// 	case strings.Contains(dataTableAction.RawQuery, "file_key_staging"):
	// 		return ctx.readLocalFiles(dataTableAction)
	// 	}
	// }
	resultRows, err2 := execQuery(ctx.Dbpool, dataTableAction, &dataTableAction.RawQuery)
	if err2 != nil {
		httpStatus = http.StatusInternalServerError
		err = fmt.Errorf("while executing raw query: %v", err2)
		return
	}

	results = &map[string]interface{}{
		"rows": resultRows,
	}
	httpStatus = http.StatusOK
	return
}

// ExecRawQueryMap ------------------------------------------------------
// These are queries to load reference data for widget, e.g. dropdown list of items
func (ctx *Context) ExecRawQueryMap(dataTableAction *DataTableAction) (results *map[string]interface{}, httpStatus int, err error) {
	// fmt.Println("ExecRawQueryMap:")
	resultMap := make(map[string]interface{}, len(dataTableAction.RawQueryMap))
	for k, v := range dataTableAction.RawQueryMap {
		// fmt.Println("Query:",v)
		resultRows, err2 := execQuery(ctx.Dbpool, dataTableAction, &v)
		if err2 != nil {
			httpStatus = http.StatusInternalServerError
			err = fmt.Errorf("while executing raw query: %v", err2)
			return
		}
		resultMap[k] = resultRows
	}
	results = &map[string]interface{}{
		"result_map": resultMap,
	}
	httpStatus = http.StatusOK
	return
}

type Chartype rune
// Single character type for csv options
func (s *Chartype) String() string {
	return fmt.Sprintf("%#U", *s)
}

func (s *Chartype) Set(value string) error {
	r := []rune(value)
	if len(r) > 1 || r[0] == '\n' {
		return errors.New("sep must be a single char not '\\n'")
	}
	*s = Chartype(r[0])
	return nil
}

func DetectDelimiter(buf []byte) (sep_flag Chartype, err error) {
	// auto detect the separator based on the first line
	nb := len(buf)
	if nb > 2048 {
		nb = 2048
	}
	txt := string(buf[0:nb])
	cn := strings.Count(txt, ",")
	pn := strings.Count(txt, "|")
	tn := strings.Count(txt, "\t")
	td := strings.Count(txt, "~")
	switch {
	case (cn > pn) && (cn > tn) && (cn > td):
		sep_flag = ','
	case (pn > cn) && (pn > tn) && (pn > td):
		sep_flag = '|'
	case (tn > cn) && (tn > pn) && (tn > td):
		sep_flag = '\t'
	case (td > cn) && (td > pn) && (td > tn):
		sep_flag = '~'
	default:
		return 0, fmt.Errorf("error: cannot determine the csv-delimit used in buf")
	}
	return
}


// InsertRawRows ------------------------------------------------------
// Insert row function using a raw text buffer containing cst/tsv rows
// Delegates to InsertRows
func (ctx *Context) InsertRawRows(dataTableAction *DataTableAction, token string) (results *map[string]interface{}, httpStatus int, err error) {
	httpStatus = http.StatusOK
	// Copy Data so to re-use dataTableAction with different sets of Data
	requestTable := dataTableAction.FromClauses[0].Table
	inData := &dataTableAction.Data
	for irow := range *inData {

		buf := (*inData)[irow]["raw_rows"]
		userEmail := (*inData)[irow]["user_email"];
		if buf == nil || userEmail == nil {
			log.Printf("Error request is missing raw_rows or user_email from request with Table %s", requestTable)
			httpStatus = http.StatusInternalServerError
			err = errors.New("error while reading raw_rows from request")
			return
		}
		var byteBuf []byte
		switch bb := buf.(type) {
		case string:
			// Got raw_rows -- convert to list of rows
			byteBuf = []byte(bb)
		case []byte:
			byteBuf = bb
		default:
			log.Printf("Error raw_rows is invalid type")
			httpStatus = http.StatusInternalServerError
			err = errors.New("error while reading raw_rows from request")
			return
		}
		// byteBuf as the raw_rows
		var sepFlag Chartype
		sepFlag, err = DetectDelimiter(byteBuf)
		if err != nil {
			log.Printf("Error while detecting delimiters for raw_rows: %v",err)
			httpStatus = http.StatusBadRequest
			err = errors.New("error while detecting delimiters for raw_rows")
			return
		}
		r := csv.NewReader(bytes.NewReader(byteBuf))
		r.Comma = rune(sepFlag)
		// r.ReuseRecord = true
		headers, err2 := r.Read()
		if err2 == io.EOF {
			log.Printf("Error raw_rows contain no data")
			httpStatus = http.StatusBadRequest
			err = errors.New("error, raw_rows from request contain no data")
			return
		}
		// Put the parsed row as elm back in dataTableAction.Data
		dataTableAction.Data = make([]map[string]interface{}, 0)
		for {
			record, err2 := r.Read()
			if err2 == io.EOF {
				break
			}
			if err2 != nil {
				log.Printf("Error parsing raw_rows: %v",err2)
				httpStatus = http.StatusBadRequest
				err = fmt.Errorf("error while parsing raw_rows: %v", err2)
				return
			}
			row := make(map[string]interface{})
			for i := range headers {
				if record[i] == "" {
					row[headers[i]] = nil
				} else {
					row[headers[i]] = record[i]
				}
			}
			row["user_email"] = userEmail
			dataTableAction.Data = append(dataTableAction.Data, row)
		}
		if len(dataTableAction.Data) == 0 {
			log.Printf("Error raw_rows contain no data (2)")
			httpStatus = http.StatusBadRequest
			err = errors.New("error, raw_rows from request contain no data")
			return
		}

		// Pre-Processing hook
		switch requestTable {
		case "raw_rows/process_mapping":
			// Put the table name in each row
			var tableName string
			client := dataTableAction.Data[0]["client"]
			org := dataTableAction.Data[0]["org"]
			objectType := dataTableAction.Data[0]["object_type"]
			if client != nil && objectType != nil {
				if org == nil || org == "" {
					tableName = fmt.Sprintf("%s_%s",client, objectType)
				} else {
					tableName = fmt.Sprintf("%s_%s_%s",client, org, objectType)
				}
				if tableName != "" {
					for irow := range dataTableAction.Data {
						dataTableAction.Data[irow]["table_name"] = tableName
					}
				}	
			}
			if tableName == "" {
				tableName = dataTableAction.Data[0]["table_name"].(string)
			}
			// Remove existing rows in database
			stmt :=  `DELETE FROM jetsapi.process_mapping 
			WHERE table_name = $1`
			_, err = ctx.Dbpool.Exec(context.Background(), stmt, tableName)
			if err != nil {
				log.Printf("Error while deleting from process_mapping: %v",err)
				httpStatus = http.StatusBadRequest
				return
			}		
			dataTableAction.FromClauses[0].Table = "process_mapping"
		}
		// send it through InsertRow
		results, httpStatus, err = ctx.InsertRows(dataTableAction, token)
		if err != nil {
			log.Printf("while calling InsertRows: %v", err)
			return
		}
	}
	return
}

// InsertRows ------------------------------------------------------
// Main insert row function with pre processing hooks for validating/authorizing the request
// Main insert row function with post processing hooks for starting pipelines
// Inserting rows using pre-defined sql statements, keyed by table name provided in dataTableAction
func (ctx *Context) InsertRows(dataTableAction *DataTableAction, token string) (results *map[string]interface{}, httpStatus int, err error) {
	returnedKey := make([]int, len(dataTableAction.Data))
	var loaderCompletedMetric, loaderFailedMetric, serverCompletedMetric, serverFailedMetric string
	httpStatus = http.StatusOK
	sqlStmt, ok := sqlInsertStmts[dataTableAction.FromClauses[0].Table]
	if !ok {
		httpStatus = http.StatusBadRequest
		err = errors.New("error: unknown table")
		return
	}
	// Check if stmt is reserved for admin only
	if sqlStmt.AdminOnly {
		userEmail, err2 := user.ExtractTokenID(token)
		if err2 != nil || userEmail != *ctx.AdminEmail {
			httpStatus = http.StatusUnauthorized
			err = errors.New("error: unauthorized, only admin can delete users")
			return
		}
	}
	row := make([]interface{}, len(sqlStmt.ColumnKeys))
	for irow := range dataTableAction.Data {
		// Pre-Processing hook
		switch {
		case strings.HasSuffix(dataTableAction.FromClauses[0].Table, "pipeline_execution_status"):
			if dataTableAction.Data[irow]["input_session_id"] == nil {
				inSessionId := dataTableAction.Data[irow]["session_id"]
				inputRegistryKey := dataTableAction.Data[irow]["main_input_registry_key"]
				if inputRegistryKey != nil {
					stmt := "SELECT session_id FROM jetsapi.input_registry WHERE key = $1"
					err = ctx.Dbpool.QueryRow(context.Background(), stmt, inputRegistryKey).Scan(&inSessionId)
					if err != nil {
						log.Printf("While getting session_id from input_registry table %s: %v", dataTableAction.FromClauses[0].Table, err)
						httpStatus = http.StatusInternalServerError
						err = errors.New("error while reading from a table")
						return
					}
				}
				dataTableAction.Data[irow]["input_session_id"] = inSessionId
			}

		case strings.HasPrefix(dataTableAction.FromClauses[0].Table, "WORKSPACE/"):
			sqlStmt.Stmt = strings.ReplaceAll(sqlStmt.Stmt, "$SCHEMA", dataTableAction.FromClauses[0].Schema)
		}
		for jcol, colKey := range sqlStmt.ColumnKeys {
			row[jcol] = dataTableAction.Data[irow][colKey]
		}

		// fmt.Printf("Insert Row with stmt %s\n", sqlStmt.Stmt)
		// fmt.Printf("Insert Row on table %s: %v\n", dataTableAction.FromClauses[0].Table, row)
		// Executing the InserRow Stmt
		if strings.Contains(sqlStmt.Stmt, "RETURNING key") {
			err = ctx.Dbpool.QueryRow(context.Background(), sqlStmt.Stmt, row...).Scan(&returnedKey[irow])
		} else {
			_, err = ctx.Dbpool.Exec(context.Background(), sqlStmt.Stmt, row...)
		}
		if err != nil {
			log.Printf("While inserting in table %s: %v", dataTableAction.FromClauses[0].Table, err)
			if strings.Contains(err.Error(), "duplicate key value") {
				httpStatus = http.StatusConflict
				err = errors.New("duplicate key value")
				return	
			} else {
				httpStatus = http.StatusInternalServerError
				err = errors.New("error while inserting into a table")
			}
		}
	}
	// Post Processing Hook
	var name string
	switch dataTableAction.FromClauses[0].Table {
	case "input_loader_status":
		// Run the loader
		row := make(map[string]interface{}, len(sqlStmt.ColumnKeys))
		for irow := range dataTableAction.Data {
			for _, colKey := range sqlStmt.ColumnKeys {
				v := dataTableAction.Data[irow][colKey]
				if v != nil {
					switch vv := v.(type) {
					case string:
						row[colKey] = vv
					case int:
						row[colKey] = strconv.Itoa(vv)
					}
				}
			}
			// extract the columns we need for the loader
			objType := row["object_type"]
			client := row["client"]
			clientOrg := row["org"]
			sourcePeriodKey := row["source_period_key"]
			fileKey := row["file_key"]
			sessionId := row["session_id"]
			userEmail := row["user_email"]
			v := dataTableAction.Data[irow]["loaderFailedMetric"]
			if v != nil {
				loaderFailedMetric = v.(string)
			}
			v = dataTableAction.Data[irow]["loaderCompletedMetric"]
			if v != nil {
				loaderCompletedMetric = v.(string)
			}
			if objType == nil || client == nil || fileKey == nil || sessionId == nil || userEmail == nil {
				log.Printf(
					"error while preparing to run loader: unexpected nil among: objType: %v, client: %v, fileKey: %v, sessionId: %v, userEmail %v",
					objType, client, fileKey, sessionId, userEmail)
				httpStatus = http.StatusInternalServerError
				err = errors.New("error while running loader command")
				return
			}
			loaderCommand := []string{
				"-in_file", fileKey.(string),
				"-client", client.(string),
				"-org", clientOrg.(string),
				"-objectType", objType.(string),
				"-sourcePeriodKey", sourcePeriodKey.(string),
				"-sessionId", sessionId.(string),
				"-userEmail", userEmail.(string),
				"-nbrShards", strconv.Itoa(ctx.NbrShards),
			}
			if loaderCompletedMetric != "" {
				loaderCommand = append(loaderCommand, "-loaderCompletedMetric")
				loaderCommand = append(loaderCommand, loaderCompletedMetric)
			}
			if loaderFailedMetric != "" {
				loaderCommand = append(loaderCommand, "-loaderFailedMetric")
				loaderCommand = append(loaderCommand, loaderFailedMetric)
			}
			var reportName string
			if clientOrg.(string) != "" {
				reportName = fmt.Sprintf("loader/client=%s/object_type=%s/org=%s", client.(string), objType.(string), clientOrg.(string))
			} else {
				reportName = fmt.Sprintf("loader/client=%s/object_type=%s", client.(string), objType.(string))
			}
			runReportsCommand := []string{
				"-client", client.(string),
				"-sessionId", sessionId.(string),
				"-reportName", reportName,
				"-filePath", strings.Replace(fileKey.(string), os.Getenv("JETS_s3_INPUT_PREFIX"), os.Getenv("JETS_s3_OUTPUT_PREFIX"), 1),
			}
			switch {
			// Call loader synchronously
			case ctx.DevMode:
				if ctx.UsingSshTunnel {
					loaderCommand = append(loaderCommand, "-usingSshTunnel")
					runReportsCommand = append(runReportsCommand, "-usingSshTunnel")
				}
				// Call loader synchronously
				cmd := exec.Command("/usr/local/bin/loader", loaderCommand...)
				var b1 bytes.Buffer
				cmd.Stdout = &b1
				cmd.Stderr = &b1
				log.Printf("Executing loader command '%v'", loaderCommand)
				err = cmd.Run()
				if err != nil {
					log.Printf("while executing loader command '%v': %v", loaderCommand, err)
					log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
					log.Println("LOADER CAPTURED OUTPUT BEGIN")
					log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
					b1.WriteTo(os.Stdout)
					log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
					log.Println("LOADER CAPTURED OUTPUT END")
					log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
					httpStatus = http.StatusInternalServerError
					err = errors.New("error while running loader command")
					return
				}
				log.Println("============================")
				log.Println("LOADER CAPTURED OUTPUT BEGIN")
				log.Println("============================")
				b1.WriteTo(os.Stdout)
				log.Println("============================")
				log.Println("LOADER CAPTURED OUTPUT END")
				log.Println("============================")

				// Call run_report synchronously
				cmd = exec.Command("/usr/local/bin/run_reports", runReportsCommand...)
				var b2 bytes.Buffer
				cmd.Stdout = &b2
				cmd.Stderr = &b2
				log.Printf("Executing run_reports command '%v'", runReportsCommand)
				err = cmd.Run()
				if err != nil {
					log.Printf("while executing run_reports command '%v': %v", runReportsCommand, err)
					log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
					log.Println("REPORTS CAPTURED OUTPUT BEGIN")
					log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
					b2.WriteTo(os.Stdout)
					log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
					log.Println("REPORTS CAPTURED OUTPUT END")
					log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
					httpStatus = http.StatusInternalServerError
					err = errors.New("error while running run_reports command")
					return
				}
				log.Println("============================")
				log.Println("REPORTS CAPTURED OUTPUT BEGIN")
				log.Println("============================")
				b2.WriteTo(os.Stdout)
				log.Println("============================")
				log.Println("REPORTS CAPTURED OUTPUT END")
				log.Println("============================")

			default:
				// StartExecution load file
				log.Printf("calling StartExecution loaderSM loaderCommand: %s", loaderCommand)
				name, err = awsi.StartExecution(os.Getenv("JETS_LOADER_SM_ARN"), 
					map[string]interface{}{
						"loaderCommand": loaderCommand,
						"reportsCommand": runReportsCommand,
					}, sessionId.(string))
				if err != nil {
					log.Printf("while calling StartExecution '%v': %v", loaderCommand, err)
					httpStatus = http.StatusInternalServerError
					err = errors.New("error while calling StartExecution")
					return
				}
				fmt.Println("Loader State Machine", name, "started")
			}
		}
	case "pipeline_execution_status", "short/pipeline_execution_status":
		// Need to get:
		//	- DevMode: run_report_only, run_server_only, run_server_reports
		//  - State Machine URI: serverSM, or reportsSM
		// from process_config table
		// ----------------------------
		var devModeCode, stateMachineName string
		stmt := "SELECT devmode_code, state_machine_name FROM jetsapi.process_config WHERE process_name = $1"
		err = ctx.Dbpool.QueryRow(context.Background(), stmt, dataTableAction.Data[0]["process_name"]).Scan(&devModeCode, &stateMachineName)
		if err != nil {
			log.Printf("While getting devModeCode, stateMachineName from process_config: %v", err)
			httpStatus = http.StatusInternalServerError
			err = errors.New("error while reading from a table")
			return
		}

		// Run the server -- prepare the command line arguments
		row := make(map[string]interface{}, len(sqlStmt.ColumnKeys))
		for irow := range dataTableAction.Data {
			// returnedKey is the key of the row inserted in the db, here it correspond to peKey
			if returnedKey[irow] <= 0 {
				log.Printf(
					"error while preparing to run server/argo: unexpected value for returnedKey from insert to pipeline_execution_status table: %v", returnedKey)
				httpStatus = http.StatusInternalServerError
				err = errors.New("error while preparing server command")
				return
			}
			for _, colKey := range sqlStmt.ColumnKeys {
				v := dataTableAction.Data[irow][colKey]
				if v != nil {
					switch vv := v.(type) {
					case string:
						row[colKey] = vv
					case int:
						row[colKey] = strconv.Itoa(vv)
					}
				}
			}
			peKey := strconv.Itoa(returnedKey[irow])
			processName := row["process_name"]
			//* TODO We should lookup main_input_file_key rather than file_key here
			fileKey := dataTableAction.Data[irow]["file_key"]
			sessionId := row["session_id"]
			userEmail := row["user_email"]
			v := dataTableAction.Data[irow]["serverFailedMetric"]
			if v != nil {
				serverFailedMetric = v.(string)
			}
			v = dataTableAction.Data[irow]["serverCompletedMetric"]
			if v != nil {
				serverCompletedMetric = v.(string)
			}
			// At minimum check userEmail and sessionId (although the last one is not strictly required since it's in the peKey records)
			if userEmail == nil || sessionId == nil {
				log.Printf(
					"error while preparing to run server: unexpected nil among: userEmail %v, sessionId %v", userEmail, sessionId)
				httpStatus = http.StatusInternalServerError
				err = errors.New("error while preparing argo/server command")
				return
			}
			runReportsCommand := []string{
				"-processName", processName.(string),
				"-sessionId", sessionId.(string),
				"-filePath", strings.Replace(fileKey.(string), os.Getenv("JETS_s3_INPUT_PREFIX"), os.Getenv("JETS_s3_OUTPUT_PREFIX"), 1),
			}
			switch {
			// Call server synchronously
			case ctx.DevMode:
				if devModeCode == "run_server_only" || devModeCode == "run_server_reports" {
					// DevMode: Lock session id & register run on last shard (unless error)
					// loop over every chard to exec in succession
					for shardId := 0; shardId < ctx.NbrShards; shardId++ {
						serverArgs := []string{
							"-peKey", peKey,
							"-userEmail", userEmail.(string),
							"-shardId", strconv.Itoa(shardId),
							"-nbrShards", strconv.Itoa(ctx.NbrShards),
						}
						if serverCompletedMetric != "" {
							serverArgs = append(serverArgs, "-serverCompletedMetric")
							serverArgs = append(serverArgs, serverCompletedMetric)
						}
						if serverFailedMetric != "" {
							serverArgs = append(serverArgs, "-serverFailedMetric")
							serverArgs = append(serverArgs, serverFailedMetric)
						}
						if ctx.UsingSshTunnel {
							serverArgs = append(serverArgs, "-usingSshTunnel")
						}
						if shardId < ctx.NbrShards-1 {
							serverArgs = append(serverArgs, "-doNotLockSessionId")
						}
						log.Printf("Run server: %s", serverArgs)
						cmd := exec.Command("/usr/local/bin/server", serverArgs...)
						var b bytes.Buffer
						cmd.Stdout = &b
						cmd.Stderr = &b
						log.Printf("Executing server command '%v'", serverArgs)
						err = cmd.Run()
						if err != nil {
							log.Printf("while executing server command '%v': %v", serverArgs, err)
							log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
							log.Println("SERVER CAPTURED OUTPUT BEGIN")
							log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
							b.WriteTo(os.Stdout)
							log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
							log.Println("SERVER CAPTURED OUTPUT END")
							log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
							httpStatus = http.StatusInternalServerError
							err = errors.New("error while running server command")
							return
						}
						log.Println("============================")
						log.Println("SERVER CAPTURED OUTPUT BEGIN")
						log.Println("============================")
						b.WriteTo(os.Stdout)
						log.Println("============================")
						log.Println("SERVER CAPTURED OUTPUT END")
						log.Println("============================")
					}
				}

				if devModeCode == "run_reports_only" || devModeCode == "run_server_reports" {
					// Call run_report synchronously
					if ctx.UsingSshTunnel {
						runReportsCommand = append(runReportsCommand, "-usingSshTunnel")
					}
					cmd := exec.Command("/usr/local/bin/run_reports", runReportsCommand...)
					var b2 bytes.Buffer
					cmd.Stdout = &b2
					cmd.Stderr = &b2
					log.Printf("Executing run_reports command '%v'", runReportsCommand)
					err = cmd.Run()
					if err != nil {
						log.Printf("while executing run_reports command '%v': %v", runReportsCommand, err)
						log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
						log.Println("REPORTS CAPTURED OUTPUT BEGIN")
						log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
						b2.WriteTo(os.Stdout)
						log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
						log.Println("REPORTS CAPTURED OUTPUT END")
						log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
						httpStatus = http.StatusInternalServerError
						err = errors.New("error while running run_reports command")
						return
					}
					log.Println("============================")
					log.Println("REPORTS CAPTURED OUTPUT BEGIN")
					log.Println("============================")
					b2.WriteTo(os.Stdout)
					log.Println("============================")
					log.Println("REPORTS CAPTURED OUTPUT END")
					log.Println("============================")
				}

			default:
				// Invoke states to execute a process
				// Rules Server arguments
				serverCommands := make([][]string, 0)
				for shardId := 0; shardId < ctx.NbrShards; shardId++ {
					serverArgs := []string{
						"-peKey", peKey,
						"-userEmail", userEmail.(string),
						"-shardId", strconv.Itoa(shardId),
						"-nbrShards", strconv.Itoa(ctx.NbrShards),
						"-doNotLockSessionId",
					}
					if serverCompletedMetric != "" {
						serverArgs = append(serverArgs, "-serverCompletedMetric")
						serverArgs = append(serverArgs, serverCompletedMetric)
					}
					if serverFailedMetric != "" {
						serverArgs = append(serverArgs, "-serverFailedMetric")
						serverArgs = append(serverArgs, serverFailedMetric)
					}
					serverCommands = append(serverCommands, serverArgs)
				}
				smInput := map[string]interface{}{
					"serverCommands": serverCommands,
					"reportsCommand": runReportsCommand,
					"successUpdate": []string{
						"-peKey", peKey,
						"-status", "completed",
					},
					"errorUpdate": []string{
						"-peKey", peKey,
						"-status", "failed",
					},
				}
				processArn := strings.TrimSuffix(os.Getenv("JETS_SERVER_SM_ARN"), "serverSM")
				processArn += stateMachineName 

				// StartExecution execute rule
				log.Printf("calling StartExecution on processArn: %s", processArn)
				log.Printf("calling StartExecution with: %s", smInput)
				name, err = awsi.StartExecution(processArn, smInput, sessionId.(string))
				if err != nil {
					log.Printf("while calling StartExecution on processUrn '%s': %v", processArn, err)
					httpStatus = http.StatusInternalServerError
					err = errors.New("error while calling StartExecution")
					return
				}
				fmt.Println("Server State Machine", name, "started")
			}
		} // irow := range dataTableAction.Data
	} // switch dataTableAction.FromClauses[0].Table
	results = &map[string]interface{}{
		"returned_keys": &returnedKey,
	}
	return
}

// utility method
func execQuery(dbpool *pgxpool.Pool, dataTableAction *DataTableAction, query *string) (*[][]interface{}, error) {
	// //DEV
	// fmt.Println("*** UI Query:", *query)
	resultRows := make([][]interface{}, 0, dataTableAction.Limit)
	rows, err := dbpool.Query(context.Background(), *query)
	if err != nil {
		log.Printf("While executing dataTable query: %v", err)
		return &resultRows, err
	}
	defer rows.Close()
	nCol := len(rows.FieldDescriptions())
	for rows.Next() {
		dataRow := make([]interface{}, nCol)
		for i := 0; i < nCol; i++ {
			dataRow[i] = &sql.NullString{}
		}
		// scan the row
		if err = rows.Scan(dataRow...); err != nil {
			log.Printf("While scanning the row: %v", err)
			return &resultRows, err
		}
		flatRow := make([]interface{}, nCol)
		for i := 0; i < nCol; i++ {
			ns := dataRow[i].(*sql.NullString)
			if ns.Valid {
				flatRow[i] = ns.String
			} else {
				flatRow[i] = nil
			}
		}
		resultRows = append(resultRows, flatRow)
	}
	return &resultRows, nil
}

// DoReadAction ------------------------------------------------------
func (ctx *Context) DoReadAction(dataTableAction *DataTableAction) (*map[string]interface{}, int, error) {

	// // Check if we're in dev mode and the query is delegated to a proxy implementation
	// if ctx.DevMode && len(*ctx.unitTestDir) > 0 {
	// 	// We're in dev mode, see if we override the table being queried
	// 	switch dataTableAction.FromClauses[0].Table {
	// 	case "file_key_staging":
	// 		return ctx.readLocalFiles(dataTableAction)
	// 	}
	// }

	// to package up the result
	results := make(map[string]interface{})

	var columnsDef []DataTableColumnDef
	if len(dataTableAction.Columns) == 0 {
		// Get table column definition
		//* TODO use cache
		tableSchema, err := schema.GetTableSchema(ctx.Dbpool, dataTableAction.FromClauses[0].Schema, dataTableAction.FromClauses[0].Table)
		if err != nil {
			return nil, http.StatusInternalServerError, fmt.Errorf("While schema.GetTableSchema for %s.%s: %v", dataTableAction.FromClauses[0].Schema, dataTableAction.FromClauses[0].Table, err)
		}
		columnsDef = make([]DataTableColumnDef, 0, len(tableSchema.Columns))
		for _, colDef := range tableSchema.Columns {
			columnsDef = append(columnsDef, DataTableColumnDef{
				Name:      colDef.ColumnName,
				Label:     colDef.ColumnName,
				Tooltips:  colDef.ColumnName,
				IsNumeric: isNumeric(colDef.DataType)})
			dataTableAction.Columns = append(dataTableAction.Columns, Column{Column: colDef.ColumnName})
		}
		sort.Slice(columnsDef, func(l, r int) bool { return columnsDef[l].Name < columnsDef[r].Name })
		// need to reset the column index due to the sort
		for i := range columnsDef {
			columnsDef[i].Index = i
		}
		dataTableAction.Columns = make([]Column, 0, len(tableSchema.Columns))
		for i := range columnsDef {
			dataTableAction.Columns = append(dataTableAction.Columns, Column{Column: columnsDef[i].Name})
		}

		dataTableAction.SortColumn = columnsDef[0].Name
		results["columnDef"] = columnsDef
		// Add table's label
		if dataTableAction.FromClauses[0].Schema == "public" {
			results["label"] = 	fmt.Sprintf("Table %s", dataTableAction.FromClauses[0].Table)
		} else {
			results["label"] = 	fmt.Sprintf("Table %s.%s", dataTableAction.FromClauses[0].Schema, dataTableAction.FromClauses[0].Table)
		}
	}

	// Get table schema
	// //*
	// value, ok := tableSchemaCache.Get(dataTableAction.getKey())
	// if !ok {
	// 	// Not in cache
	// 	//*
	// 	log.Println("DataTableSchema key",dataTableAction.getKey(),"is not in the cache")
	// 	tableSchema, err := schema.GetTableSchema(ctx.Dbpool, dataTableAction.Schema, dataTableAction.FromClauses[0].Table)
	// 	if err != nil {
	// 		log.Printf("While schema.GetTableSchema for %s.%s: %v", dataTableAction.Schema, dataTableAction.FromClauses[0].Table, err)
	// 		ERROR(w, http.StatusInternalServerError, errors.New("error while schema.GetTableSchema"))
	// 	}
	// 	value = *tableSchema
	// 	tableSchemaCache.Add(dataTableAction.getKey(), value)
	// }
	// tableDefinition, ok := value.(schema.TableDefinition)
	// if !ok {
	// 	log.Println("While casting cache value to schema.TableDefinition")
	// 	ERROR(w, http.StatusInternalServerError, errors.New("error while schema.GetTableSchema2"))
	// }
	// //*

	// Build the query
	// SELECT "key", "user_name", "client", "process", "status", "submitted_at" FROM "jetsapi"."pipelines" ORDER BY "key" ASC OFFSET 5 LIMIT 10;
	var buf strings.Builder
	fromClause := dataTableAction.makeFromClauses()
	buf.WriteString(dataTableAction.makeSelectColumns())
	buf.WriteString(" FROM ")
	buf.WriteString(fromClause)
	buf.WriteString(" ")
	whereClause := dataTableAction.makeWhereClause()
	buf.WriteString(whereClause)
	if len(dataTableAction.SortColumn) > 0 {
		buf.WriteString(" ORDER BY ")
		buf.WriteString(pgx.Identifier{dataTableAction.SortColumn}.Sanitize())
		if !dataTableAction.SortAscending {
			buf.WriteString(" DESC ")
		}
	}
	buf.WriteString(" OFFSET ")
	buf.WriteString(fmt.Sprintf("%d", dataTableAction.Offset))
	buf.WriteString(" LIMIT ")
	buf.WriteString(fmt.Sprintf("%d", dataTableAction.Limit))

	// Perform the query
	query := buf.String()
	resultRows, err := execQuery(ctx.Dbpool, dataTableAction, &query)
	if err != nil {
		return nil, http.StatusInternalServerError,
			fmt.Errorf("While executing query from tables %s: %v", fromClause, err)
	}

	// get the total nbr of row
	//* TODO add where clause to filter deleted items
	stmt := fmt.Sprintf("SELECT count(*) FROM %s %s", fromClause, whereClause)
	var totalRowCount int
	err = ctx.Dbpool.QueryRow(context.Background(), stmt).Scan(&totalRowCount)
	if err != nil {
		return nil, http.StatusInternalServerError,
			fmt.Errorf("While getting total row count from tables %s: %v", fromClause, err)
	}

	results["totalRowCount"] = totalRowCount
	results["rows"] = resultRows
	return &results, http.StatusOK, nil
}

// DoPreviewFileAction ------------------------------------------------------
func (ctx *Context) DoPreviewFileAction(dataTableAction *DataTableAction) (*map[string]interface{}, int, error) {

	// Validation
	if len(dataTableAction.WhereClauses) == 0 || 
		len(dataTableAction.WhereClauses[0].Values) == 0 ||
		dataTableAction.WhereClauses[0].Column != "file_key" {
			return nil, http.StatusBadRequest, fmt.Errorf("invalid request, expecting file_key in where clause")
	}
	awsBucket := os.Getenv("JETS_BUCKET")
	awsRegion := os.Getenv("JETS_REGION")
	if awsBucket == "" || awsRegion == "" {
		return nil, http.StatusInternalServerError, fmt.Errorf("missing env JETS_BUCKET or JETS_REGION")
	}

	// to package up the result
	fileKey := dataTableAction.WhereClauses[0].Values[0]
	results := map[string]interface{} {
		"label": fmt.Sprintf("Preview of %s", fileKey),
	}
	results["columnDef"] = []DataTableColumnDef{
		{
			Name:      "line",
			Label:     "Line",
			IsNumeric: false,
		},
	}

	// Download object using a download manager to a temp file (fileHd)
	fileHd, err := os.CreateTemp("", "jetstore")
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("failed to open temp input file: %v", err)
	}
	fmt.Println("Temp input file name:", fileHd.Name())
	defer os.Remove(fileHd.Name())

	// Download the object
	nsz, err := awsi.DownloadFromS3(awsBucket, awsRegion, fileKey, fileHd)
	if err != nil {
		return nil, http.StatusBadRequest, fmt.Errorf("failed to download input file: %v", err)
	}
	fmt.Println("downloaded", nsz,"bytes from s3")

	// Read the file
	fileHd.Seek(0, 0)
	fileScanner := bufio.NewScanner(fileHd)
	resultRows := make([][]interface{}, 0, dataTableAction.Limit)
	nbrLines := 0

	fileScanner.Split(bufio.ScanLines)

	done := false
	for !done && fileScanner.Scan() {
		row := []interface{} {
			fileScanner.Text(),
		}
		resultRows = append(resultRows, row)
		nbrLines += 1
		if nbrLines == dataTableAction.Limit {
			done = true
		}
	}
	results["totalRowCount"] = nbrLines
	results["rows"] = resultRows
	return &results, http.StatusOK, nil
}

// func (ctx *Context) readLocalFiles(dataTableAction *DataTableAction) (*map[string]interface{}, int, error) {
// 	fileSystem := os.DirFS(*ctx.unitTestDir)
// 	dirData := make([]map[string]string, 0)
// 	key := 1
// 	err := fs.WalkDir(fileSystem, ".", func(path string, info fs.DirEntry, err error) error {
// 		if err != nil {
// 			log.Printf("ERROR while walking unit test directory %q: %v", path, err)
// 			return err
// 		}
// 		if info.IsDir() {
// 			// fmt.Printf("visiting directory: %+v \n", info.Name())
// 			return nil
// 		}
// 		// fmt.Printf("visited file: %q\n", path)
// 		pathSplit := strings.Split(path, "/")
// 		if len(pathSplit) != 3 {
// 			log.Printf("Invalid path found while walking unit test directory %q: skipping it", path)
// 			return nil
// 		}
// 		if strings.HasPrefix(pathSplit[2], "err_") {
// 			// log.Printf("Found loader error file while walking unit test directory %q: skipping it", path)
// 			return nil
// 		}
// 		data := make(map[string]string, 5)
// 		data["key"] = strconv.Itoa(key)
// 		key += 1
// 		data["client"] = pathSplit[0]
// 		data["object_type"] = pathSplit[1]
// 		data["file_key"] = *ctx.unitTestDir + "/" + path
// 		data["last_update"] = time.Now().Format(time.RFC3339)
// 		dirData = append(dirData, data)
// 		return nil
// 	})
// 	if err != nil {
// 		return nil, http.StatusInternalServerError, fmt.Errorf("error walking the unit test directory path %q: %v", *ctx.unitTestDir, err)
// 	}

// 	// package the result, sending back only the requested collumns
// 	resultRows := make([][]string, 0, len(dirData))
// 	for iRow := range dirData {
// 		var row []string
// 		//* Need to port the raw queries to named parametrized queries as non raw queries!
// 		if len(dataTableAction.Columns) > 0 {
// 			row = make([]string, len(dataTableAction.Columns))
// 			for iCol, col := range dataTableAction.Columns {
// 				row[iCol] = dirData[iRow][col.Column]
// 			}
// 		} else {
// 			row = make([]string, 1)
// 			row[0] = dirData[iRow]["file_key"]
// 		}
// 		resultRows = append(resultRows, row)
// 	}

// 	results := make(map[string]interface{})
// 	results["rows"] = resultRows
// 	results["totalRowCount"] = len(dirData)
// 	// fmt.Println("file_key_staging DEV MODE:")
// 	// json.NewEncoder(os.Stdout).Encode(results)
// 	return &results, http.StatusOK, nil
// }
