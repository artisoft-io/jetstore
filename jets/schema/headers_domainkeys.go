package schema

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// available functions for preprocessing input column values used in domain keys
var preprocessingFunctions map[string]bool
var preprocessingFncRe *regexp.Regexp
var domainKeySeparator string
func init() {
	preprocessingFncRe = regexp.MustCompile(`^(.*?)\((.*?)\)$`)
	preprocessingFunctions = map[string]bool{
		"format_date": true,
		"remove_mi": true,
	}
	domainKeySeparator = os.Getenv("JETS_DOMAIN_KEY_SEPARATOR")
}
type DomainKeyInfo struct {
	// list of input column name making the domain key
	ColumnNames      []string
	// list of input column position making the domain key
	ColumnPos        []int
	// list of pre-processing functions for the input column (one per column)
	PreprocessFnc    []string
	// Object type associated with the Domain Key
	ObjectType       string
	// Column position of column `objectType`:domain_key in the output table
	DomainKeyPos     int
	// Column position of column `objectType`:shard_id in the output table
	ShardIdPos       int
}
type HeadersAndDomainKeysInfo struct {
	TableName         string
	RawHeaders        []string
	Headers           []string
	HashingOverriden  bool
	HashingAlgo       string
	HashingSeed       uuid.UUID
	// key is the header
	HeadersPosMap     map[string]int
	// key is ObjectType of DomainKeyInfo
	DomainKeysInfoMap map[string]*DomainKeyInfo
	// Reserved columns removed from RawHeaders and included in Headers 
	ReservedColumns   map[string]bool
	FillerColumns     map[string]bool
	// column prefix used for fixed-width input record
	// The prefix is the first record type (the one with offset == 0)
	// This is empty string for non fixed-width records, therefore ignored
	FixedWidthColumnPrefix string
}
func NewHeadersAndDomainKeysInfo(tableName string) (*HeadersAndDomainKeysInfo, error) {
	headersDKInfo := HeadersAndDomainKeysInfo {
		TableName:         tableName,
		DomainKeysInfoMap: make(map[string]*DomainKeyInfo, 0),
		RawHeaders:        make([]string, 0),
		Headers:           make([]string, 0),
		HeadersPosMap:     make(map[string]int, 0),
		ReservedColumns:   make(map[string]bool, 0),
		FillerColumns:     make(map[string]bool, 0),
		HashingAlgo:       strings.ToLower(os.Getenv("JETS_DOMAIN_KEY_HASH_ALGO")),
	}
	if headersDKInfo.HashingAlgo == "" {
		headersDKInfo.HashingAlgo = "none"
	}
	var err error
	seed := os.Getenv("JETS_DOMAIN_KEY_HASH_SEED")
	if seed != "" {
		headersDKInfo.HashingSeed, err = uuid.Parse(os.Getenv("JETS_DOMAIN_KEY_HASH_SEED"))
		if err != nil {
			return nil, fmt.Errorf("while initializing uuid from JETS_DOMAIN_KEY_HASH_SEED: %v", err)
		}
	}
	switch headersDKInfo.HashingAlgo {
	case "md5", "sha1", "none":
	default:
		return nil, fmt.Errorf("error invalid JETS_DOMAIN_KEY_HASH_ALGO, must be md5, sha1, or none (not case sensitive): %s", headersDKInfo.HashingAlgo)
	}
	return &headersDKInfo, nil
}
func (dkInfo *HeadersAndDomainKeysInfo)InitializeStagingTable(rawHeaders *[]string, mainObjectType string, domainKeysJson *string, fixedWidthColumnPrefix string) error {
	dkInfo.RawHeaders = append(dkInfo.RawHeaders, (*rawHeaders)...)
	dkInfo.ReservedColumns["file_key"] = true
	dkInfo.ReservedColumns["jets:key"] = true
	dkInfo.ReservedColumns["last_update"] = true
	dkInfo.ReservedColumns["session_id"] = true
	dkInfo.FixedWidthColumnPrefix = fixedWidthColumnPrefix
	return dkInfo.Initialize(mainObjectType, domainKeysJson)
}
func (dkInfo *HeadersAndDomainKeysInfo)InitializeDomainTable(domainHeaders *[]string, mainObjectType string, domainKeysJson *string) error {
	dkInfo.RawHeaders = append(dkInfo.RawHeaders, (*domainHeaders)...)
	dkInfo.ReservedColumns["last_update"] = true
	dkInfo.ReservedColumns["session_id"] = true
	return dkInfo.Initialize(mainObjectType, domainKeysJson)
}

