package rete

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// This LookupTable implementation is using sqlite3 as the data storage.

type LookupTableSqlite3 struct {
	spec                *LookupTableNode
	lookupDb            *sql.DB
	maxKey              int64
	selectStmt          string
	selectRandStmt      string
	selectMultiRandStmt string
	columnsSpec         []ColumnSpec
	lookupCache         *rdf.Node
}

type ColumnSpec struct {
	columnResource *rdf.Node
	// dataSpec       interface{}
}

func NewLookupTableSqlite3(rmgr *rdf.ResourceManager, metaGraph *rdf.RdfGraph, spec *LookupTableNode,
	lookupDb *sql.DB) (LookupTable, error) {

	lookupTable := &LookupTableSqlite3{
		spec:        spec,
		lookupDb:    lookupDb,
		lookupCache: rmgr.NewResource(fmt.Sprintf("jets:cache:%s", spec.Name)),
	}
	// initialize the lookupTable
	// Make sure that the lookup table name has a corresponding resource
	rmgr.NewResource(spec.Name)
	// Get table's max key
	err := lookupDb.QueryRow(fmt.Sprintf(`SELECT MAX(__key__) FROM "%s"`, spec.Name)).Scan(&lookupTable.maxKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get max key for table %s: %v", spec.Name, err)
	}
	// log.Println("Got max_key:", lookupTable.maxKey)
	// Create lookup statements
	lookupTable.columnsSpec = make([]ColumnSpec, len(spec.Columns))
	var buf strings.Builder
	buf.WriteString("SELECT ")
	isFirst := true
	for i := range spec.Columns {
		if !isFirst {
			buf.WriteString(", ")
		}
		isFirst = false
		buf.WriteString(fmt.Sprintf(`"%s"`, spec.Columns[i].Name))
		// columnResource as a resource from column name
		lookupTable.columnsSpec[i].columnResource = rmgr.GetResource(spec.Columns[i].Name)
	}
	buf.WriteString(fmt.Sprintf(` FROM "%s" WHERE `, spec.Name))
	stmt := buf.String()
	lookupTable.selectStmt = fmt.Sprintf(`%s "jets:key" = ?`, stmt)
	lookupTable.selectRandStmt = fmt.Sprintf(`%s __key__ = ?`, stmt)
	lookupTable.selectMultiRandStmt = fmt.Sprintf(
		`%s "jets:key" = (SELECT "jets:key" FROM "%s" WHERE __key__ = ?)`, stmt, spec.Name)
	// log.Println("selectStmt:", lookupTable.selectStmt)
	// log.Println("selectRandStmt:", lookupTable.selectRandStmt)
	// log.Println("selectMultiRandStmt:", lookupTable.selectMultiRandStmt)

	return lookupTable, nil
}

func (tbl *LookupTableSqlite3) insertResultIntoSession(sess *rdf.RdfSession, s *rdf.Node, resultRow []interface{}) error {
	rmgr := sess.ResourceMgr
	for i := range tbl.spec.Columns {
		value := rdf.Null()
		// r := resultRow[i].(*interface{})
		// log.Println("TypeOf:", reflect.TypeOf(*r))
		if resultRow[i] != nil && *resultRow[i].(*interface{}) != nil {
			switch tbl.spec.Columns[i].Type {
			case "text", "string":
				v, ok := (*resultRow[i].(*interface{})).(string)
				if !ok {
					return fmt.Errorf("expecting string type in lookup for text")
				}
				value = rmgr.NewTextLiteral(v)
			case "date":
				v, ok := (*resultRow[i].(*interface{})).(string)
				if !ok {
					return fmt.Errorf("expecting string type in lookup for date")
				}
				d, err := rdf.NewLDate(v)
				if err != nil {
					log.Printf("Invalid date in lookup %s: %v", tbl.spec.Name, err)
				} else {
					value = rmgr.NewDateLiteral(d)
				}
			case "datetime":
				v, ok := (*resultRow[i].(*interface{})).(string)
				if !ok {
					return fmt.Errorf("expecting sql.NullString type in lookup for datetime")
				}
				d, err := rdf.NewLDatetime(v)
				if err != nil {
					log.Printf("Invalid datetime in lookup %s: %v", tbl.spec.Name, err)
				} else {
					value = rmgr.NewDatetimeLiteral(d)
				}
			case "int", "bool", "long", "integer":
				switch v := (*resultRow[i].(*interface{})).(type) {
				case int:
					value = rmgr.NewIntLiteral(v)
				case int64:
					value = rmgr.NewIntLiteral(int(v))
				default:
					return fmt.Errorf("expecting int/int64 type in lookup for int/bool/long")
				}
			case "double":
				switch v := (*resultRow[i].(*interface{})).(type) {
				case float64:
					value = rmgr.NewDoubleLiteral(v)
				default:
					return fmt.Errorf("expecting float64 type in lookup for double")
				}
			default:
				return fmt.Errorf("unknown datatype %s for column %s in lookup table %s configuration",
					tbl.spec.Columns[i].Type, tbl.spec.Columns[i].Name, tbl.spec.Name)
			}
		}
		_, err := sess.InsertInferred(s, tbl.columnsSpec[i].columnResource, value)
		if err != nil {
			return fmt.Errorf("while InsertInferred in lookup: %v", err)
		}
	}
	return nil
}

