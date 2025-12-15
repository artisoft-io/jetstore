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
		return fmt.Errorf("failed to query jet_rules: %w", err)
	}
	jetRulesData := make([][]any, 0)
	rulePropsData := make([][]any, 0)
	ruleTermsData := make([][]any, 0)
	for _, jetRule := range jetRuleModel.Jetrules {

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
				return fmt.Errorf("failed to find rete node vertex %d for antecedent term in rule %s", rt.Vertex, jetRule.Name)
			}
			ruleTermsData = append(ruleTermsData, []any{maxKey, reteNodeDbKey, true})
		}
		for _, rt := range jetRule.Consequents {
			reteNodeDbKey, ok := w.reteNode2DbKey[rt.UniqueKey()]
			if !ok {
				return fmt.Errorf("failed to find rete node vertex %d for consequent term in rule %s", rt.Vertex, jetRule.Name)
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
		return fmt.Errorf("failed to query rete_nodes: %w", err)
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
		if rn.Type == "root" {
		fmt.Println("rule_term.Type", rn.Type)
			w.reteNode2DbKey[rn.UniqueKey()] = maxReteNodeKey
			reteNodeData = append(reteNodeData, []any{maxReteNodeKey, rn.Vertex, rn.Type,
				0, 0, 0, 0, 0,
				"root vertex", 0, w.mainFileKey, 0, 0, 0})
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
		objectKey := 0
		if rn.ObjectKey != 0 {
			objectKey, ok = w.rm.resourceKeyToDbKey[rn.ObjectKey]
			if !ok {
				return fmt.Errorf("failed to find object resource key %d in rete_nodes @ vertex %d", rn.ObjectKey, rn.Vertex)
			}
		}
		isNot := 0
		if rn.IsNot {
			isNot = 1
		}
		w.reteNode2DbKey[rn.UniqueKey()] = maxReteNodeKey
		var objValue, filterValue, salience int
		if rn.ObjectExpr != nil {
			objValue = rn.ObjectExpr.Value
		}
		if rn.Filter != nil {
			filterValue = rn.Filter.Value
		}
		if len(rn.Salience) > 0 {
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
	fmt.Println("*** ReteNodes Unique Keys to DB Keys Mapping ***")
	for k, v := range w.reteNode2DbKey {
		fmt.Printf("  %s => %d\n", k, v)
	}
	fmt.Println("*** ReteNodes Data ***")
	for _, row := range reteNodeData {
		fmt.Printf("%s:%02d:%02d:%02d => %v\n", row[2], row[1], row[13], row[10], row)
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
		return fmt.Errorf("failed to query expressions: %w", err)
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
// expr is the resource key, so we can call persist directly.
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
		if !w.seenResources[node.Value] {
			*data = append(*data, []any{node.Value, "resource", nil, nil, nil, nil, nil, nil, "", w.mainFileKey})
			w.seenResources[node.Value] = true
		}
	case "unary":
		// Recursively save the argument
		err := w.saveExpression(ctx, data, node.Arg)
		if err != nil {
			return err
		}
		w.maxExprKey++
		node.Value = w.maxExprKey
		*data = append(*data, []any{node.Value, "unary", node.Arg.Value, nil, nil, nil, nil, nil, node.Op, w.mainFileKey})
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
		*data = append(*data, []any{node.Value, "binary", node.Lhs.Value, node.Rhs.Value, nil, nil, nil, nil, node.Op, w.mainFileKey})
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
		return fmt.Errorf("failed to query lookup_tables: %w", err)
	}

	// Insert new entries
	insertStmt := "INSERT INTO lookup_tables (key, name, table_name, csv_file, lookup_key, lookup_resources, source_file_key) VALUES (?, ?, ?, ?, ?, ?, ?)"
	insertColumns := "INSERT INTO lookup_columns (lookup_table_key, name, type, as_array) VALUES (?, ?, ?, ?)"
	data := make([][]any, 0)
	cData := make([][]any, 0)
	for _, lt := range jetRuleModel.LookupTables {
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
	// Delete existing rule sequences for current main file
	deleteStmt := "DELETE FROM rule_sequences WHERE source_file_key = ?"
	_, err := db.ExecContext(ctx, deleteStmt, w.mainFileKey)
	if err != nil {
		return fmt.Errorf("failed to delete existing rule sequences: %w", err)
	}
	deleteStmt = "DELETE FROM main_rule_sets WHERE ruleset_file_key = ?"
	_, err = db.ExecContext(ctx, deleteStmt, w.mainFileKey)
	if err != nil {
		return fmt.Errorf("failed to delete existing main_rule_sets: %w", err)
	}

	maxKey, err := getMaxKey(ctx, db, "rule_sequences")
	if err != nil {
		return fmt.Errorf("failed to query rule_sequences: %w", err)
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

// Save Classes into workspace db
func (w *WorkspaceDB) SaveClassesAndTables(ctx context.Context, db *sql.DB, jetRuleModel *rete.JetruleModel) error {
	// Load existing classes put them in a set and keep tack of the max key
	var maxClassKey int
	className2Key := make(map[string]int)
	newClasses := make(map[string]bool)
	rows, err := db.Query("SELECT key, name FROM domain_classes")
	if err != nil {
		return fmt.Errorf("failed to query domain_classes: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var key int
		var name string
		err = rows.Scan(&key, &name)
		if err != nil {
			return fmt.Errorf("failed to scan class row: %w", err)
		}
		className2Key[name] = key
		if key > maxClassKey {
			maxClassKey = key
		}
	}
	// Insert new classes that are not in className2Key
	classStmt := "INSERT INTO domain_classes (key, name, as_table, source_file_key) VALUES (?, ?, ?, ?)"
	classData := make([][]any, 0, len(jetRuleModel.Classes))
	// Insert classes
	for _, class := range jetRuleModel.Classes {
		if className2Key[class.Name] == 0 {
			fileKey := w.sourceMgr.GetOrAddDbKey(class.SourceFileName)
			maxClassKey++
			className2Key[class.Name] = maxClassKey
			newClasses[class.Name] = true
			classData = append(classData, []any{maxClassKey, class.Name, class.AsTable, fileKey})
		}
	}
	// Execute class insert
	if len(classData) > 0 {
		err = DoStatement(ctx, db, classStmt, classData)
		if err != nil {
			return fmt.Errorf("failed to insert classes: %w", err)
		}
	}

	// Insert data properties for the new classes
	dataPropertiesStmt := "INSERT INTO data_properties (key, domain_class_key, name, type, as_array) VALUES (?, ?, ?, ?, ?)"
	dataPropertiesData := make([][]any, 0)
	var maxDataPropKey int
	dataProperties2Key := make(map[string]int)

	maxDataPropKey, err = getMaxKey(ctx, db, "data_properties")
	if err != nil {
		return fmt.Errorf("failed to query data_properties: %w", err)
	}

	for _, class := range jetRuleModel.Classes {
		if newClasses[class.Name] {
			classKey := className2Key[class.Name]
			for _, dp := range class.DataProperties {
				maxDataPropKey++
				dataProperties2Key[dp.Name] = maxDataPropKey
				dataPropertiesData = append(dataPropertiesData, []any{
					maxDataPropKey,
					classKey,
					dp.Name,
					dp.Type,
					dp.AsArray,
				})
			}
		}
	}
	// Execute data properties insert
	if len(dataPropertiesData) > 0 {
		err = DoStatement(ctx, db, dataPropertiesStmt, dataPropertiesData)
		if err != nil {
			return fmt.Errorf("failed to insert data properties: %w", err)
		}
	}

	// Insert base classes for the new classes
	baseClassStmt := "INSERT INTO base_classes (domain_class_key, base_class_key) VALUES (?, ?)"
	baseClassData := make([][]any, 0, 2*len(jetRuleModel.Classes))
	for _, class := range jetRuleModel.Classes {
		if newClasses[class.Name] {
			// Insert it's base classes
			classKey := className2Key[class.Name]
			for _, baseClass := range class.BaseClasses {
				baseClassData = append(baseClassData, []any{classKey, className2Key[baseClass]})
			}
		}
	}
	// Execute base classes insert
	if len(baseClassData) > 0 {
		err = DoStatement(ctx, db, baseClassStmt, baseClassData)
		if err != nil {
			return fmt.Errorf("failed to insert base classes: %w", err)
		}
	}

	return w.SaveTables(ctx, db, className2Key, dataProperties2Key, jetRuleModel)
}

// Save Tables into workspace db
func (w *WorkspaceDB) SaveTables(ctx context.Context, db *sql.DB, className2Key,
	dataProperties2Key map[string]int, jetRuleModel *rete.JetruleModel) error {

	// Load existing tables put them in a set and keep tack of the max key
	var maxTableKey int
	tableName2Key := make(map[string]int)
	rows, err := db.Query("SELECT key, name FROM domain_tables")
	if err != nil {
		return fmt.Errorf("failed to query domain_tables: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var key int
		var name string
		err = rows.Scan(&key, &name)
		if err != nil {
			return fmt.Errorf("failed to scan table row: %w", err)
		}
		tableName2Key[name] = key
		if key > maxTableKey {
			maxTableKey = key
		}
	}
	// Insert new tables that are not in tableName2Key
	tableStmt := "INSERT INTO domain_tables (key, domain_class_key, name) VALUES (?, ?, ?)"
	tableData := make([][]any, 0, len(jetRuleModel.Tables))
	columnStmt := "INSERT INTO domain_columns (domain_table_key, data_property_key, name, type, as_array) VALUES (?, ?, ?, ?, ?)"
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
			//***
			fmt.Println("*** domain_tables ROW", tableData[len(tableData)-1])

			// Table's columns
			for _, column := range table.Columns {
				dataPropertyKey, ok := dataProperties2Key[column.ColumnName]
				if ok {
					columnData = append(columnData, []any{maxTableKey, dataPropertyKey, column.ColumnName, column.Type, column.AsArray})
				} else {
					return fmt.Errorf("failed to find data property key for column %s in table %s", column.ColumnName, table.TableName)
				}
				//***
				// fmt.Println("*** domain_columns ROW", columnData[len(columnData)-1])
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
