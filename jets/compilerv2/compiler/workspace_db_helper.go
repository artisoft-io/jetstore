package compiler

// This file contains helper methods for saving the compiled workspace to workspace.db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// Function to get max key from table
func getMaxKey(ctx context.Context, db *sql.DB, tableName string) (int, error) {
	var maxKey sql.NullInt64
	query := fmt.Sprintf("SELECT MAX(key) FROM %s", tableName)
	err := db.QueryRowContext(ctx, query).Scan(&maxKey)
	if err != nil && !strings.Contains(err.Error(), "converting NULL to int is unsupported") {
		return 0, fmt.Errorf("failed to get max key from %s: %w", tableName, err)
	}
	if maxKey.Valid {
		return int(maxKey.Int64), nil
	}
	return 0, nil
}

// Function to get key from table based on column name and value
func getKeyByColumn(ctx context.Context, db *sql.DB, tableName, columnName string, columnValue any) (int, error) {
	var key sql.NullInt64
	query := fmt.Sprintf("SELECT key FROM %s WHERE %s = ?", tableName, columnName)
	err := db.QueryRowContext(ctx, query, columnValue).Scan(&key)
	if err != nil && !strings.Contains(err.Error(), "converting NULL to int is unsupported") && err != sql.ErrNoRows {
		return 0, fmt.Errorf("failed to get key from %s where %s = %v: %w", tableName, columnName, columnValue, err)
	}
	if key.Valid {
		return int(key.Int64), nil
	}
	return 0, nil
}

// Save metadata triples into triples table of workspace db
func (w *WorkspaceDB) SaveMetadataTriples(ctx context.Context, db *sql.DB, jetRuleModel *rete.JetruleModel) error {
	// Delete existing metadata triples for current main file
	deleteStmt := "DELETE FROM triples WHERE source_file_key = ?"
	_, err := db.ExecContext(ctx, deleteStmt, w.mainFileKey)
	if err != nil {
		return fmt.Errorf("failed to delete existing metadata triples: %w", err)
	}

	// Insert new metadata triples
	insertStmt := "INSERT INTO triples (subject_key, predicate_key, object_key, source_file_key) VALUES (?, ?, ?, ?)"
	data := make([][]any, 0)
	for _, triple := range jetRuleModel.Triples {
		subjectKey, ok := w.rm.resourceKeyToDbKey[triple.SubjectKey]
		if !ok {
			return fmt.Errorf("failed to find subject resource key %d in metadata triple", triple.SubjectKey)
		}
		predicateKey, ok := w.rm.resourceKeyToDbKey[triple.PredicateKey]
		if !ok {
			return fmt.Errorf("failed to find predicate resource key %d in metadata triple", triple.PredicateKey)
		}
		objectKey, ok := w.rm.resourceKeyToDbKey[triple.ObjectKey]
		if !ok {
			return fmt.Errorf("failed to find object resource key %d in metadata triple", triple.ObjectKey)
		}
		data = append(data, []any{subjectKey, predicateKey, objectKey, w.mainFileKey})
	}
	if len(data) > 0 {
		err = DoStatement(ctx, db, insertStmt, data)
		if err != nil {
			return fmt.Errorf("failed to insert metadata triples: %w", err)
		}
	}
	return nil
}