func (tbl *LookupTableSqlite3) Lookup(rs *ReteSession, tblName *string, key *string) (*rdf.Node, error) {
	if rs == nil || tblName == nil || key == nil {
		return nil, fmt.Errorf("error: arguments to Lookup cannot be nil")
	}
	sess := rs.RdfSession
	rmgr := sess.ResourceMgr
	jr := rmgr.JetsResources
	// the result subject which is associated to the lookup row for key
	row := rmgr.NewResource(fmt.Sprintf("jets:lookup:%s", *key))

	// Check if the result of the lookup was already put in the rdf_session by a previous call
	if sess.Contains(tbl.lookupCache, jr.Jets__lookup_row, row) {
		// Already got it
		return row, nil
	}
	// Query the lookup table, make destination
	resultRow := make([]interface{}, len(tbl.columnsSpec))
	for i := range resultRow {
		resultRow[i] = new(interface{})
	}
	err := tbl.lookupDb.QueryRow(tbl.selectStmt, *key).Scan(resultRow...)
	if err != nil {
		if err == sql.ErrNoRows {
			// No row returned
			return rdf.Null(), nil
		}
		return nil, fmt.Errorf("failed query lookup table %s for key %s: %v", tbl.spec.Name, *key, err)
	}

	// Put the result into the rdf session
	err = tbl.insertResultIntoSession(sess, row, resultRow)
	if err != nil {
		return nil, err
	}
	// Put the result into the cache
	_, err = sess.InsertInferred(tbl.lookupCache, jr.Jets__lookup_row, row)
	if err != nil {
		return nil, fmt.Errorf("while InsertInferred in lookup (2): %v", err)
	}
	return row, nil
}

func (tbl *LookupTableSqlite3) MultiLookup(rs *ReteSession, tblName *string, key *string) (*rdf.Node, error) {
	if rs == nil || tblName == nil || key == nil {
		return nil, fmt.Errorf("error: arguments to MultiLookup cannot be nil")
	}
	sess := rs.RdfSession
	rmgr := sess.ResourceMgr
	jr := rmgr.JetsResources
	// the result subject which is associated to the lookup row for key
	row := rmgr.NewResource(fmt.Sprintf("jets:lookup:multi:%s", *key))

	// Check if the result of the lookup was already put in the rdf_session by a previous call
	if sess.Contains(tbl.lookupCache, jr.Jets__lookup_multi_rows, row) {
		// Already got it
		return row, nil
	}
	// Query the lookup table, make a copy of the destination
	rows, err := tbl.lookupDb.Query(tbl.selectStmt, *key)
	if err != nil {
		if err == sql.ErrNoRows {
			// No row returned
			return rdf.Null(), nil
		}
		return nil, fmt.Errorf("failed query lookup table %s for key %s: %v", tbl.spec.Name, *key, err)
	}
	defer rows.Close()
	resultRow := make([]interface{}, len(tbl.columnsSpec))
	for rows.Next() {
		// Scan the row
		for i := range resultRow {
			resultRow[i] = new(interface{})
		}
		if err = rows.Scan(resultRow...); err != nil {
			return nil, fmt.Errorf("while scanning row for multi lookup: %v", err)
		}
		// Put the result into the rdf session
		s := rmgr.NewBNode()
		err = tbl.insertResultIntoSession(sess, s, resultRow)
		if err != nil {
			return nil, err
		}
		_, err := sess.InsertInferred(row, jr.Jets__lookup_multi_rows, s)
		if err != nil {
			return nil, fmt.Errorf("while InsertInferred in MultiLookup: %v", err)
		}
	}

	// Put the result into the cache
	_, err = sess.InsertInferred(tbl.lookupCache, jr.Jets__lookup_multi_rows, row)
	if err != nil {
		return nil, fmt.Errorf("while InsertInferred in lookup (4): %v", err)
	}
	return row, nil
}

