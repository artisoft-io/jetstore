"""JetRuleValidator tests"""

import sys
import json
from typing import Dict
from absl import flags
from absl.testing import absltest
import io

from jetrule_compiler import JetRuleCompiler, InputProvider
from jetrule_context import JetRuleContext

FLAGS = flags.FLAGS

class JetRulesCompilerTest(absltest.TestCase):

  def _get_from_file(self, fname: str) -> JetRuleContext:
    in_provider = InputProvider('jetstore-tools/jetrule-grammar')
    compiler = JetRuleCompiler()
    compiler.compileJetRuleFile(fname, in_provider)
    # print('Compiler working memory for import files')
    # print(compiler.imported_rule_files)
    # print('***')
    return compiler.jetrule_ctx

  def _get_augmented_data(self, input_data: str) -> JetRuleContext:
    compiler = JetRuleCompiler()
    compiler.compileJetRule(input_data)
    jetrule_ctx = compiler.jetrule_ctx
    return jetrule_ctx


  def test_import1(self):
    jetrule_ctx = self._get_from_file("import_test1.jr")

    self.assertEqual(jetrule_ctx.ERROR, False)

    # validate the whole result
    expected = """{"literals": [{"type": "int", "id": "isTrue", "value": "1"}, {"type": "text", "id": "NOT_IN_CONTRACT", "value": "NOT COVERED IN CONTRACT"}, {"type": "text", "id": "EXCLUDED_STATE", "value": "STATE"}], "resources": [{"id": "acme:ProcedureLookup", "type": "resource", "value": "acme:ProcedureLookup"}, {"id": "cPROC_RID", "type": "resource", "value": "PROC_RID"}, {"id": "cPROC_MID", "type": "resource", "value": "PROC_MID"}, {"id": "cPROC_DESC", "type": "resource", "value": "PROC_DESC"}], "lookup_tables": [{"name": "acme:ProcedureLookup", "table": "acme__cm_proc_codes", "key": ["PROC_CODE"], "columns": ["PROC_RID", "PROC_MID", "PROC_DESC"], "resources": ["cPROC_RID", "cPROC_MID", "cPROC_DESC"]}], "jet_rules": [], "rete_nodes": [{"vertex": 0, "parent_vertex": 0, "label": "Head node", "antecedent_node": null, "consequent_nodes": [], "children_vertexes": []}]}"""
    # print('GOT:',json.dumps(jetrule_ctx.jetRules, indent=2))
    # print()
    # print('COMPACT:',json.dumps(jetrule_ctx.jetRules))

    # validate the whole result
    self.assertEqual(json.dumps(jetrule_ctx.jetRules), expected)

  def test_import2(self):
    jetrule_ctx = self._get_from_file("import_test2.jr")

    # for err in jetrule_ctx.errors:
    #   print('***', err)
    # print('***')
    self.assertEqual(jetrule_ctx.ERROR, True)
    self.assertEqual(jetrule_ctx.errors[0], "Error in file 'import_test21.jr' line 8:19 no viable alternative at input 'acme:lookup_table'")
    self.assertEqual(jetrule_ctx.errors[1], "Error in file 'import_test2.jr' line 5:1 extraneous input 'bad' expecting {<EOF>, '[', 'int', 'uint', 'long', 'ulong', 'double', 'text', 'resource', 'volatile_resource', 'lookup_table', COMMENT}")
    self.assertEqual(len(jetrule_ctx.errors), 2)

  def test_import3(self):
    jetrule_ctx = self._get_from_file("import_test3.jr")

    # for err in jetrule_ctx.errors:
    #   print('***', err)
    # print('***')
    self.assertEqual(jetrule_ctx.ERROR, True)

    self.assertEqual(jetrule_ctx.errors[0], "Error in file 'import_test3.jr' line 8:5 mismatched input 'true' expecting Identifier")
    self.assertEqual(jetrule_ctx.errors[1], "Error in file 'import_test31.jr' line 7:10 mismatched input 'lookup_table' expecting Identifier")
    self.assertEqual(jetrule_ctx.errors[2], "Error in file 'import_test32.jr' line 5:8 mismatched input ':' expecting {',', ']'}")
    self.assertEqual(jetrule_ctx.errors[3], "Error in file 'import_test32.jr' line 9:1 extraneous input ';' expecting {<EOF>, '[', 'int', 'uint', 'long', 'ulong', 'double', 'text', 'resource', 'volatile_resource', 'lookup_table', COMMENT}")
    self.assertEqual(jetrule_ctx.errors[4], "Error in file 'import_test3.jr' line 16:1 extraneous input 'ztext' expecting {<EOF>, '[', 'int', 'uint', 'long', 'ulong', 'double', 'text', 'resource', 'volatile_resource', 'lookup_table', COMMENT}")

    self.assertEqual(len(jetrule_ctx.errors), 5)


  def test_import4(self):
    jetrule_ctx = self._get_from_file("import_test4.jr")

    # for err in jetrule_ctx.errors:
    #   print('***', err)
    # print('***')
    self.assertEqual(jetrule_ctx.ERROR, True)

    self.assertEqual(jetrule_ctx.errors[0], "Error in file 'import_test4.jr' line 8:5 mismatched input 'true' expecting Identifier")
    self.assertEqual(jetrule_ctx.errors[1], "Error in file 'import_test41.jr' line 8:19 no viable alternative at input 'acme:lookup_table'")
    self.assertEqual(jetrule_ctx.errors[2], "Error in file 'import_test42.jr' line 7:10 mismatched input 'lookup_table' expecting Identifier")
    self.assertEqual(jetrule_ctx.errors[3], "Error in file 'import_test4.jr' line 17:1 extraneous input 'ztext' expecting {<EOF>, '[', 'int', 'uint', 'long', 'ulong', 'double', 'text', 'resource', 'volatile_resource', 'lookup_table', COMMENT}")

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
    optimized_expected = """{"literals": [], "resources": [{"type": "resource", "id": "rdf:type", "value": "rdf:type"}, {"type": "resource", "id": "acme:Claim", "value": "acme:Claim"}, {"type": "volatile_resource", "id": "is_good", "value": "is_good"}], "lookup_tables": [], "jet_rules": [{"name": "RuleC4", "properties": {}, "authoredLabel": "[RuleC4]:(?clm01 rdf:type acme:Claim).[?clm01].(?clm01 is_good ?good).[?clm01 or true] -> (?clm01 is_good true);", "normalizedLabel": "[RuleC4]:(?x1 rdf:type acme:Claim).[(?x1 or true) and ?x1].(?x1 is_good ?x2) -> (?x1 is_good true);", "label": "[RuleC4]:(?clm01 rdf:type acme:Claim).[(?clm01 or true) and ?clm01].(?clm01 is_good ?good) -> (?clm01 is_good true);", "alpha_nodes": [{"vertex": 1, "parent_vertex": 0, "label": "(?x1 rdf:type acme:Claim).[(?x1 or true) and ?x1]", "antecedent_node": {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?clm01", "is_binded": false}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim).[(?x1 or true) and ?x1]", "label": "(?clm01 rdf:type acme:Claim).[(?clm01 or true) and ?clm01]", "filter": {"type": "binary", "lhs": {"type": "binary", "lhs": {"type": "var", "id": "?x1", "label": "?clm01", "is_binded": false}, "op": "or", "rhs": {"type": "keyword", "value": "true"}}, "op": "and", "rhs": {"type": "var", "id": "?x1", "label": "?clm01", "is_binded": false}}, "vertex": 1, "parent_vertex": 0, "beta_relation_vars": ["?x1"], "pruned_var": []}, "consequent_nodes": [], "children_vertexes": [2]}, {"vertex": 2, "parent_vertex": 1, "label": "(?x1 is_good ?x2)", "antecedent_node": {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?clm01", "is_binded": true}, {"type": "identifier", "value": "is_good"}, {"type": "var", "id": "?x2", "label": "?good", "is_binded": false}], "normalizedLabel": "(?x1 is_good ?x2)", "label": "(?clm01 is_good ?good)", "vertex": 2, "parent_vertex": 1, "beta_relation_vars": ["?x1"], "pruned_var": ["?x2"]}, "consequent_nodes": [{"type": "consequent", "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x1 is_good true)", "label": "(?clm01 is_good true)", "vertex": 2}], "children_vertexes": []}]}], "rete_nodes": [{"vertex": 0, "parent_vertex": 0, "label": "Head node", "antecedent_node": null, "consequent_nodes": [], "children_vertexes": [1]}, {"vertex": 1, "parent_vertex": 0, "label": "(?x1 rdf:type acme:Claim).[(?x1 or true) and ?x1]", "antecedent_node": {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?clm01", "is_binded": false}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim).[(?x1 or true) and ?x1]", "label": "(?clm01 rdf:type acme:Claim).[(?clm01 or true) and ?clm01]", "filter": {"type": "binary", "lhs": {"type": "binary", "lhs": {"type": "var", "id": "?x1", "label": "?clm01", "is_binded": false}, "op": "or", "rhs": {"type": "keyword", "value": "true"}}, "op": "and", "rhs": {"type": "var", "id": "?x1", "label": "?clm01", "is_binded": false}}, "vertex": 1, "parent_vertex": 0, "beta_relation_vars": ["?x1"], "pruned_var": []}, "consequent_nodes": [], "children_vertexes": [2]}, {"vertex": 2, "parent_vertex": 1, "label": "(?x1 is_good ?x2)", "antecedent_node": {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?clm01", "is_binded": true}, {"type": "identifier", "value": "is_good"}, {"type": "var", "id": "?x2", "label": "?good", "is_binded": false}], "normalizedLabel": "(?x1 is_good ?x2)", "label": "(?clm01 is_good ?good)", "vertex": 2, "parent_vertex": 1, "beta_relation_vars": ["?x1"], "pruned_var": ["?x2"]}, "consequent_nodes": [{"type": "consequent", "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x1 is_good true)", "label": "(?clm01 is_good true)", "vertex": 2}], "children_vertexes": []}]}"""
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

  # Result:
  # "authoredLabel": "[RuleC5]:(?clm01 reverse_of ?clm02).(?clm03 rdf:type acme:Claim).(?clm01 rdf:type acme:Claim).(?clm02 rdf:type acme:Claim) -> (?clm01 is_good true);",
  # "normalizedLabel": "[RuleC5]:(?x1 rdf:type acme:Claim).(?x2 rdf:type acme:Claim).(?x2 reverse_of ?x3).(?x3 rdf:type acme:Claim) -> (?x2 is_good true);",
  # "label": "[RuleC5]:(?clm03 rdf:type acme:Claim).(?clm01 rdf:type acme:Claim).(?clm01 reverse_of ?clm02).(?clm02 rdf:type acme:Claim) -> (?clm01 is_good true);"    

    jetrule_ctx = self._get_augmented_data(data)

    # if jetrule_ctx.ERROR:
    #   print("GOT ERROR!")
    # for err in jetrule_ctx.errors:
    #   print('***', err)
    # print('***')

    self.assertEqual(jetrule_ctx.ERROR, True)
    self.assertEqual(jetrule_ctx.errors[0], "Error rule RuleC5: Identifier 'reverse_of' is not defined in this context '(?clm01 reverse_of ?clm02)', it must be defined.")

    optimized_data = jetrule_ctx.jetRules
    optimized_expected = """{"literals": [], "resources": [{"type": "resource", "id": "rdf:type", "value": "rdf:type"}, {"type": "resource", "id": "acme:Claim", "value": "acme:Claim"}, {"type": "volatile_resource", "id": "is_good", "value": "is_good"}], "lookup_tables": [], "jet_rules": [{"name": "RuleC5", "properties": {}, "authoredLabel": "[RuleC5]:(?clm01 reverse_of ?clm02).(?clm03 rdf:type acme:Claim).(?clm01 rdf:type acme:Claim).(?clm02 rdf:type acme:Claim) -> (?clm01 is_good true);", "normalizedLabel": "[RuleC5]:(?x1 rdf:type acme:Claim).(?x2 rdf:type acme:Claim).(?x2 reverse_of ?x3).(?x3 rdf:type acme:Claim) -> (?x2 is_good true);", "label": "[RuleC5]:(?clm03 rdf:type acme:Claim).(?clm01 rdf:type acme:Claim).(?clm01 reverse_of ?clm02).(?clm02 rdf:type acme:Claim) -> (?clm01 is_good true);", "alpha_nodes": [{"vertex": 1, "parent_vertex": 0, "label": "(?x1 rdf:type acme:Claim)", "antecedent_node": {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?clm03", "is_binded": false}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim)", "label": "(?clm03 rdf:type acme:Claim)", "vertex": 1, "parent_vertex": 0, "beta_relation_vars": [], "pruned_var": ["?x1"]}, "consequent_nodes": [], "children_vertexes": [2]}, {"vertex": 2, "parent_vertex": 1, "label": "(?x2 rdf:type acme:Claim)", "antecedent_node": {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x2", "label": "?clm01", "is_binded": false}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x2 rdf:type acme:Claim)", "label": "(?clm01 rdf:type acme:Claim)", "vertex": 2, "parent_vertex": 1, "beta_relation_vars": ["?x2"], "pruned_var": ["?x1"]}, "consequent_nodes": [], "children_vertexes": [3]}, {"vertex": 3, "parent_vertex": 2, "label": "(?x2 reverse_of ?x3)", "antecedent_node": {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x2", "label": "?clm01", "is_binded": true}, {"type": "identifier", "value": "reverse_of"}, {"type": "var", "id": "?x3", "label": "?clm02", "is_binded": false}], "normalizedLabel": "(?x2 reverse_of ?x3)", "label": "(?clm01 reverse_of ?clm02)", "vertex": 3, "parent_vertex": 2, "beta_relation_vars": ["?x2", "?x3"], "pruned_var": ["?x1"]}, "consequent_nodes": [], "children_vertexes": [4]}, {"vertex": 4, "parent_vertex": 3, "label": "(?x3 rdf:type acme:Claim)", "antecedent_node": {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x3", "label": "?clm02", "is_binded": true}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x3 rdf:type acme:Claim)", "label": "(?clm02 rdf:type acme:Claim)", "vertex": 4, "parent_vertex": 3, "beta_relation_vars": ["?x2"], "pruned_var": ["?x1", "?x3"]}, "consequent_nodes": [{"type": "consequent", "triple": [{"type": "var", "id": "?x2", "label": "?clm01"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x2 is_good true)", "label": "(?clm01 is_good true)", "vertex": 4}], "children_vertexes": []}]}], "rete_nodes": [{"vertex": 0, "parent_vertex": 0, "label": "Head node", "antecedent_node": null, "consequent_nodes": [], "children_vertexes": [1]}, {"vertex": 1, "parent_vertex": 0, "label": "(?x1 rdf:type acme:Claim)", "antecedent_node": {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?clm03", "is_binded": false}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim)", "label": "(?clm03 rdf:type acme:Claim)", "vertex": 1, "parent_vertex": 0, "beta_relation_vars": [], "pruned_var": ["?x1"]}, "consequent_nodes": [], "children_vertexes": [2]}, {"vertex": 2, "parent_vertex": 1, "label": "(?x2 rdf:type acme:Claim)", "antecedent_node": {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x2", "label": "?clm01", "is_binded": false}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x2 rdf:type acme:Claim)", "label": "(?clm01 rdf:type acme:Claim)", "vertex": 2, "parent_vertex": 1, "beta_relation_vars": ["?x2"], "pruned_var": ["?x1"]}, "consequent_nodes": [], "children_vertexes": [3]}, {"vertex": 3, "parent_vertex": 2, "label": "(?x2 reverse_of ?x3)", "antecedent_node": {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x2", "label": "?clm01", "is_binded": true}, {"type": "identifier", "value": "reverse_of"}, {"type": "var", "id": "?x3", "label": "?clm02", "is_binded": false}], "normalizedLabel": "(?x2 reverse_of ?x3)", "label": "(?clm01 reverse_of ?clm02)", "vertex": 3, "parent_vertex": 2, "beta_relation_vars": ["?x2", "?x3"], "pruned_var": ["?x1"]}, "consequent_nodes": [], "children_vertexes": [4]}, {"vertex": 4, "parent_vertex": 3, "label": "(?x3 rdf:type acme:Claim)", "antecedent_node": {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x3", "label": "?clm02", "is_binded": true}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x3 rdf:type acme:Claim)", "label": "(?clm02 rdf:type acme:Claim)", "vertex": 4, "parent_vertex": 3, "beta_relation_vars": ["?x2"], "pruned_var": ["?x1", "?x3"]}, "consequent_nodes": [{"type": "consequent", "triple": [{"type": "var", "id": "?x2", "label": "?clm01"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x2 is_good true)", "label": "(?clm01 is_good true)", "vertex": 4}], "children_vertexes": []}]}"""
    # print()
    # print('OPTIMIZED GOT:',json.dumps(optimized_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(optimized_data))
    self.assertEqual(json.dumps(optimized_data), optimized_expected)



if __name__ == '__main__':
  absltest.main()
