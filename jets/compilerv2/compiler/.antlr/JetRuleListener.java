// Generated from /home/michel/projects/repos/jetstore/jets/compilerv2/compiler/JetRule.g4 by ANTLR 4.13.1
import org.antlr.v4.runtime.tree.ParseTreeListener;

/**
 * This interface defines a complete listener for a parse tree produced by
 * {@link JetRuleParser}.
 */
public interface JetRuleListener extends ParseTreeListener {
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#jetrule}.
	 * @param ctx the parse tree
	 */
	void enterJetrule(JetRuleParser.JetruleContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#jetrule}.
	 * @param ctx the parse tree
	 */
	void exitJetrule(JetRuleParser.JetruleContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#statement}.
	 * @param ctx the parse tree
	 */
	void enterStatement(JetRuleParser.StatementContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#statement}.
	 * @param ctx the parse tree
	 */
	void exitStatement(JetRuleParser.StatementContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#jetCompilerDirectiveStmt}.
	 * @param ctx the parse tree
	 */
	void enterJetCompilerDirectiveStmt(JetRuleParser.JetCompilerDirectiveStmtContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#jetCompilerDirectiveStmt}.
	 * @param ctx the parse tree
	 */
	void exitJetCompilerDirectiveStmt(JetRuleParser.JetCompilerDirectiveStmtContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#defineJetStoreConfigStmt}.
	 * @param ctx the parse tree
	 */
	void enterDefineJetStoreConfigStmt(JetRuleParser.DefineJetStoreConfigStmtContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#defineJetStoreConfigStmt}.
	 * @param ctx the parse tree
	 */
	void exitDefineJetStoreConfigStmt(JetRuleParser.DefineJetStoreConfigStmtContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#jetstoreConfig}.
	 * @param ctx the parse tree
	 */
	void enterJetstoreConfig(JetRuleParser.JetstoreConfigContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#jetstoreConfig}.
	 * @param ctx the parse tree
	 */
	void exitJetstoreConfig(JetRuleParser.JetstoreConfigContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#jetstoreConfigSeq}.
	 * @param ctx the parse tree
	 */
	void enterJetstoreConfigSeq(JetRuleParser.JetstoreConfigSeqContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#jetstoreConfigSeq}.
	 * @param ctx the parse tree
	 */
	void exitJetstoreConfigSeq(JetRuleParser.JetstoreConfigSeqContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#jetstoreConfigItem}.
	 * @param ctx the parse tree
	 */
	void enterJetstoreConfigItem(JetRuleParser.JetstoreConfigItemContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#jetstoreConfigItem}.
	 * @param ctx the parse tree
	 */
	void exitJetstoreConfigItem(JetRuleParser.JetstoreConfigItemContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#defineClassStmt}.
	 * @param ctx the parse tree
	 */
	void enterDefineClassStmt(JetRuleParser.DefineClassStmtContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#defineClassStmt}.
	 * @param ctx the parse tree
	 */
	void exitDefineClassStmt(JetRuleParser.DefineClassStmtContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#classStmt}.
	 * @param ctx the parse tree
	 */
	void enterClassStmt(JetRuleParser.ClassStmtContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#classStmt}.
	 * @param ctx the parse tree
	 */
	void exitClassStmt(JetRuleParser.ClassStmtContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#subClassOfStmt}.
	 * @param ctx the parse tree
	 */
	void enterSubClassOfStmt(JetRuleParser.SubClassOfStmtContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#subClassOfStmt}.
	 * @param ctx the parse tree
	 */
	void exitSubClassOfStmt(JetRuleParser.SubClassOfStmtContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#dataPropertyDefinitions}.
	 * @param ctx the parse tree
	 */
	void enterDataPropertyDefinitions(JetRuleParser.DataPropertyDefinitionsContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#dataPropertyDefinitions}.
	 * @param ctx the parse tree
	 */
	void exitDataPropertyDefinitions(JetRuleParser.DataPropertyDefinitionsContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#dataPropertyType}.
	 * @param ctx the parse tree
	 */
	void enterDataPropertyType(JetRuleParser.DataPropertyTypeContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#dataPropertyType}.
	 * @param ctx the parse tree
	 */
	void exitDataPropertyType(JetRuleParser.DataPropertyTypeContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#groupingPropertyStmt}.
	 * @param ctx the parse tree
	 */
	void enterGroupingPropertyStmt(JetRuleParser.GroupingPropertyStmtContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#groupingPropertyStmt}.
	 * @param ctx the parse tree
	 */
	void exitGroupingPropertyStmt(JetRuleParser.GroupingPropertyStmtContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#asTableStmt}.
	 * @param ctx the parse tree
	 */
	void enterAsTableStmt(JetRuleParser.AsTableStmtContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#asTableStmt}.
	 * @param ctx the parse tree
	 */
	void exitAsTableStmt(JetRuleParser.AsTableStmtContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#asTableFlag}.
	 * @param ctx the parse tree
	 */
	void enterAsTableFlag(JetRuleParser.AsTableFlagContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#asTableFlag}.
	 * @param ctx the parse tree
	 */
	void exitAsTableFlag(JetRuleParser.AsTableFlagContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#defineRuleSeqStmt}.
	 * @param ctx the parse tree
	 */
	void enterDefineRuleSeqStmt(JetRuleParser.DefineRuleSeqStmtContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#defineRuleSeqStmt}.
	 * @param ctx the parse tree
	 */
	void exitDefineRuleSeqStmt(JetRuleParser.DefineRuleSeqStmtContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#ruleSetSeq}.
	 * @param ctx the parse tree
	 */
	void enterRuleSetSeq(JetRuleParser.RuleSetSeqContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#ruleSetSeq}.
	 * @param ctx the parse tree
	 */
	void exitRuleSetSeq(JetRuleParser.RuleSetSeqContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#ruleSetDefinitions}.
	 * @param ctx the parse tree
	 */
	void enterRuleSetDefinitions(JetRuleParser.RuleSetDefinitionsContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#ruleSetDefinitions}.
	 * @param ctx the parse tree
	 */
	void exitRuleSetDefinitions(JetRuleParser.RuleSetDefinitionsContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#defineLiteralStmt}.
	 * @param ctx the parse tree
	 */
	void enterDefineLiteralStmt(JetRuleParser.DefineLiteralStmtContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#defineLiteralStmt}.
	 * @param ctx the parse tree
	 */
	void exitDefineLiteralStmt(JetRuleParser.DefineLiteralStmtContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#int32LiteralStmt}.
	 * @param ctx the parse tree
	 */
	void enterInt32LiteralStmt(JetRuleParser.Int32LiteralStmtContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#int32LiteralStmt}.
	 * @param ctx the parse tree
	 */
	void exitInt32LiteralStmt(JetRuleParser.Int32LiteralStmtContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#uInt32LiteralStmt}.
	 * @param ctx the parse tree
	 */
	void enterUInt32LiteralStmt(JetRuleParser.UInt32LiteralStmtContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#uInt32LiteralStmt}.
	 * @param ctx the parse tree
	 */
	void exitUInt32LiteralStmt(JetRuleParser.UInt32LiteralStmtContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#int64LiteralStmt}.
	 * @param ctx the parse tree
	 */
	void enterInt64LiteralStmt(JetRuleParser.Int64LiteralStmtContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#int64LiteralStmt}.
	 * @param ctx the parse tree
	 */
	void exitInt64LiteralStmt(JetRuleParser.Int64LiteralStmtContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#uInt64LiteralStmt}.
	 * @param ctx the parse tree
	 */
	void enterUInt64LiteralStmt(JetRuleParser.UInt64LiteralStmtContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#uInt64LiteralStmt}.
	 * @param ctx the parse tree
	 */
	void exitUInt64LiteralStmt(JetRuleParser.UInt64LiteralStmtContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#doubleLiteralStmt}.
	 * @param ctx the parse tree
	 */
	void enterDoubleLiteralStmt(JetRuleParser.DoubleLiteralStmtContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#doubleLiteralStmt}.
	 * @param ctx the parse tree
	 */
	void exitDoubleLiteralStmt(JetRuleParser.DoubleLiteralStmtContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#stringLiteralStmt}.
	 * @param ctx the parse tree
	 */
	void enterStringLiteralStmt(JetRuleParser.StringLiteralStmtContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#stringLiteralStmt}.
	 * @param ctx the parse tree
	 */
	void exitStringLiteralStmt(JetRuleParser.StringLiteralStmtContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#dateLiteralStmt}.
	 * @param ctx the parse tree
	 */
	void enterDateLiteralStmt(JetRuleParser.DateLiteralStmtContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#dateLiteralStmt}.
	 * @param ctx the parse tree
	 */
	void exitDateLiteralStmt(JetRuleParser.DateLiteralStmtContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#datetimeLiteralStmt}.
	 * @param ctx the parse tree
	 */
	void enterDatetimeLiteralStmt(JetRuleParser.DatetimeLiteralStmtContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#datetimeLiteralStmt}.
	 * @param ctx the parse tree
	 */
	void exitDatetimeLiteralStmt(JetRuleParser.DatetimeLiteralStmtContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#booleanLiteralStmt}.
	 * @param ctx the parse tree
	 */
	void enterBooleanLiteralStmt(JetRuleParser.BooleanLiteralStmtContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#booleanLiteralStmt}.
	 * @param ctx the parse tree
	 */
	void exitBooleanLiteralStmt(JetRuleParser.BooleanLiteralStmtContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#intExpr}.
	 * @param ctx the parse tree
	 */
	void enterIntExpr(JetRuleParser.IntExprContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#intExpr}.
	 * @param ctx the parse tree
	 */
	void exitIntExpr(JetRuleParser.IntExprContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#uintExpr}.
	 * @param ctx the parse tree
	 */
	void enterUintExpr(JetRuleParser.UintExprContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#uintExpr}.
	 * @param ctx the parse tree
	 */
	void exitUintExpr(JetRuleParser.UintExprContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#doubleExpr}.
	 * @param ctx the parse tree
	 */
	void enterDoubleExpr(JetRuleParser.DoubleExprContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#doubleExpr}.
	 * @param ctx the parse tree
	 */
	void exitDoubleExpr(JetRuleParser.DoubleExprContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#declIdentifier}.
	 * @param ctx the parse tree
	 */
	void enterDeclIdentifier(JetRuleParser.DeclIdentifierContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#declIdentifier}.
	 * @param ctx the parse tree
	 */
	void exitDeclIdentifier(JetRuleParser.DeclIdentifierContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#defineResourceStmt}.
	 * @param ctx the parse tree
	 */
	void enterDefineResourceStmt(JetRuleParser.DefineResourceStmtContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#defineResourceStmt}.
	 * @param ctx the parse tree
	 */
	void exitDefineResourceStmt(JetRuleParser.DefineResourceStmtContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#namedResourceStmt}.
	 * @param ctx the parse tree
	 */
	void enterNamedResourceStmt(JetRuleParser.NamedResourceStmtContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#namedResourceStmt}.
	 * @param ctx the parse tree
	 */
	void exitNamedResourceStmt(JetRuleParser.NamedResourceStmtContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#volatileResourceStmt}.
	 * @param ctx the parse tree
	 */
	void enterVolatileResourceStmt(JetRuleParser.VolatileResourceStmtContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#volatileResourceStmt}.
	 * @param ctx the parse tree
	 */
	void exitVolatileResourceStmt(JetRuleParser.VolatileResourceStmtContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#resourceValue}.
	 * @param ctx the parse tree
	 */
	void enterResourceValue(JetRuleParser.ResourceValueContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#resourceValue}.
	 * @param ctx the parse tree
	 */
	void exitResourceValue(JetRuleParser.ResourceValueContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#lookupTableStmt}.
	 * @param ctx the parse tree
	 */
	void enterLookupTableStmt(JetRuleParser.LookupTableStmtContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#lookupTableStmt}.
	 * @param ctx the parse tree
	 */
	void exitLookupTableStmt(JetRuleParser.LookupTableStmtContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#csvLocation}.
	 * @param ctx the parse tree
	 */
	void enterCsvLocation(JetRuleParser.CsvLocationContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#csvLocation}.
	 * @param ctx the parse tree
	 */
	void exitCsvLocation(JetRuleParser.CsvLocationContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#stringList}.
	 * @param ctx the parse tree
	 */
	void enterStringList(JetRuleParser.StringListContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#stringList}.
	 * @param ctx the parse tree
	 */
	void exitStringList(JetRuleParser.StringListContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#stringSeq}.
	 * @param ctx the parse tree
	 */
	void enterStringSeq(JetRuleParser.StringSeqContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#stringSeq}.
	 * @param ctx the parse tree
	 */
	void exitStringSeq(JetRuleParser.StringSeqContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#columnDefSeq}.
	 * @param ctx the parse tree
	 */
	void enterColumnDefSeq(JetRuleParser.ColumnDefSeqContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#columnDefSeq}.
	 * @param ctx the parse tree
	 */
	void exitColumnDefSeq(JetRuleParser.ColumnDefSeqContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#columnDefinitions}.
	 * @param ctx the parse tree
	 */
	void enterColumnDefinitions(JetRuleParser.ColumnDefinitionsContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#columnDefinitions}.
	 * @param ctx the parse tree
	 */
	void exitColumnDefinitions(JetRuleParser.ColumnDefinitionsContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#jetRuleStmt}.
	 * @param ctx the parse tree
	 */
	void enterJetRuleStmt(JetRuleParser.JetRuleStmtContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#jetRuleStmt}.
	 * @param ctx the parse tree
	 */
	void exitJetRuleStmt(JetRuleParser.JetRuleStmtContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#ruleProperties}.
	 * @param ctx the parse tree
	 */
	void enterRuleProperties(JetRuleParser.RulePropertiesContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#ruleProperties}.
	 * @param ctx the parse tree
	 */
	void exitRuleProperties(JetRuleParser.RulePropertiesContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#propertyValue}.
	 * @param ctx the parse tree
	 */
	void enterPropertyValue(JetRuleParser.PropertyValueContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#propertyValue}.
	 * @param ctx the parse tree
	 */
	void exitPropertyValue(JetRuleParser.PropertyValueContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#antecedent}.
	 * @param ctx the parse tree
	 */
	void enterAntecedent(JetRuleParser.AntecedentContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#antecedent}.
	 * @param ctx the parse tree
	 */
	void exitAntecedent(JetRuleParser.AntecedentContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#consequent}.
	 * @param ctx the parse tree
	 */
	void enterConsequent(JetRuleParser.ConsequentContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#consequent}.
	 * @param ctx the parse tree
	 */
	void exitConsequent(JetRuleParser.ConsequentContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#atom}.
	 * @param ctx the parse tree
	 */
	void enterAtom(JetRuleParser.AtomContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#atom}.
	 * @param ctx the parse tree
	 */
	void exitAtom(JetRuleParser.AtomContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#objectAtom}.
	 * @param ctx the parse tree
	 */
	void enterObjectAtom(JetRuleParser.ObjectAtomContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#objectAtom}.
	 * @param ctx the parse tree
	 */
	void exitObjectAtom(JetRuleParser.ObjectAtomContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#keywords}.
	 * @param ctx the parse tree
	 */
	void enterKeywords(JetRuleParser.KeywordsContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#keywords}.
	 * @param ctx the parse tree
	 */
	void exitKeywords(JetRuleParser.KeywordsContext ctx);
	/**
	 * Enter a parse tree produced by the {@code SelfExprTerm}
	 * labeled alternative in {@link JetRuleParser#exprTerm}.
	 * @param ctx the parse tree
	 */
	void enterSelfExprTerm(JetRuleParser.SelfExprTermContext ctx);
	/**
	 * Exit a parse tree produced by the {@code SelfExprTerm}
	 * labeled alternative in {@link JetRuleParser#exprTerm}.
	 * @param ctx the parse tree
	 */
	void exitSelfExprTerm(JetRuleParser.SelfExprTermContext ctx);
	/**
	 * Enter a parse tree produced by the {@code BinaryExprTerm2}
	 * labeled alternative in {@link JetRuleParser#exprTerm}.
	 * @param ctx the parse tree
	 */
	void enterBinaryExprTerm2(JetRuleParser.BinaryExprTerm2Context ctx);
	/**
	 * Exit a parse tree produced by the {@code BinaryExprTerm2}
	 * labeled alternative in {@link JetRuleParser#exprTerm}.
	 * @param ctx the parse tree
	 */
	void exitBinaryExprTerm2(JetRuleParser.BinaryExprTerm2Context ctx);
	/**
	 * Enter a parse tree produced by the {@code UnaryExprTerm}
	 * labeled alternative in {@link JetRuleParser#exprTerm}.
	 * @param ctx the parse tree
	 */
	void enterUnaryExprTerm(JetRuleParser.UnaryExprTermContext ctx);
	/**
	 * Exit a parse tree produced by the {@code UnaryExprTerm}
	 * labeled alternative in {@link JetRuleParser#exprTerm}.
	 * @param ctx the parse tree
	 */
	void exitUnaryExprTerm(JetRuleParser.UnaryExprTermContext ctx);
	/**
	 * Enter a parse tree produced by the {@code ObjectAtomExprTerm}
	 * labeled alternative in {@link JetRuleParser#exprTerm}.
	 * @param ctx the parse tree
	 */
	void enterObjectAtomExprTerm(JetRuleParser.ObjectAtomExprTermContext ctx);
	/**
	 * Exit a parse tree produced by the {@code ObjectAtomExprTerm}
	 * labeled alternative in {@link JetRuleParser#exprTerm}.
	 * @param ctx the parse tree
	 */
	void exitObjectAtomExprTerm(JetRuleParser.ObjectAtomExprTermContext ctx);
	/**
	 * Enter a parse tree produced by the {@code UnaryExprTerm3}
	 * labeled alternative in {@link JetRuleParser#exprTerm}.
	 * @param ctx the parse tree
	 */
	void enterUnaryExprTerm3(JetRuleParser.UnaryExprTerm3Context ctx);
	/**
	 * Exit a parse tree produced by the {@code UnaryExprTerm3}
	 * labeled alternative in {@link JetRuleParser#exprTerm}.
	 * @param ctx the parse tree
	 */
	void exitUnaryExprTerm3(JetRuleParser.UnaryExprTerm3Context ctx);
	/**
	 * Enter a parse tree produced by the {@code UnaryExprTerm2}
	 * labeled alternative in {@link JetRuleParser#exprTerm}.
	 * @param ctx the parse tree
	 */
	void enterUnaryExprTerm2(JetRuleParser.UnaryExprTerm2Context ctx);
	/**
	 * Exit a parse tree produced by the {@code UnaryExprTerm2}
	 * labeled alternative in {@link JetRuleParser#exprTerm}.
	 * @param ctx the parse tree
	 */
	void exitUnaryExprTerm2(JetRuleParser.UnaryExprTerm2Context ctx);
	/**
	 * Enter a parse tree produced by the {@code BinaryExprTerm}
	 * labeled alternative in {@link JetRuleParser#exprTerm}.
	 * @param ctx the parse tree
	 */
	void enterBinaryExprTerm(JetRuleParser.BinaryExprTermContext ctx);
	/**
	 * Exit a parse tree produced by the {@code BinaryExprTerm}
	 * labeled alternative in {@link JetRuleParser#exprTerm}.
	 * @param ctx the parse tree
	 */
	void exitBinaryExprTerm(JetRuleParser.BinaryExprTermContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#binaryOp}.
	 * @param ctx the parse tree
	 */
	void enterBinaryOp(JetRuleParser.BinaryOpContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#binaryOp}.
	 * @param ctx the parse tree
	 */
	void exitBinaryOp(JetRuleParser.BinaryOpContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#unaryOp}.
	 * @param ctx the parse tree
	 */
	void enterUnaryOp(JetRuleParser.UnaryOpContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#unaryOp}.
	 * @param ctx the parse tree
	 */
	void exitUnaryOp(JetRuleParser.UnaryOpContext ctx);
	/**
	 * Enter a parse tree produced by {@link JetRuleParser#tripleStmt}.
	 * @param ctx the parse tree
	 */
	void enterTripleStmt(JetRuleParser.TripleStmtContext ctx);
	/**
	 * Exit a parse tree produced by {@link JetRuleParser#tripleStmt}.
	 * @param ctx the parse tree
	 */
	void exitTripleStmt(JetRuleParser.TripleStmtContext ctx);
}