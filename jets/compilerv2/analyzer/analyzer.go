package analyzer

// This file contains the Analyzer which performs analysis of the rete model after parsing, including validation and transformation logic that is separate from the parsing logic in the listener. This is where we can put logic that needs to be applied across the entire model, or that requires multiple passes through the model.

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/artisoft-io/jetstore/jets/compilerv2/compiler"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

type Analyzer struct {
	compiler      *compiler.Compiler
	analyzerModel *AnalyzeModel
	basePath      string
	runOptions    []string
	saveJson      bool
}

// AnalyzeModel is the output of the analysis
// When runOption contains "predicates":
//
//	InputPredicates: list of all predicates used in the antecedents of the rules
//	OutputPredicates: list of all predicates used in the consequents of the rules
//
// InputClasses and OutputClasses are the classes used in the predicates in the antecedents and consequents, respectively.
// When runOption contains "dependencies-rules":
//
//	DependencyPropertyName: the property name to determin how it is inferred and what depend on it.
//	AntecedentRules: list of all rules that participate at inferring the DependencyPropertyName
//	ConsequentRules: list of all downstream rules that depend on the DependencyPropertyName, either directly or indirectly.
//
// When runOption contains "dependencies-properties":
//
//	AntecedentProperties: list of all properties that participate at inferring the DependencyPropertyName
//	ConsequentProperties: list of all downstream properties that depend on the DependencyPropertyName, either directly or indirectly.
//
// This is useful for impact analysis when changing the logic of the rules that infer the DependencyPropertyName.
type AnalyzeModel struct {
	MainRuleFileName       string   `json:"main_rule_file_name"`
	InputPredicates        []string `json:"input_predicates"`
	OutputPredicates       []string `json:"output_predicates"`
	InputClasses           []string `json:"input_classes"`
	OutputClasses          []string `json:"output_classes"`
	DependencyPropertyName string   `json:"dependency_property_name"`
	AntecedentRules        []string `json:"antecedent_rules"`
	ConsequentRules        []string `json:"consequent_rules"`
	AntecedentProperties   []string `json:"antecedent_properties"`
	ConsequentProperties   []string `json:"consequent_properties"`
}

func NewAnalyzer(basePath, mainRuleFileName, runOptions, dependencyPropertyName string,
	saveJson bool, compiler *compiler.Compiler) *Analyzer {

	return &Analyzer{
		compiler: compiler,
		basePath: basePath,
		analyzerModel: &AnalyzeModel{
			MainRuleFileName:       mainRuleFileName,
			DependencyPropertyName: dependencyPropertyName,
		},
		runOptions: strings.Split(runOptions, ","),
		saveJson:   saveJson,
	}
}

func (a *Analyzer) Analyze() (err error) {
	model := a.compiler.JetRuleModel()
	if a.compiler.Trace() {
		log.Printf("** Starting analysis of model with %d classes and %d rules\n", len(model.Classes), len(model.Jetrules))
	}
	// Perform analysis logic here, such as validating the model, checking for consistency, etc.
	for _, option := range a.runOptions {
		switch option {
		case "predicates":
			err = a.analyzePredicates(model)
			if err != nil {
				return fmt.Errorf("error analyzing predicates: %w", err)
			}
		case "dependencies-rules":
			err = a.analyzeRuleDependencies(model)
			if err != nil {
				return fmt.Errorf("error analyzing rule dependencies: %w", err)
			}
		case "dependencies-properties":
			err = a.analyzePropertyDependencies(model)
			if err != nil {
				return fmt.Errorf("error analyzing property dependencies: %w", err)
			}
		default:
			log.Printf("** Unknown analysis option: %s\n", option)
		}
	}

	if a.saveJson {
		err = a.SaveModel()
		if err != nil {
			return fmt.Errorf("error saving analysis model: %w", err)
		}
	}

	return nil
}

func (c *Analyzer) SaveModel() error {
	// OutJsonFileName validate the output path to remain inside the
	// base path to prevent path traversal (CWE-73).
	outPath, err := c.OutJsonFileName()
	if err != nil {
		log.Println("** ERROR invalid output path:", err.Error())
		return err
	}
	log.Println("Saving json to", outPath)
	data, err := c.ToJson()
	if err != nil {
		log.Println("** ERROR converting to json:", err.Error())
		return fmt.Errorf("while converting to json: %w", err)
	}
	err = os.WriteFile(outPath, data, 0644)
	if err != nil {
		log.Println("** ERROR saving json:", err.Error())
		return fmt.Errorf("while saving json: %w", err)
	}
	return nil
}

func (a *Analyzer) ToJson() ([]byte, error) {
	return json.Marshal(a.analyzerModel)
}

// Define stub functions for the different analysis options. These can be implemented with the actual logic to analyze the model based on the requirements.

