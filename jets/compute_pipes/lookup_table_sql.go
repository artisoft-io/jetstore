package compute_pipes

import (
	"context"
	"database/sql"
	"fmt"
	// "log"
	"strings"

	"github.com/jackc/pgx/v4/pgxpool"
)

// Lookup table in memory loaded from jetstore db

// data is the mapping of the looup key -> values
// columnsMap is the mapping of the return column name -> position in the returned row (values)
type LookupTableSql struct {
	spec       *LookupSpec
	data       map[string]*[]interface{}
	columnsMap map[string]int
}

func NewLookupTableSql(dbpool *pgxpool.Pool, spec *LookupSpec, env map[string]interface{}) (LookupTable, error) {
	tbl := &LookupTableSql{
		spec:       spec,
		data:       make(map[string]*[]interface{}),
		columnsMap: make(map[string]int),
	}
	// load the lookup table according to spec
	// Issue the query and get the returned columns info
	// Perform parameter substitution to the query
	for k,v := range env {
		if strings.Contains(spec.Query, k) {
			str, ok := v.(string)
			if !ok {
				str = fmt.Sprintf("%v", v)
			}
			spec.Query = strings.ReplaceAll(spec.Query, k, str)
		}
	}
	// log.Println("NewLookupTableSql query is:\n", spec.Query)
	rows, err := dbpool.Query(context.Background(), spec.Query)
	if err != nil {
		return nil, fmt.Errorf("while querying the lookup table %s: %v", spec.Key, err)
	}
	defer rows.Close()
	// keep track of the column name and their pos in the returned row
	// create a slice that will be used to scan the data
	columnsPos := make(map[string]int)
	fd := rows.FieldDescriptions()
	// Check that the spec has all the returned columns from the query
	if len(fd) != len(spec.Columns) {
		return nil, fmt.Errorf(
			"error: lookup table spec column length does not match the nbr of columns returned by the query for table %s",
			spec.Key)
	}
	columns := make([]interface{}, len(fd))
	for i := range fd {
		columName := string(fd[i].Name)
		columnsPos[columName] = i
		switch spec.Columns[i].RdfType {
		case "text", "string":
			columns[i] = &sql.NullString{}
		case "int", "int32", "integer":
			columns[i] = &sql.NullInt32{}
		case "long", "int64":
			columns[i] = &sql.NullInt64{}
		case "double", "float64", "double precision":
			columns[i] = &sql.NullFloat64{}
		case "date", "datetime":
			columns[i] = &sql.NullTime{}
		}
	}
	// Keep a mapping of the returned column names to their position in the returned row
	for i, valueColumn := range spec.LookupValues {
		tbl.columnsMap[valueColumn] = i
	}
	// scan the rows and make a map of key -> values
	keys := make([]string, len(spec.LookupKey))
	for rows.Next() {
		if err = rows.Scan(columns...); err != nil {
			return nil, fmt.Errorf("while scanning the row for lookup table %s: %v", spec.Key, err)
		}
		// If a key component is null, the corresponding key component will be the empty string
		for i, key := range spec.LookupKey {
			pos, ok := columnsPos[key]
			if !ok {
				return nil, fmt.Errorf("error: key column '%s' is not in the query of lookup table %s", key, spec.Key)
			}
			value, ok := columns[pos].(*sql.NullString)
			if !ok {
				return nil, fmt.Errorf("error: key column '%s' is not a string for lookup table %s", key, spec.Key)
			}
			keys[i] = value.String
		}
		lookupKey := strings.Join(keys, "")
		// the associated values
		values := make([]interface{}, len(spec.LookupValues))
		for i, valueColumn := range spec.LookupValues {
			pos, ok := columnsPos[valueColumn]
			if !ok {
				return nil, fmt.Errorf("error: value column '%s' is not in the query of lookup table %s", valueColumn, spec.Key)
			}
			switch vv := columns[pos].(type) {
			case *sql.NullString:
				if vv.Valid {
					values[i] = vv.String
				}
			case *sql.NullInt32:
				if vv.Valid {
					values[i] = vv.Int32
				}
			case *sql.NullInt64:
				if vv.Valid {
					values[i] = vv.Int64
				}
			case *sql.NullFloat64:
				if vv.Valid {
					values[i] = vv.Float64
				}
			case *sql.NullTime:
				if vv.Valid {
					values[i] = vv.Time
				}
			}
		}
		// save the lookup row
		tbl.data[lookupKey] = &values
	}
	return tbl, nil
}

func (tbl *LookupTableSql) Lookup(key *string) (*[]interface{}, error) {
	if key == nil {
		return nil, fmt.Errorf("error: cannot do a lookup with a null key for lookup table %s", tbl.spec.Key)
	}
	return tbl.data[*key], nil
}

func (tbl *LookupTableSql) LookupValue(row *[]interface{}, columnName string) (interface{}, error) {
	pos, ok := tbl.columnsMap[columnName]
	if !ok {
		return nil, fmt.Errorf("error: column named %s is not a column returned by the lookup table %s",
			columnName, tbl.spec.Key)
	}
	return (*row)[pos], nil
}

func (tbl *LookupTableSql) ColumnMap() map[string]int {
	return tbl.columnsMap
}
