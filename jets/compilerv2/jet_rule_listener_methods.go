package main

import (
	"strings"

	"github.com/artisoft-io/jetstore/jets/compilerv2/parser"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// This file contains the overrides for the listener methods to build the model

// =====================================================================================
// Compiler Directives
// -------------------------------------------------------------------------------------
// ExitJetCompilerDirectiveStmt is called when production jetCompilerDirectiveStmt is exited.
func (s *JetRuleListener) ExitJetCompilerDirectiveStmt(ctx *parser.JetCompilerDirectiveStmtContext) {
	if ctx.GetVarName() != nil && ctx.GetDeclValue() != nil {
		name := ctx.GetVarName().GetText()
		value := ctx.GetDeclValue().GetText()
		if name == "source_file" {
			// Strip quotes if present
			s.currentRuleFileName = StripQuotes(value)
		} else {
			s.jetRuleModel.CompilerDirectives[name] = StripQuotes(value)
		}
	}
}

// =====================================================================================
// JetStore Config
// -------------------------------------------------------------------------------------
// ExitJetstoreConfigItem is called when production jetstoreConfigItem is exited.
func (s *JetRuleListener) ExitJetstoreConfigItem(ctx *parser.JetstoreConfigItemContext) {
	if ctx.GetConfigKey() != nil && (ctx.GetRdfTypeList() != nil || ctx.GetConfigValue() != nil) {
		key := ctx.GetConfigKey().GetText()
		if ctx.GetRdfTypeList() != nil {
			var values []string
			for _, rt := range ctx.GetRdfTypeList() {
				values = append(values, rt.GetText())
			}
			s.jetRuleModel.JetstoreConfig[key] = strings.Join(values, ",")
		} else if ctx.GetConfigValue() != nil {
			s.jetRuleModel.JetstoreConfig[key] = ctx.GetConfigValue().GetText()
		}
	}
}

// =====================================================================================
// Class Definition
// -------------------------------------------------------------------------------------
// EnterDefineClassStmt is called when production defineClassStmt is entered.
func (s *JetRuleListener) EnterDefineClassStmt(ctx *parser.DefineClassStmtContext) {
	if ctx.GetClassName() != nil {
		s.currentClass = &rete.ClassNode{
			Type:           "class",
			Name:           EscR(ctx.GetClassName().GetText()),
			BaseClasses:    []string{},
			DataProperties: []rete.DataPropertyNode{},
			SourceFileName: s.currentRuleFileName,
		}
	}
}

// ExitSubClassOfStmt is called when production subClassOfStmt is exited.
func (s *JetRuleListener) ExitSubClassOfStmt(ctx *parser.SubClassOfStmtContext) {
	if ctx.GetBaseClassName() != nil {
		baseClass := EscR(ctx.GetBaseClassName().GetText())
		s.currentClass.BaseClasses = append(s.currentClass.BaseClasses, baseClass)
	}
}

// ExitAsTableStmt is called when production asTableStmt is exited.
func (s *JetRuleListener) ExitAsTableStmt(ctx *parser.AsTableStmtContext) {
	if ctx.GetAsTable() != nil {
		asTable := strings.ToUpper(ctx.GetAsTable().GetText())
		s.currentClass.AsTable = asTable == "TRUE"
	}
}

// ExitDefineClassStmt is called when production defineClassStmt is exited.
func (s *JetRuleListener) ExitDefineClassStmt(ctx *parser.DefineClassStmtContext) {
	if s.currentClass != nil {
		s.jetRuleModel.Classes = append(s.jetRuleModel.Classes, *s.currentClass)
		s.currentClass = nil
	}
}

// ExitDataPropertyDefinitions is called when production dataPropertyDefinitions is exited.
func (s *JetRuleListener) ExitDataPropertyDefinitions(ctx *parser.DataPropertyDefinitionsContext) {
	if ctx.GetDataPName() != nil && ctx.GetDataPType() != nil {
		dp := rete.DataPropertyNode{
			Type:      ctx.GetDataPType().GetText(),
			Name:      EscR(ctx.GetDataPName().GetText()),
			ClassName: s.currentClass.Name,
			AsArray:   ctx.GetArray() != nil,
		}
		s.currentClass.DataProperties = append(s.currentClass.DataProperties, dp)
	}
}

// =====================================================================================
// Rule Sequence Definition
// -------------------------------------------------------------------------------------
// EnterDefineRuleSeqStmt is called when production defineRuleSeqStmt is entered.
func (s *JetRuleListener) EnterDefineRuleSeqStmt(ctx *parser.DefineRuleSeqStmtContext) {
	s.currentRuleSequence = &rete.RuleSequence{}
}

// ExitRuleSetDefinitions is called when production ruleSetDefinitions is exited.
func (s *JetRuleListener) ExitRuleSetDefinitions(ctx *parser.RuleSetDefinitionsContext) {
	if ctx.GetRsName() != nil && s.currentRuleSequence != nil {
		s.currentRuleSequence.RuleSets =
			append(s.currentRuleSequence.RuleSets, StripQuotes(ctx.GetRsName().GetText()))
	}
}

// ExitDefineRuleSeqStmt is called when production defineRuleSeqStmt is exited.
func (s *JetRuleListener) ExitDefineRuleSeqStmt(ctx *parser.DefineRuleSeqStmtContext) {
	if s.currentRuleSequence != nil {
		if ctx.GetRuleseqName() != nil {
			s.currentRuleSequence.Name = ctx.GetRuleseqName().GetText()
			s.jetRuleModel.RuleSequences = append(s.jetRuleModel.RuleSequences, *s.currentRuleSequence)
		}
		s.currentRuleSequence = nil
	}
}

// Utility methods

// Escape resource name that conflicts with keywords such as rdf:type becomes rdf:"type"
// this function removes the quotes
func EscR(s string) string {
	if len(s) > 4 && strings.Contains(s, ":\"") {
		return strings.ReplaceAll(s, "\"", "")
	}
	return s
}

func StripQuotes(s string) string {
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	return s
}