func (tbl *LookupTableSqlite3) LookupRand(rs *ReteSession, tblName *string) (*rdf.Node, error) {
	if rs == nil || tblName == nil {
		return nil, fmt.Errorf("error: arguments to LookupRand cannot be nil")
	}
	sess := rs.RdfSession
	rmgr := sess.ResourceMgr
	jr := rmgr.JetsResources
	key := rand.Int63n(tbl.maxKey)
	// the result subject which is associated to the lookup row for key
	row := rmgr.NewResource(fmt.Sprintf("jets:lookup:rand:%d", key))

	// Check if the result of the lookup was already put in the rdf_session by a previous call
	if sess.Contains(tbl.lookupCache, jr.Jets__lookup_row, row) {
		// Already got it
		return row, nil
	}
	// Query the lookup table, make a copy of the destination
	resultRow := make([]interface{}, len(tbl.columnsSpec))
	for i := range resultRow {
		resultRow[i] = new(interface{})
	}
	err := tbl.lookupDb.QueryRow(tbl.selectRandStmt, key).Scan(resultRow...)
	if err != nil {
		if err == sql.ErrNoRows {
			// No row returned
			return rdf.Null(), nil
		}
		return nil, fmt.Errorf("failed query lookup table %s for rand key %d: %v", tbl.spec.Name, key, err)
	}

	// Put the result into the rdf session
	err = tbl.insertResultIntoSession(sess, row, resultRow)
	if err != nil {
		return nil, err
	}
	// Put the result into the cache
	_, err = sess.InsertInferred(tbl.lookupCache, jr.Jets__lookup_row, row)
	if err != nil {
		return nil, fmt.Errorf("while InsertInferred in lookup (3): %v", err)
	}
	return row, nil
}

func (tbl *LookupTableSqlite3) MultiLookupRand(rs *ReteSession, tblName *string) (*rdf.Node, error) {
	if rs == nil || tblName == nil {
		return nil, fmt.Errorf("error: arguments to MultiLookupRand cannot be nil")
	}
	sess := rs.RdfSession
	rmgr := sess.ResourceMgr
	jr := rmgr.JetsResources
	key := rand.Int63n(tbl.maxKey)
	// the result subject which is associated to the lookup row for key
	row := rmgr.NewResource(fmt.Sprintf("jets:lookup:multi:rand:%d", key))

	// Check if the result of the lookup was already put in the rdf_session by a previous call
	if sess.Contains(tbl.lookupCache, jr.Jets__lookup_multi_rows, row) {
		// Already got it
		return row, nil
	}
	// Query the lookup table, make a copy of the destination
	rows, err := tbl.lookupDb.Query(tbl.selectMultiRandStmt, key)
	if err != nil {
		if err == sql.ErrNoRows {
			// No row returned
			return rdf.Null(), nil
		}
		return nil, fmt.Errorf("failed query multi rand lookup for table %s for key %d: %v", tbl.spec.Name, key, err)
	}
	defer rows.Close()
	resultRow := make([]interface{}, len(tbl.columnsSpec))
	for rows.Next() {
		// Scan the row
		for i := range resultRow {
			resultRow[i] = new(interface{})
		}
		if err = rows.Scan(resultRow...); err != nil {
			return nil, fmt.Errorf("while scanning row for multi lookup: %v", err)
		}
		// Put the result into the rdf session
		s := rmgr.NewBNode()
		err = tbl.insertResultIntoSession(sess, s, resultRow)
		if err != nil {
			return nil, err
		}
		_, err := sess.InsertInferred(row, jr.Jets__lookup_multi_rows, s)
		if err != nil {
			return nil, fmt.Errorf("while InsertInferred in MultiLookupRand: %v", err)
		}
	}

	// Put the result into the cache
	_, err = sess.InsertInferred(tbl.lookupCache, jr.Jets__lookup_multi_rows, row)
	if err != nil {
		return nil, fmt.Errorf("while InsertInferred in MultiLookupRand: %v", err)
	}
	return row, nil
}
