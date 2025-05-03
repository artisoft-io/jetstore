// Code generated from JetRule.g4 by ANTLR 4.13.2. DO NOT EDIT.

package parser // JetRule

import "github.com/antlr4-go/antlr/v4"

// BaseJetRuleListener is a complete listener for a parse tree produced by JetRuleParser.
type BaseJetRuleListener struct{}

var _ JetRuleListener = &BaseJetRuleListener{}

// VisitTerminal is called when a terminal node is visited.
func (s *BaseJetRuleListener) VisitTerminal(node antlr.TerminalNode) {}

// VisitErrorNode is called when an error node is visited.
func (s *BaseJetRuleListener) VisitErrorNode(node antlr.ErrorNode) {}

// EnterEveryRule is called when any rule is entered.
func (s *BaseJetRuleListener) EnterEveryRule(ctx antlr.ParserRuleContext) {}

// ExitEveryRule is called when any rule is exited.
func (s *BaseJetRuleListener) ExitEveryRule(ctx antlr.ParserRuleContext) {}

// EnterJetrule is called when production jetrule is entered.
func (s *BaseJetRuleListener) EnterJetrule(ctx *JetruleContext) {}

// ExitJetrule is called when production jetrule is exited.
func (s *BaseJetRuleListener) ExitJetrule(ctx *JetruleContext) {}

// EnterStatement is called when production statement is entered.
func (s *BaseJetRuleListener) EnterStatement(ctx *StatementContext) {}

// ExitStatement is called when production statement is exited.
func (s *BaseJetRuleListener) ExitStatement(ctx *StatementContext) {}

// EnterJetCompilerDirectiveStmt is called when production jetCompilerDirectiveStmt is entered.
func (s *BaseJetRuleListener) EnterJetCompilerDirectiveStmt(ctx *JetCompilerDirectiveStmtContext) {}

// ExitJetCompilerDirectiveStmt is called when production jetCompilerDirectiveStmt is exited.
func (s *BaseJetRuleListener) ExitJetCompilerDirectiveStmt(ctx *JetCompilerDirectiveStmtContext) {}

// EnterDefineJetStoreConfigStmt is called when production defineJetStoreConfigStmt is entered.
func (s *BaseJetRuleListener) EnterDefineJetStoreConfigStmt(ctx *DefineJetStoreConfigStmtContext) {}

// ExitDefineJetStoreConfigStmt is called when production defineJetStoreConfigStmt is exited.
func (s *BaseJetRuleListener) ExitDefineJetStoreConfigStmt(ctx *DefineJetStoreConfigStmtContext) {}

// EnterJetstoreConfig is called when production jetstoreConfig is entered.
func (s *BaseJetRuleListener) EnterJetstoreConfig(ctx *JetstoreConfigContext) {}

// ExitJetstoreConfig is called when production jetstoreConfig is exited.
func (s *BaseJetRuleListener) ExitJetstoreConfig(ctx *JetstoreConfigContext) {}

// EnterJetstoreConfigSeq is called when production jetstoreConfigSeq is entered.
func (s *BaseJetRuleListener) EnterJetstoreConfigSeq(ctx *JetstoreConfigSeqContext) {}

// ExitJetstoreConfigSeq is called when production jetstoreConfigSeq is exited.
func (s *BaseJetRuleListener) ExitJetstoreConfigSeq(ctx *JetstoreConfigSeqContext) {}

// EnterJetstoreConfigItem is called when production jetstoreConfigItem is entered.
func (s *BaseJetRuleListener) EnterJetstoreConfigItem(ctx *JetstoreConfigItemContext) {}

// ExitJetstoreConfigItem is called when production jetstoreConfigItem is exited.
func (s *BaseJetRuleListener) ExitJetstoreConfigItem(ctx *JetstoreConfigItemContext) {}

// EnterDefineClassStmt is called when production defineClassStmt is entered.
func (s *BaseJetRuleListener) EnterDefineClassStmt(ctx *DefineClassStmtContext) {}

