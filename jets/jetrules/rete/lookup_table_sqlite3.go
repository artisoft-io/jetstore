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
	// minKey and maxKey bound __key__, which lookup_rand and multi_lookup_rand
	// draw from. hasRows is false for an empty table.
	minKey              int64
	maxKey              int64
	hasRows             bool
	selectStmt          string
	selectRandStmt      string
	selectMultiRandStmt string
	columnsSpec         []ColumnSpec
	// lookupCache is the table's own resource, <T>: a row served is cached as
	// (<T> jets:lookup_row subject), for every kind of lookup, as C++ caches it
	// (lookup_sql_helper.cc:105). It was jets:cache:<T>, with jets:lookup_multi_rows
	// for a multi-lookup, until rules_benchmark I-15.
	lookupCache *rdf.Node
	// subjectPrefix and randSubjectPrefix are C++'s (lookup_sql_helper.h:201):
	// jets:<T>: for lookup, multi_lookup and lookup_rand, and jets:rand:<T>: for
	// multi_lookup_rand. They were jets:lookup:, with no table name, so two tables
	// looked up by one key in a session shared a subject (rules_benchmark I-15).
	subjectPrefix     string
	randSubjectPrefix string
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
		lookupCache:       rmgr.NewResource(spec.Name),
		subjectPrefix:     fmt.Sprintf("jets:%s:", spec.Name),
		randSubjectPrefix: fmt.Sprintf("jets:rand:%s:", spec.Name),
	}
	// Get the range of __key__ for the random lookups. Both bounds are read,
	// rather than assuming a base: the Go packager numbers __key__ from 1 and
	// lookup.db files built by the earlier packager number it from 0.
	var minKey, maxKey sql.NullInt64
	err := lookupDb.QueryRow(fmt.Sprintf(`SELECT MIN(__key__), MAX(__key__) FROM "%s"`, spec.Name)).Scan(&minKey, &maxKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get the key range for table %s: %v", spec.Name, err)
	}
	lookupTable.hasRows = minKey.Valid && maxKey.Valid
	lookupTable.minKey, lookupTable.maxKey = minKey.Int64, maxKey.Int64
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
	// The row's own rowid, last: served as jets:row_id (insertResultIntoSession)
	if !isFirst {
		buf.WriteString(", ")
	}
	buf.WriteString("_rowid_")
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
	// jets:row_id is the row's SQLite rowid. The packager inserts a table's
	// rows in CSV order into a new table, so it numbers them from 1 in that
	// order, and it tells apart rows that share a jets:key.
	switch v := (*resultRow[len(tbl.spec.Columns)].(*interface{})).(type) {
	case int64:
		if _, err := sess.InsertInferred(s, rmgr.JetsResources.Jets__row_id, rmgr.NewIntLiteral(int(v))); err != nil {
			return fmt.Errorf("while InsertInferred of jets:row_id in lookup: %v", err)
		}
	default:
		return fmt.Errorf("expecting int64 rowid in lookup table %s, got %T", tbl.spec.Name, v)
	}
	return nil
}

// The four lookups follow C++'s LookupTable::lookup_internal and
// lookup_internal_rand (jets/rete/lookup_sql_helper.cc): the subject is
// jets:<T>:<key> (jets:rand:<T>:<n> for multi_lookup_rand), a cached subject is
// returned as it is, and a miss returns null and writes nothing. As on C++,
// lookup and multi_lookup on one table and key share the subject and its cache
// triple, so whichever runs first in a session decides what the other returns.

func (tbl *LookupTableSqlite3) Lookup(rs *ReteSession, tblName *string, key *string) (*rdf.Node, error) {
	if rs == nil || tblName == nil || key == nil {
		return nil, fmt.Errorf("error: arguments to Lookup cannot be nil")
	}
	return tbl.lookup(rs, tbl.subjectPrefix+*key, false, tbl.selectStmt, *key)
}