func (dk *DomainKeyInfo)String() string {
	var buf strings.Builder
	buf.WriteString("    DomainKeyInfo:")
	buf.WriteString("\n      ObjectType:")
	buf.WriteString(dk.ObjectType)
	buf.WriteString("\n      ColumnNames & Pos:")
	for i := range dk.ColumnNames {
		buf.WriteString(fmt.Sprintf("(%s:%d), ", dk.ColumnNames[i], dk.ColumnPos[i]))
	}
	buf.WriteString("\n      Target DomainKeyPos:")
	buf.WriteString(strconv.Itoa(dk.DomainKeyPos))
	buf.WriteString("\n      Target ShardIdPos:")
	buf.WriteString(strconv.Itoa(dk.ShardIdPos))
	buf.WriteString("\n")
	return buf.String()
}

func (dkInfo *HeadersAndDomainKeysInfo)String() string {
	var buf strings.Builder
	buf.WriteString("HeadersAndDomainKeysInfo:")
	buf.WriteString("\n  RawHeaders:")
	buf.WriteString(strings.Join(dkInfo.RawHeaders, ","))
	buf.WriteString("\n  Headers:")
	buf.WriteString(strings.Join(dkInfo.Headers, ","))
	buf.WriteString("\n  ReservedColumns:")
	keys := reflect.ValueOf(dkInfo.ReservedColumns).MapKeys()
	for i := range keys {
		buf.WriteString(keys[i].String())
		buf.WriteString(",")
	}
	buf.WriteString("\n  HashingAlgo: ")
	buf.WriteString(dkInfo.HashingAlgo)
	if dkInfo.HashingOverriden {
		buf.WriteString(" (hashing algo overriden)")
	}
	buf.WriteString("\n  HashingSeed: ")
	buf.WriteString(dkInfo.HashingSeed.String())
	buf.WriteString("\n  HeadersPos:")
	for k,v := range dkInfo.HeadersPosMap {
		buf.WriteString(fmt.Sprintf("(%s:%d), ", k, v))
	}
	buf.WriteString("\n  DomainKeysInfoMap:")
	for _,v := range dkInfo.DomainKeysInfoMap {
		buf.WriteString(v.String())
	}
	buf.WriteString("\n")
	return buf.String()
}

func parseColumn(column *string) []string {
	v := preprocessingFncRe.FindStringSubmatch(*column)
	if len(v) < 3 {
		return nil
	}
	if len(v[1]) == 0 || len(v[2]) == 0 {
		return nil
	}
	if !preprocessingFunctions[v[1]] {
		return nil
	}
	return v
}