// Save Rules to jet_rules, rule_properties, and rule_terms tables of workspace db
func (w *WorkspaceDB) SaveJetRules(ctx context.Context, db *sql.DB, jetRuleModel *rete.JetruleModel) error {
	// Delete existing rules for current main file
	var deleteStmt string
	var err error
	deleteStmt = "DELETE FROM rule_properties WHERE rule_key IN (SELECT key FROM jet_rules WHERE source_file_key = ?)"
	_, err = db.ExecContext(ctx, deleteStmt, w.mainFileKey)
	if err != nil {
		return fmt.Errorf("failed to delete existing rule_properties: %w", err)
	}
	deleteStmt = "DELETE FROM rule_terms WHERE rule_key IN (SELECT key FROM jet_rules WHERE source_file_key = ?)"
	_, err = db.ExecContext(ctx, deleteStmt, w.mainFileKey)
	if err != nil {
		return fmt.Errorf("failed to delete existing rule_terms: %w", err)
	}
	deleteStmt = "DELETE FROM jet_rules WHERE source_file_key = ?"
	_, err = db.ExecContext(ctx, deleteStmt, w.mainFileKey)
	if err != nil {
		return fmt.Errorf("failed to delete existing jet_rules: %w", err)
	}

	// Insert new rules
	ruleInsertStmt := "INSERT INTO jet_rules (key, name, optimization, salience, authored_label, normalized_label, label, source_file_key) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"
	rulePropInsertStmt := "INSERT INTO rule_properties (rule_key, name, value) VALUES (?, ?, ?)"
	ruleTermInsertStmt := "INSERT INTO rule_terms (rule_key, rete_node_key, is_antecedent) VALUES (?, ?, ?)"

	// Prepare data for insertion
	maxKey, err := getMaxKey(ctx, db, "jet_rules")
	if err != nil {
		return err
	}
	jetRulesData := make([][]any, 0)
	rulePropsData := make([][]any, 0)
	ruleTermsData := make([][]any, 0)
	for _, jetRule := range jetRuleModel.Jetrules {
		if w.sourceMgr.IsPreExisting(jetRule.SourceFileName) {
			continue
		}

		// JetRule
		maxKey++
		jetRulesData = append(jetRulesData, []any{maxKey, jetRule.Name, jetRule.Optimization, jetRule.Salience,
			jetRule.AuthoredLabel, jetRule.NormalizedLabel, jetRule.Label, w.mainFileKey})

		// Rule Properties
		for propName, propValue := range jetRule.Properties {
			rulePropsData = append(rulePropsData, []any{maxKey, propName, propValue})
		}

		// Rule Terms
		for _, rt := range jetRule.Antecedents {
			reteNodeDbKey, ok := w.reteNode2DbKey[rt.UniqueKey()]
			if !ok {
				return fmt.Errorf("failed to find rete node vertex %d (ukey: %s) for antecedent term %s", rt.Vertex, rt.UniqueKey(), rt.NormalizedLabel)
			}
			ruleTermsData = append(ruleTermsData, []any{maxKey, reteNodeDbKey, true})
		}
		for _, rt := range jetRule.Consequents {
			// fmt.Printf("*** Saving Rule %s, consequent %s %s\n", jetRule.Name, rt.UniqueKey(), rt.NormalizedLabel)
			reteNodeDbKey, ok := w.reteNode2DbKey[rt.UniqueKey()]
			if !ok {
				return fmt.Errorf("failed to find rete node vertex %d (ukey: %s) for consequent term %s", rt.Vertex, rt.UniqueKey(), rt.NormalizedLabel)
			}
			ruleTermsData = append(ruleTermsData, []any{maxKey, reteNodeDbKey, false})
		}
	}
	if len(jetRulesData) > 0 {
		err = DoStatement(ctx, db, ruleInsertStmt, jetRulesData)
		if err != nil {
			return fmt.Errorf("failed to insert jet_rules: %w", err)
		}
	}
	if len(rulePropsData) > 0 {
		err = DoStatement(ctx, db, rulePropInsertStmt, rulePropsData)
		if err != nil {
			return fmt.Errorf("failed to insert rule_properties: %w", err)
		}
	}
	if len(ruleTermsData) > 0 {
		err = DoStatement(ctx, db, ruleTermInsertStmt, ruleTermsData)
		if err != nil {
			return fmt.Errorf("failed to insert rule_terms: %w", err)
		}
	}
	return nil
}

