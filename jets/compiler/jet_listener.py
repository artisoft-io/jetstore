from typing import Dict
import antlr4 as a4
import json
import re
from JetRuleParser import JetRuleParser
from JetRuleLexer import JetRuleLexer
from JetRuleListener import JetRuleListener

class JetListener(JetRuleListener):

  def __init__(self, main_rule_file=None):

    # keeping track which file we're parsing
    self.main_rule_file = main_rule_file
    self.current_file_name = None

    # Compiler directives
    self.compiler_directives = {}

    # JetStore config obj, only one per main
    self.jetstore_config = {}

    # Define our state model
    self.classes = []
    self.base_classes = []
    self.data_properties = []
    self.grouping_properties = []
    self.as_table = None

    self.literals = []
    self.resources = []
    self.lookups = []
    
    self.ruleSequences = []
    self.main_rule_sets = []

    self.rules = []
    self.triples = []
    self.jetRules = None
    self.implicit_number_re = re.compile(r'^\+?\-?\d+\.?\d*$')

    # Defining intermediate structure for Jet Rule
    self.ruleProps = {}
    self.ruleAntecedents = []

  # Enter a parse tree produced by JetRuleParser#jetrule.
  def enterJetrule(self, ctx:JetRuleParser.JetruleContext):
    # print('Starting Visiting Rule File...')
    pass

  # Exit a parse tree produced by JetRuleParser#jetrule.
  def exitJetrule(self, ctx:JetRuleParser.JetruleContext):
    # print('Finished Visiting Rule File')
    self.jetRules = {
      'literals': self.literals,
      'resources': self.resources,
      'lookup_tables': self.lookups,
      'jet_rules': self.rules,
    }
    if self.ruleSequences:
      self.jetRules['rule_sequences'] = self.ruleSequences
    if self.classes:
      self.jetRules['classes'] = self.classes
    if self.triples:
      self.jetRules['triples'] = self.triples
    if self.compiler_directives:
      self.jetRules['compiler_directives'] = self.compiler_directives
    if self.jetstore_config:
      self.jetstore_config['type'] = 'jsconfig'
      if self.main_rule_file:
        self.jetstore_config['source_file_name'] = self.main_rule_file
      self.jetRules['jetstore_config'] = dict(sorted(self.jetstore_config.items()))

  # =====================================================================================
  # Compiler Directives
  # -------------------------------------------------------------------------------------
  def exitJetCompilerDirectiveStmt(self, ctx:JetRuleParser.JetCompilerDirectiveStmtContext):
    name = self.escape(ctx.varName.getText()) if ctx.varName else None
    value = self.escapeString(ctx.declValue.text) if ctx.declValue else None
    if name and value:
      if name == 'source_file':
        self.current_file_name = value
      else:
        self.compiler_directives[name] = value

  # Exit a parse tree produced by JetRuleParser#defineLiteralStmt.
  def exitDefineLiteralStmt(self, ctx:JetRuleParser.DefineLiteralStmtContext):
    if self.current_file_name and len(self.literals) > 0:
      self.literals[-1]['source_file_name'] = self.current_file_name
  
  # Exit a parse tree produced by JetRuleParser#defineResourceStmt.
  def exitDefineResourceStmt(self, ctx:JetRuleParser.DefineResourceStmtContext):
    if self.current_file_name and len(self.resources) > 0:
      self.resources[-1]['source_file_name'] = self.current_file_name

  # =====================================================================================
  # JetStore Config
  # -------------------------------------------------------------------------------------
  # Exit a parse tree produced by JetRuleParser#jetstoreConfigItem.
  def exitJetstoreConfigItem(self, ctx:JetRuleParser.JetstoreConfigItemContext):      
    if not ctx.configKey:
      return
    keys = []
    for v in ctx.rdfTypeList:
      keys.append(self.escapeString(v.getText()))
    if len(keys) > 0:
      self.jetstore_config[ctx.configKey.text] = keys
      return
    if not ctx.configValue:
      return
    self.jetstore_config[ctx.configKey.text] = ctx.configValue.getText()

  # =====================================================================================
  # Class Definition
  # -------------------------------------------------------------------------------------
  # Enter a parse tree produced by JetRuleParser#defineClassStmt.
  def enterDefineClassStmt(self, ctx:JetRuleParser.DefineClassStmtContext):
    # Entering a class definition
    self.base_classes = []
    self.data_properties = []
    self.grouping_properties = []
    self.as_table = None

  # Exit a parse tree produced by JetRuleParser#defineClassStmt.
  def exitDefineClassStmt(self, ctx:JetRuleParser.DefineClassStmtContext):
    # Putting the class definition together
    if not ctx.className:
      return

    class_def = {
      'type': 'class',
      'name': self.escape(ctx.className.getText()), 
      'base_classes': self.base_classes, 
      'data_properties': self.data_properties, 
    }
    if self.current_file_name:
      class_def['source_file_name'] = self.current_file_name
    if self.as_table:
      class_def['as_table'] = self.as_table
    if self.grouping_properties:
      class_def['grouping_properties'] = self.grouping_properties
    self.classes.append(class_def)

  # Exit a parse tree produced by JetRuleParser#subClassOfStmt.
  def exitSubClassOfStmt(self, ctx:JetRuleParser.SubClassOfStmtContext):
    if ctx.baseClassName:
      self.base_classes.append(self.escape(ctx.baseClassName.getText()))

  # Exit a parse tree produced by JetRuleParser#asTableStmt.
  def exitAsTableStmt(self, ctx:JetRuleParser.AsTableStmtContext):
    self.as_table = ctx.asTable.getText()

  # Exit a parse tree produced by JetRuleParser#groupingPropertyStmt.
  def exitGroupingPropertyStmt(self, ctx:JetRuleParser.GroupingPropertyStmtContext):
    if ctx.groupingPropertyName:
      self.grouping_properties.append(self.escape(ctx.groupingPropertyName.getText()))

  # Exit a parse tree produced by JetRuleParser#dataPropertyDefinitions.
  def exitDataPropertyDefinitions(self, ctx:JetRuleParser.DataPropertyDefinitionsContext):
    if not ctx.dataPName or not ctx.dataPType:
      return
    data_property = {
      'name': self.escape(ctx.dataPName.getText()), 
      'type': ctx.dataPType.getText(), 
    }
    if ctx.array:
      data_property['as_array'] = 'true'
    else:
      data_property['as_array'] = 'false'
    self.data_properties.append(data_property)


  # =====================================================================================
  # Rule Sequence Definition
  # -------------------------------------------------------------------------------------
  # Enter a parse tree produced by JetRuleParser#defineRuleSeqStmt.
  def enterDefineRuleSeqStmt(self, ctx:JetRuleParser.DefineRuleSeqStmtContext):
    # Entering a class definition
    self.main_rule_sets = []

  # Exit a parse tree produced by JetRuleParser#ruleSetDefinitions.
  def exitRuleSetDefinitions(self, ctx:JetRuleParser.RuleSetDefinitionsContext):
    if ctx.rsName:
      self.main_rule_sets.append(self.escapeString(ctx.rsName.text))

  # Exit a parse tree produced by JetRuleParser#defineRuleSeqStmt.
  def exitDefineRuleSeqStmt(self, ctx:JetRuleParser.DefineRuleSeqStmtContext):
    # Putting the class definition together
    if not ctx.ruleseqName:
      return
    ruleSeq = {  
      'type': 'ruleseq',
      'name': ctx.ruleseqName.text,    
      'main_rule_sets': self.main_rule_sets, 
    }
    if self.current_file_name:
      ruleSeq['source_file_name'] = self.current_file_name
    self.ruleSequences.append(ruleSeq)


  # =====================================================================================
  # Literals
  # -------------------------------------------------------------------------------------
  def exitInt32LiteralStmt(self, ctx:JetRuleParser.Int32LiteralStmtContext):
    # print('@@@ exitInt32LiteralStmt ::',ctx.Int32Type(),'::',ctx.declIdentifier(),'::',ctx.ASSIGN(),'::',ctx.intExpr(),'||+',ctx.declValue.PLUS(),'||-',ctx.declValue.MINUS(),'||D',ctx.declValue.DIGITS())
    if ctx.varType and ctx.varName and ctx.declValue:
      self.literals.append({ 'type': ctx.varType.text, 'id': ctx.varName.getText(), 'value':  ctx.declValue.getText()})

  def exitUInt32LiteralStmt(self, ctx:JetRuleParser.UInt32LiteralStmtContext):
    if ctx.varType and ctx.varName and ctx.declValue:
      self.literals.append({ 'type': ctx.varType.text, 'id': ctx.varName.getText(), 'value':  ctx.declValue.getText()})

  def exitInt64LiteralStmt(self, ctx:JetRuleParser.Int64LiteralStmtContext):
    if ctx.varType and ctx.varName and ctx.declValue:
      self.literals.append({ 'type': ctx.varType.text, 'id': ctx.varName.getText(), 'value':  ctx.declValue.getText()})

  def exitUInt64LiteralStmt(self, ctx:JetRuleParser.UInt64LiteralStmtContext):
    if ctx.varType and ctx.varName and ctx.declValue:
      self.literals.append({ 'type': ctx.varType.text, 'id': ctx.varName.getText(), 'value':  ctx.declValue.getText()})

  def exitDoubleLiteralStmt(self, ctx:JetRuleParser.DoubleLiteralStmtContext):
    if ctx.varType and ctx.varName and ctx.declValue:
      self.literals.append({ 'type': ctx.varType.text, 'id': ctx.varName.getText(), 'value':  ctx.declValue.getText()})

  def exitStringLiteralStmt(self, ctx:JetRuleParser.StringLiteralStmtContext):
    if ctx.varType and ctx.varName and ctx.declValue:
      self.literals.append({ 'type': ctx.varType.text, 'id': ctx.varName.getText(), 'value': self.escapeString(ctx.declValue.text)})

  def exitDateLiteralStmt(self, ctx:JetRuleParser.DateLiteralStmtContext):
    if ctx.varType and ctx.varName and ctx.declValue:
      self.literals.append({ 'type': ctx.varType.text, 'id': ctx.varName.getText(), 'value': self.escapeString(ctx.declValue.text)})

  def exitDatetimeLiteralStmt(self, ctx:JetRuleParser.DatetimeLiteralStmtContext):
    if ctx.varType and ctx.varName and ctx.declValue:
      self.literals.append({ 'type': ctx.varType.text, 'id': ctx.varName.getText(), 'value': self.escapeString(ctx.declValue.text)})

  # Exit a parse tree produced by JetRuleParser#booleanLiteralStmt.
  def exitBooleanLiteralStmt(self, ctx:JetRuleParser.BooleanLiteralStmtContext):
    if ctx.varType and ctx.varName and ctx.declValue:
      self.literals.append({ 'type': ctx.varType.text, 'id': ctx.varName.getText(), 'value': self.escapeString(ctx.declValue.text)})

  # =====================================================================================
  # Resources
  # -------------------------------------------------------------------------------------
  def exitNamedResourceStmt(self, ctx:JetRuleParser.NamedResourceStmtContext):
    if not ctx.resCtx or not ctx.resName:
      return
    id = self.escape(ctx.resName.getText())
    value = None
    if ctx.resCtx.resVal:
      value = ctx.resCtx.resVal.text
    elif ctx.resCtx.kws:
      value = ctx.resCtx.kws.getText()

    if value is None:
      return

    if value[0]=='"':
      value = self.escapeString(ctx.resCtx.resVal.text)
      self.resources.append({ 'type': 'resource', 'id': id, 'value':  value})
    else:
      symbol = value
      self.resources.append({ 'type': 'resource', 'id': id, 'symbol': symbol, 'value':  None})

  def exitVolatileResourceStmt(self, ctx:JetRuleParser.VolatileResourceStmtContext):
    id = self.escape(ctx.resName.getText()) if ctx.resName else None
    value = self.escapeString(ctx.resVal.text) if ctx.resVal else None
    self.resources.append({ 'type': 'volatile_resource', 'id': id, 'value': value })

  # =====================================================================================
  # Lookup Tables
  # -------------------------------------------------------------------------------------
  # Enter a parse tree produced by JetRuleParser#lookupTableStmt.
  def enterLookupTableStmt(self, ctx:JetRuleParser.LookupTableStmtContext):
      self.columnsDefs = []

  # Exit a parse tree produced by JetRuleParser#columnDefinitions.
  def exitColumnDefinitions(self, ctx:JetRuleParser.ColumnDefinitionsContext):
    if not ctx.columnName or not ctx.columnType: return
    columnsDef = {
      'name': self.escapeString(ctx.columnName.text),
      'type': ctx.columnType.getText(), 
    }
    if ctx.array:
      columnsDef['as_array'] = 'true'
    else:
      columnsDef['as_array'] = 'false'
    self.columnsDefs.append(columnsDef)

  def exitLookupTableStmt(self, ctx:JetRuleParser.LookupTableStmtContext):
    csvLocationCtx: JetRuleParser.CsvLocationContext = ctx.csvLocation()
    if not csvLocationCtx: return
    if not csvLocationCtx.tblStorageName and not csvLocationCtx.csvFileName: return
    if not ctx.tblKeys: return
    
    keys = []
    for v in ctx.tblKeys.seqCtx.slist:
      keys.append(self.escapeString(v.text))

    lookupTbl = {
      'type': 'lookup',
      'name': ctx.lookupName.getText(), 
      'key': keys, 
      'columns': self.columnsDefs
    }

    # case it's from a table
    if csvLocationCtx.tblStorageName:
      lookupTbl['table'] = csvLocationCtx.tblStorageName.getText()

    # case it's from a csv file
    if csvLocationCtx.csvFileName:
      lookupTbl['csv_file'] = self.escapeString(csvLocationCtx.csvFileName.text)
    
    if self.current_file_name:
      lookupTbl['source_file_name'] = self.current_file_name
    self.lookups.append(lookupTbl)

  # =====================================================================================
  # Jet Rules
  # -------------------------------------------------------------------------------------
  # Enter a parse tree produced by JetRuleParser#jetRuleStmt.
  def enterJetRuleStmt(self, ctx:JetRuleParser.JetRuleStmtContext):
    # Entering a Jet Rule
    # Reseting intermediate structure for Jet Rule
    self.ruleProps = {}
    self.ruleAntecedents = []
    self.ruleConsequents = []

  # Exit a parse tree produced by JetRuleParser#jetRuleStmt.
  def exitJetRuleStmt(self, ctx:JetRuleParser.JetRuleStmtContext):
    if not ctx.ruleName:
      return
    # Putting the rule together
    jet_rule = {
      'name': ctx.ruleName.text, 
      'properties': self.ruleProps, 
      'antecedents': self.ruleAntecedents,
      'consequents': self.ruleConsequents  
    }
    if self.current_file_name:
      jet_rule['source_file_name'] = self.current_file_name
    self.rules.append(jet_rule)

  # Exit a parse tree produced by JetRuleParser#ruleProperties.
  def exitRuleProperties(self, ctx:JetRuleParser.RulePropertiesContext):
    key = ctx.key.text
    val = ctx.valCtx.val
    # val = self.escapeString(val.text) if val else ctx.valCtx.intval.getText()
    if val:
      val = val.text 
    else: 
      if ctx.valCtx.intval:
        val = ctx.valCtx.intval.getText()
    self.ruleProps[key] = val

  # Function to remove the escape \" for resource with name clashing reserved keywords
  def escape(self, txt:str) -> str:
    if not txt:
      return txt
    pos1 = txt.find(':')
    if pos1>0:
      pos2 = txt.find('"')
      if pos2 == pos1+1:
        return txt.replace('"', '')
    return txt

  # Function to escape String tokens
  def escapeString(self, txt: str) -> str:
    # make sure it's a String
    if not txt or txt[0]!='"':
      return txt
    return txt.replace('\\"', '"')[1:-1]

  # Function to identify the type of the triple atom
  # This function require the use of escape function first, the call of escape
  # is not included here to avoid duplication in function call
  def parseObjectAtom(self, txt:str, kws: JetRuleParser.KeywordsContext) -> Dict[str, str]:
    # possible inputs:
    #   ?clm        -> {type: "var", value: "?clm"}
    #   rdf:type    -> {type: "identifier", value: "rdf:type"}
    #   localVal    -> {type: "identifier", value: "localVal"}
    #   "XYZ"       -> {type: "text", value: "XYZ"}
    #   text("XYZ") -> {type: "text", value: "XYZ"}
    #   int(1)      -> {type: "int", value: "1"}
    #   bool("1")   -> {type: "bool", value: "1"}
    #   true        -> {type: "keyword", value: "true"}
    #   -123        -> {type: "int", value: "-123"}
    #   +12.3       -> {type: "double", value: "+12.3"}
    if not txt: return None
    if txt[0] == '?': return {'type': 'var', 'value': txt}
    if txt[0] == '"': return {'type': 'text', 'value': self.escapeString(txt)}
    v = txt.split('(')
    if len(v) > 1:
      if v[0] in ['text', 'date', 'datetime', 'bool']:
        return {'type': v[0], 'value': self.escapeString(v[1])[:-1]}
      else:
        return {'type': v[0], 'value': v[1][0:-1]}
    # Check if it's a keyword
    if kws:
      return {'type': "keyword", 'value': txt}
    # Check if it's an int or double as digits
    if self.implicit_number_re.match(txt):
      # got an int or double
      if '.' in txt:
        return {'type': "double", 'value': txt}
      else:
        return {'type': "int", 'value': txt}

    # default is an identifier
    return {'type': "identifier", 'value': txt}

  # Exit a parse tree produced by JetRuleParser#antecedent.
  def exitAntecedent(self, ctx:JetRuleParser.AntecedentContext):
    try:
      subject = self.parseObjectAtom(self.escape(ctx.s.getText()), None)
      predicate = self.parseObjectAtom(self.escape(ctx.p.getText()), None)
      object = self.parseObjectAtom(ctx.o.getText(), ctx.o.kws)
      antecedent = { 'type': 'antecedent', 'isNot': True if ctx.n else False, 'triple':[subject, predicate, object] }
      if ctx.f and ctx.f.expr:
        antecedent['filter'] = ctx.f.expr
      self.ruleAntecedents.append(antecedent)
    except:
      pass

  # =====================================================================================
  # Triples
  # -------------------------------------------------------------------------------------
  # Exit a parse tree produced by JetRuleParser#tripleStmt.
  def exitTripleStmt(self, ctx:JetRuleParser.TripleStmtContext):
    try:
      subject = self.parseObjectAtom(self.escape(ctx.s.getText()), None)
      predicate = self.parseObjectAtom(self.escape(ctx.p.getText()), None)
      object = self.parseObjectAtom(ctx.o.getText(), ctx.o.kws)
      triple = { 'type': 'triple', 
                'subject':subject, 'predicate':predicate, 'object':object,
                'source_file_name': self.current_file_name }
      self.triples.append(triple)
    except:
      pass

  # Exit a parse tree produced by JetRuleParser#consequent.
  def exitConsequent(self, ctx:JetRuleParser.ConsequentContext):
    try:
      subject = self.parseObjectAtom(self.escape(ctx.s.getText()), None)
      predicate = self.parseObjectAtom(self.escape(ctx.p.getText()), None)
      self.ruleConsequents.append({ 'type': 'consequent', 'triple':[subject, predicate, ctx.o.expr] })
    except:
      pass

  # Exit a parse tree produced by JetRuleParser#BinaryExprTerm.
  def exitBinaryExprTerm(self, ctx:JetRuleParser.BinaryExprTermContext):
    try:
      ctx.expr = {'type': 'binary', 'lhs': ctx.lhs.expr, 'op': ctx.op.getText(), 'rhs': ctx.rhs.expr}
    except:
      pass

  # Exit a parse tree produced by JetRuleParser#BinaryExprTerm2.
  def exitBinaryExprTerm2(self, ctx:JetRuleParser.BinaryExprTerm2Context):
    try:
      ctx.expr = {'type': 'binary', 'lhs': ctx.lhs.expr, 'op': ctx.op.getText(), 'rhs': ctx.rhs.expr}
    except:
      pass

  # Exit a parse tree produced by JetRuleParser#UnaryExprTerm.
  def exitUnaryExprTerm(self, ctx:JetRuleParser.UnaryExprTermContext):
    try:
      ctx.expr = {'type': 'unary', 'op': ctx.op.getText(), 'arg': ctx.arg.expr}
    except:
      pass

  # Exit a parse tree produced by JetRuleParser#UnaryExprTerm2.
  def exitUnaryExprTerm2(self, ctx:JetRuleParser.UnaryExprTerm2Context):
    try:
      ctx.expr = {'type': 'unary', 'op': ctx.op.getText(), 'arg': ctx.arg.expr}
    except:
      pass

  # Exit a parse tree produced by JetRuleParser#SelfExprTerm.
  def exitSelfExprTerm(self, ctx:JetRuleParser.SelfExprTermContext):
    try:
      ctx.expr = ctx.selfExpr.expr
    except:
      pass

  # Exit a parse tree produced by JetRuleParser#UnaryExprTerm3.
  def exitUnaryExprTerm3(self, ctx:JetRuleParser.UnaryExprTerm3Context):
    try:
      ctx.expr = {'type': 'unary', 'op': ctx.op.getText(), 'arg': ctx.arg.expr}
    except:
      pass

  # Exit a parse tree produced by JetRuleParser#ObjectAtomExprTerm.
  def exitObjectAtomExprTerm(self, ctx:JetRuleParser.ObjectAtomExprTermContext):
    # ctx.ident is type ObjectAtomContext
    ident = self.escape(ctx.ident.getText())
    ctx.expr = self.parseObjectAtom(ident, ctx.ident.kws)

if __name__ == "__main__":
  
  data =  a4.FileStream('test.jr', encoding='utf-8')
  
  # lexer
  lexer = JetRuleLexer(data)
  stream = a4.CommonTokenStream(lexer)
  
  # parser
  parser = JetRuleParser(stream)
  tree = parser.jetrule()

  # evaluator
  listener = JetListener()
  walker = a4.ParseTreeWalker()
  walker.walk(listener, tree)

  # Save the JetRule data structure
  with open('test_data/test.jr.json', 'wt', encoding='utf-8') as f:
    f.write(json.dumps(listener.jetRules, indent=4))

  print('Result saved to test.jr.json')