// initialize (domainKeysJson string)
// --------------------------------------------------------------------------------------
// Compute output table columns and associated domain keys
// passing domainKeysJson as argument for completeness
func (dkInfo *HeadersAndDomainKeysInfo)Initialize(mainObjectType string, domainKeysJson *string) error {
	var ok bool
	if *domainKeysJson == "" {
		log.Println("No domain key defined (domainKeysJson is empty), using jets:key as default")
		*domainKeysJson = "\"jets:key\""
	}
	var f interface{}
	err := json.Unmarshal([]byte(*domainKeysJson), &f)
	if err != nil {
		fmt.Println("while parsing domainKeysJson using json parser:", err)
		return err
	}
	// Extract the domain keys structure from the json
	switch value := f.(type) {
	case string:
			// fmt.Println("*** Domain Key is single column", value)
			dkInfo.DomainKeysInfoMap[mainObjectType] = &DomainKeyInfo{
				ColumnNames: []string{value},
				ObjectType: mainObjectType,
			}
	case []interface{}:
		// fmt.Println("*** Domain Key is a composite key", value)
		ck := make([]string, 0)
		for i := range value {
			if reflect.TypeOf(value[i]).Kind() == reflect.String {
				ck = append(ck, value[i].(string))
			}
		}
		dkInfo.DomainKeysInfoMap[mainObjectType] = &DomainKeyInfo{
			ColumnNames: ck,
			ObjectType: mainObjectType,
		}
	case map[string]interface{}:
		// fmt.Println("*** Domain Key is a struct of composite keys", value)
		for k, v := range value {
			switch vv := v.(type) {
			case string:
				if k == "jets:hashing_override" {
					dkInfo.HashingAlgo = vv
					dkInfo.HashingOverriden = true
				} else {
					dkInfo.DomainKeysInfoMap[k] = &DomainKeyInfo{
						ColumnNames: []string{vv},
						ObjectType: k,
					}
				}
			case []interface{}:
				ck := make([]string, 0)
				for i := range vv {
					if reflect.TypeOf(vv[i]).Kind() == reflect.String {
						ck = append(ck, vv[i].(string))
					}
				}
				dkInfo.DomainKeysInfoMap[k] = &DomainKeyInfo{
					ColumnNames: ck,
					ObjectType: k,
				}
			default:
					log.Println("domainKeysJson contains",vv,"which is of a type that is not supported")
			}
		}
	default:
		log.Println("domainKeysJson contains",value,"which is of a type that is not supported")
	}

	// Extract the preprocessing functions that are decorating the column names (if any)
	// regex to extract preprocessing function, e.g., format_date(columnName)
	for _,dk := range dkInfo.DomainKeysInfoMap {
		dk.PreprocessFnc = make([]string, len(dk.ColumnNames))
		for i := range dk.ColumnNames {
			v := parseColumn(&dk.ColumnNames[i])
			if v != nil {
				dk.ColumnNames[i] = v[2]
				dk.PreprocessFnc[i] = v[1]
			}		
		}
	}

	// Complete the reserved columns by adding the domain keys
	for k := range dkInfo.DomainKeysInfoMap {
		dkInfo.ReservedColumns[fmt.Sprintf("%s:domain_key", k)] = true
		dkInfo.ReservedColumns[fmt.Sprintf("%s:shard_id", k)] = true
	}

	// Drop input columns (rawHeaders) matching the reserved column names
	// Drop input columns (rawHeaders) that appear more than once (e.g. 'Filler' columns)
	// compute headers of output table
	for ipos := range dkInfo.RawHeaders {
		h := dkInfo.RawHeaders[ipos]
		if !dkInfo.ReservedColumns[h] && !dkInfo.FillerColumns[h] {
			_,ok := dkInfo.HeadersPosMap[h]
			if ok {
				// Column is duplicated, mark it as a filler
				// (there will still be one column named by the value of h)
				dkInfo.FillerColumns[h] = true
			} else {
				dkInfo.Headers = append(dkInfo.Headers, h)
				dkInfo.HeadersPosMap[h] = ipos	
			}
		}
	}
	// Add reserved columns (sessionId, shardId, DomainKeys, etc) to the headers,
	// Adding reserved columns
	for k := range dkInfo.ReservedColumns {
		dkInfo.HeadersPosMap[k] = len(dkInfo.Headers)
		dkInfo.Headers = append(dkInfo.Headers, k)
	}

	// Complete the initialization of DomainKeyInfo since we now have the headers
	// k: objectType
	// v: *DomainKeyInfo
	for objectType,v := range dkInfo.DomainKeysInfoMap {
		v.ColumnPos = make([]int, len(v.ColumnNames))
		for jpos, columnName := range v.ColumnNames {
			if dkInfo.FixedWidthColumnPrefix != "" {
				columnName = fmt.Sprintf("%s.%s", dkInfo.FixedWidthColumnPrefix, columnName)
			}
			v.ColumnPos[jpos], ok = dkInfo.HeadersPosMap[columnName]
			if !ok {
				err := fmt.Errorf(
					"error while getting domain keys: column name '%s' not found in headers of table %s, see if table jetsapi.domain_keys_registry has an invaid record", 
					columnName, dkInfo.TableName)
				log.Println(err)
				return err
			}
		}
		domainKey := fmt.Sprintf("%s:domain_key", objectType)
		shardId := fmt.Sprintf("%s:shard_id", objectType)
		v.DomainKeyPos = dkInfo.HeadersPosMap[domainKey]
		v.ShardIdPos = dkInfo.HeadersPosMap[shardId]
	}
	return nil
}