// ExitDefineClassStmt is called when production defineClassStmt is exited.
func (s *BaseJetRuleListener) ExitDefineClassStmt(ctx *DefineClassStmtContext) {}

// EnterClassStmt is called when production classStmt is entered.
func (s *BaseJetRuleListener) EnterClassStmt(ctx *ClassStmtContext) {}

// ExitClassStmt is called when production classStmt is exited.
func (s *BaseJetRuleListener) ExitClassStmt(ctx *ClassStmtContext) {}

// EnterSubClassOfStmt is called when production subClassOfStmt is entered.
func (s *BaseJetRuleListener) EnterSubClassOfStmt(ctx *SubClassOfStmtContext) {}

// ExitSubClassOfStmt is called when production subClassOfStmt is exited.
func (s *BaseJetRuleListener) ExitSubClassOfStmt(ctx *SubClassOfStmtContext) {}

// EnterDataPropertyDefinitions is called when production dataPropertyDefinitions is entered.
func (s *BaseJetRuleListener) EnterDataPropertyDefinitions(ctx *DataPropertyDefinitionsContext) {}

// ExitDataPropertyDefinitions is called when production dataPropertyDefinitions is exited.
func (s *BaseJetRuleListener) ExitDataPropertyDefinitions(ctx *DataPropertyDefinitionsContext) {}

// EnterDataPropertyType is called when production dataPropertyType is entered.
func (s *BaseJetRuleListener) EnterDataPropertyType(ctx *DataPropertyTypeContext) {}

// ExitDataPropertyType is called when production dataPropertyType is exited.
func (s *BaseJetRuleListener) ExitDataPropertyType(ctx *DataPropertyTypeContext) {}

// EnterGroupingPropertyStmt is called when production groupingPropertyStmt is entered.
func (s *BaseJetRuleListener) EnterGroupingPropertyStmt(ctx *GroupingPropertyStmtContext) {}

// ExitGroupingPropertyStmt is called when production groupingPropertyStmt is exited.
func (s *BaseJetRuleListener) ExitGroupingPropertyStmt(ctx *GroupingPropertyStmtContext) {}

// EnterAsTableStmt is called when production asTableStmt is entered.
func (s *BaseJetRuleListener) EnterAsTableStmt(ctx *AsTableStmtContext) {}

// ExitAsTableStmt is called when production asTableStmt is exited.
func (s *BaseJetRuleListener) ExitAsTableStmt(ctx *AsTableStmtContext) {}

// EnterAsTableFlag is called when production asTableFlag is entered.
func (s *BaseJetRuleListener) EnterAsTableFlag(ctx *AsTableFlagContext) {}

// ExitAsTableFlag is called when production asTableFlag is exited.
func (s *BaseJetRuleListener) ExitAsTableFlag(ctx *AsTableFlagContext) {}

// EnterDefineRuleSeqStmt is called when production defineRuleSeqStmt is entered.
func (s *BaseJetRuleListener) EnterDefineRuleSeqStmt(ctx *DefineRuleSeqStmtContext) {}

// ExitDefineRuleSeqStmt is called when production defineRuleSeqStmt is exited.
func (s *BaseJetRuleListener) ExitDefineRuleSeqStmt(ctx *DefineRuleSeqStmtContext) {}

// EnterRuleSetSeq is called when production ruleSetSeq is entered.
func (s *BaseJetRuleListener) EnterRuleSetSeq(ctx *RuleSetSeqContext) {}

// ExitRuleSetSeq is called when production ruleSetSeq is exited.
func (s *BaseJetRuleListener) ExitRuleSetSeq(ctx *RuleSetSeqContext) {}

// EnterRuleSetDefinitions is called when production ruleSetDefinitions is entered.
func (s *BaseJetRuleListener) EnterRuleSetDefinitions(ctx *RuleSetDefinitionsContext) {}