// Save Rete Nodes and beta_row_config into workspace db
func (w *WorkspaceDB) SaveReteNodes(ctx context.Context, db *sql.DB, jetRuleModel *rete.JetruleModel) error {
	// Delete existing rete_nodes for current main file
	reteNodeDeleteStmt := "DELETE FROM rete_nodes WHERE source_file_key = ?"
	_, err := db.ExecContext(ctx, reteNodeDeleteStmt, w.mainFileKey)
	if err != nil {
		return fmt.Errorf("failed to delete rete_nodes tables: %w", err)
	}
	maxReteNodeKey, err := getMaxKey(ctx, db, "rete_nodes")
	if err != nil {
		return err
	}
	reteNodeInsertStmt := "INSERT INTO rete_nodes (key, vertex, type, " +
		"subject_key, predicate_key, object_key, obj_expr_key, filter_expr_key, " +
		"normalizedLabel, parent_vertex, source_file_key, is_negation, salience, consequent_seq) " +
		"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"

	// Delete existing beta_row_config for current main file
	betaRowDeleteStmt := "DELETE FROM beta_row_config WHERE source_file_key = ?"
	_, err = db.ExecContext(ctx, betaRowDeleteStmt, w.mainFileKey)
	if err != nil {
		return fmt.Errorf("failed to delete beta_row_config tables: %w", err)
	}
	maxBetaRowKey, err := getMaxKey(ctx, db, "beta_row_config")
	if err != nil {
		return fmt.Errorf("failed to query beta_row_config: %w", err)
	}
	betaRowInsertStmt := "INSERT INTO beta_row_config (key, vertex, seq, source_file_key, row_pos, is_binded, id)" +
		"VALUES (?, ?, ?, ?, ?, ?, ?)"
	// Prepare data for insertion
	reteNodeData := make([][]any, 0)
	betaRowData := make([][]any, 0)
	for _, rn := range jetRuleModel.ReteNodes {
		// Rete Node
		maxReteNodeKey++
		if rn.Type == "head_node" {
			w.reteNode2DbKey[rn.UniqueKey()] = maxReteNodeKey
			reteNodeData = append(reteNodeData, []any{maxReteNodeKey, rn.Vertex, rn.Type,
				nil, nil, nil, nil, nil,
				nil, 0, w.mainFileKey, nil, nil, 0})
			continue
		}
		subjectKey, ok := w.rm.resourceKeyToDbKey[rn.SubjectKey]
		if !ok {
			return fmt.Errorf("failed to find subject resource key %d in rete_nodes @ vertex %d", rn.SubjectKey, rn.Vertex)
		}
		predicateKey, ok := w.rm.resourceKeyToDbKey[rn.PredicateKey]
		if !ok {
			return fmt.Errorf("failed to find predicate resource key %d in rete_nodes @ vertex %d", rn.PredicateKey, rn.Vertex)
		}
		var objectKey any
		if rn.ObjectKey != 0 {
			objectKey, ok = w.rm.resourceKeyToDbKey[rn.ObjectKey]
			if !ok {
				return fmt.Errorf("failed to find object resource key %d in rete_nodes @ vertex %d", rn.ObjectKey, rn.Vertex)
			}
		}
		var isNot any
		if rn.Type == "antecedent" {
			isNot = rn.IsNot
		}
		w.reteNode2DbKey[rn.UniqueKey()] = maxReteNodeKey
		var objValue, filterValue, salience any
		if rn.ObjectExpr != nil {
			objValue = rn.ObjectExprKey
		}
		if rn.Filter != nil {
			filterValue = rn.FilterKey
		}
		if len(rn.Salience) > 0 && rn.Salience[0] > 0 {
	 		salience = rn.Salience[0]
		}
		reteNodeData = append(reteNodeData, []any{maxReteNodeKey, rn.Vertex, rn.Type,
			subjectKey, predicateKey, objectKey, objValue, filterValue,
			rn.NormalizedLabel, rn.ParentVertex, w.mainFileKey, isNot, salience, rn.ConsequentSeq})

		// Beta Row Config
		for seq, br := range rn.BetaVarNodes {
			maxBetaRowKey++
			isBinded := 0
			if br.IsBinded {
				isBinded = 1
			}
			betaRowData = append(betaRowData, []any{maxBetaRowKey, rn.Vertex, seq, w.mainFileKey,
				br.VarPos, isBinded, br.Id})
		}
	}

	if len(reteNodeData) > 0 {
		err = DoStatement(ctx, db, reteNodeInsertStmt, reteNodeData)
		if err != nil {
			return fmt.Errorf("failed to insert rete_nodes: %w", err)
		}
	}
	if len(betaRowData) > 0 {
		err = DoStatement(ctx, db, betaRowInsertStmt, betaRowData)
		if err != nil {
			return fmt.Errorf("failed to insert beta_row_config: %w", err)
		}
	}
	return nil
}

