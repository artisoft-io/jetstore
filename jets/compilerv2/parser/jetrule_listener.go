// Code generated from JetRule.g4 by ANTLR 4.13.2. DO NOT EDIT.

package parser // JetRule

import "github.com/antlr4-go/antlr/v4"

// JetRuleListener is a complete listener for a parse tree produced by JetRuleParser.
type JetRuleListener interface {
	antlr.ParseTreeListener

	// EnterJetrule is called when entering the jetrule production.
	EnterJetrule(c *JetruleContext)

	// EnterStatement is called when entering the statement production.
	EnterStatement(c *StatementContext)

	// EnterJetCompilerDirectiveStmt is called when entering the jetCompilerDirectiveStmt production.
	EnterJetCompilerDirectiveStmt(c *JetCompilerDirectiveStmtContext)

	// EnterDefineJetStoreConfigStmt is called when entering the defineJetStoreConfigStmt production.
	EnterDefineJetStoreConfigStmt(c *DefineJetStoreConfigStmtContext)

	// EnterJetstoreConfig is called when entering the jetstoreConfig production.
	EnterJetstoreConfig(c *JetstoreConfigContext)

	// EnterJetstoreConfigSeq is called when entering the jetstoreConfigSeq production.
	EnterJetstoreConfigSeq(c *JetstoreConfigSeqContext)

	// EnterJetstoreConfigItem is called when entering the jetstoreConfigItem production.
	EnterJetstoreConfigItem(c *JetstoreConfigItemContext)

	// EnterDefineClassStmt is called when entering the defineClassStmt production.
	EnterDefineClassStmt(c *DefineClassStmtContext)

	// EnterClassStmt is called when entering the classStmt production.
	EnterClassStmt(c *ClassStmtContext)

	// EnterSubClassOfStmt is called when entering the subClassOfStmt production.
	EnterSubClassOfStmt(c *SubClassOfStmtContext)

	// EnterDataPropertyDefinitions is called when entering the dataPropertyDefinitions production.
	EnterDataPropertyDefinitions(c *DataPropertyDefinitionsContext)

	// EnterDataPropertyType is called when entering the dataPropertyType production.
	EnterDataPropertyType(c *DataPropertyTypeContext)

	// EnterGroupingPropertyStmt is called when entering the groupingPropertyStmt production.
	EnterGroupingPropertyStmt(c *GroupingPropertyStmtContext)

	// EnterAsTableStmt is called when entering the asTableStmt production.
	EnterAsTableStmt(c *AsTableStmtContext)

	// EnterAsTableFlag is called when entering the asTableFlag production.
	EnterAsTableFlag(c *AsTableFlagContext)

	// EnterDefineRuleSeqStmt is called when entering the defineRuleSeqStmt production.
	EnterDefineRuleSeqStmt(c *DefineRuleSeqStmtContext)

	// EnterRuleSetSeq is called when entering the ruleSetSeq production.
	EnterRuleSetSeq(c *RuleSetSeqContext)

	// EnterRuleSetDefinitions is called when entering the ruleSetDefinitions production.
	EnterRuleSetDefinitions(c *RuleSetDefinitionsContext)

	// EnterDefineLiteralStmt is called when entering the defineLiteralStmt production.
	EnterDefineLiteralStmt(c *DefineLiteralStmtContext)

	// EnterInt32LiteralStmt is called when entering the int32LiteralStmt production.
	EnterInt32LiteralStmt(c *Int32LiteralStmtContext)

	// EnterUInt32LiteralStmt is called when entering the uInt32LiteralStmt production.
	EnterUInt32LiteralStmt(c *UInt32LiteralStmtContext)

	// EnterInt64LiteralStmt is called when entering the int64LiteralStmt production.
	EnterInt64LiteralStmt(c *Int64LiteralStmtContext)

	// EnterUInt64LiteralStmt is called when entering the uInt64LiteralStmt production.
	EnterUInt64LiteralStmt(c *UInt64LiteralStmtContext)

	// EnterDoubleLiteralStmt is called when entering the doubleLiteralStmt production.
	EnterDoubleLiteralStmt(c *DoubleLiteralStmtContext)

	// EnterStringLiteralStmt is called when entering the stringLiteralStmt production.
	EnterStringLiteralStmt(c *StringLiteralStmtContext)

	// EnterDateLiteralStmt is called when entering the dateLiteralStmt production.
	EnterDateLiteralStmt(c *DateLiteralStmtContext)

	// EnterDatetimeLiteralStmt is called when entering the datetimeLiteralStmt production.
	EnterDatetimeLiteralStmt(c *DatetimeLiteralStmtContext)

	// EnterBooleanLiteralStmt is called when entering the booleanLiteralStmt production.
	EnterBooleanLiteralStmt(c *BooleanLiteralStmtContext)

	// EnterIntExpr is called when entering the intExpr production.
	EnterIntExpr(c *IntExprContext)

	// EnterUintExpr is called when entering the uintExpr production.
	EnterUintExpr(c *UintExprContext)

	// EnterDoubleExpr is called when entering the doubleExpr production.
	EnterDoubleExpr(c *DoubleExprContext)

	// EnterDeclIdentifier is called when entering the declIdentifier production.
	EnterDeclIdentifier(c *DeclIdentifierContext)

	// EnterDefineResourceStmt is called when entering the defineResourceStmt production.
	EnterDefineResourceStmt(c *DefineResourceStmtContext)

	// EnterNamedResourceStmt is called when entering the namedResourceStmt production.
	EnterNamedResourceStmt(c *NamedResourceStmtContext)

	// EnterVolatileResourceStmt is called when entering the volatileResourceStmt production.
	EnterVolatileResourceStmt(c *VolatileResourceStmtContext)

	// EnterResourceValue is called when entering the resourceValue production.
	EnterResourceValue(c *ResourceValueContext)

	// EnterLookupTableStmt is called when entering the lookupTableStmt production.
	EnterLookupTableStmt(c *LookupTableStmtContext)

	// EnterCsvLocation is called when entering the csvLocation production.
	EnterCsvLocation(c *CsvLocationContext)

	// EnterStringList is called when entering the stringList production.
	EnterStringList(c *StringListContext)

	// EnterStringSeq is called when entering the stringSeq production.
	EnterStringSeq(c *StringSeqContext)

	// EnterColumnDefSeq is called when entering the columnDefSeq production.
	EnterColumnDefSeq(c *ColumnDefSeqContext)

	// EnterColumnDefinitions is called when entering the columnDefinitions production.
	EnterColumnDefinitions(c *ColumnDefinitionsContext)

	// EnterJetRuleStmt is called when entering the jetRuleStmt production.
	EnterJetRuleStmt(c *JetRuleStmtContext)

	// EnterRuleProperties is called when entering the ruleProperties production.
	EnterRuleProperties(c *RulePropertiesContext)

	// EnterPropertyValue is called when entering the propertyValue production.
	EnterPropertyValue(c *PropertyValueContext)

	// EnterAntecedent is called when entering the antecedent production.
	EnterAntecedent(c *AntecedentContext)

	// EnterConsequent is called when entering the consequent production.
	EnterConsequent(c *ConsequentContext)

	// EnterAtom is called when entering the atom production.
	EnterAtom(c *AtomContext)

	// EnterObjectAtom is called when entering the objectAtom production.
	EnterObjectAtom(c *ObjectAtomContext)

	// EnterKeywords is called when entering the keywords production.
	EnterKeywords(c *KeywordsContext)

	// EnterSelfExprTerm is called when entering the SelfExprTerm production.
	EnterSelfExprTerm(c *SelfExprTermContext)

	// EnterBinaryExprTerm2 is called when entering the BinaryExprTerm2 production.
	EnterBinaryExprTerm2(c *BinaryExprTerm2Context)

	// EnterUnaryExprTerm is called when entering the UnaryExprTerm production.
	EnterUnaryExprTerm(c *UnaryExprTermContext)

	// EnterObjectAtomExprTerm is called when entering the ObjectAtomExprTerm production.
	EnterObjectAtomExprTerm(c *ObjectAtomExprTermContext)

	// EnterUnaryExprTerm3 is called when entering the UnaryExprTerm3 production.
	EnterUnaryExprTerm3(c *UnaryExprTerm3Context)

	// EnterUnaryExprTerm2 is called when entering the UnaryExprTerm2 production.
	EnterUnaryExprTerm2(c *UnaryExprTerm2Context)

	// EnterBinaryExprTerm is called when entering the BinaryExprTerm production.
	EnterBinaryExprTerm(c *BinaryExprTermContext)

	// EnterBinaryOp is called when entering the binaryOp production.
	EnterBinaryOp(c *BinaryOpContext)

	// EnterUnaryOp is called when entering the unaryOp production.
	EnterUnaryOp(c *UnaryOpContext)

	// EnterTripleStmt is called when entering the tripleStmt production.
	EnterTripleStmt(c *TripleStmtContext)

	// ExitJetrule is called when exiting the jetrule production.
	ExitJetrule(c *JetruleContext)

	// ExitStatement is called when exiting the statement production.
	ExitStatement(c *StatementContext)

	// ExitJetCompilerDirectiveStmt is called when exiting the jetCompilerDirectiveStmt production.
	ExitJetCompilerDirectiveStmt(c *JetCompilerDirectiveStmtContext)

	// ExitDefineJetStoreConfigStmt is called when exiting the defineJetStoreConfigStmt production.
	ExitDefineJetStoreConfigStmt(c *DefineJetStoreConfigStmtContext)

	// ExitJetstoreConfig is called when exiting the jetstoreConfig production.
	ExitJetstoreConfig(c *JetstoreConfigContext)

	// ExitJetstoreConfigSeq is called when exiting the jetstoreConfigSeq production.
	ExitJetstoreConfigSeq(c *JetstoreConfigSeqContext)

	// ExitJetstoreConfigItem is called when exiting the jetstoreConfigItem production.
	ExitJetstoreConfigItem(c *JetstoreConfigItemContext)

	// ExitDefineClassStmt is called when exiting the defineClassStmt production.
	ExitDefineClassStmt(c *DefineClassStmtContext)

	// ExitClassStmt is called when exiting the classStmt production.
	ExitClassStmt(c *ClassStmtContext)

	// ExitSubClassOfStmt is called when exiting the subClassOfStmt production.
	ExitSubClassOfStmt(c *SubClassOfStmtContext)

	// ExitDataPropertyDefinitions is called when exiting the dataPropertyDefinitions production.
	ExitDataPropertyDefinitions(c *DataPropertyDefinitionsContext)

	// ExitDataPropertyType is called when exiting the dataPropertyType production.
	ExitDataPropertyType(c *DataPropertyTypeContext)

	// ExitGroupingPropertyStmt is called when exiting the groupingPropertyStmt production.
	ExitGroupingPropertyStmt(c *GroupingPropertyStmtContext)

	// ExitAsTableStmt is called when exiting the asTableStmt production.
	ExitAsTableStmt(c *AsTableStmtContext)

	// ExitAsTableFlag is called when exiting the asTableFlag production.
	ExitAsTableFlag(c *AsTableFlagContext)

	// ExitDefineRuleSeqStmt is called when exiting the defineRuleSeqStmt production.
	ExitDefineRuleSeqStmt(c *DefineRuleSeqStmtContext)

	// ExitRuleSetSeq is called when exiting the ruleSetSeq production.
	ExitRuleSetSeq(c *RuleSetSeqContext)

	// ExitRuleSetDefinitions is called when exiting the ruleSetDefinitions production.
	ExitRuleSetDefinitions(c *RuleSetDefinitionsContext)

	// ExitDefineLiteralStmt is called when exiting the defineLiteralStmt production.
	ExitDefineLiteralStmt(c *DefineLiteralStmtContext)

	// ExitInt32LiteralStmt is called when exiting the int32LiteralStmt production.
	ExitInt32LiteralStmt(c *Int32LiteralStmtContext)

	// ExitUInt32LiteralStmt is called when exiting the uInt32LiteralStmt production.
	ExitUInt32LiteralStmt(c *UInt32LiteralStmtContext)

	// ExitInt64LiteralStmt is called when exiting the int64LiteralStmt production.
	ExitInt64LiteralStmt(c *Int64LiteralStmtContext)

	// ExitUInt64LiteralStmt is called when exiting the uInt64LiteralStmt production.
	ExitUInt64LiteralStmt(c *UInt64LiteralStmtContext)

	// ExitDoubleLiteralStmt is called when exiting the doubleLiteralStmt production.
	ExitDoubleLiteralStmt(c *DoubleLiteralStmtContext)

	// ExitStringLiteralStmt is called when exiting the stringLiteralStmt production.
	ExitStringLiteralStmt(c *StringLiteralStmtContext)

	// ExitDateLiteralStmt is called when exiting the dateLiteralStmt production.
	ExitDateLiteralStmt(c *DateLiteralStmtContext)

	// ExitDatetimeLiteralStmt is called when exiting the datetimeLiteralStmt production.
	ExitDatetimeLiteralStmt(c *DatetimeLiteralStmtContext)

	// ExitBooleanLiteralStmt is called when exiting the booleanLiteralStmt production.
	ExitBooleanLiteralStmt(c *BooleanLiteralStmtContext)

	// ExitIntExpr is called when exiting the intExpr production.
	ExitIntExpr(c *IntExprContext)

	// ExitUintExpr is called when exiting the uintExpr production.
	ExitUintExpr(c *UintExprContext)

	// ExitDoubleExpr is called when exiting the doubleExpr production.
	ExitDoubleExpr(c *DoubleExprContext)

	// ExitDeclIdentifier is called when exiting the declIdentifier production.
	ExitDeclIdentifier(c *DeclIdentifierContext)

	// ExitDefineResourceStmt is called when exiting the defineResourceStmt production.
	ExitDefineResourceStmt(c *DefineResourceStmtContext)

	// ExitNamedResourceStmt is called when exiting the namedResourceStmt production.
	ExitNamedResourceStmt(c *NamedResourceStmtContext)

	// ExitVolatileResourceStmt is called when exiting the volatileResourceStmt production.
	ExitVolatileResourceStmt(c *VolatileResourceStmtContext)

	// ExitResourceValue is called when exiting the resourceValue production.
	ExitResourceValue(c *ResourceValueContext)

	// ExitLookupTableStmt is called when exiting the lookupTableStmt production.
	ExitLookupTableStmt(c *LookupTableStmtContext)

	// ExitCsvLocation is called when exiting the csvLocation production.
	ExitCsvLocation(c *CsvLocationContext)

	// ExitStringList is called when exiting the stringList production.
	ExitStringList(c *StringListContext)

	// ExitStringSeq is called when exiting the stringSeq production.
	ExitStringSeq(c *StringSeqContext)

	// ExitColumnDefSeq is called when exiting the columnDefSeq production.
	ExitColumnDefSeq(c *ColumnDefSeqContext)

	// ExitColumnDefinitions is called when exiting the columnDefinitions production.
	ExitColumnDefinitions(c *ColumnDefinitionsContext)

	// ExitJetRuleStmt is called when exiting the jetRuleStmt production.
	ExitJetRuleStmt(c *JetRuleStmtContext)

	// ExitRuleProperties is called when exiting the ruleProperties production.
	ExitRuleProperties(c *RulePropertiesContext)

	// ExitPropertyValue is called when exiting the propertyValue production.
	ExitPropertyValue(c *PropertyValueContext)

	// ExitAntecedent is called when exiting the antecedent production.
	ExitAntecedent(c *AntecedentContext)

	// ExitConsequent is called when exiting the consequent production.
	ExitConsequent(c *ConsequentContext)

	// ExitAtom is called when exiting the atom production.
	ExitAtom(c *AtomContext)

	// ExitObjectAtom is called when exiting the objectAtom production.
	ExitObjectAtom(c *ObjectAtomContext)

	// ExitKeywords is called when exiting the keywords production.
	ExitKeywords(c *KeywordsContext)

	// ExitSelfExprTerm is called when exiting the SelfExprTerm production.
	ExitSelfExprTerm(c *SelfExprTermContext)

	// ExitBinaryExprTerm2 is called when exiting the BinaryExprTerm2 production.
	ExitBinaryExprTerm2(c *BinaryExprTerm2Context)

	// ExitUnaryExprTerm is called when exiting the UnaryExprTerm production.
	ExitUnaryExprTerm(c *UnaryExprTermContext)

	// ExitObjectAtomExprTerm is called when exiting the ObjectAtomExprTerm production.
	ExitObjectAtomExprTerm(c *ObjectAtomExprTermContext)

	// ExitUnaryExprTerm3 is called when exiting the UnaryExprTerm3 production.
	ExitUnaryExprTerm3(c *UnaryExprTerm3Context)

	// ExitUnaryExprTerm2 is called when exiting the UnaryExprTerm2 production.
	ExitUnaryExprTerm2(c *UnaryExprTerm2Context)

	// ExitBinaryExprTerm is called when exiting the BinaryExprTerm production.
	ExitBinaryExprTerm(c *BinaryExprTermContext)

	// ExitBinaryOp is called when exiting the binaryOp production.
	ExitBinaryOp(c *BinaryOpContext)

	// ExitUnaryOp is called when exiting the unaryOp production.
	ExitUnaryOp(c *UnaryOpContext)

	// ExitTripleStmt is called when exiting the tripleStmt production.
	ExitTripleStmt(c *TripleStmtContext)
}
