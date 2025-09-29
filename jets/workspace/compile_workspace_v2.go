package workspace

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/artisoft-io/jetstore/jets/compilerv2/compiler"
	"github.com/artisoft-io/jetstore/jets/dbutils"
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

	// Compile the workspace locally
	var fullModel *rete.JetruleModel
	buf.WriteString(fmt.Sprintf("Compiling workspace %s at version %s\n", workspaceName, version))
	// For  each main rule files in workspace control, create a compiler instance
	// and compile the rule file
	// Collect the set of main rule files
	mainRuleFiles := make(map[string]bool)
	for i := range workspaceControl.RuleSets {
		mainRuleFiles[workspaceControl.RuleSets[i]] = true
	}
	for i := range workspaceControl.RuleSequences {
		for j := range workspaceControl.RuleSequences[i].RuleSets {
			mainRuleFiles[workspaceControl.RuleSequences[i].RuleSets[j]] = true
		}
	}
	// Keep reference to all classes, tables, and lookup tables
	var classes []*rete.ClassNode
	var tables []*rete.TableNode
	var lookupTables []*rete.LookupTableNode

	for name := range mainRuleFiles {
		buf.WriteString(fmt.Sprintf("Compiling rule file: %s\n", name))
		jrCompiler = compiler.NewCompiler(
			fmt.Sprintf("%s/%s", workspaceHome, workspaceName),
			name, true, workspaceControl.UseTraceMode, workspaceControl.AutoAddResources)
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

		// name is the file path relative to workspace home, extract file name (last component)
		fileName := filepath.Base(name)

		// Collect the classes, tables, and lookup tables
		classes = append(classes, jrCompiler.JetRuleModel().Classes...)
		tables = append(tables, jrCompiler.JetRuleModel().Tables...)
		lookupTables = append(lookupTables, jrCompiler.JetRuleModel().LookupTables...)

		// Split the compiled model into '.rete.json',  '.model.json' and  '.triple.json '
		// into the build directory
		// rete
		fullModel = jrCompiler.JetRuleModel()
		reteModel := &rete.JetruleModel{
			MainRuleFileName: fullModel.MainRuleFileName,
			Resources:        fullModel.Resources,
			LookupTables:     fullModel.LookupTables,
			ReteNodes:        fullModel.ReteNodes,
		}
		fpath := fmt.Sprintf("%s/%s/build/%s.rete.json", workspaceHome,
			wprefix, strings.TrimSuffix(fileName, ".jr"))
		log.Println("Writing JetStore Rete Network of", name, "to:", fpath)
		file, err := os.OpenFile(fpath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			err = fmt.Errorf("while opening .rete.json for write (compile_workspace_v2):%v", err)
			return err.Error(), err
		}
		encoder := json.NewEncoder(file)
		encoder.Encode(reteModel)
		file.Close()

		// classes and tables model
		clsTblModel := &rete.JetruleModel{
			MainRuleFileName: fullModel.MainRuleFileName,
			Classes:          fullModel.Classes,
			Tables:           fullModel.Tables,
		}
		fpath = fmt.Sprintf("%s/%s/build/%s.model.json", workspaceHome,
			wprefix, strings.TrimSuffix(fileName, ".jr"))
		log.Println("Writing JetStore Classes and Tables Model of", name, "to:", fpath)
		file, err = os.OpenFile(fpath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			err = fmt.Errorf("while opening .model.json for write (compile_workspace_v2):%v", err)
			return err.Error(), err
		}
		encoder = json.NewEncoder(file)
		encoder.Encode(clsTblModel)
		file.Close()

		// triples
		tripleModel := &rete.JetruleModel{
			MainRuleFileName: fullModel.MainRuleFileName,
			Triples:          fullModel.Triples,
		}
		fpath = fmt.Sprintf("%s/%s/build/%s.triple.json", workspaceHome,
			wprefix, strings.TrimSuffix(fileName, ".jr"))
		log.Println("Writing JetStore Triples Model of", name, "to:", fpath)
		file, err = os.OpenFile(fpath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			err = fmt.Errorf("while opening .triple.json for write (compile_workspace_v2):%v", err)
			return err.Error(), err
		}
		encoder = json.NewEncoder(file)
		encoder.Encode(tripleModel)
		file.Close()
	}

	// All files are now compiled
	log.Println("All main rule files compiled, now package the lookup.db file")
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
		buf.WriteString(fmt.Sprintf("While creating reports.tgz: %v", err))
		log.Println(err)
		return buf.String(), err
	}
	log.Println("Workspace reports archived in retports.tgz")

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
	log.Println("Writing JetStore Classes to:", fpath)
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
	log.Println("Writing JetStore Properties to:", fpath)
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
	log.Println("Writing JetStore Tables to:", fpath)
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

	if devMode {
		log.Println("IN DEV MODE = Skipping copy large object to DB")
	} else {
		// Copy the sqlite files & the tar file to db
		buf.WriteString("\nCopy the sqlite file to db\n")
		sourcesPath := []string{
			fmt.Sprintf("%s/%s/lookup.db", workspaceHome, workspaceName),
			fmt.Sprintf("%s/%s/workspace.db", workspaceHome, workspaceName),
			fmt.Sprintf("%s/%s/workspace.tgz", workspaceHome, workspaceName),
			fmt.Sprintf("%s/%s/reports.tgz", workspaceHome, workspaceName),
		}
		fileNames := []string{"lookup.db", "workspace.db", "workspace.tgz", "reports.tgz"}
		fo := []dbutils.FileDbObject{
			{WorkspaceName: workspaceName, ContentType: "sqlite", UserEmail: "system"},
			{WorkspaceName: workspaceName, ContentType: "sqlite", UserEmail: "system"},
			{WorkspaceName: workspaceName, ContentType: "workspace.tgz", UserEmail: "system"},
			{WorkspaceName: workspaceName, ContentType: "reports.tgz", UserEmail: "system"}}
		for i := range sourcesPath {
			fo[i].FileName = fileNames[i]
			data, err := os.ReadFile(sourcesPath[i])
			if err != nil {
				return "", err
			}
			_, err = fo[i].WriteObject(dbpool, data)
			if err != nil {
				buf.WriteString("Failed to upload file to db:")
				buf.WriteString(err.Error())
				buf.WriteString("\n")
				return buf.String(), fmt.Errorf("failed to upload file to db: %v", err)
			}
		}
	}
	err = UpdateWorkspaceVersionDb(dbpool, workspaceName, version)
	if err != nil {
		buf.WriteString("Failed to update worspace version to db:")
		buf.WriteString(err.Error())
		buf.WriteString("\n")
	}

	return buf.String(), err
}