func (dkInfo *HeadersAndDomainKeysInfo)GetHeaderPos() []int {
	ret := make([]int, len(dkInfo.Headers))
	for i,k := range dkInfo.Headers {
		ret[i] = dkInfo.HeadersPosMap[k]
	}
	return ret
}

func (dkInfo *HeadersAndDomainKeysInfo)joinUpper(columns *[]string, sep string) string {
	if columns == nil {
		return ""
	}
	doUpper := !(dkInfo.HashingOverriden && dkInfo.HashingAlgo == "none")
	var buf strings.Builder
	sz := len(sep)
	for ipos := range *columns {
		if ipos > 0 && sz > 0 {
			buf.WriteString(sep)
		}
		if doUpper {
			buf.WriteString(strings.ToUpper((*columns)[ipos]))
		} else {
			buf.WriteString((*columns)[ipos])
		}
	}
	return buf.String()
}

func (dkInfo *HeadersAndDomainKeysInfo)makeGroupingKey(columns *[]string) string {
	var groupingKey string
	switch dkInfo.HashingAlgo {
	case "md5":
		groupingKey = dkInfo.joinUpper(columns, domainKeySeparator)
		groupingKey = uuid.NewMD5(dkInfo.HashingSeed, []byte(groupingKey)).String()
	case "sha1":
		groupingKey = dkInfo.joinUpper(columns, domainKeySeparator)
		groupingKey = uuid.NewSHA1(dkInfo.HashingSeed, []byte(groupingKey)).String()
	default:
		groupingKey = dkInfo.joinUpper(columns, domainKeySeparator)
	}
	return groupingKey
}

func (dkInfo *HeadersAndDomainKeysInfo)IsDomainKeyIsJetsKey(objectType *string) bool {
	if objectType != nil {
		dk := dkInfo.DomainKeysInfoMap[*objectType]
		if dk != nil && len(dk.ColumnPos) == 1 && dk.ColumnNames[0] == "jets:key" {
			return true
		}
	}
	return false
}