// Save Expresions into workspace db based on filters and object expressions
func (w *WorkspaceDB) SaveExpressions(ctx context.Context, db *sql.DB, jetRuleModel *rete.JetruleModel) error {
	deleteStmt := "DELETE FROM expressions WHERE source_file_key = ?"
	_, err := db.ExecContext(ctx, deleteStmt, w.mainFileKey)
	if err != nil {
		return fmt.Errorf("failed to delete expressions tables: %w", err)
	}
	w.maxExprKey, err = getMaxKey(ctx, db, "expressions")
	if err != nil {
		return err
	}
	insertStmt := "INSERT INTO expressions (key, type, arg0_key, arg1_key, arg2_key, arg3_key, " +
		"arg4_key, arg5_key, op, source_file_key) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"

	data := make([][]any, 0)
	for _, rn := range jetRuleModel.ReteNodes {
		if rn.Filter != nil {
			err = w.saveExpression(ctx, &data, rn.Filter)
			if err != nil {
				return err
			}
			rn.FilterKey = rn.Filter.Value
		}
		if rn.ObjectExpr != nil {
			err = w.saveExpression(ctx, &data, rn.ObjectExpr)
			if err != nil {
				return err
			}
			rn.ObjectExprKey = rn.ObjectExpr.Value
		}
	}
	if len(data) > 0 {
		err = DoStatement(ctx, db, insertStmt, data)
		if err != nil {
			return fmt.Errorf("failed to insert expressions: %w", err)
		}
	}
	return nil
}

// Save a single expression into workspace db recursively.
// Add expression to expressions table recursivelly and return the key
// Put resource entities as well: resource (constant) and var (binded)
func (w *WorkspaceDB) saveExpression(ctx context.Context, data *[][]any, node *rete.ExpressionNode) error {
	var ok bool
	if node == nil {
		return nil
	}
	switch node.Type {
	case "identifier":
		// Case resource (constant) and var (binded)
		node.Value, ok = w.rm.resourceKeyToDbKey[node.Value]
		if !ok {
			return fmt.Errorf("failed to find resource key %d in expression", node.Value)
		}
		w.maxExprKey++
		*data = append(*data, []any{w.maxExprKey, "resource", node.Value, nil, nil, nil, nil, nil, nil, w.mainFileKey})
		node.Value = w.maxExprKey
	case "unary":
		// Recursively save the argument
		err := w.saveExpression(ctx, data, node.Arg)
		if err != nil {
			return err
		}
		w.maxExprKey++
		node.Value = w.maxExprKey
		*data = append(*data, []any{w.maxExprKey, "unary", node.Arg.Value, nil, nil, nil, nil, nil, node.Op, w.mainFileKey})
	case "binary":
		// Recursively save lhs and rhs
		err := w.saveExpression(ctx, data, node.Lhs)
		if err != nil {
			return err
		}
		err = w.saveExpression(ctx, data, node.Rhs)
		if err != nil {
			return err
		}
		w.maxExprKey++
		node.Value = w.maxExprKey
		*data = append(*data, []any{w.maxExprKey, "binary", node.Lhs.Value, node.Rhs.Value, nil, nil, nil, nil, node.Op, w.mainFileKey})
	}
	return nil
}