// ExitRuleSetDefinitions is called when production ruleSetDefinitions is exited.
func (s *BaseJetRuleListener) ExitRuleSetDefinitions(ctx *RuleSetDefinitionsContext) {}

// EnterDefineLiteralStmt is called when production defineLiteralStmt is entered.
func (s *BaseJetRuleListener) EnterDefineLiteralStmt(ctx *DefineLiteralStmtContext) {}

// ExitDefineLiteralStmt is called when production defineLiteralStmt is exited.
func (s *BaseJetRuleListener) ExitDefineLiteralStmt(ctx *DefineLiteralStmtContext) {}

// EnterInt32LiteralStmt is called when production int32LiteralStmt is entered.
func (s *BaseJetRuleListener) EnterInt32LiteralStmt(ctx *Int32LiteralStmtContext) {}

// ExitInt32LiteralStmt is called when production int32LiteralStmt is exited.
func (s *BaseJetRuleListener) ExitInt32LiteralStmt(ctx *Int32LiteralStmtContext) {}

// EnterUInt32LiteralStmt is called when production uInt32LiteralStmt is entered.
func (s *BaseJetRuleListener) EnterUInt32LiteralStmt(ctx *UInt32LiteralStmtContext) {}

// ExitUInt32LiteralStmt is called when production uInt32LiteralStmt is exited.
func (s *BaseJetRuleListener) ExitUInt32LiteralStmt(ctx *UInt32LiteralStmtContext) {}

// EnterInt64LiteralStmt is called when production int64LiteralStmt is entered.
func (s *BaseJetRuleListener) EnterInt64LiteralStmt(ctx *Int64LiteralStmtContext) {}

// ExitInt64LiteralStmt is called when production int64LiteralStmt is exited.
func (s *BaseJetRuleListener) ExitInt64LiteralStmt(ctx *Int64LiteralStmtContext) {}

// EnterUInt64LiteralStmt is called when production uInt64LiteralStmt is entered.
func (s *BaseJetRuleListener) EnterUInt64LiteralStmt(ctx *UInt64LiteralStmtContext) {}

// ExitUInt64LiteralStmt is called when production uInt64LiteralStmt is exited.
func (s *BaseJetRuleListener) ExitUInt64LiteralStmt(ctx *UInt64LiteralStmtContext) {}

// EnterDoubleLiteralStmt is called when production doubleLiteralStmt is entered.
func (s *BaseJetRuleListener) EnterDoubleLiteralStmt(ctx *DoubleLiteralStmtContext) {}

// ExitDoubleLiteralStmt is called when production doubleLiteralStmt is exited.
func (s *BaseJetRuleListener) ExitDoubleLiteralStmt(ctx *DoubleLiteralStmtContext) {}

// EnterStringLiteralStmt is called when production stringLiteralStmt is entered.
func (s *BaseJetRuleListener) EnterStringLiteralStmt(ctx *StringLiteralStmtContext) {}

// ExitStringLiteralStmt is called when production stringLiteralStmt is exited.
func (s *BaseJetRuleListener) ExitStringLiteralStmt(ctx *StringLiteralStmtContext) {}

// EnterDateLiteralStmt is called when production dateLiteralStmt is entered.
func (s *BaseJetRuleListener) EnterDateLiteralStmt(ctx *DateLiteralStmtContext) {}

// ExitDateLiteralStmt is called when production dateLiteralStmt is exited.
func (s *BaseJetRuleListener) ExitDateLiteralStmt(ctx *DateLiteralStmtContext) {}

// EnterDatetimeLiteralStmt is called when production datetimeLiteralStmt is entered.
func (s *BaseJetRuleListener) EnterDatetimeLiteralStmt(ctx *DatetimeLiteralStmtContext) {}

// ExitDatetimeLiteralStmt is called when production datetimeLiteralStmt is exited.
func (s *BaseJetRuleListener) ExitDatetimeLiteralStmt(ctx *DatetimeLiteralStmtContext) {}

