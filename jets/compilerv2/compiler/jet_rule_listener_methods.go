package compiler

import (
	"fmt"
	"strings"

	"github.com/artisoft-io/jetstore/jets/compilerv2/parser"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/artisoft-io/jetstore/jets/stack"
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
		name := EscR(ctx.GetClassName().GetText())
		s.AddR(name)
		s.currentClass = &rete.ClassNode{
			Type:           "class",
			Name:           name,
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
		s.AddR(baseClass)
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
		name := EscR(ctx.GetDataPName().GetText())
		s.AddR(name)
		dp := rete.DataPropertyNode{
			Type:      ctx.GetDataPType().GetText(),
			Name:      name,
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

// =====================================================================================
// Literals
// -------------------------------------------------------------------------------------
// ExitInt32LiteralStmt is called when production int32Literal is exited.
func (s *JetRuleListener) ExitInt32LiteralStmt(ctx *parser.Int32LiteralStmtContext) {
	if ctx.GetVarType() != nil && ctx.GetVarName() != nil && ctx.GetDeclValue() != nil {
		s.AddResource(rete.ResourceNode{
			Type:  ctx.GetVarType().GetText(),
			Id:    ctx.GetVarName().GetText(),
			Value: ctx.GetDeclValue().GetText(),
		})
	}
}

// exitUInt32LiteralStmt is called when production uint32Literal is exited.
func (s *JetRuleListener) ExitUInt32LiteralStmt(ctx *parser.UInt32LiteralStmtContext) {
	if ctx.GetVarType() != nil && ctx.GetVarName() != nil && ctx.GetDeclValue() != nil {
		s.AddResource(rete.ResourceNode{
			Type:  ctx.GetVarType().GetText(),
			Id:    ctx.GetVarName().GetText(),
			Value: ctx.GetDeclValue().GetText(),
		})
	}
}

// ExitInt64LiteralStmt is called when production int64Literal is exited.
func (s *JetRuleListener) ExitInt64LiteralStmt(ctx *parser.Int64LiteralStmtContext) {
	if ctx.GetVarType() != nil && ctx.GetVarName() != nil && ctx.GetDeclValue() != nil {
		s.AddResource(rete.ResourceNode{
			Type:  ctx.GetVarType().GetText(),
			Id:    ctx.GetVarName().GetText(),
			Value: ctx.GetDeclValue().GetText(),
		})
	}
}

// ExitUInt64LiteralStmt is called when production uint64Literal is exited.
func (s *JetRuleListener) ExitUInt64LiteralStmt(ctx *parser.UInt64LiteralStmtContext) {
	if ctx.GetVarType() != nil && ctx.GetVarName() != nil && ctx.GetDeclValue() != nil {
		s.AddResource(rete.ResourceNode{
			Type:  ctx.GetVarType().GetText(),
			Id:    ctx.GetVarName().GetText(),
			Value: ctx.GetDeclValue().GetText(),
		})
	}
}

// exitDoubleLiteralStmt is called when production doubleLiteral is exited.
func (s *JetRuleListener) ExitDoubleLiteralStmt(ctx *parser.DoubleLiteralStmtContext) {
	if ctx.GetVarType() != nil && ctx.GetVarName() != nil && ctx.GetDeclValue() != nil {
		s.AddResource(rete.ResourceNode{
			Type:  ctx.GetVarType().GetText(),
			Id:    ctx.GetVarName().GetText(),
			Value: ctx.GetDeclValue().GetText(),
		})
	}
}

// exitStringLiteralStmt is called when production stringLiteral is exited.
func (s *JetRuleListener) ExitStringLiteralStmt(ctx *parser.StringLiteralStmtContext) {
	if ctx.GetVarType() != nil && ctx.GetVarName() != nil && ctx.GetDeclValue() != nil {
		s.AddResource(rete.ResourceNode{
			Type:  ctx.GetVarType().GetText(),
			Id:    ctx.GetVarName().GetText(),
			Value: StripQuotes(ctx.GetDeclValue().GetText()),
		})
	}
}

// exitDateLiteralStmt is called when production dateLiteral is exited.
func (s *JetRuleListener) ExitDateLiteralStmt(ctx *parser.DateLiteralStmtContext) {
	if ctx.GetVarType() != nil && ctx.GetVarName() != nil && ctx.GetDeclValue() != nil {
		s.AddResource(rete.ResourceNode{
			Type:  ctx.GetVarType().GetText(),
			Id:    ctx.GetVarName().GetText(),
			Value: StripQuotes(ctx.GetDeclValue().GetText()),
		})
	}
}

// exitDatetimeLiteralStmt is called when production datetimeLiteral is exited.
func (s *JetRuleListener) ExitDatetimeLiteralStmt(ctx *parser.DatetimeLiteralStmtContext) {
	if ctx.GetVarType() != nil && ctx.GetVarName() != nil && ctx.GetDeclValue() != nil {
		s.AddResource(rete.ResourceNode{
			Type:  ctx.GetVarType().GetText(),
			Id:    ctx.GetVarName().GetText(),
			Value: StripQuotes(ctx.GetDeclValue().GetText()),
		})
	}
}

// exitBooleanLiteralStmt is called when production booleanLiteral is exited.
func (s *JetRuleListener) ExitBooleanLiteralStmt(ctx *parser.BooleanLiteralStmtContext) {
	if ctx.GetVarType() != nil && ctx.GetVarName() != nil && ctx.GetDeclValue() != nil {
		s.AddResource(rete.ResourceNode{
			Type:  ctx.GetVarType().GetText(),
			Id:    ctx.GetVarName().GetText(),
			Value: ctx.GetDeclValue().GetText(),
		})
	}
}

// =====================================================================================
// Resource Definitions
// -------------------------------------------------------------------------------------
// exitNamedResourceStmt is called when production namedResourceStmt is exited.
func (s *JetRuleListener) ExitNamedResourceStmt(ctx *parser.NamedResourceStmtContext) {
	if ctx.GetResCtx() == nil || ctx.GetResName() == nil {
		return
	}
	id := EscR(ctx.GetResName().GetText())
	var typ, value string
	switch {
	case ctx.GetResCtx().GetResVal() != nil:
		value = StripQuotes(ctx.GetResCtx().GetResVal().GetText())
		typ = "resource"
	case ctx.GetResCtx().GetKws() != nil:
		value = StripQuotes(ctx.GetResCtx().GetKws().GetText())
		typ = "symbol"
	}
	if len(value) == 0 {
		return
	}
	s.AddResource(rete.ResourceNode{
		Type:           typ,
		Id:             id,
		Value:          value,
		SourceFileName: s.currentRuleFileName,
	})
}

// exitVolatileResourceStmt is called when production volatileResourceStmt is exited.
func (s *JetRuleListener) ExitVolatileResourceStmt(ctx *parser.VolatileResourceStmtContext) {
	var id, value string
	if ctx.GetResName() != nil {
		id = StripQuotes(ctx.GetResName().GetText())
	}
	if ctx.GetResVal() != nil {
		value = StripQuotes(ctx.GetResVal().GetText())
	}
	s.AddResource(rete.ResourceNode{
		Type:           "volatile_resource",
		Id:             id,
		Value:          value,
		SourceFileName: s.currentRuleFileName,
	})
}

// =====================================================================================
// Lookup Table Definitions
// -------------------------------------------------------------------------------------
// enterLookupTableStmt is called when production lookupTableStmt is entered.
func (s *JetRuleListener) EnterLookupTableStmt(ctx *parser.LookupTableStmtContext) {
	s.currentLookupTableColumns = []rete.LookupTableColumn{}
}

// exitColumnDefinitions is called when production columnDefinitions is exited.
func (s *JetRuleListener) ExitColumnDefinitions(ctx *parser.ColumnDefinitionsContext) {
	if ctx.GetColumnName() != nil && ctx.GetColumnType() != nil {
		name := StripQuotes(ctx.GetColumnName().GetText())
		// Validate that name can be used as an identifier in rules
		if !s.IsValidIdentifier(name) {
			fmt.Fprintf(s.errorLog, "** error: invalid column name for lookup table: %s\n", name)
			return
		}
		s.AddR(name)
		col := rete.LookupTableColumn{
			Name:    name,
			Type:    ctx.GetColumnType().GetText(),
			IsArray: ctx.GetArray() != nil,
		}
		s.currentLookupTableColumns = append(s.currentLookupTableColumns, col)
	}
}

// exitLookupTableColumn is called when production lookupTableColumn is exited.
func (s *JetRuleListener) ExitLookupTableStmt(ctx *parser.LookupTableStmtContext) {
	// Note $table_name (TableName) is not used, all lookups are csv-based ($csv_file)
	// so for now enforcing that ctx.CsvLocation().GetCsvFileName() != nil
	if ctx.CsvLocation() == nil || ctx.CsvLocation().GetCsvFileName() == nil || ctx.GetTblKeys() == nil {
		s.parseLog.WriteString("** Warning: lookup table must have $csv_file and keys defined\n")
		return
	}
	var keys []string
	for _, key := range ctx.GetTblKeys().GetSeqCtx().GetSlist() {
		keys = append(keys, StripQuotes(key.GetText()))
	}

	name := ctx.GetLookupName().GetText()
	s.AddR(name)
	lookupTbl := rete.LookupTableNode{
		Type:           "lookup",
		Name:           name,
		CsvFile:        StripQuotes(ctx.CsvLocation().GetCsvFileName().GetText()),
		Key:            keys,
		Columns:        s.currentLookupTableColumns,
		SourceFileName: s.currentRuleFileName,
	}
	s.jetRuleModel.LookupTables = append(s.jetRuleModel.LookupTables, lookupTbl)
	s.currentLookupTableColumns = nil
}

// =====================================================================================
// JetRule Definitions
// -------------------------------------------------------------------------------------
// enterJetRuleStmt is called when production jetRuleStmt is entered.
func (s *JetRuleListener) EnterJetRuleStmt(ctx *parser.JetRuleStmtContext) {
	s.currentRuleProperties = make(map[string]string)
	s.currentRuleVarByValue = make(map[string]string)
	s.currentJetruleNode = &rete.JetruleNode{}
}

// exitJetRuleStmt is called when production jetRuleStmt is exited.
func (s *JetRuleListener) ExitJetRuleStmt(ctx *parser.JetRuleStmtContext) {
	// Rule name is required
	if ctx.GetRuleName() == nil {
		s.errorLog.WriteString("** error: rule without a name encountered, skipping\n")
		return
	}
	s.currentJetruleNode.Name = ctx.GetRuleName().GetText()
	s.currentJetruleNode.Properties = s.currentRuleProperties
	s.currentJetruleNode.Antecedents = s.currentRuleAntecedents
	s.currentJetruleNode.Consequents = s.currentRuleConsequents
	s.currentJetruleNode.SourceFileName = s.currentRuleFileName

	s.ValidateJetruleNode(s.currentJetruleNode)

	// Reset current rule state
	s.currentRuleProperties = nil
	s.currentRuleAntecedents = nil
	s.currentRuleConsequents = nil
	s.currentRuleVarByValue = make(map[string]string)

	// Append to the model
	s.jetRuleModel.Jetrules = append(s.jetRuleModel.Jetrules, *s.currentJetruleNode)
	s.currentJetruleNode = nil
}

// exitRuleProperties is called when production ruleProperties is exited.
func (s *JetRuleListener) ExitRuleProperties(ctx *parser.RulePropertiesContext) {
	if ctx.GetKey() == nil || ctx.GetValCtx() == nil {
		return
	}
	key := ctx.GetKey().GetText()
	var value string
	switch {
	case ctx.GetValCtx().GetVal() != nil:
		value = ctx.GetValCtx().GetVal().GetText()
	case ctx.GetValCtx().GetIntval() != nil:
		value = ctx.GetValCtx().GetIntval().GetText()
	}
	s.currentRuleProperties[key] = value
}

// Expression Definitions
// -------------------------------------------------------------------------------------
// exitBinaryExprTerm is called when production binaryExprTerm is exited.
func (s *JetRuleListener) ExitBinaryExprTerm(ctx *parser.BinaryExprTermContext) {
	// Pop the top two expressions from the stack
	rhs, ok := s.inProgressExpr.Pop()
	if !ok {
		return
	}
	lhs, ok := s.inProgressExpr.Pop()
	if !ok {
		return
	}
	// Create a new binary expression node
	binaryExpr := rete.ExpressionNode{
		Type: "binary",
		Op:   ctx.GetOp().GetText(),
		Lhs:  lhs,
		Rhs:  rhs,
	}
	// Push the new binary expression onto the stack
	s.inProgressExpr.Push(&binaryExpr)
}

// exitBinaryExprTerm2 is called when production binaryExprTerm2 is exited.
func (s *JetRuleListener) ExitBinaryExprTerm2(ctx *parser.BinaryExprTerm2Context) {
	// Pop the top two expressions from the stack
	rhs, ok := s.inProgressExpr.Pop()
	if !ok {
		return
	}
	lhs, ok := s.inProgressExpr.Pop()
	if !ok {
		return
	}
	// Create a new binary expression node
	binaryExpr := rete.ExpressionNode{
		Type: "binary",
		Op:   ctx.GetOp().GetText(),
		Lhs:  lhs,
		Rhs:  rhs,
	}
	// Push the new binary expression onto the stack
	s.inProgressExpr.Push(&binaryExpr)
}

// exitUnaryExprTerm is called when production unaryExprTerm is exited.
func (s *JetRuleListener) ExitUnaryExprTerm(ctx *parser.UnaryExprTermContext) {
	// Pop the top expression from the stack
	expr, ok := s.inProgressExpr.Pop()
	if !ok {
		return
	}
	// Create a new unary expression node
	unaryExpr := rete.ExpressionNode{
		Type: "unary",
		Op:   ctx.GetOp().GetText(),
		Arg:  expr,
	}
	// Push the new unary expression onto the stack
	s.inProgressExpr.Push(&unaryExpr)
}

// exitUnaryExprTerm2 is called when production unaryExprTerm2 is exited.
func (s *JetRuleListener) ExitUnaryExprTerm2(ctx *parser.UnaryExprTerm2Context) {
	// Pop the top expression from the stack
	expr, ok := s.inProgressExpr.Pop()
	if !ok {
		return
	}
	// Create a new unary expression node
	unaryExpr := rete.ExpressionNode{
		Type: "unary",
		Op:   ctx.GetOp().GetText(),
		Arg:  expr,
	}
	// Push the new unary expression onto the stack
	s.inProgressExpr.Push(&unaryExpr)
}

// exitSelfExprTerm is called when production selfExprTerm is exited.
func (s *JetRuleListener) ExitSelfExprTerm(ctx *parser.SelfExprTermContext) {
	// Nothing here since it correspond to a pop and then a push of the same expression
}

// exitUnaryExprTerm3 is called when production unaryExprTerm3 is exited.
func (s *JetRuleListener) ExitUnaryExprTerm3(ctx *parser.UnaryExprTerm3Context) {
	// Pop the top expression from the stack
	expr, ok := s.inProgressExpr.Pop()
	if !ok {
		return
	}
	// Create a new unary expression node
	unaryExpr := rete.ExpressionNode{
		Type: "unary",
		Op:   ctx.GetOp().GetText(),
		Arg:  expr,
	}
	// Push the new unary expression onto the stack
	s.inProgressExpr.Push(&unaryExpr)
}

// exitObjectAtomExprTerm is called when production objectAtomExprTerm is exited.
func (s *JetRuleListener) ExitObjectAtomExprTerm(ctx *parser.ObjectAtomExprTermContext) {
	NodeTxt := StripQuotes(ctx.GetIdent().GetText())
	kws := ""
	if ctx.GetIdent().GetKws() != nil {
		kws = ctx.GetIdent().GetKws().GetText()
	}

	// Create a new identifier (resource / volatile_resource) expression node
	varNode := rete.ExpressionNode{
		Type:  "identifier",
		Value: s.ParseObjectAtom(NodeTxt, kws),
	}
	// Push the new identifier expression onto the stack
	s.inProgressExpr.Push(&varNode)
}

// Antecedent Definition
// -------------------------------------------------------------------------------------
// enterAntecedent is called when production antecedent is entered.
func (s *JetRuleListener) EnterAntecedent(ctx *parser.AntecedentContext) {
	// Reset the in-progress expression stack
	s.inProgressExpr = stack.NewStack[rete.ExpressionNode](5)
}

// exitAntecedent is called when production antecedent is exited.
func (s *JetRuleListener) ExitAntecedent(ctx *parser.AntecedentContext) {
	if ctx.GetS() == nil || ctx.GetP() == nil || ctx.GetO() == nil {
		return
	}
	kws := ""
	if ctx.GetO().GetKws() != nil {
		kws = ctx.GetO().GetKws().GetText()
	}
	term := rete.RuleTerm{
		Type:         "antecedent",
		IsNot:        ctx.GetN() != nil,
		SubjectKey:   s.ParseObjectAtom(EscR(ctx.GetS().GetText()), ""),
		PredicateKey: s.ParseObjectAtom(EscR(ctx.GetP().GetText()), ""),
		ObjectKey:    s.ParseObjectAtom(ctx.GetO().GetText(), kws),
	}
	// Add filter
	if s.inProgressExpr.Len() > 0 {
		expr, ok := s.inProgressExpr.Pop()
		if ok {
			term.Filter = expr
		}
	}
	s.ValidateRuleTerm(&term)

	// Clear the in-progress expression stack
	s.inProgressExpr = nil

	// Append to the current rule antecedents
	s.currentRuleAntecedents = append(s.currentRuleAntecedents, term)
}

// Consequent Definition
// -------------------------------------------------------------------------------------
// enterConsequent is called when production consequent is entered.
func (s *JetRuleListener) EnterConsequent(ctx *parser.ConsequentContext) {
	// Reset the in-progress expression stack
	s.inProgressExpr = stack.NewStack[rete.ExpressionNode](5)
}

// exitConsequent is called when production consequent is exited.
func (s *JetRuleListener) ExitConsequent(ctx *parser.ConsequentContext) {
	if ctx.GetS() == nil || ctx.GetP() == nil || ctx.GetO() == nil {
		return
	}
	term := rete.RuleTerm{
		Type:         "consequent",
		SubjectKey:   s.ParseObjectAtom(EscR(ctx.GetS().GetText()), ""),
		PredicateKey: s.ParseObjectAtom(EscR(ctx.GetP().GetText()), ""),
	}
	// Add object expression
	if s.inProgressExpr.Len() > 0 {
		expr, ok := s.inProgressExpr.Pop()
		if ok {
			term.ObjectExpr = expr
		}
	}
	// Clear the in-progress expression stack
	s.inProgressExpr = nil

	// Append to the current rule consequents
	s.currentRuleConsequents = append(s.currentRuleConsequents, term)
}

// =====================================================================================
// Triple
// -------------------------------------------------------------------------------------
// exitTripleStmt is called when production tripleStmt is exited.
func (s *JetRuleListener) ExitTripleStmt(ctx *parser.TripleStmtContext) {
	if ctx.GetS() == nil || ctx.GetP() == nil || ctx.GetO() == nil {
		return
	}
	kws := ""
	if ctx.GetO().GetKws() != nil {
		kws = ctx.GetO().GetKws().GetText()
	}
	triple := rete.TripleNode{
		SubjectKey:   s.ParseObjectAtom(EscR(ctx.GetS().GetText()), ""),
		PredicateKey: s.ParseObjectAtom(EscR(ctx.GetP().GetText()), ""),
		ObjectKey:    s.ParseObjectAtom(ctx.GetO().GetText(), kws),
	}
	s.jetRuleModel.Triples = append(s.jetRuleModel.Triples, triple)
}