// Save Lookup Tables into workspace db
func (w *WorkspaceDB) SaveLookupTables(ctx context.Context, db *sql.DB, jetRuleModel *rete.JetruleModel) error {
	// Delete existing lookup tables for current main file
	deleteStmt := "DELETE FROM lookup_tables WHERE source_file_key = ?"
	_, err := db.ExecContext(ctx, deleteStmt, w.mainFileKey)
	if err != nil {
		return fmt.Errorf("failed to delete existing lookup tables: %w", err)
	}

	maxKey, err := getMaxKey(ctx, db, "lookup_tables")
	if err != nil {
		return err
	}

	// Insert new entries
	insertStmt := "INSERT INTO lookup_tables (key, name, table_name, csv_file, lookup_key, lookup_resources, source_file_key) VALUES (?, ?, ?, ?, ?, ?, ?)"
	insertColumns := "INSERT INTO lookup_columns (lookup_table_key, name, type, as_array) VALUES (?, ?, ?, ?)"
	data := make([][]any, 0)
	cData := make([][]any, 0)
	for _, lt := range jetRuleModel.LookupTables {
		dbKey, err := getKeyByColumn(ctx, db, "lookup_tables", "name", lt.Name)
		if err != nil {
			return fmt.Errorf("failed to query lookup_tables by name: %w", err)
		}
		if dbKey != 0 {
			continue
		}
		maxKey++
		data = append(data, []any{maxKey, lt.Name, nil, lt.CsvFile, strings.Join(lt.Key, ","), strings.Join(lt.Resources, ","), w.mainFileKey})
		for _, col := range lt.Columns {
			cData = append(cData, []any{maxKey, col.Name, col.Type, col.IsArray})
		}
	}
	if len(data) > 0 {
		err = DoStatement(ctx, db, insertStmt, data)
		if err != nil {
			return fmt.Errorf("failed to insert lookup_tables: %w", err)
		}
		err = DoStatement(ctx, db, insertColumns, cData)
		if err != nil {
			return fmt.Errorf("failed to insert lookup_columns: %w", err)
		}
	}
	return nil
}

// Save Rule Sequences into workspace db
func (w *WorkspaceDB) SaveRuleSequences(ctx context.Context, db *sql.DB, jetRuleModel *rete.JetruleModel) error {
	// // Delete existing rule sequences for current main file
	// deleteStmt := "DELETE FROM rule_sequences WHERE source_file_key = ?"
	// _, err := db.ExecContext(ctx, deleteStmt, w.mainFileKey)
	// if err != nil {
	// 	return fmt.Errorf("failed to delete existing rule sequences: %w", err)
	// }
	// deleteStmt = "DELETE FROM main_rule_sets WHERE ruleset_file_key = ?"
	// _, err = db.ExecContext(ctx, deleteStmt, w.mainFileKey)
	// if err != nil {
	// 	return fmt.Errorf("failed to delete existing main_rule_sets: %w", err)
	// }

	maxKey, err := getMaxKey(ctx, db, "rule_sequences")
	if err != nil {
		return err
	}

	// Insert new entries
	insertStmt := "INSERT INTO rule_sequences (key, name, source_file_key) VALUES (?, ?, ?)"
	insertMRS := "INSERT INTO main_rule_sets (rule_sequence_key, main_ruleset_name, ruleset_file_key, seq) VALUES (?, ?, ?, ?)"
	rsData := make([][]any, 0)
	mrsData := make([][]any, 0)
	for _, rs := range jetRuleModel.RuleSequences {
		maxKey++
		rsData = append(rsData, []any{maxKey, rs.Name, w.mainFileKey})
		for seq, rsName := range rs.RuleSets {
			if w.sourceMgr.IsPreExisting(rsName) {
				continue
			}
			ruleSetFileKey := w.sourceMgr.GetOrAddDbKey(rsName)
			mrsData = append(mrsData, []any{maxKey, rsName, ruleSetFileKey, seq})
		}
	}
	if len(rsData) > 0 {
		err = DoStatement(ctx, db, insertStmt, rsData)
		if err != nil {
			return fmt.Errorf("failed to insert rule_sequences: %w", err)
		}
		err = DoStatement(ctx, db, insertMRS, mrsData)
		if err != nil {
			return fmt.Errorf("failed to insert main_rule_sets: %w", err)
		}
	}
	return nil
}