// Analyze predicates used in the antecedents and consequents of the rules
// Populate a.analyzerModel.InputPredicates, OutputPredicates, InputClasses, OutputClasses
func (a *Analyzer) analyzePredicates(model *rete.JetruleModel) error {
	// Collect all predicates that does not start with _0: and exist as a property in the Consequents terms.
	// This will give the list of output predicates that are inferred by the rules.
	// Collect all predicates that does not start with _0: and exist as a property in the Antecedents terms and are not in the Consequents terms.
	// This will give the list of input predicates that are used by the rules but not inferred by any rule.
	// Collect all classes used in the predicates in the Antecedents and Consequents, and populate InputClasses and OutputClasses.
	var inputPredicates []string
	var outputPredicates []string
	var inputClasses []string
	var outputClasses []string

	// Make a map of model.Resources[i].Key to model.Resources[i]
	resourceMap := make(map[int]*rete.ResourceNode)
	for i := range model.Resources {
		resource := model.Resources[i]
		resourceMap[resource.Key] = resource
	}

	// Make a map of data_property name to class name
	dataPropertyToClass := make(map[string]string)
	for _, cls := range model.Tables {
		for _, prop := range cls.Columns {
			dataPropertyToClass[prop.ColumnName] = cls.TableName
		}
	}

	// Collect the output predicates and classes from the Consequents
	for _, rule := range model.Jetrules {
		for _, consequent := range rule.Consequents {
			predicate := resourceMap[consequent.PredicateKey]
			if predicate != nil && predicate.Type == "resource" &&
				predicate.Id != "rdf:type" && predicate.Id != "jets:key" {

				className := dataPropertyToClass[predicate.Id]
				if className != "" {
					// Consider only predicates having a class
					outputClasses = appendIfMissing(outputClasses, className)
					outputPredicates = appendIfMissing(outputPredicates, predicate.Id)
				}
			}
		}
	}
	// Collect the input predicates and classes from the Antecedents
	for _, rule := range model.Jetrules {
		for _, antecedent := range rule.Antecedents {
			predicate := resourceMap[antecedent.PredicateKey]
			if predicate != nil && predicate.Type == "resource" &&
				predicate.Id != "rdf:type" && predicate.Id != "jets:key" {

				className := dataPropertyToClass[predicate.Id]
				if className != "" {
					// Consider only predicates having a class
					// Consider only predicates that are not in the output predicates
					if !slices.Contains(outputPredicates, predicate.Id) {
						inputClasses = appendIfMissing(inputClasses, className)
						inputPredicates = appendIfMissing(inputPredicates, predicate.Id)
					}
				}
			}
		}
	}

	a.analyzerModel.InputPredicates = inputPredicates
	a.analyzerModel.OutputPredicates = outputPredicates
	a.analyzerModel.InputClasses = inputClasses
	a.analyzerModel.OutputClasses = outputClasses

	log.Printf("** Found %d input predicates, %d output predicates, %d input classes, and %d output classes\n",
		len(inputPredicates), len(outputPredicates), len(inputClasses), len(outputClasses))

	return nil
}

// Analyze rule dependencies based on the DependencyPropertyName
// Populate a.analyzerModel.AntecedentRules and ConsequentRules

// Implement appendIfMissing to append to a slice if the value is not already in the slice
func appendIfMissing(slice []string, value string) []string {
	for _, v := range slice {
		if v == value {
			return slice
		}
	}
	return append(slice, value)
}

// Analyze rule dependencies based on the DependencyPropertyName
// Populate a.analyzerModel.AntecedentRules and ConsequentRules

func (a *Analyzer) analyzeRuleDependencies(_ *rete.JetruleModel) error {
	// Analyze rule dependencies based on the DependencyPropertyName
	// Populate a.analyzerModel.AntecedentRules and ConsequentRules
	return nil
}

func (a *Analyzer) analyzePropertyDependencies(_ *rete.JetruleModel) error {
	// Analyze property dependencies based on the DependencyPropertyName
	// Populate a.analyzerModel.AntecedentProperties and ConsequentProperties
	return nil
}

// OutJsonFileName resolves the analysis output file path within the analyzer
// base path. The output file name is derived from the (externally controlled)
// main rule file name, so the resolved path is validated to remain inside the
// base path to prevent path traversal (CWE-73).
func (a *Analyzer) OutJsonFileName() (string, error) {
	cleanBase := filepath.Clean(a.basePath)
	outPath := filepath.Clean(filepath.Join(cleanBase, MakeAOutputFileName(a.analyzerModel.MainRuleFileName)))
	if outPath != cleanBase && !strings.HasPrefix(outPath, cleanBase+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid output file name %q: resolved path %q is outside of base path %q",
			a.analyzerModel.MainRuleFileName, outPath, cleanBase)
	}
	return outPath, nil
}

func MakeAOutputFileName(mainSourceFileName string) string {
	return fmt.Sprintf("%s.analysis.json", strings.TrimSuffix(mainSourceFileName, ".jetrule"))
}