// EnterBooleanLiteralStmt is called when production booleanLiteralStmt is entered.
func (s *BaseJetRuleListener) EnterBooleanLiteralStmt(ctx *BooleanLiteralStmtContext) {}

// ExitBooleanLiteralStmt is called when production booleanLiteralStmt is exited.
func (s *BaseJetRuleListener) ExitBooleanLiteralStmt(ctx *BooleanLiteralStmtContext) {}

// EnterIntExpr is called when production intExpr is entered.
func (s *BaseJetRuleListener) EnterIntExpr(ctx *IntExprContext) {}

// ExitIntExpr is called when production intExpr is exited.
func (s *BaseJetRuleListener) ExitIntExpr(ctx *IntExprContext) {}

// EnterUintExpr is called when production uintExpr is entered.
func (s *BaseJetRuleListener) EnterUintExpr(ctx *UintExprContext) {}

// ExitUintExpr is called when production uintExpr is exited.
func (s *BaseJetRuleListener) ExitUintExpr(ctx *UintExprContext) {}

// EnterDoubleExpr is called when production doubleExpr is entered.
func (s *BaseJetRuleListener) EnterDoubleExpr(ctx *DoubleExprContext) {}

// ExitDoubleExpr is called when production doubleExpr is exited.
func (s *BaseJetRuleListener) ExitDoubleExpr(ctx *DoubleExprContext) {}

// EnterDeclIdentifier is called when production declIdentifier is entered.
func (s *BaseJetRuleListener) EnterDeclIdentifier(ctx *DeclIdentifierContext) {}

// ExitDeclIdentifier is called when production declIdentifier is exited.
func (s *BaseJetRuleListener) ExitDeclIdentifier(ctx *DeclIdentifierContext) {}

// EnterDefineResourceStmt is called when production defineResourceStmt is entered.
func (s *BaseJetRuleListener) EnterDefineResourceStmt(ctx *DefineResourceStmtContext) {}

// ExitDefineResourceStmt is called when production defineResourceStmt is exited.
func (s *BaseJetRuleListener) ExitDefineResourceStmt(ctx *DefineResourceStmtContext) {}

// EnterNamedResourceStmt is called when production namedResourceStmt is entered.
func (s *BaseJetRuleListener) EnterNamedResourceStmt(ctx *NamedResourceStmtContext) {}

// ExitNamedResourceStmt is called when production namedResourceStmt is exited.
func (s *BaseJetRuleListener) ExitNamedResourceStmt(ctx *NamedResourceStmtContext) {}

// EnterVolatileResourceStmt is called when production volatileResourceStmt is entered.
func (s *BaseJetRuleListener) EnterVolatileResourceStmt(ctx *VolatileResourceStmtContext) {}

// ExitVolatileResourceStmt is called when production volatileResourceStmt is exited.
func (s *BaseJetRuleListener) ExitVolatileResourceStmt(ctx *VolatileResourceStmtContext) {}

// EnterResourceValue is called when production resourceValue is entered.
func (s *BaseJetRuleListener) EnterResourceValue(ctx *ResourceValueContext) {}

// ExitResourceValue is called when production resourceValue is exited.
func (s *BaseJetRuleListener) ExitResourceValue(ctx *ResourceValueContext) {}

// EnterLookupTableStmt is called when production lookupTableStmt is entered.
func (s *BaseJetRuleListener) EnterLookupTableStmt(ctx *LookupTableStmtContext) {}

// ExitLookupTableStmt is called when production lookupTableStmt is exited.
func (s *BaseJetRuleListener) ExitLookupTableStmt(ctx *LookupTableStmtContext) {}

// EnterCsvLocation is called when production csvLocation is entered.
func (s *BaseJetRuleListener) EnterCsvLocation(ctx *CsvLocationContext) {}

