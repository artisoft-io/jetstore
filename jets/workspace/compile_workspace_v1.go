package workspace

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/artisoft-io/jetstore/jets/datatable/wsfile"
	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/artisoft-io/jetstore/jets/run_reports/tarextract"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Legacy workspace compilation function

func compileWorkspaceV1(dbpool *pgxpool.Pool, workspaceName, version string) (string, error) {

	compilerPath := fmt.Sprintf("%s/%s/compile_workspace.sh", workspaceHome, workspaceName)

	// Compile the workspace locally
	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("Compiling workspace %s at version %s\n", workspaceName, version))
	err := wsfile.RunCommand(&buf, compilerPath, nil, workspaceName)
	if err != nil {
		log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
		cmdLog := buf.String()
		log.Println(cmdLog)
		log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
		return cmdLog, fmt.Errorf("while executing compile_workspace command '%v': %v", compilerPath, err)
	}

	// Archive reports
	inputPath := []string{fmt.Sprintf("%s/%s/reports/", workspaceHome, workspaceName)}
	outputPath := fmt.Sprintf("%s/%s/reports.tgz", workspaceHome, workspaceName)
	err = tarextract.CreateTarGz(fmt.Sprintf("%s/%s", workspaceHome, workspaceName), inputPath, outputPath)
	// command := "tar"
	// args := []string{"cfvz", "reports.tgz", "reports/"}
	buf.WriteString("\nArchiving the reports\n")
	// err = wsfile.RunCommand(&buf, command, &args, workspaceName)
	// cmdLog := buf.String()
	if err != nil {
		buf.WriteString(fmt.Sprintf("While creating reports.tgz: %v", err))
		log.Println(err)
		return buf.String(), err
	}
	log.Println("Workspace reports archived in retports.tgz")

	// Compile the workspace-wide classes and tables, save it in the workspace build directory
	// Get all the main rule files from the workspace_control.json
	workspaceControl, err := rete.LoadWorkspaceControl(workspaceControlPath)
	if err != nil {
		err = fmt.Errorf("while loading workspace_control.json: %v", err)
		return err.Error(), err
	}
	// Collect all the main rule files
	mainRules := make(map[string]bool)
	for _, name := range workspaceControl.RuleSets {
		mainRules[name] = true
	}
	for i := range workspaceControl.RuleSequences {
		for _, name := range workspaceControl.RuleSequences[i].RuleSets {
			mainRules[name] = true
		}
	}
	// For each main rule file, read the compiled rete network to load only the domain classes and tables
	// definition
	domainClasses := make(map[string]*rete.ClassNode)
	domainTables := make(map[string]*rete.TableNode)
	domainProperties := make(map[string]*rete.DataPropertyNode)
	for name := range mainRules {
		fpath := fmt.Sprintf("%s/%s/build/%s.model.json", workspaceHome,
			wprefix, strings.TrimSuffix(name, ".jr"))
		log.Println("Reading JetStore Model of", name, "from:", fpath)
		file, err := os.ReadFile(fpath)
		if err != nil {
			err = fmt.Errorf("while reading json file (.model.json):%v", err)
			return err.Error(), err
		}
		model := rete.JetruleModel{}
		err = json.Unmarshal(file, &model)
		if err != nil {
			err = fmt.Errorf("while unmarshaling .model.json (compile_workspace):%v", err)
			return err.Error(), err
		}
		for i := range model.Classes {
			class := model.Classes[i]
			domainClasses[class.Name] = class
			for j := range class.DataProperties {
				class.DataProperties[j].ClassName = class.Name
				domainProperties[class.DataProperties[j].Name] = &class.DataProperties[j]
			}
		}
		for i := range model.Tables {
			table := model.Tables[i]
			domainTables[table.TableName] = table
		}
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

	log.Println("Package JetRule Lookup Tables to lookup.db")
	// Create the lookup.db file from the csv files in lookups dir
	// Load the lookup definitions from the compiled jr.json files

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