// Save Jetstore Config into workspace db
func (w *WorkspaceDB) SaveJetstoreConfig(ctx context.Context, db *sql.DB, jetRuleModel *rete.JetruleModel) error {
	// Delete existing config for current main file
	deleteStmt := "DELETE FROM jetstore_config WHERE source_file_key = ?"
	_, err := db.ExecContext(ctx, deleteStmt, w.mainFileKey)
	if err != nil {
		return fmt.Errorf("failed to delete existing jetstore_config: %w", err)
	}

	// Insert new config entries
	insertStmt := "INSERT INTO jetstore_config (source_file_key, config_key, config_value) VALUES (?, ?, ?)"
	data := make([][]any, 0)
	for key, value := range jetRuleModel.JetstoreConfig {
		data = append(data, []any{w.mainFileKey, key, value})
	}
	if len(data) > 0 {
		err = DoStatement(ctx, db, insertStmt, data)
		if err != nil {
			return fmt.Errorf("failed to insert jetstore_config: %w", err)
		}
	}
	return nil
}

func getKeyNameFromTable(_ context.Context, db *sql.DB, tableName, keyColumn, nameColumn string) (map[string]int, error) {
	result := make(map[string]int)
	rows, err := db.Query(fmt.Sprintf("SELECT %s, %s FROM %s", keyColumn, nameColumn, tableName))
	if err != nil {
		return nil, fmt.Errorf("failed to query %s: %w", tableName, err)
	}
	defer rows.Close()
	for rows.Next() {
		var key int
		var name string
		err = rows.Scan(&key, &name)
		if err != nil {
			return nil, fmt.Errorf("failed to scan %s row: %w", tableName, err)
		}
		result[name] = key
	}
	return result, nil
}

// Save Classes into workspace db
func (w *WorkspaceDB) SaveClassesAndTables(ctx context.Context, db *sql.DB, jetRuleModel *rete.JetruleModel) error {
	// Load existing classes put them in a set since they are referenced as base classes to new classes
	className2Key, err := getKeyNameFromTable(ctx, db, "domain_classes", "key", "name")
	if err != nil {
		return err
	}
	maxClassKey, err := getMaxKey(ctx, db, "domain_classes")
	if err != nil {
		return err
	}

	// Load existing data properties since they are reference by Domain Tables
	dataProperties2Key, err := getKeyNameFromTable(ctx, db, "data_properties", "key", "name")
	if err != nil {
		return err
	}
	maxDataPropKey, err := getMaxKey(ctx, db, "data_properties")
	if err != nil {
		return err
	}

	// The insert stmts
	classStmt := "INSERT INTO domain_classes (key, name, as_table, source_file_key) VALUES (?, ?, ?, ?)"
	dataPropertiesStmt := "INSERT INTO data_properties (key, domain_class_key, name, type, as_array) VALUES (?, ?, ?, ?, ?)"
	baseClassStmt := "INSERT INTO base_classes (domain_class_key, base_class_key) VALUES (?, ?)"

	// Insert the new classes, it's data properties and base classes
	classData := make([][]any, 0, len(jetRuleModel.Classes))
	dataPropertiesData := make([][]any, 0)
	baseClassData := make([][]any, 0, 2*len(jetRuleModel.Classes))

	// Initialize classData with owl:Thing if className2Key is empty (no classes entered yet in db)
	if len(className2Key) == 0 {
		maxClassKey++
		className2Key["owl:Thing"] = maxClassKey
		classData = append(classData, []any{maxClassKey, "owl:Thing", 0, -1})
	}

	// Insert classes
	for _, class := range jetRuleModel.Classes {
		if className2Key[class.Name] == 0 {
			// New class
			fileKey := w.sourceMgr.GetOrAddDbKey(class.SourceFileName)
			maxClassKey++
			className2Key[class.Name] = maxClassKey
			classData = append(classData, []any{maxClassKey, class.Name, class.AsTable, fileKey})
			// It's data properties
			for _, dp := range class.DataProperties {
				maxDataPropKey++
				dataProperties2Key[dp.Name] = maxDataPropKey
				dataPropertiesData = append(dataPropertiesData, []any{
					maxDataPropKey,
					maxClassKey,
					dp.Name,
					dp.Type,
					dp.AsArray,
				})
			}
			// Insert it's base classes
			for _, baseClass := range class.BaseClasses {
				baseClsKey, ok := className2Key[baseClass]
				if !ok {
					return fmt.Errorf("failed to find db key for base class %s of class %s", baseClass, class.Name)
				}
				baseClassData = append(baseClassData, []any{maxClassKey, baseClsKey})
			}
		}
	}

	// Execute the insert stmt
	if len(classData) > 0 {
		err = DoStatement(ctx, db, classStmt, classData)
		if err != nil {
			return fmt.Errorf("failed to insert classes: %w", err)
		}
	}
	if len(dataPropertiesData) > 0 {
		err = DoStatement(ctx, db, dataPropertiesStmt, dataPropertiesData)
		if err != nil {
			return fmt.Errorf("failed to insert data properties: %w", err)
		}
	}
	if len(baseClassData) > 0 {
		err = DoStatement(ctx, db, baseClassStmt, baseClassData)
		if err != nil {
			return fmt.Errorf("failed to insert base classes: %w", err)
		}
	}

	return w.SaveTables(ctx, db, className2Key, dataProperties2Key, jetRuleModel)
}

