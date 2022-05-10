"""JetRuleValidator tests"""

import os
import sys
import json
from typing import Dict
from absl import flags
from absl.testing import absltest
import io

from jetrule_compiler import JetRuleCompiler, InputProvider
from jetrule_context import JetRuleContext
from jetrule_rete_sqlite import JetRuleReteSQLite

FLAGS = flags.FLAGS

class JetRulesCompilerTest(absltest.TestCase):

  def _get_from_file(self, fname: str) -> JetRuleContext:
    in_provider = InputProvider('test_data')
    compiler = JetRuleCompiler()
    compiler.compileJetRuleFile(fname, in_provider)
    # print('Compiler working memory for import files')
    # print(compiler.imported_file_info_list)
    # print('***')
    return compiler.jetrule_ctx

  def _get_augmented_data(self, input_data: str) -> JetRuleContext:
    compiler = JetRuleCompiler()
    compiler.compileJetRule(input_data)
    jetrule_ctx = compiler.jetrule_ctx
    return jetrule_ctx


  def test_import1(self):
    jetrule_ctx = self._get_from_file("import_test1.jr")

    if jetrule_ctx.errors:
      for err in jetrule_ctx.errors:
        print('1***', err)
      print('1***')
    self.assertEqual(jetrule_ctx.ERROR, False)

    # validate the whole result
    expected = """{"resources": [{"type": "int", "id": "isTrue", "value": "1", "source_file_name": "import_test1.jr", "key": 0}, {"type": "text", "id": "NOT_IN_CONTRACT", "value": "NOT COVERED IN CONTRACT", "source_file_name": "import_test1.jr", "key": 1}, {"type": "text", "id": "EXCLUDED_STATE", "value": "STATE", "source_file_name": "import_test1.jr", "key": 2}, {"id": "acme:ProcedureLookup", "type": "resource", "value": "acme:ProcedureLookup", "source_file_name": "import_test11.jr", "key": 3}, {"id": "PROC_RID", "type": "resource", "value": "PROC_RID", "source_file_name": "import_test11.jr", "key": 4}, {"id": "PROC_MID", "type": "resource", "value": "PROC_MID", "source_file_name": "import_test11.jr", "key": 5}, {"id": "PROC_DESC", "type": "resource", "value": "PROC_DESC", "source_file_name": "import_test11.jr", "key": 6}], "lookup_tables": [{"type": "lookup", "name": "acme:ProcedureLookup", "key": ["PROC_CODE"], "columns": [{"name": "PROC_RID", "type": "text", "as_array": "false"}, {"name": "PROC_MID", "type": "text", "as_array": "false"}, {"name": "PROC_DESC", "type": "text", "as_array": "false"}], "table": "acme__cm_proc_codes", "source_file_name": "import_test11.jr", "resources": ["PROC_RID", "PROC_MID", "PROC_DESC"]}], "jet_rules": [], "imports": {"import_test1.jr": ["import_test11.jr"]}}"""
    # print('GOT:',json.dumps(jetrule_ctx.jetRules, indent=2))
    # print()
    # print('COMPACT:',json.dumps(jetrule_ctx.jetRules))
    self.assertEqual(json.dumps(jetrule_ctx.jetRules), expected)

  def test_import2(self):
    jetrule_ctx = self._get_from_file("import_test2.jr")

    # for err in jetrule_ctx.errors:
    #   print('2***', err)
    # print('2***')
    
    self.assertEqual(jetrule_ctx.ERROR, True)
    self.assertEqual(jetrule_ctx.errors[0], "Error in file 'import_test21.jr' line 8:19 no viable alternative at input 'acme:lookup_table'")
    self.assertEqual(jetrule_ctx.errors[1], "Error in file 'import_test2.jr' line 5:1 extraneous input 'bad' expecting {<EOF>, '[', '@JetCompilerDirective', 'class', 'jetstore_config', 'rule_sequence', 'triple', 'int', 'uint', 'long', 'ulong', 'double', 'text', 'date', 'datetime', 'bool', 'resource', 'volatile_resource', 'lookup_table', COMMENT}")
    self.assertEqual(len(jetrule_ctx.errors), 2)

  def test_import3(self):
    jetrule_ctx = self._get_from_file("import_test3.jr")

    # for err in jetrule_ctx.errors:
    #   print('3***', err)
    # print('3***')
    
    self.assertEqual(jetrule_ctx.ERROR, True)

    self.assertEqual(jetrule_ctx.errors[0], "Error in file 'import_test3.jr' line 8:5 mismatched input 'true' expecting Identifier")
    self.assertEqual(jetrule_ctx.errors[1], "Error in file 'import_test31.jr' line 7:10 mismatched input 'lookup_table' expecting Identifier")
    self.assertEqual(jetrule_ctx.errors[2], "Error in file 'import_test32.jr' line 5:8 mismatched input ':' expecting {',', ']'}")
    self.assertEqual(jetrule_ctx.errors[3], "Error in file 'import_test32.jr' line 9:1 extraneous input ';' expecting {<EOF>, '[', '@JetCompilerDirective', 'class', 'jetstore_config', 'rule_sequence', 'triple', 'int', 'uint', 'long', 'ulong', 'double', 'text', 'date', 'datetime', 'bool', 'resource', 'volatile_resource', 'lookup_table', COMMENT}")
    self.assertEqual(jetrule_ctx.errors[4], "Error in file 'import_test3.jr' line 16:1 extraneous input 'ztext' expecting {<EOF>, '[', '@JetCompilerDirective', 'class', 'jetstore_config', 'rule_sequence', 'triple', 'int', 'uint', 'long', 'ulong', 'double', 'text', 'date', 'datetime', 'bool', 'resource', 'volatile_resource', 'lookup_table', COMMENT}")

    self.assertEqual(len(jetrule_ctx.errors), 5)


  def test_import4(self):
    jetrule_ctx = self._get_from_file("import_test4.jr")

    # for err in jetrule_ctx.errors:
    #   print('4***', err)
    # print('4***')
    self.assertEqual(jetrule_ctx.ERROR, True)

    self.assertEqual(jetrule_ctx.errors[0], "Error in file 'import_test4.jr' line 8:5 mismatched input 'true' expecting Identifier")
    self.assertEqual(jetrule_ctx.errors[1], "Error in file 'import_test41.jr' line 8:19 no viable alternative at input 'acme:lookup_table'")
    self.assertEqual(jetrule_ctx.errors[2], "Error in file 'import_test42.jr' line 7:10 mismatched input 'lookup_table' expecting Identifier")
    self.assertEqual(jetrule_ctx.errors[3], "Error in file 'import_test4.jr' line 17:1 extraneous input 'ztext' expecting {<EOF>, '[', '@JetCompilerDirective', 'class', 'jetstore_config', 'rule_sequence', 'triple', 'int', 'uint', 'long', 'ulong', 'double', 'text', 'date', 'datetime', 'bool', 'resource', 'volatile_resource', 'lookup_table', COMMENT}")
 
    self.assertEqual(len(jetrule_ctx.errors), 4)


  def test_compiler1(self):
    data = """
      # =======================================================================================
      resource rdf:type = "rdf:type";
      resource acme:Claim = "acme:Claim";
      volatile_resource is_good = "is_good";
      [RuleC4]: 
        (?clm01 rdf:type acme:Claim).[?clm01]
        (?clm01 is_good ?good).[?clm01 or true]
        ->
        (?clm01 is_good true).
      ;
    """
    jetrule_ctx = self._get_augmented_data(data)

    # Validate variables
    self.assertEqual(len(jetrule_ctx.errors), 0)

    optimized_data = jetrule_ctx.jetRules
    optimized_expected = """{"resources": [{"type": "resource", "id": "rdf:type", "value": "rdf:type", "source_fname": "predefined", "key": 0}, {"type": "resource", "id": "acme:Claim", "value": "acme:Claim", "key": 1}, {"type": "volatile_resource", "id": "is_good", "value": "is_good", "key": 2}, {"type": "var", "id": "?x1", "is_binded": false, "var_pos": 0, "vertex": 1, "key": 3, "source_file_name": null}, {"type": "var", "id": "?x1", "is_binded": true, "vertex": 1, "is_antecedent": false, "var_pos": 0, "key": 4, "source_file_name": null}, {"type": "keyword", "value": "true", "inline": true, "key": 5, "source_file_name": null}, {"type": "var", "id": "?x1", "is_binded": true, "vertex": 1, "is_antecedent": false, "var_pos": 0, "key": 6, "source_file_name": null}, {"type": "var", "id": "?x1", "is_binded": true, "vertex": 2, "is_antecedent": true, "var_pos": 0, "key": 7, "source_file_name": null}, {"type": "var", "id": "?x2", "is_binded": false, "vertex": 2, "key": 8, "source_file_name": null}, {"type": "var", "id": "?x1", "is_binded": true, "vertex": 2, "is_antecedent": false, "var_pos": 0, "key": 9, "source_file_name": null}, {"type": "keyword", "value": "true", "inline": true, "key": 10, "source_file_name": null}], "lookup_tables": [], "jet_rules": [{"name": "RuleC4", "properties": {}, "optimization": true, "salience": 100, "antecedents": [{"type": "antecedent", "isNot": false, "normalizedLabel": "(?x1 rdf:type acme:Claim).[(?x1 or true) and ?x1]", "filter": {"type": "binary", "lhs": {"type": "binary", "lhs": 4, "op": "or", "rhs": 5}, "op": "and", "rhs": 6}, "vertex": 1, "parent_vertex": 0, "beta_relation_vars": ["?x1"], "pruned_var": [], "beta_var_nodes": [{"type": "var", "id": "?x1", "is_binded": false, "var_pos": 0, "vertex": 1, "key": 3, "source_file_name": null}], "children_vertexes": [2], "subject_key": 3, "predicate_key": 0, "object_key": 1}, {"type": "antecedent", "isNot": false, "normalizedLabel": "(?x1 is_good ?x2)", "vertex": 2, "parent_vertex": 1, "beta_relation_vars": ["?x1"], "pruned_var": ["?x2"], "beta_var_nodes": [{"type": "var", "id": "?x1", "is_binded": true, "var_pos": 0, "vertex": 2}], "children_vertexes": [], "rules": ["RuleC4"], "salience": [100], "subject_key": 7, "predicate_key": 2, "object_key": 8}], "consequents": [{"type": "consequent", "normalizedLabel": "(?x1 is_good true)", "vertex": 2, "consequent_seq": 0, "consequent_for_rule": "RuleC4", "consequent_salience": 100, "subject_key": 9, "predicate_key": 2, "object_key": 10}], "authoredLabel": "[RuleC4]:(?clm01 rdf:type acme:Claim).[?clm01].(?clm01 is_good ?good).[?clm01 or true] -> (?clm01 is_good true);", "source_file_name": null, "normalizedLabel": "[RuleC4]:(?x1 rdf:type acme:Claim).[(?x1 or true) and ?x1].(?x1 is_good ?x2) -> (?x1 is_good true);", "label": "[RuleC4]:(?clm01 rdf:type acme:Claim).[(?clm01 or true) and ?clm01].(?clm01 is_good ?good) -> (?clm01 is_good true);"}], "imports": {}}"""
    # print()
    # print('OPTIMIZED GOT:',json.dumps(optimized_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(optimized_data))
    self.assertEqual(json.dumps(optimized_data), optimized_expected)

  def test_compiler2(self):
    data = """
      # =======================================================================================
      resource rdf:type = "rdf:type";
      resource acme:Claim = "acme:Claim";
      volatile_resource is_good = "is_good";
      [RuleC5]: 
        (?clm01 reverse_of ?clm02).
        (?clm03 rdf:type acme:Claim).
        (?clm01 rdf:type acme:Claim).
        (?clm02 rdf:type acme:Claim)
        ->
        (?clm01 is_good true).
      ;
    """
    jetrule_ctx = self._get_augmented_data(data)

    # if jetrule_ctx.ERROR:
    #   print("GOT ERROR!")
    # for err in jetrule_ctx.errors:
    #   print('***', err)
    # print('***')

    self.assertEqual(jetrule_ctx.ERROR, True)
    self.assertEqual(jetrule_ctx.errors[0], "Error rule RuleC5: Identifier 'reverse_of' is not defined in this context '(?clm01 reverse_of ?clm02)', it must be defined.")

  def test_specialcase1(self):
    data = """
      # =======================================================================================
      resource rdf:type = "rdf:type";
      resource acme:Claim = "acme:Claim";
      resource acme:EClaim = "acme:EClaim";
      volatile_resource is_good = "is_good";
      volatile_resource related_to = "related_to";
      [RuleSC1]: 
        (?clm01 rdf:type acme:Claim).
        (?clm01 is_good ?good).[?good].
        (?clm01 related_to ?clm02)
        ->
        (?clm01 rdf:type acme:EClaim).
        (?clm02 rdf:type acme:EClaim)
      ;
    """
    jetrule_ctx = self._get_augmented_data(data)

    if jetrule_ctx.ERROR:
      print("GOT ERROR!")
    for err in jetrule_ctx.errors:
      print('***', err)
    # print('***')

    expected = """{"main_rule_file_name": null, "support_rule_file_names": null, "resources": [{"type": "resource", "id": "rdf:type", "value": "rdf:type", "source_fname": "predefined", "key": 0}, {"type": "resource", "id": "acme:Claim", "value": "acme:Claim", "key": 1}, {"type": "resource", "id": "acme:EClaim", "value": "acme:EClaim", "key": 2}, {"type": "volatile_resource", "id": "is_good", "value": "is_good", "key": 3}, {"type": "volatile_resource", "id": "related_to", "value": "related_to", "key": 4}, {"type": "var", "id": "?x1", "is_binded": false, "var_pos": 0, "vertex": 1, "key": 5, "source_file_name": null}, {"type": "var", "id": "?x1", "is_binded": true, "vertex": 2, "is_antecedent": true, "var_pos": 0, "key": 6, "source_file_name": null}, {"type": "var", "id": "?x2", "is_binded": false, "var_pos": 2, "vertex": 2, "key": 7, "source_file_name": null}, {"type": "var", "id": "?x2", "is_binded": true, "vertex": 2, "is_antecedent": false, "var_pos": 1, "key": 8, "source_file_name": null}, {"type": "var", "id": "?x1", "is_binded": true, "vertex": 3, "is_antecedent": true, "var_pos": 0, "key": 9, "source_file_name": null}, {"type": "var", "id": "?x3", "is_binded": false, "var_pos": 2, "vertex": 3, "key": 10, "source_file_name": null}, {"type": "var", "id": "?x1", "is_binded": true, "vertex": 3, "is_antecedent": false, "var_pos": 0, "key": 11, "source_file_name": null}, {"type": "var", "id": "?x3", "is_binded": true, "vertex": 3, "is_antecedent": false, "var_pos": 1, "key": 12, "source_file_name": null}], "lookup_tables": [], "rete_nodes": [{"vertex": 0, "parent_vertex": 0, "children_vertexes": [1], "type": "head_node"}, {"type": "antecedent", "isNot": false, "normalizedLabel": "(?x1 rdf:type acme:Claim)", "vertex": 1, "parent_vertex": 0, "beta_relation_vars": ["?x1"], "pruned_var": [], "beta_var_nodes": [{"type": "var", "id": "?x1", "is_binded": false, "var_pos": 0, "vertex": 1, "key": 5, "source_file_name": null}], "children_vertexes": [2], "subject_key": 5, "predicate_key": 0, "object_key": 1}, {"type": "antecedent", "isNot": false, "normalizedLabel": "(?x1 is_good ?x2).[?x2]", "filter": 8, "vertex": 2, "parent_vertex": 1, "beta_relation_vars": ["?x1", "?x2"], "pruned_var": [], "beta_var_nodes": [{"type": "var", "id": "?x1", "is_binded": true, "var_pos": 0, "vertex": 2}, {"type": "var", "id": "?x2", "is_binded": false, "var_pos": 2, "vertex": 2, "key": 7, "source_file_name": null}], "children_vertexes": [3], "subject_key": 6, "predicate_key": 3, "object_key": 7}, {"type": "antecedent", "isNot": false, "normalizedLabel": "(?x1 related_to ?x3)", "vertex": 3, "parent_vertex": 2, "beta_relation_vars": ["?x1", "?x3"], "pruned_var": ["?x2"], "beta_var_nodes": [{"type": "var", "id": "?x1", "is_binded": true, "var_pos": 0, "vertex": 3}, {"type": "var", "id": "?x3", "is_binded": false, "var_pos": 2, "vertex": 3, "key": 10, "source_file_name": null}], "children_vertexes": [], "rules": ["RuleSC1"], "salience": [100], "subject_key": 9, "predicate_key": 4, "object_key": 10}, {"type": "consequent", "normalizedLabel": "(?x1 rdf:type acme:EClaim)", "vertex": 3, "consequent_seq": 0, "consequent_for_rule": "RuleSC1", "consequent_salience": 100, "subject_key": 11, "predicate_key": 0, "object_key": 2}, {"type": "consequent", "normalizedLabel": "(?x3 rdf:type acme:EClaim)", "vertex": 3, "consequent_seq": 1, "consequent_for_rule": "RuleSC1", "consequent_salience": 100, "subject_key": 12, "predicate_key": 0, "object_key": 2}]}"""
    # print('GOT:',json.dumps(jetrule_ctx.jetReteNodes, indent=4))
    # print()
    # print('COMPACT:',json.dumps(jetrule_ctx.jetReteNodes))
    self.assertEqual(jetrule_ctx.ERROR, False)
    self.assertEqual(json.dumps(jetrule_ctx.jetReteNodes), expected)

  def test_resource_from_rule1(self):
    data = """
      # =======================================================================================
      # Defining resources from rules
      @JetCompilerDirective extract_resources_from_rules = "true";
      [RuleRFR1]: 
        (?clm01 rdf:type acme:Claim).
        (?clm01 _0:good ?good).[?good + _0:good].
        (?clm01 related_to ?clm02)
        ->
        (?clm01 rdf:type acme:EClaim)
      ;
    """
    jetrule_ctx = self._get_augmented_data(data)

    if jetrule_ctx.ERROR:
      print("GOT ERROR!")
    for err in jetrule_ctx.errors:
      print('***', err)

    expected = """{"resources": [{"id": "rdf:type", "type": "resource", "value": "rdf:type", "source_file_name": "predefined", "key": 0}, {"id": "acme:Claim", "type": "resource", "value": "acme:Claim", "key": 1}, {"id": "good", "type": "volatile_resource", "value": "good", "key": 2}, {"id": "related_to", "type": "resource", "value": "related_to", "key": 3}, {"id": "acme:EClaim", "type": "resource", "value": "acme:EClaim", "key": 4}, {"type": "var", "id": "?x1", "is_binded": false, "var_pos": 0, "vertex": 1, "key": 5, "source_file_name": null}, {"type": "var", "id": "?x1", "is_binded": true, "vertex": 2, "is_antecedent": true, "var_pos": 0, "key": 6, "source_file_name": null}, {"type": "var", "id": "?x2", "is_binded": false, "var_pos": 2, "vertex": 2, "key": 7, "source_file_name": null}, {"type": "var", "id": "?x2", "is_binded": true, "vertex": 2, "is_antecedent": false, "var_pos": 1, "key": 8, "source_file_name": null}, {"type": "var", "id": "?x1", "is_binded": true, "vertex": 3, "is_antecedent": true, "var_pos": 0, "key": 9, "source_file_name": null}, {"type": "var", "id": "?x3", "is_binded": false, "vertex": 3, "key": 10, "source_file_name": null}, {"type": "var", "id": "?x1", "is_binded": true, "vertex": 3, "is_antecedent": false, "var_pos": 0, "key": 11, "source_file_name": null}], "lookup_tables": [], "jet_rules": [{"name": "RuleRFR1", "properties": {}, "optimization": true, "salience": 100, "antecedents": [{"type": "antecedent", "isNot": false, "normalizedLabel": "(?x1 rdf:type acme:Claim)", "vertex": 1, "parent_vertex": 0, "beta_relation_vars": ["?x1"], "pruned_var": [], "beta_var_nodes": [{"type": "var", "id": "?x1", "is_binded": false, "var_pos": 0, "vertex": 1, "key": 5, "source_file_name": null}], "children_vertexes": [2], "subject_key": 5, "predicate_key": 0, "object_key": 1}, {"type": "antecedent", "isNot": false, "normalizedLabel": "(?x1 good ?x2).[?x2 + good]", "filter": {"type": "binary", "lhs": 8, "op": "+", "rhs": 2}, "vertex": 2, "parent_vertex": 1, "beta_relation_vars": ["?x1", "?x2"], "pruned_var": [], "beta_var_nodes": [{"type": "var", "id": "?x1", "is_binded": true, "var_pos": 0, "vertex": 2}, {"type": "var", "id": "?x2", "is_binded": false, "var_pos": 2, "vertex": 2, "key": 7, "source_file_name": null}], "children_vertexes": [3], "subject_key": 6, "predicate_key": 2, "object_key": 7}, {"type": "antecedent", "isNot": false, "normalizedLabel": "(?x1 related_to ?x3)", "vertex": 3, "parent_vertex": 2, "beta_relation_vars": ["?x1"], "pruned_var": ["?x2", "?x3"], "beta_var_nodes": [{"type": "var", "id": "?x1", "is_binded": true, "var_pos": 0, "vertex": 3}], "children_vertexes": [], "rules": ["RuleRFR1"], "salience": [100], "subject_key": 9, "predicate_key": 3, "object_key": 10}], "consequents": [{"type": "consequent", "normalizedLabel": "(?x1 rdf:type acme:EClaim)", "vertex": 3, "consequent_seq": 0, "consequent_for_rule": "RuleRFR1", "consequent_salience": 100, "subject_key": 11, "predicate_key": 0, "object_key": 4}], "authoredLabel": "[RuleRFR1]:(?clm01 rdf:type acme:Claim).(?clm01 _0:good ?good).[?good + _0:good].(?clm01 related_to ?clm02) -> (?clm01 rdf:type acme:EClaim);", "source_file_name": null, "normalizedLabel": "[RuleRFR1]:(?x1 rdf:type acme:Claim).(?x1 good ?x2).[?x2 + good].(?x1 related_to ?x3) -> (?x1 rdf:type acme:EClaim);", "label": "[RuleRFR1]:(?clm01 rdf:type acme:Claim).(?clm01 good ?good).[?good + good].(?clm01 related_to ?clm02) -> (?clm01 rdf:type acme:EClaim);"}], "imports": {}}"""
    # print('GOT:',json.dumps(jetrule_ctx.jetRules, indent=4))
    # print()
    # print('COMPACT:',json.dumps(jetrule_ctx.jetRules))
    self.assertEqual(jetrule_ctx.ERROR, False)
    self.assertEqual(json.dumps(jetrule_ctx.jetRules), expected)

  def test_rule_with_null1(self):
    data = """
      # =======================================================================================
      # Testing the null keyword in a rule filter
      resource rdf:type = "rdf:type";
      resource acme:Claim = "acme:Claim";
      resource acme:CodeClaim = "acme:CodeClaim";
      resource code = "code";

      [RuleNull1]: 
        (?clm01 rdf:type acme:Claim).
        (?clm01 code ?code).
        [not(?code == null)]
        ->
        (?clm01 rdf:type acme:CodeClaim).
      ;
    """
    jetrule_ctx = self._get_augmented_data(data)

    if jetrule_ctx.ERROR:
      print("GOT ERROR!")
    for err in jetrule_ctx.errors:
      print('***', err)
    
    expected = """{"resources": [{"type": "resource", "id": "rdf:type", "value": "rdf:type", "source_fname": "predefined", "key": 0}, {"type": "resource", "id": "acme:Claim", "value": "acme:Claim", "key": 1}, {"type": "resource", "id": "acme:CodeClaim", "value": "acme:CodeClaim", "key": 2}, {"type": "resource", "id": "code", "value": "code", "key": 3}, {"type": "var", "id": "?x1", "is_binded": false, "var_pos": 0, "vertex": 1, "key": 4, "source_file_name": null}, {"type": "var", "id": "?x1", "is_binded": true, "vertex": 2, "is_antecedent": true, "var_pos": 0, "key": 5, "source_file_name": null}, {"type": "var", "id": "?x2", "is_binded": false, "var_pos": 2, "vertex": 2, "key": 6, "source_file_name": null}, {"type": "var", "id": "?x2", "is_binded": true, "vertex": 2, "is_antecedent": false, "var_pos": 1, "key": 7, "source_file_name": null}, {"type": "keyword", "value": "null", "inline": true, "key": 8, "source_file_name": null}, {"type": "var", "id": "?x1", "is_binded": true, "vertex": 2, "is_antecedent": false, "var_pos": 0, "key": 9, "source_file_name": null}], "lookup_tables": [], "jet_rules": [{"name": "RuleNull1", "properties": {}, "optimization": true, "salience": 100, "antecedents": [{"type": "antecedent", "isNot": false, "normalizedLabel": "(?x1 rdf:type acme:Claim)", "vertex": 1, "parent_vertex": 0, "beta_relation_vars": ["?x1"], "pruned_var": [], "beta_var_nodes": [{"type": "var", "id": "?x1", "is_binded": false, "var_pos": 0, "vertex": 1, "key": 4, "source_file_name": null}], "children_vertexes": [2], "subject_key": 4, "predicate_key": 0, "object_key": 1}, {"type": "antecedent", "isNot": false, "normalizedLabel": "(?x1 code ?x2).[not (?x2 == null)]", "filter": {"type": "unary", "op": "not", "arg": {"type": "binary", "lhs": 7, "op": "==", "rhs": 8}}, "vertex": 2, "parent_vertex": 1, "beta_relation_vars": ["?x1", "?x2"], "pruned_var": [], "beta_var_nodes": [{"type": "var", "id": "?x1", "is_binded": true, "var_pos": 0, "vertex": 2}, {"type": "var", "id": "?x2", "is_binded": false, "var_pos": 2, "vertex": 2, "key": 6, "source_file_name": null}], "children_vertexes": [], "rules": ["RuleNull1"], "salience": [100], "subject_key": 5, "predicate_key": 3, "object_key": 6}], "consequents": [{"type": "consequent", "normalizedLabel": "(?x1 rdf:type acme:CodeClaim)", "vertex": 2, "consequent_seq": 0, "consequent_for_rule": "RuleNull1", "consequent_salience": 100, "subject_key": 9, "predicate_key": 0, "object_key": 2}], "authoredLabel": "[RuleNull1]:(?clm01 rdf:type acme:Claim).(?clm01 code ?code).[not (?code == null)] -> (?clm01 rdf:type acme:CodeClaim);", "source_file_name": null, "normalizedLabel": "[RuleNull1]:(?x1 rdf:type acme:Claim).(?x1 code ?x2).[not (?x2 == null)] -> (?x1 rdf:type acme:CodeClaim);", "label": "[RuleNull1]:(?clm01 rdf:type acme:Claim).(?clm01 code ?code).[not (?code == null)] -> (?clm01 rdf:type acme:CodeClaim);"}], "imports": {}}"""
    # print('GOT:',json.dumps(jetrule_ctx.jetRules, indent=2))
    # print()
    # print('COMPACT:',json.dumps(jetrule_ctx.jetRules))
    self.assertEqual(jetrule_ctx.ERROR, False)
    self.assertEqual(json.dumps(jetrule_ctx.jetRules), expected)

  def test_class_def1(self):
    fname = "classes_test.jr"
    in_provider = InputProvider('test_data')

    compiler = JetRuleCompiler()
    # jetrule_ctx = compiler.processJetRuleFile(fname, in_provider)
    jetrule_ctx = compiler.compileJetRuleFile(fname, in_provider)

    for err in jetrule_ctx.errors:
      print('ERROR ::',err)
    self.assertFalse(jetrule_ctx.ERROR, "Unexpected JetRuleCompiler Errors")

    data = jetrule_ctx.jetRules
    expected = 'xx'
    with open("test_data/classes_test.jr.json", 'rt', encoding='utf-8') as f:
      expected = json.loads(f.read())

    # print()
    # print('GOT:',json.dumps(data, indent=2))
    self.assertEqual(json.dumps(data), json.dumps(expected))


  def test_triples1(self):
    data = """
      # =======================================================================================
      # Testing triple statement
      @JetCompilerDirective extract_resources_from_rules = "true";

      triple(iState, rdf:type, jets:State);
      triple(iDistConfigTC,top:entity_property,_0:distObjTC);
      triple(iDistConfigTC,top:operator,"<");
      triple(iDistConfigTC,top:value_property,_0:yearDistance);
      triple(iExZ0,STATE,"NY");
      triple(iExZ0,ZIPCODE,"06390");
    """
    jetrule_ctx = self._get_augmented_data(data)

    if jetrule_ctx.ERROR:
      print("GOT ERROR!")
    for err in jetrule_ctx.errors:
      print('***', err)

    expected = """{"resources": [{"id": "iState", "type": "resource", "value": "iState", "key": 0}, {"id": "rdf:type", "type": "resource", "value": "rdf:type", "source_file_name": "predefined", "key": 1}, {"id": "jets:State", "type": "resource", "value": "jets:State", "key": 2}, {"id": "iDistConfigTC", "type": "resource", "value": "iDistConfigTC", "key": 3}, {"id": "top:entity_property", "type": "resource", "value": "top:entity_property", "key": 4}, {"id": "distObjTC", "type": "volatile_resource", "value": "distObjTC", "key": 5}, {"id": "top:operator", "type": "resource", "value": "top:operator", "key": 6}, {"id": "top:value_property", "type": "resource", "value": "top:value_property", "key": 7}, {"id": "yearDistance", "type": "volatile_resource", "value": "yearDistance", "key": 8}, {"id": "iExZ0", "type": "resource", "value": "iExZ0", "key": 9}, {"id": "STATE", "type": "resource", "value": "STATE", "key": 10}, {"id": "ZIPCODE", "type": "resource", "value": "ZIPCODE", "key": 11}, {"type": "text", "value": "<", "inline": true, "key": 12, "source_file_name": null}, {"type": "text", "value": "NY", "inline": true, "key": 13, "source_file_name": null}, {"type": "text", "value": "06390", "inline": true, "key": 14, "source_file_name": null}], "lookup_tables": [], "jet_rules": [], "imports": {}, "triples": [{"type": "triple", "subject_key": 0, "predicate_key": 1, "object_key": 2}, {"type": "triple", "subject_key": 3, "predicate_key": 4, "object_key": 5}, {"type": "triple", "subject_key": 3, "predicate_key": 6, "object_key": 12}, {"type": "triple", "subject_key": 3, "predicate_key": 7, "object_key": 8}, {"type": "triple", "subject_key": 9, "predicate_key": 10, "object_key": 13}, {"type": "triple", "subject_key": 9, "predicate_key": 11, "object_key": 14}]}"""
    # print('GOT:',json.dumps(jetrule_ctx.jetRules, indent=4))
    # print()
    # print('COMPACT:',json.dumps(jetrule_ctx.jetRules))
    self.assertEqual(jetrule_ctx.ERROR, False)
    self.assertEqual(json.dumps(jetrule_ctx.jetRules), expected)


  def test_ruleseq1(self):
    data = """
      # =======================================================================================
      # Testing Rule Sequence
      rule_sequence primaryPipeline {
        $main_rule_sets = [# first pipeline
          "main_rulesets/process1.jr",
          "main_rulesets/process2.jr",
          "main_rulesets/process3.jr"
        ]
      };
      rule_sequence otherPipeline {
        $main_rule_sets = [   # second pipeline
          "main_rulesets/process1.jr",
          "main_rulesets/process2.jr",
          "main_rulesets/process3.jr"
        ]
      };
    """
    jetrule_ctx = self._get_augmented_data(data)

    if jetrule_ctx.ERROR:
      print("GOT ERROR!")
    for err in jetrule_ctx.errors:
      print('***', err)

    expected = """{"resources": [], "lookup_tables": [], "jet_rules": [], "imports": {}, "rule_sequences": [{"type": "ruleseq", "name": "primaryPipeline", "main_rule_sets": ["main_rulesets/process1.jr", "main_rulesets/process2.jr", "main_rulesets/process3.jr"]}, {"type": "ruleseq", "name": "otherPipeline", "main_rule_sets": ["main_rulesets/process1.jr", "main_rulesets/process2.jr", "main_rulesets/process3.jr"]}]}"""
    # print('GOT:',json.dumps(jetrule_ctx.jetRules, indent=4))
    # print()
    # print('COMPACT:',json.dumps(jetrule_ctx.jetRules))
    self.assertEqual(jetrule_ctx.ERROR, False)
    self.assertEqual(json.dumps(jetrule_ctx.jetRules), expected)

  def test_ruleseq01(self):
    fname = "rule_sequence_test1.jr"
    in_provider = InputProvider('test_data')

    compiler = JetRuleCompiler()
    jetrule_ctx = compiler.compileJetRuleFile(fname, in_provider)

    for err in jetrule_ctx.errors:
      print('ERROR ::',err)      
    self.assertFalse(jetrule_ctx.ERROR, "Unexpected JetRuleCompiler Errors")

    data = jetrule_ctx.jetRules
    # print()
    # print('GOT01:',json.dumps(data, indent=2))

    expected = 'xx'
    with open("test_data/expected_rule_sequence_test1.jr.json", 'rt', encoding='utf-8') as f:
      expected = json.loads(f.read())

    self.assertEqual(json.dumps(data), json.dumps(expected))

    # save the workspace.db
    rete_db_helper = JetRuleReteSQLite(compiler.jetrule_ctx)
    if os.path.exists("test_data/rule_sequence_test1.db"):
      os.remove("test_data/rule_sequence_test1.db")

    err = rete_db_helper.saveReteConfig("test_data/rule_sequence_test1.db")
    self.assertEqual(err, "Error main rule file 'test_ruleseq/main_rules1.jr' in rule sequence not found, make sure it exist")

  def test_ruleseq02(self):
    fname = "rule_sequence_test2.jr"
    in_provider = InputProvider('test_data')

    compiler = JetRuleCompiler()
    jetrule_ctx = compiler.compileJetRuleFile(fname, in_provider)

    for err in jetrule_ctx.errors:
      print('ERROR ::',err)      
    self.assertFalse(jetrule_ctx.ERROR, "Unexpected JetRuleCompiler Errors")

    data = jetrule_ctx.jetRules
    # print()
    # print('GOT02:',json.dumps(data, indent=2))

    expected = 'xx'
    with open("test_data/expected_rule_sequence_test2.jr.json", 'rt', encoding='utf-8') as f:
      expected = json.loads(f.read())

    self.assertEqual(json.dumps(data), json.dumps(expected))

    # save the workspace.db
    rete_db_helper = JetRuleReteSQLite(compiler.jetrule_ctx)
    if os.path.exists("test_data/rule_sequence_test2.db"):
      os.remove("test_data/rule_sequence_test2.db")

    err = rete_db_helper.saveReteConfig("test_data/rule_sequence_test2.db")
    if err:
      print('ERROR while saving JetRule file to rete_db: {0}.'.format(str(err)))
    self.assertEqual(err, None)


if __name__ == '__main__':
  absltest.main()