// ExitCsvLocation is called when production csvLocation is exited.
func (s *BaseJetRuleListener) ExitCsvLocation(ctx *CsvLocationContext) {}

// EnterStringList is called when production stringList is entered.
func (s *BaseJetRuleListener) EnterStringList(ctx *StringListContext) {}

// ExitStringList is called when production stringList is exited.
func (s *BaseJetRuleListener) ExitStringList(ctx *StringListContext) {}

// EnterStringSeq is called when production stringSeq is entered.
func (s *BaseJetRuleListener) EnterStringSeq(ctx *StringSeqContext) {}

// ExitStringSeq is called when production stringSeq is exited.
func (s *BaseJetRuleListener) ExitStringSeq(ctx *StringSeqContext) {}

// EnterColumnDefSeq is called when production columnDefSeq is entered.
func (s *BaseJetRuleListener) EnterColumnDefSeq(ctx *ColumnDefSeqContext) {}

// ExitColumnDefSeq is called when production columnDefSeq is exited.
func (s *BaseJetRuleListener) ExitColumnDefSeq(ctx *ColumnDefSeqContext) {}

// EnterColumnDefinitions is called when production columnDefinitions is entered.
func (s *BaseJetRuleListener) EnterColumnDefinitions(ctx *ColumnDefinitionsContext) {}

// ExitColumnDefinitions is called when production columnDefinitions is exited.
func (s *BaseJetRuleListener) ExitColumnDefinitions(ctx *ColumnDefinitionsContext) {}

// EnterJetRuleStmt is called when production jetRuleStmt is entered.
func (s *BaseJetRuleListener) EnterJetRuleStmt(ctx *JetRuleStmtContext) {}

// ExitJetRuleStmt is called when production jetRuleStmt is exited.
func (s *BaseJetRuleListener) ExitJetRuleStmt(ctx *JetRuleStmtContext) {}

// EnterRuleProperties is called when production ruleProperties is entered.
func (s *BaseJetRuleListener) EnterRuleProperties(ctx *RulePropertiesContext) {}

// ExitRuleProperties is called when production ruleProperties is exited.
func (s *BaseJetRuleListener) ExitRuleProperties(ctx *RulePropertiesContext) {}

// EnterPropertyValue is called when production propertyValue is entered.
func (s *BaseJetRuleListener) EnterPropertyValue(ctx *PropertyValueContext) {}

// ExitPropertyValue is called when production propertyValue is exited.
func (s *BaseJetRuleListener) ExitPropertyValue(ctx *PropertyValueContext) {}

// EnterAntecedent is called when production antecedent is entered.
func (s *BaseJetRuleListener) EnterAntecedent(ctx *AntecedentContext) {}

// ExitAntecedent is called when production antecedent is exited.
func (s *BaseJetRuleListener) ExitAntecedent(ctx *AntecedentContext) {}

// EnterConsequent is called when production consequent is entered.
func (s *BaseJetRuleListener) EnterConsequent(ctx *ConsequentContext) {}

// ExitConsequent is called when production consequent is exited.
func (s *BaseJetRuleListener) ExitConsequent(ctx *ConsequentContext) {}

// EnterAtom is called when production atom is entered.
func (s *BaseJetRuleListener) EnterAtom(ctx *AtomContext) {}

// ExitAtom is called when production atom is exited.
func (s *BaseJetRuleListener) ExitAtom(ctx *AtomContext) {}

// EnterObjectAtom is called when production objectAtom is entered.
func (s *BaseJetRuleListener) EnterObjectAtom(ctx *ObjectAtomContext) {}

// ExitObjectAtom is called when production objectAtom is exited.
func (s *BaseJetRuleListener) ExitObjectAtom(ctx *ObjectAtomContext) {}

// EnterKeywords is called when production keywords is entered.
func (s *BaseJetRuleListener) EnterKeywords(ctx *KeywordsContext) {}

// ExitKeywords is called when production keywords is exited.
func (s *BaseJetRuleListener) ExitKeywords(ctx *KeywordsContext) {}