func applyPreprocessingFunction(fncName, value string) (string, error) {
	// IMPORTANT NOTE: If adding preprocessing functions here, MUST update preprocessingFunctions global var
	switch fncName {
	case "":
		return value, nil
	case "format_date":
		year, month, day, err := rdf.ParseDateComponents(value)
		if err != nil {
			return "", fmt.Errorf("while applyPreprocessingFunction %s: %v", fncName, err)
		}
		formatedDate := fmt.Sprintf("%d%02d%02d",year, month, day)
		return formatedDate, nil

	case "remove_mi":	// remove last 2 char if last-1 is a space, e.g. "michel f" becomes "michel"
		l := len(value)
		if l < 3 {
			return value, nil
		}
		v := value[l-2]
		s := []byte(" ")
		if v == s[0] {
			return value[:l-2], nil
		} 
		return value, nil

	default:
		return "", fmt.Errorf("unknown pre-processing function " + fncName)
	}
}
func (dkInfo *HeadersAndDomainKeysInfo)ComputeGroupingKey(NumberOfShards int, objectType *string, record *[]string, recordTypeOffset int, jetsKey *string) (string, int, error) {
	dk := dkInfo.DomainKeysInfoMap[*objectType]
	if dk == nil {
		groupingKey := *jetsKey
		return groupingKey, ComputeShardId(NumberOfShards, groupingKey), nil		
	}
	if len(dk.ColumnPos) == 1 {
		if dk.ColumnNames[0] == "jets:key" {
			cols := []string{*jetsKey}
			groupingKey := dkInfo.makeGroupingKey(&cols)
			return groupingKey, ComputeShardId(NumberOfShards, groupingKey), nil
		}
		recPos := dk.ColumnPos[0] + recordTypeOffset
		if recPos < len(*record) {
			// apply the pre-processing function, e.g. format_date for domain key generation
			value, err := applyPreprocessingFunction(dk.PreprocessFnc[0], (*record)[recPos])
			if err != nil {
				return "", 0, err
			}
			cols := []string{value}
			groupingKey := dkInfo.makeGroupingKey(&cols)
			return groupingKey, ComputeShardId(NumberOfShards, groupingKey), nil
		}
		return "", 0, fmt.Errorf("error: domain key is invalid, make sure it is not a reserved column for ObjectType %s", *objectType)
	}
	cols := make([]string, len(dk.ColumnPos))
	var err error
	for ipos := range dk.ColumnPos {
		recPos := dk.ColumnPos[ipos] + recordTypeOffset
		if recPos < len(*record) {
			cols[ipos], err = applyPreprocessingFunction(dk.PreprocessFnc[ipos], (*record)[recPos])
			if err != nil {
				return "", 0, err
			}
		} else {
			return "", 0, fmt.Errorf("error: domain key is invalid, make sure it is not a reserved column for ObjectType %s", *objectType)
		}
	}
	groupingKey := dkInfo.makeGroupingKey(&cols)
	return groupingKey, ComputeShardId(NumberOfShards, groupingKey), nil		
}
// Alternate version for output records - same as ComputeGroupingKey using interface{} as record
func (dkInfo *HeadersAndDomainKeysInfo)ComputeGroupingKeyI(NumberOfShards int, objectType *string, record *[]interface{}) (string, int, error) {
	dk := dkInfo.DomainKeysInfoMap[*objectType]
	if dk == nil {
		return "", 0, fmt.Errorf("unexpected error: no domain key info found for objecttype %s", *objectType)
	}
	if len(dk.ColumnPos) == 1 {
		switch groupingKey := (*record)[dk.ColumnPos[0]].(type) {
		case string:
			cols := []string{groupingKey}
			groupingKey = dkInfo.makeGroupingKey(&cols)
			return groupingKey, ComputeShardId(NumberOfShards, groupingKey), nil
		case nil:
			log.Println("Error: Domain Key column is NULL")
			return "", 0, nil
		default:
			log.Printf("Error: Domain Key column is not a string, it's %s", reflect.TypeOf(groupingKey).Kind())
			return "", 0, fmt.Errorf("error: Domain Key column is not a string, it's %s", reflect.TypeOf(groupingKey).Kind())
		}
	}
	cols := make([]string, len(dk.ColumnPos))
	for ipos := range dk.ColumnPos {
		switch value := (*record)[dk.ColumnPos[ipos]].(type) {
		case string:
			cols[ipos] = value
		case nil:
			log.Println("Error: Domain Key column", dk.ColumnNames[ipos],"is NULL")
			return "", 0, nil
		default:
			log.Println("Error: Domain Key column", dk.ColumnNames[ipos],"is not a string, it's", reflect.TypeOf(value).Kind())
			return "", 0, fmt.Errorf("error: Domain Key column is not a string, it's %s", reflect.TypeOf(value).Kind())
		}
	}
	groupingKey := dkInfo.makeGroupingKey(&cols)
	return groupingKey, ComputeShardId(NumberOfShards, groupingKey), nil		
}

func ComputeShardId(NumberOfShards int, key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	res := h.Sum32() % uint32(NumberOfShards)
	// log.Println("COMPUTE SHARD for key ",key,"hashed to", v,"on",NumberOfShards,"shard id =",res)
	return int(res)
}
func TableExists(dbpool *pgxpool.Pool, schema, table string) (exists bool, err error) {
	err = dbpool.QueryRow(context.Background(), "select exists (select from pg_tables where schemaname = $1 and tablename = $2)", schema, table).Scan(&exists)
	if err != nil {
		err = fmt.Errorf("QueryRow failed: %v", err)
	}
	return exists, err
}

