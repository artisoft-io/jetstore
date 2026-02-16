package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/artisoft-io/jetstore/jets/compilerv2/compiler"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/artisoft-io/jetstore/jets/run_reports/tarextract"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Workspace compilation function v2

func compileWorkspaceV2(dbpool *pgxpool.Pool, workspaceControl *rete.WorkspaceControl, version string) (string, error) {

	var jrCompiler *compiler.Compiler
	var err error
	var buf strings.Builder
	workspaceName := workspaceControl.WorkspaceName

	// Make the build directory if not exists
	buildDir := fmt.Sprintf("%s/%s/build", workspaceHome, workspaceName)
	err = os.MkdirAll(buildDir, 0770)
	if err != nil {
		return "", fmt.Errorf("while creating build directory: %v", err)
	}

	// Remove existing workspace.db if exists
	err = os.Remove(fmt.Sprintf("%s/%s/workspace.db", workspaceHome, workspaceName))
	if err != nil {
		log.Printf("failed to remove existing workspace.db: %v", err)
	}

	// Compile the workspace locally
	var fullModel *rete.JetruleModel
	fmt.Fprintf(&buf, "(Compiler V2) Compiling workspace %s at version %s\n", workspaceName, version)

	// b,_ := json.Marshal(workspaceControl)
	// fmt.Fprintf(&buf, "Workspace control:\n%s\n", string(b))
	
	// For  each main rule files in workspace control, create a compiler instance
	// and compile the rule file
	// Collect the set of main rule files
	mainRuleFiles := make(map[string]bool)
	for i := range workspaceControl.RuleSets {
		mainRuleFiles[workspaceControl.RuleSets[i]] = true
	}
	// Verify the integrity of workspace_control.json:
	// Make sure all rule sets in rule sequences exist in rule sets
	for i := range workspaceControl.RuleSequences {
		for j := range workspaceControl.RuleSequences[i].RuleSets {
			if _, ok := mainRuleFiles[workspaceControl.RuleSequences[i].RuleSets[j]]; !ok {
				return "", fmt.Errorf("rule set '%s' in rule sequence '%s' not found in workspace control rule sets",
					workspaceControl.RuleSequences[i].RuleSets[j], workspaceControl.RuleSequences[i].Name)
			}
		}
	}
	// Keep reference to all classes, tables, and lookup tables
	var classes []*rete.ClassNode
	var tables []*rete.TableNode
	var lookupTables []*rete.LookupTableNode

	workspacePath := fmt.Sprintf("%s/%s", workspaceHome, workspaceName)
	for name := range mainRuleFiles {
		// name is the file path relative to workspace home
		fmt.Fprintf(&buf, "Compiling rule file: %s\n", name)
		jrCompiler = compiler.NewCompiler(
			workspacePath, name /*saveJson*/, true, workspaceControl.UseTraceMode,
			workspaceControl.AutoAddResources)
		err = jrCompiler.Compile()
		if err != nil {
			log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
			buf.WriteString(jrCompiler.ParseLog().String())
			buf.WriteString(jrCompiler.ErrorLog().String())
			cmdLog := buf.String()
			log.Println(cmdLog)
			log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
			return cmdLog, fmt.Errorf("while compiling rule file '%s': %v", name, err)
		}
		buf.WriteString(jrCompiler.ParseLog().String())

		// Collect the classes, tables, and lookup tables
		classes = append(classes, jrCompiler.JetRuleModel().Classes...)
		tables = append(tables, jrCompiler.JetRuleModel().Tables...)
		lookupTables = append(lookupTables, jrCompiler.JetRuleModel().LookupTables...)

		// Split the compiled model into '.rete.json',  '.model.json' and  '.triples.json '
		// into the build directory

		// Save the rete network in .rete.json
		fullModel = jrCompiler.JetRuleModel()
		reteModel := &rete.JetruleModel{
			MainRuleFileName: fullModel.MainRuleFileName,
			Resources:        fullModel.Resources,
			LookupTables:     fullModel.LookupTables,
			ReteNodes:        fullModel.ReteNodes,
		}
		// Save the rete network to build directory
		// name is the file path relative to workspace home
		fpath := fmt.Sprintf("%s/%s/build/%s.rete.json", workspaceHome,
			wprefix, strings.TrimSuffix(name, ".jr"))
		// Make sure the directory exists
		err = os.MkdirAll(filepath.Dir(fpath), 0770)
		if err != nil {
			return "", fmt.Errorf("while creating build sub-directory: %v", err)
		}
		// log.Println("Writing JetStore Rete Network of", name, "to:", fpath)
		file, err := os.OpenFile(fpath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			err = fmt.Errorf("while opening .rete.json for write (compile_workspace_v2):%v", err)
			return err.Error(), err
		}
		encoder := json.NewEncoder(file)
		encoder.Encode(reteModel)
		file.Close()

		// Save classes and tables in .model.json
		clsTblModel := &rete.JetruleModel{
			MainRuleFileName: fullModel.MainRuleFileName,
			Classes:          fullModel.Classes,
			Tables:           fullModel.Tables,
		}
		// Save in the build directory
		fpath = fmt.Sprintf("%s/%s/build/%s.model.json", workspaceHome,
			wprefix, strings.TrimSuffix(name, ".jr"))
		// log.Println("Writing JetStore Classes and Tables Model of", name, "to:", fpath)
		file, err = os.OpenFile(fpath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			err = fmt.Errorf("while opening .model.json for write (compile_workspace_v2):%v", err)
			return err.Error(), err
		}
		encoder = json.NewEncoder(file)
		encoder.Encode(clsTblModel)
		file.Close()

		// Save rule config in .config.json
		ruleConfig := &rete.JetruleModel{
			MainRuleFileName: fullModel.MainRuleFileName,
			JetstoreConfig:   fullModel.JetstoreConfig,
		}
		// Save in the build directory
		fpath = fmt.Sprintf("%s/%s/build/%s.config.json", workspaceHome,
			wprefix, strings.TrimSuffix(name, ".jr"))
		log.Println("Writing JetStore rule config of", name, "to:", fpath)
		file, err = os.OpenFile(fpath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			err = fmt.Errorf("while opening .config.json for write (compile_workspace_v2):%v", err)
			return err.Error(), err
		}
		encoder = json.NewEncoder(file)
		encoder.Encode(ruleConfig)
		file.Close()

		// Save triples in .triples.json
		tripleModel := &rete.JetruleModel{
			MainRuleFileName: fullModel.MainRuleFileName,
			Triples:          fullModel.Triples,
		}
		// Save in the build directory
		fpath = fmt.Sprintf("%s/%s/build/%s.triples.json", workspaceHome,
			wprefix, strings.TrimSuffix(name, ".jr"))
		// log.Println("Writing JetStore Triples Model of", name, "to:", fpath)
		file, err = os.OpenFile(fpath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			err = fmt.Errorf("while opening .triples.json for write (compile_workspace_v2):%v", err)
			return err.Error(), err
		}
		encoder = json.NewEncoder(file)
		encoder.Encode(tripleModel)
		file.Close()
	}

	// Add all rule sequences
	buf.WriteString("Add rule sequences to workspace.db\n")
	wdb, err := compiler.NewWorkspaceDB(context.TODO(), workspacePath)
	if err != nil {
		return buf.String(), fmt.Errorf("while creating workspace.db: %w", err)
	}
	wcPath := fmt.Sprintf("%s/workspace_control.json", workspacePath)
	workspaceControl, err = rete.LoadWorkspaceControl(wcPath)
	if err != nil {
		return buf.String(), fmt.Errorf("while loading workspace control: %w", err)
	}
	err = wdb.SaveRuleSequences(context.TODO(), wdb.DB, workspaceControl)
	if err != nil {
		return buf.String(), fmt.Errorf("failed to save rule sequences: %w", err)
	}

	// All files are now compiled
	buf.WriteString("All main rule files compiled, now package the lookup.db file\n")
	err = PackageLookupTablesToSqlite(lookupTables)
	if err != nil {
		log.Println("Error packaging lookup tables to SQLite:", err)
		return buf.String(), err
	}
	buf.WriteString("Lookup tables packaged into lookup.db\n")

	// Archive reports
	inputPath := []string{fmt.Sprintf("%s/%s/reports/", workspaceHome, workspaceName)}
	outputPath := fmt.Sprintf("%s/%s/reports.tgz", workspaceHome, workspaceName)
	err = tarextract.CreateTarGz(fmt.Sprintf("%s/%s", workspaceHome, workspaceName), inputPath, outputPath)
	buf.WriteString("\nArchiving the reports\n")
	if err != nil {
		buf.WriteString(fmt.Sprintf("While creating reports.tgz: %v\n", err))
		log.Println(err)
		return buf.String(), err
	}
	buf.WriteString("Workspace reports archived in reports.tgz\n")

	// Save the workspace-wide classes and tables in the workspace build directory
	// Create a map to save them in a lookup-like structure
	domainClasses := make(map[string]*rete.ClassNode)
	domainTables := make(map[string]*rete.TableNode)
	domainProperties := make(map[string]*rete.DataPropertyNode)
	for _, cls := range classes {
		domainClasses[cls.Name] = cls
		for _, prop := range cls.DataProperties {
			domainProperties[prop.Name] = &prop
		}
	}
	for _, tbl := range tables {
		domainTables[tbl.TableName] = tbl
	}
	// Save the indexed list of classes, properties and tables to the root of build directory
	fpath := fmt.Sprintf("%s/%s/build/classes.json", workspaceHome, wprefix)
	// log.Println("Writing JetStore Classes to:", fpath)
	file, err := os.OpenFile(fpath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		err = fmt.Errorf("while opening classes.json for write (compile_workspace):%v", err)
		return err.Error(), err
	}
	encoder := json.NewEncoder(file)
	encoder.Encode(domainClasses)
	file.Close()

	// Properties
	fpath = fmt.Sprintf("%s/%s/build/properties.json", workspaceHome, wprefix)
	// log.Println("Writing JetStore Properties to:", fpath)
	file, err = os.OpenFile(fpath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		err = fmt.Errorf("while opening properties.json for write (compile_workspace):%v", err)
		return err.Error(), err
	}
	encoder = json.NewEncoder(file)
	encoder.Encode(domainProperties)
	file.Close()

	// Tables
	fpath = fmt.Sprintf("%s/%s/build/tables.json", workspaceHome, wprefix)
	// log.Println("Writing JetStore Tables to:", fpath)
	file, err = os.OpenFile(fpath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		err = fmt.Errorf("while opening tables.json for write (compile_workspace):%v", err)
		return err.Error(), err
	}
	encoder = json.NewEncoder(file)
	encoder.Encode(domainTables)
	file.Close()

	// Archive the build rules and cpipes config
	inputPath = []string{
		fmt.Sprintf("%s/%s/workspace_control.json", workspaceHome, workspaceName),
		fmt.Sprintf("%s/%s/build/", workspaceHome, workspaceName),
		fmt.Sprintf("%s/%s/pipes_config/", workspaceHome, workspaceName),
	}
	outputPath = fmt.Sprintf("%s/%s/workspace.tgz", workspaceHome, workspaceName)
	buf.WriteString("\nArchiving the build and cpipes config directories\n")
	err = tarextract.CreateTarGz(fmt.Sprintf("%s/%s", workspaceHome, workspaceName), inputPath, outputPath)
	if err != nil {
		buf.WriteString("Error:")
		buf.WriteString(err.Error())
	}
	cmdLog := buf.String()
	if err != nil {
		log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
		log.Println(cmdLog)
		log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
		return cmdLog, fmt.Errorf("while archiving the jet_rules folder : %v", err)
	}

	log.Println("COMPILE WORKSPACE CAPTURED OUTPUT:")
	log.Println("============================")
	log.Println(cmdLog)
	log.Println("============================")

	if dbpool == nil {
		return buf.String(), nil
	}
	if devMode {
		log.Println("IN DEV MODE = Skipping copy large object to DB")
	} else {
		buf.WriteString("\nCopy the sqlite file to db\n")
		err = UploadWorkspaceAssets(dbpool, workspaceName, version)
	}
	if err == nil {
		err = UpdateWorkspaceVersionDb(dbpool, workspaceName, version)
	}
	if err != nil {
		buf.WriteString("Failed to update worspace version to db:")
		buf.WriteString(err.Error())
		buf.WriteString("\n")
	}

	return buf.String(), err
}