func (tbl *LookupTableSqlite3) MultiLookup(rs *ReteSession, tblName *string, key *string) (*rdf.Node, error) {
	if rs == nil || tblName == nil || key == nil {
		return nil, fmt.Errorf("error: arguments to MultiLookup cannot be nil")
	}
	return tbl.lookup(rs, tbl.subjectPrefix+*key, true, tbl.selectStmt, *key)
}

func (tbl *LookupTableSqlite3) LookupRand(rs *ReteSession, tblName *string) (*rdf.Node, error) {
	if rs == nil || tblName == nil {
		return nil, fmt.Errorf("error: arguments to LookupRand cannot be nil")
	}
	if !tbl.hasRows {
		return rdf.Null(), nil
	}
	key := tbl.randKey()
	return tbl.lookup(rs, fmt.Sprintf("%s%d", tbl.subjectPrefix, key), false, tbl.selectRandStmt, key)
}

func (tbl *LookupTableSqlite3) MultiLookupRand(rs *ReteSession, tblName *string) (*rdf.Node, error) {
	if rs == nil || tblName == nil {
		return nil, fmt.Errorf("error: arguments to MultiLookupRand cannot be nil")
	}
	if !tbl.hasRows {
		return rdf.Null(), nil
	}
	key := tbl.randKey()
	return tbl.lookup(rs, fmt.Sprintf("%s%d", tbl.randSubjectPrefix, key), true, tbl.selectMultiRandStmt, key)
}

// lookup serves the rows stmt selects with arg under the subject named subjectName:
// the first row's columns on the subject itself, or with multi each row's on a
// blank node, linked by (subject jets:lookup_multi_rows row). It returns null,
// and writes nothing, when stmt selects no row.
func (tbl *LookupTableSqlite3) lookup(rs *ReteSession, subjectName string, multi bool,
	stmt string, arg any) (*rdf.Node, error) {
	sess := rs.RdfSession
	rmgr := sess.ResourceMgr
	jr := rmgr.JetsResources
	subject := rmgr.NewResource(subjectName)

	// Check if the result of the lookup was already put in the rdf_session by a previous call
	if sess.Contains(tbl.lookupCache, jr.Jets__lookup_row, subject) {
		return subject, nil
	}
	rows, err := tbl.lookupDb.Query(stmt, arg)
	if err != nil {
		return nil, fmt.Errorf("failed query lookup table %s for key %v: %v", tbl.spec.Name, arg, err)
	}
	defer rows.Close()
	count := 0
	resultRow := make([]interface{}, len(tbl.columnsSpec)+1) // the columns, then the rowid
	for rows.Next() {
		for i := range resultRow {
			resultRow[i] = new(interface{})
		}
		if err = rows.Scan(resultRow...); err != nil {
			return nil, fmt.Errorf("while scanning row of lookup table %s for key %v: %v", tbl.spec.Name, arg, err)
		}
		row := subject
		if multi {
			row = rmgr.NewBNode()
		}
		if err = tbl.insertResultIntoSession(sess, row, resultRow); err != nil {
			return nil, err
		}
		count++
		if !multi {
			break
		}
		if _, err = sess.InsertInferred(subject, jr.Jets__lookup_multi_rows, row); err != nil {
			return nil, fmt.Errorf("while InsertInferred in multi lookup: %v", err)
		}
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("while reading lookup table %s for key %v: %v", tbl.spec.Name, arg, err)
	}
	if count == 0 {
		return rdf.Null(), nil
	}
	// Put the result into the cache
	if _, err = sess.InsertInferred(tbl.lookupCache, jr.Jets__lookup_row, subject); err != nil {
		return nil, fmt.Errorf("while InsertInferred of the lookup cache: %v", err)
	}
	return subject, nil
}

// randKey draws a __key__ uniformly from [minKey, maxKey], both included.
func (tbl *LookupTableSqlite3) randKey() int64 {
	return tbl.minKey + rand.Int63n(tbl.maxKey-tbl.minKey+1)
}
