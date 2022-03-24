# Generated from JetRule.g4 by ANTLR 4.9.3
from antlr4 import *
if __name__ is not None and "." in __name__:
    from .JetRuleParser import JetRuleParser
else:
    from JetRuleParser import JetRuleParser

# This class defines a complete listener for a parse tree produced by JetRuleParser.
class JetRuleListener(ParseTreeListener):

    # Enter a parse tree produced by JetRuleParser#jetrule.
    def enterJetrule(self, ctx:JetRuleParser.JetruleContext):
        pass

    # Exit a parse tree produced by JetRuleParser#jetrule.
    def exitJetrule(self, ctx:JetRuleParser.JetruleContext):
        pass


    # Enter a parse tree produced by JetRuleParser#statement.
    def enterStatement(self, ctx:JetRuleParser.StatementContext):
        pass

    # Exit a parse tree produced by JetRuleParser#statement.
    def exitStatement(self, ctx:JetRuleParser.StatementContext):
        pass


    # Enter a parse tree produced by JetRuleParser#jetCompilerDirectiveStmt.
    def enterJetCompilerDirectiveStmt(self, ctx:JetRuleParser.JetCompilerDirectiveStmtContext):
        pass

    # Exit a parse tree produced by JetRuleParser#jetCompilerDirectiveStmt.
    def exitJetCompilerDirectiveStmt(self, ctx:JetRuleParser.JetCompilerDirectiveStmtContext):
        pass


    # Enter a parse tree produced by JetRuleParser#defineClassStmt.
    def enterDefineClassStmt(self, ctx:JetRuleParser.DefineClassStmtContext):
        pass

    # Exit a parse tree produced by JetRuleParser#defineClassStmt.
    def exitDefineClassStmt(self, ctx:JetRuleParser.DefineClassStmtContext):
        pass


    # Enter a parse tree produced by JetRuleParser#subClassOfStmt.
    def enterSubClassOfStmt(self, ctx:JetRuleParser.SubClassOfStmtContext):
        pass

    # Exit a parse tree produced by JetRuleParser#subClassOfStmt.
    def exitSubClassOfStmt(self, ctx:JetRuleParser.SubClassOfStmtContext):
        pass


    # Enter a parse tree produced by JetRuleParser#dataPropertyStmt.
    def enterDataPropertyStmt(self, ctx:JetRuleParser.DataPropertyStmtContext):
        pass

    # Exit a parse tree produced by JetRuleParser#dataPropertyStmt.
    def exitDataPropertyStmt(self, ctx:JetRuleParser.DataPropertyStmtContext):
        pass


    # Enter a parse tree produced by JetRuleParser#asTableStmt.
    def enterAsTableStmt(self, ctx:JetRuleParser.AsTableStmtContext):
        pass

    # Exit a parse tree produced by JetRuleParser#asTableStmt.
    def exitAsTableStmt(self, ctx:JetRuleParser.AsTableStmtContext):
        pass


    # Enter a parse tree produced by JetRuleParser#asTableFlag.
    def enterAsTableFlag(self, ctx:JetRuleParser.AsTableFlagContext):
        pass

    # Exit a parse tree produced by JetRuleParser#asTableFlag.
    def exitAsTableFlag(self, ctx:JetRuleParser.AsTableFlagContext):
        pass


    # Enter a parse tree produced by JetRuleParser#dataPropertyType.
    def enterDataPropertyType(self, ctx:JetRuleParser.DataPropertyTypeContext):
        pass

    # Exit a parse tree produced by JetRuleParser#dataPropertyType.
    def exitDataPropertyType(self, ctx:JetRuleParser.DataPropertyTypeContext):
        pass


    # Enter a parse tree produced by JetRuleParser#defineLiteralStmt.
    def enterDefineLiteralStmt(self, ctx:JetRuleParser.DefineLiteralStmtContext):
        pass

    # Exit a parse tree produced by JetRuleParser#defineLiteralStmt.
    def exitDefineLiteralStmt(self, ctx:JetRuleParser.DefineLiteralStmtContext):
        pass


    # Enter a parse tree produced by JetRuleParser#int32LiteralStmt.
    def enterInt32LiteralStmt(self, ctx:JetRuleParser.Int32LiteralStmtContext):
        pass

    # Exit a parse tree produced by JetRuleParser#int32LiteralStmt.
    def exitInt32LiteralStmt(self, ctx:JetRuleParser.Int32LiteralStmtContext):
        pass


    # Enter a parse tree produced by JetRuleParser#uInt32LiteralStmt.
    def enterUInt32LiteralStmt(self, ctx:JetRuleParser.UInt32LiteralStmtContext):
        pass

    # Exit a parse tree produced by JetRuleParser#uInt32LiteralStmt.
    def exitUInt32LiteralStmt(self, ctx:JetRuleParser.UInt32LiteralStmtContext):
        pass


    # Enter a parse tree produced by JetRuleParser#int64LiteralStmt.
    def enterInt64LiteralStmt(self, ctx:JetRuleParser.Int64LiteralStmtContext):
        pass

    # Exit a parse tree produced by JetRuleParser#int64LiteralStmt.
    def exitInt64LiteralStmt(self, ctx:JetRuleParser.Int64LiteralStmtContext):
        pass


    # Enter a parse tree produced by JetRuleParser#uInt64LiteralStmt.
    def enterUInt64LiteralStmt(self, ctx:JetRuleParser.UInt64LiteralStmtContext):
        pass

    # Exit a parse tree produced by JetRuleParser#uInt64LiteralStmt.
    def exitUInt64LiteralStmt(self, ctx:JetRuleParser.UInt64LiteralStmtContext):
        pass


    # Enter a parse tree produced by JetRuleParser#doubleLiteralStmt.
    def enterDoubleLiteralStmt(self, ctx:JetRuleParser.DoubleLiteralStmtContext):
        pass

    # Exit a parse tree produced by JetRuleParser#doubleLiteralStmt.
    def exitDoubleLiteralStmt(self, ctx:JetRuleParser.DoubleLiteralStmtContext):
        pass


    # Enter a parse tree produced by JetRuleParser#stringLiteralStmt.
    def enterStringLiteralStmt(self, ctx:JetRuleParser.StringLiteralStmtContext):
        pass

    # Exit a parse tree produced by JetRuleParser#stringLiteralStmt.
    def exitStringLiteralStmt(self, ctx:JetRuleParser.StringLiteralStmtContext):
        pass


    # Enter a parse tree produced by JetRuleParser#dateLiteralStmt.
    def enterDateLiteralStmt(self, ctx:JetRuleParser.DateLiteralStmtContext):
        pass

    # Exit a parse tree produced by JetRuleParser#dateLiteralStmt.
    def exitDateLiteralStmt(self, ctx:JetRuleParser.DateLiteralStmtContext):
        pass


    # Enter a parse tree produced by JetRuleParser#datetimeLiteralStmt.
    def enterDatetimeLiteralStmt(self, ctx:JetRuleParser.DatetimeLiteralStmtContext):
        pass

    # Exit a parse tree produced by JetRuleParser#datetimeLiteralStmt.
    def exitDatetimeLiteralStmt(self, ctx:JetRuleParser.DatetimeLiteralStmtContext):
        pass


    # Enter a parse tree produced by JetRuleParser#intExpr.
    def enterIntExpr(self, ctx:JetRuleParser.IntExprContext):
        pass

    # Exit a parse tree produced by JetRuleParser#intExpr.
    def exitIntExpr(self, ctx:JetRuleParser.IntExprContext):
        pass


    # Enter a parse tree produced by JetRuleParser#uintExpr.
    def enterUintExpr(self, ctx:JetRuleParser.UintExprContext):
        pass

    # Exit a parse tree produced by JetRuleParser#uintExpr.
    def exitUintExpr(self, ctx:JetRuleParser.UintExprContext):
        pass


    # Enter a parse tree produced by JetRuleParser#doubleExpr.
    def enterDoubleExpr(self, ctx:JetRuleParser.DoubleExprContext):
        pass

    # Exit a parse tree produced by JetRuleParser#doubleExpr.
    def exitDoubleExpr(self, ctx:JetRuleParser.DoubleExprContext):
        pass


    # Enter a parse tree produced by JetRuleParser#declIdentifier.
    def enterDeclIdentifier(self, ctx:JetRuleParser.DeclIdentifierContext):
        pass

    # Exit a parse tree produced by JetRuleParser#declIdentifier.
    def exitDeclIdentifier(self, ctx:JetRuleParser.DeclIdentifierContext):
        pass


    # Enter a parse tree produced by JetRuleParser#defineResourceStmt.
    def enterDefineResourceStmt(self, ctx:JetRuleParser.DefineResourceStmtContext):
        pass

    # Exit a parse tree produced by JetRuleParser#defineResourceStmt.
    def exitDefineResourceStmt(self, ctx:JetRuleParser.DefineResourceStmtContext):
        pass


    # Enter a parse tree produced by JetRuleParser#namedResourceStmt.
    def enterNamedResourceStmt(self, ctx:JetRuleParser.NamedResourceStmtContext):
        pass

    # Exit a parse tree produced by JetRuleParser#namedResourceStmt.
    def exitNamedResourceStmt(self, ctx:JetRuleParser.NamedResourceStmtContext):
        pass


    # Enter a parse tree produced by JetRuleParser#volatileResourceStmt.
    def enterVolatileResourceStmt(self, ctx:JetRuleParser.VolatileResourceStmtContext):
        pass

    # Exit a parse tree produced by JetRuleParser#volatileResourceStmt.
    def exitVolatileResourceStmt(self, ctx:JetRuleParser.VolatileResourceStmtContext):
        pass


    # Enter a parse tree produced by JetRuleParser#resourceValue.
    def enterResourceValue(self, ctx:JetRuleParser.ResourceValueContext):
        pass

    # Exit a parse tree produced by JetRuleParser#resourceValue.
    def exitResourceValue(self, ctx:JetRuleParser.ResourceValueContext):
        pass


    # Enter a parse tree produced by JetRuleParser#lookupTableStmt.
    def enterLookupTableStmt(self, ctx:JetRuleParser.LookupTableStmtContext):
        pass

    # Exit a parse tree produced by JetRuleParser#lookupTableStmt.
    def exitLookupTableStmt(self, ctx:JetRuleParser.LookupTableStmtContext):
        pass


    # Enter a parse tree produced by JetRuleParser#csvLocation.
    def enterCsvLocation(self, ctx:JetRuleParser.CsvLocationContext):
        pass

    # Exit a parse tree produced by JetRuleParser#csvLocation.
    def exitCsvLocation(self, ctx:JetRuleParser.CsvLocationContext):
        pass


    # Enter a parse tree produced by JetRuleParser#stringList.
    def enterStringList(self, ctx:JetRuleParser.StringListContext):
        pass

    # Exit a parse tree produced by JetRuleParser#stringList.
    def exitStringList(self, ctx:JetRuleParser.StringListContext):
        pass


    # Enter a parse tree produced by JetRuleParser#stringSeq.
    def enterStringSeq(self, ctx:JetRuleParser.StringSeqContext):
        pass

    # Exit a parse tree produced by JetRuleParser#stringSeq.
    def exitStringSeq(self, ctx:JetRuleParser.StringSeqContext):
        pass


    # Enter a parse tree produced by JetRuleParser#columnDefinitions.
    def enterColumnDefinitions(self, ctx:JetRuleParser.ColumnDefinitionsContext):
        pass

    # Exit a parse tree produced by JetRuleParser#columnDefinitions.
    def exitColumnDefinitions(self, ctx:JetRuleParser.ColumnDefinitionsContext):
        pass


    # Enter a parse tree produced by JetRuleParser#jetRuleStmt.
    def enterJetRuleStmt(self, ctx:JetRuleParser.JetRuleStmtContext):
        pass

    # Exit a parse tree produced by JetRuleParser#jetRuleStmt.
    def exitJetRuleStmt(self, ctx:JetRuleParser.JetRuleStmtContext):
        pass


    # Enter a parse tree produced by JetRuleParser#ruleProperties.
    def enterRuleProperties(self, ctx:JetRuleParser.RulePropertiesContext):
        pass

    # Exit a parse tree produced by JetRuleParser#ruleProperties.
    def exitRuleProperties(self, ctx:JetRuleParser.RulePropertiesContext):
        pass


    # Enter a parse tree produced by JetRuleParser#propertyValue.
    def enterPropertyValue(self, ctx:JetRuleParser.PropertyValueContext):
        pass

    # Exit a parse tree produced by JetRuleParser#propertyValue.
    def exitPropertyValue(self, ctx:JetRuleParser.PropertyValueContext):
        pass


    # Enter a parse tree produced by JetRuleParser#antecedent.
    def enterAntecedent(self, ctx:JetRuleParser.AntecedentContext):
        pass

    # Exit a parse tree produced by JetRuleParser#antecedent.
    def exitAntecedent(self, ctx:JetRuleParser.AntecedentContext):
        pass


    # Enter a parse tree produced by JetRuleParser#consequent.
    def enterConsequent(self, ctx:JetRuleParser.ConsequentContext):
        pass

    # Exit a parse tree produced by JetRuleParser#consequent.
    def exitConsequent(self, ctx:JetRuleParser.ConsequentContext):
        pass


    # Enter a parse tree produced by JetRuleParser#atom.
    def enterAtom(self, ctx:JetRuleParser.AtomContext):
        pass

    # Exit a parse tree produced by JetRuleParser#atom.
    def exitAtom(self, ctx:JetRuleParser.AtomContext):
        pass


    # Enter a parse tree produced by JetRuleParser#objectAtom.
    def enterObjectAtom(self, ctx:JetRuleParser.ObjectAtomContext):
        pass

    # Exit a parse tree produced by JetRuleParser#objectAtom.
    def exitObjectAtom(self, ctx:JetRuleParser.ObjectAtomContext):
        pass


    # Enter a parse tree produced by JetRuleParser#keywords.
    def enterKeywords(self, ctx:JetRuleParser.KeywordsContext):
        pass

    # Exit a parse tree produced by JetRuleParser#keywords.
    def exitKeywords(self, ctx:JetRuleParser.KeywordsContext):
        pass


    # Enter a parse tree produced by JetRuleParser#SelfExprTerm.
    def enterSelfExprTerm(self, ctx:JetRuleParser.SelfExprTermContext):
        pass

    # Exit a parse tree produced by JetRuleParser#SelfExprTerm.
    def exitSelfExprTerm(self, ctx:JetRuleParser.SelfExprTermContext):
        pass


    # Enter a parse tree produced by JetRuleParser#BinaryExprTerm2.
    def enterBinaryExprTerm2(self, ctx:JetRuleParser.BinaryExprTerm2Context):
        pass

    # Exit a parse tree produced by JetRuleParser#BinaryExprTerm2.
    def exitBinaryExprTerm2(self, ctx:JetRuleParser.BinaryExprTerm2Context):
        pass


    # Enter a parse tree produced by JetRuleParser#UnaryExprTerm.
    def enterUnaryExprTerm(self, ctx:JetRuleParser.UnaryExprTermContext):
        pass

    # Exit a parse tree produced by JetRuleParser#UnaryExprTerm.
    def exitUnaryExprTerm(self, ctx:JetRuleParser.UnaryExprTermContext):
        pass


    # Enter a parse tree produced by JetRuleParser#ObjectAtomExprTerm.
    def enterObjectAtomExprTerm(self, ctx:JetRuleParser.ObjectAtomExprTermContext):
        pass

    # Exit a parse tree produced by JetRuleParser#ObjectAtomExprTerm.
    def exitObjectAtomExprTerm(self, ctx:JetRuleParser.ObjectAtomExprTermContext):
        pass


    # Enter a parse tree produced by JetRuleParser#UnaryExprTerm3.
    def enterUnaryExprTerm3(self, ctx:JetRuleParser.UnaryExprTerm3Context):
        pass

    # Exit a parse tree produced by JetRuleParser#UnaryExprTerm3.
    def exitUnaryExprTerm3(self, ctx:JetRuleParser.UnaryExprTerm3Context):
        pass


    # Enter a parse tree produced by JetRuleParser#UnaryExprTerm2.
    def enterUnaryExprTerm2(self, ctx:JetRuleParser.UnaryExprTerm2Context):
        pass

    # Exit a parse tree produced by JetRuleParser#UnaryExprTerm2.
    def exitUnaryExprTerm2(self, ctx:JetRuleParser.UnaryExprTerm2Context):
        pass


    # Enter a parse tree produced by JetRuleParser#BinaryExprTerm.
    def enterBinaryExprTerm(self, ctx:JetRuleParser.BinaryExprTermContext):
        pass

    # Exit a parse tree produced by JetRuleParser#BinaryExprTerm.
    def exitBinaryExprTerm(self, ctx:JetRuleParser.BinaryExprTermContext):
        pass


    # Enter a parse tree produced by JetRuleParser#binaryOp.
    def enterBinaryOp(self, ctx:JetRuleParser.BinaryOpContext):
        pass

    # Exit a parse tree produced by JetRuleParser#binaryOp.
    def exitBinaryOp(self, ctx:JetRuleParser.BinaryOpContext):
        pass


    # Enter a parse tree produced by JetRuleParser#unaryOp.
    def enterUnaryOp(self, ctx:JetRuleParser.UnaryOpContext):
        pass

    # Exit a parse tree produced by JetRuleParser#unaryOp.
    def exitUnaryOp(self, ctx:JetRuleParser.UnaryOpContext):
        pass


    # Enter a parse tree produced by JetRuleParser#tripleStmt.
    def enterTripleStmt(self, ctx:JetRuleParser.TripleStmtContext):
        pass

    # Exit a parse tree produced by JetRuleParser#tripleStmt.
    def exitTripleStmt(self, ctx:JetRuleParser.TripleStmtContext):
        pass



del JetRuleParser