// Create the Staging Table
func (dkInfo *HeadersAndDomainKeysInfo)CreateStagingTable(dbpool *pgxpool.Pool, tableName string) (err error) {
	if tableName == "" {
		return fmt.Errorf("error in CreateStagingTable: tableName is empty")
	}
	stmt := fmt.Sprintf("DROP TABLE IF EXISTS %s", pgx.Identifier{tableName}.Sanitize())
	_, err = dbpool.Exec(context.Background(), stmt)
	if err != nil {
		return fmt.Errorf("error while droping staging table %s: %v", tableName, err)
	}
	var buf strings.Builder
	buf.WriteString("CREATE TABLE IF NOT EXISTS ")
	buf.WriteString(pgx.Identifier{tableName}.Sanitize())
	buf.WriteString("(")
	lastPos := len(dkInfo.Headers) - 1
	for ipos, header := range dkInfo.Headers {
		switch {
		case header == "file_key":
			buf.WriteString(" file_key TEXT")

		case header == "jets:key":
			buf.WriteString(
				fmt.Sprintf(" %s TEXT DEFAULT gen_random_uuid ()::text NOT NULL", 
					pgx.Identifier{header}.Sanitize()))

		case header == "session_id":
			buf.WriteString(" session_id TEXT DEFAULT '' NOT NULL")

		case header == "last_update":
			buf.WriteString(" last_update timestamp without time zone DEFAULT now() NOT NULL")

		case strings.HasSuffix(header, ":domain_key"):
			buf.WriteString(fmt.Sprintf(" %s TEXT DEFAULT '' NOT NULL", pgx.Identifier{header}.Sanitize()))

		case strings.HasSuffix(header, ":shard_id"):
			buf.WriteString(fmt.Sprintf(" %s INTEGER DEFAULT 0 NOT NULL", pgx.Identifier{header}.Sanitize()))

		default:
			buf.WriteString(fmt.Sprintf(" %s TEXT", pgx.Identifier{header}.Sanitize()))
		}
		if ipos < lastPos {
			buf.WriteString(", ")
		}
	}
	buf.WriteString(");")
	stmt = buf.String()
	log.Println(stmt)
	_, err = dbpool.Exec(context.Background(), stmt)
	if err != nil {
		return fmt.Errorf("error while creating table: %v", err)
	}

	// Create index on sessionId and shardId columns
	for k := range dkInfo.DomainKeysInfoMap {
		stmt = fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (%s, %s);`,
		pgx.Identifier{fmt.Sprintf("%s_%s_shard_idx", tableName, k)}.Sanitize(),
		pgx.Identifier{tableName}.Sanitize(),
		pgx.Identifier{"session_id"}.Sanitize(),
		pgx.Identifier{fmt.Sprintf("%s:shard_id", k)}.Sanitize())
		log.Println(stmt)
		_, err = dbpool.Exec(context.Background(), stmt)
		if err != nil {
			return fmt.Errorf("error while creating (session_id, shard_id) index: %v", err)
		}
	}

	// Create index on sessionId and domainKey columns
	for k := range dkInfo.DomainKeysInfoMap {
		stmt = fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (%s, %s ASC);`,
		pgx.Identifier{fmt.Sprintf("%s_%s_domainkey_idx", tableName, k)}.Sanitize(),
		pgx.Identifier{tableName}.Sanitize(),
		pgx.Identifier{"session_id"}.Sanitize(),
		pgx.Identifier{fmt.Sprintf("%s:domain_key", k)}.Sanitize())
		log.Println(stmt)
		_, err = dbpool.Exec(context.Background(), stmt)
		if err != nil {
			return fmt.Errorf("error while creating (session_id, domain_key) index: %v", err)
		}
	}
	return nil
}

// Utility Methods
func GetObjectTypesFromDominsKeyJson(domainKeysJson string, defaultValue string) (*[]string, error) {
	// Get the list of ObjectType from domainKeysJson if it's an elm, detault to defaultValue
	objTypes := make([]string, 0)
	if domainKeysJson != "" {
		var f interface{}
		err := json.Unmarshal([]byte(domainKeysJson), &f)
		if err != nil {
			fmt.Println("while parsing domainKeysJson using json parser:", err)
			return nil, err
		}
		// Extract the domain keys structure from the json
		switch value := f.(type) {
		case map[string]interface{}:
			for k := range value {
				if k != "jets:hashing_override" {
					objTypes = append(objTypes, k)
				}
			}		
		}
	}
	if len(objTypes) == 0 {
		objTypes = append(objTypes, defaultValue)
	}
	return &objTypes, nil
}