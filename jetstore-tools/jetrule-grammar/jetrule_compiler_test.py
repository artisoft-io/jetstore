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

  def _get_augmented_data(self, input_data: str) -> JetRuleContext:
    compiler = JetRuleCompiler()
    compiler.compileJetRule(input_data)
    jetrule_ctx = compiler.jetrule_ctx
    return jetrule_ctx
    

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
    optimized_expected = """{"literals": [], "resources": [{"type": "resource", "id": "rdf:type", "value": "rdf:type"}, {"type": "resource", "id": "acme:Claim", "value": "acme:Claim"}, {"type": "volatile_resource", "id": "is_good", "value": "is_good"}], "lookup_tables": [], "jet_rules": [{"name": "RuleC4", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim).[(?x1 or true) and ?x1]", "label": "(?clm01 rdf:type acme:Claim).[(?clm01 or true) and ?clm01]", "filter": {"type": "binary", "lhs": {"type": "binary", "lhs": {"type": "var", "id": "?x1", "label": "?clm01"}, "op": "or", "rhs": {"type": "keyword", "value": "true"}}, "op": "and", "rhs": {"type": "var", "id": "?x1", "label": "?clm01"}}, "vertex": 1, "parent_vertex": 0}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "is_good"}, {"type": "var", "id": "?x2", "label": "?good"}], "normalizedLabel": "(?x1 is_good ?x2)", "label": "(?clm01 is_good ?good)", "vertex": 2, "parent_vertex": 1}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x1 is_good true)", "label": "(?clm01 is_good true)", "vertex": 2}], "authoredLabel": "[RuleC4]:(?clm01 rdf:type acme:Claim).[?clm01].(?clm01 is_good ?good).[?clm01 or true] -> (?clm01 is_good true);", "normalizedLabel": "[RuleC4]:(?x1 rdf:type acme:Claim).[(?x1 or true) and ?x1].(?x1 is_good ?x2) -> (?x1 is_good true);", "label": "[RuleC4]:(?clm01 rdf:type acme:Claim).[(?clm01 or true) and ?clm01].(?clm01 is_good ?good) -> (?clm01 is_good true);"}]}"""
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
    self.assertEqual(jetrule_ctx.errors[0], "Error rule RuleC5: Identifier 'reverse_of' is not defined in this context '(?clm01 reverse_of ?clm02)', it must be define.")

    optimized_data = jetrule_ctx.jetRules
    optimized_expected = """{"literals": [], "resources": [{"type": "resource", "id": "rdf:type", "value": "rdf:type"}, {"type": "resource", "id": "acme:Claim", "value": "acme:Claim"}, {"type": "volatile_resource", "id": "is_good", "value": "is_good"}], "lookup_tables": [], "jet_rules": [{"name": "RuleC5", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?clm03"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim)", "label": "(?clm03 rdf:type acme:Claim)", "vertex": 1, "parent_vertex": 0}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x2", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x2 rdf:type acme:Claim)", "label": "(?clm01 rdf:type acme:Claim)", "vertex": 2, "parent_vertex": 1}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x2", "label": "?clm01"}, {"type": "identifier", "value": "reverse_of"}, {"type": "var", "id": "?x3", "label": "?clm02"}], "normalizedLabel": "(?x2 reverse_of ?x3)", "label": "(?clm01 reverse_of ?clm02)", "vertex": 3, "parent_vertex": 2}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x3", "label": "?clm02"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x3 rdf:type acme:Claim)", "label": "(?clm02 rdf:type acme:Claim)", "vertex": 4, "parent_vertex": 3}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "id": "?x2", "label": "?clm01"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x2 is_good true)", "label": "(?clm01 is_good true)", "vertex": 4}], "authoredLabel": "[RuleC5]:(?clm01 reverse_of ?clm02).(?clm03 rdf:type acme:Claim).(?clm01 rdf:type acme:Claim).(?clm02 rdf:type acme:Claim) -> (?clm01 is_good true);", "normalizedLabel": "[RuleC5]:(?x1 rdf:type acme:Claim).(?x2 rdf:type acme:Claim).(?x2 reverse_of ?x3).(?x3 rdf:type acme:Claim) -> (?x2 is_good true);", "label": "[RuleC5]:(?clm03 rdf:type acme:Claim).(?clm01 rdf:type acme:Claim).(?clm01 reverse_of ?clm02).(?clm02 rdf:type acme:Claim) -> (?clm01 is_good true);"}]}"""
    # print()
    # print('OPTIMIZED GOT:',json.dumps(optimized_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(optimized_data))
    self.assertEqual(json.dumps(optimized_data), optimized_expected)



if __name__ == '__main__':
  absltest.main()