// EnterSelfExprTerm is called when production SelfExprTerm is entered.
func (s *BaseJetRuleListener) EnterSelfExprTerm(ctx *SelfExprTermContext) {}

// ExitSelfExprTerm is called when production SelfExprTerm is exited.
func (s *BaseJetRuleListener) ExitSelfExprTerm(ctx *SelfExprTermContext) {}

// EnterBinaryExprTerm2 is called when production BinaryExprTerm2 is entered.
func (s *BaseJetRuleListener) EnterBinaryExprTerm2(ctx *BinaryExprTerm2Context) {}

// ExitBinaryExprTerm2 is called when production BinaryExprTerm2 is exited.
func (s *BaseJetRuleListener) ExitBinaryExprTerm2(ctx *BinaryExprTerm2Context) {}

// EnterUnaryExprTerm is called when production UnaryExprTerm is entered.
func (s *BaseJetRuleListener) EnterUnaryExprTerm(ctx *UnaryExprTermContext) {}

// ExitUnaryExprTerm is called when production UnaryExprTerm is exited.
func (s *BaseJetRuleListener) ExitUnaryExprTerm(ctx *UnaryExprTermContext) {}

// EnterObjectAtomExprTerm is called when production ObjectAtomExprTerm is entered.
func (s *BaseJetRuleListener) EnterObjectAtomExprTerm(ctx *ObjectAtomExprTermContext) {}

// ExitObjectAtomExprTerm is called when production ObjectAtomExprTerm is exited.
func (s *BaseJetRuleListener) ExitObjectAtomExprTerm(ctx *ObjectAtomExprTermContext) {}

// EnterUnaryExprTerm3 is called when production UnaryExprTerm3 is entered.
func (s *BaseJetRuleListener) EnterUnaryExprTerm3(ctx *UnaryExprTerm3Context) {}

// ExitUnaryExprTerm3 is called when production UnaryExprTerm3 is exited.
func (s *BaseJetRuleListener) ExitUnaryExprTerm3(ctx *UnaryExprTerm3Context) {}

// EnterUnaryExprTerm2 is called when production UnaryExprTerm2 is entered.
func (s *BaseJetRuleListener) EnterUnaryExprTerm2(ctx *UnaryExprTerm2Context) {}

// ExitUnaryExprTerm2 is called when production UnaryExprTerm2 is exited.
func (s *BaseJetRuleListener) ExitUnaryExprTerm2(ctx *UnaryExprTerm2Context) {}

// EnterBinaryExprTerm is called when production BinaryExprTerm is entered.
func (s *BaseJetRuleListener) EnterBinaryExprTerm(ctx *BinaryExprTermContext) {}

// ExitBinaryExprTerm is called when production BinaryExprTerm is exited.
func (s *BaseJetRuleListener) ExitBinaryExprTerm(ctx *BinaryExprTermContext) {}

// EnterBinaryOp is called when production binaryOp is entered.
func (s *BaseJetRuleListener) EnterBinaryOp(ctx *BinaryOpContext) {}

// ExitBinaryOp is called when production binaryOp is exited.
func (s *BaseJetRuleListener) ExitBinaryOp(ctx *BinaryOpContext) {}

// EnterUnaryOp is called when production unaryOp is entered.
func (s *BaseJetRuleListener) EnterUnaryOp(ctx *UnaryOpContext) {}

// ExitUnaryOp is called when production unaryOp is exited.
func (s *BaseJetRuleListener) ExitUnaryOp(ctx *UnaryOpContext) {}

// EnterTripleStmt is called when production tripleStmt is entered.
func (s *BaseJetRuleListener) EnterTripleStmt(ctx *TripleStmtContext) {}

// ExitTripleStmt is called when production tripleStmt is exited.
func (s *BaseJetRuleListener) ExitTripleStmt(ctx *TripleStmtContext) {}