// Save Tables into workspace db
func (w *WorkspaceDB) SaveTables(ctx context.Context, db *sql.DB,
	className2Key, dataProperties2Key map[string]int, jetRuleModel *rete.JetruleModel) error {

	// Load existing tables put them in a set and keep tack of the max key
	tableName2Key, err := getKeyNameFromTable(ctx, db, "domain_tables", "key", "name")
	if err != nil {
		return err
	}
	maxTableKey, err := getMaxKey(ctx, db, "domain_tables")
	if err != nil {
		return err
	}

	// Insert new tables that are not in tableName2Key
	tableStmt := "INSERT INTO domain_tables (key, domain_class_key, name) VALUES (?, ?, ?)"
	columnStmt := "INSERT INTO domain_columns (domain_table_key, data_property_key, name, type, as_array) VALUES (?, ?, ?, ?, ?)"
	tableData := make([][]any, 0, len(jetRuleModel.Tables))
	columnData := make([][]any, 0)

	// Insert tables & table columns
	for _, table := range jetRuleModel.Tables {
		if tableName2Key[table.TableName] == 0 {
			maxTableKey++
			tableName2Key[table.TableName] = maxTableKey
			classKey, ok := className2Key[table.ClassName]
			if ok {
				tableData = append(tableData, []any{maxTableKey, classKey, table.TableName})
			} else {
				return fmt.Errorf("failed to find class key for table %s", table.TableName)
			}

			// Table's columns
			for _, column := range table.Columns {
				dataPropertyKey, ok := dataProperties2Key[column.ColumnName]
				if ok {
					columnData = append(columnData, []any{maxTableKey, dataPropertyKey, column.ColumnName, column.Type, column.AsArray})
				} else {
					return fmt.Errorf("failed to find data property key for column %s in table %s", column.ColumnName, table.TableName)
				}
			}
		}
	}
	// Execute table insert
	if len(tableData) > 0 {
		err = DoStatement(ctx, db, tableStmt, tableData)
		if err != nil {
			return fmt.Errorf("failed to insert tables: %w", err)
		}
	}

	// Execute column insert
	if len(columnData) > 0 {
		err = DoStatement(ctx, db, columnStmt, columnData)
		if err != nil {
			return fmt.Errorf("failed to insert columns: %w", err)
		}
	}
	return nil
}
