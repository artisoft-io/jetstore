"""JetRuleValidator tests"""

import sys
import json
from typing import Dict
from absl import flags
from absl.testing import absltest
import io

import jetrule_compiler as compiler
from jetrule_context import JetRuleContext
from jetrule_rete import JetRuleRete

FLAGS = flags.FLAGS

class JetRulesReteTest(absltest.TestCase):

  def _get_augmented_data(self, data: io.StringIO) -> JetRuleContext:
    jetrule_ctx =  compiler.processJetRule(data)
    return compiler.postprocessJetRule(jetrule_ctx)


  def test_rete1(self):
    data = io.StringIO("""
      # =======================================================================================
      # Simplest rules that are valid
      # ---------------------------------------------------------------------------------------
      resource rdf:type = "rdf:type";
      resource acme:Claim = "acme:Claim";
      resource acme:PClaim = "acme:PClaim";
      volatile_resource is_good = "is_good";
      [RuleR1]: 
        (?clm01 rdf:type acme:Claim).
        ->
        (?clm01 is_good true).
      ;
      [RuleR2]: 
        (?clm01 rdf:type acme:Claim).
        (?clm01 rdf:type acme:PClaim).
        ->
        (?clm01 is_good true).
      ;
    """)
    jetrule_ctx = self._get_augmented_data(data)
    data.close()

    # Augment with rete markups
    rete = JetRuleRete(jetrule_ctx)
    rete.addReteMarkup()

    rete_data = jetrule_ctx.jetRules
    rete_expected = """{"literals": [], "resources": [{"type": "resource", "id": "rdf:type", "value": "rdf:type"}, {"type": "resource", "id": "acme:Claim", "value": "acme:Claim"}, {"type": "resource", "id": "acme:PClaim", "value": "acme:PClaim"}, {"type": "volatile_resource", "id": "is_good", "value": "is_good"}], "lookup_tables": [], "jet_rules": [{"name": "RuleR1", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim)", "label": "(?clm01 rdf:type acme:Claim)", "vertex": 1, "parent_vertex": 0}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x1 is_good true)", "label": "(?clm01 is_good true)", "vertex": 1}], "normalizedLabel": "[RuleR1]:(?x1 rdf:type acme:Claim) -> (?x1 is_good true);", "label": "[RuleR1]:(?clm01 rdf:type acme:Claim) -> (?clm01 is_good true);"}, {"name": "RuleR2", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim)", "label": "(?clm01 rdf:type acme:Claim)", "vertex": 1, "parent_vertex": 0}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:PClaim"}], "normalizedLabel": "(?x1 rdf:type acme:PClaim)", "label": "(?clm01 rdf:type acme:PClaim)", "vertex": 2, "parent_vertex": 1}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x1 is_good true)", "label": "(?clm01 is_good true)", "vertex": 2}], "normalizedLabel": "[RuleR2]:(?x1 rdf:type acme:Claim).(?x1 rdf:type acme:PClaim) -> (?x1 is_good true);", "label": "[RuleR2]:(?clm01 rdf:type acme:Claim).(?clm01 rdf:type acme:PClaim) -> (?clm01 is_good true);"}]}"""
    # print()
    # print('OPTIMIZED GOT:',json.dumps(rete_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(rete_data))
    self.assertEqual(json.dumps(rete_data), rete_expected)


  def test_rete2(self):
    data = io.StringIO("""
      # =======================================================================================
      # Simplest rules that are valid
      # ---------------------------------------------------------------------------------------
      resource rdf:type = "rdf:type";
      resource acme:Claim = "acme:Claim";
      resource acme:PClaim = "acme:PClaim";
      resource acme:EClaim = "acme:EClaim";
      resource acme:FClaim = "acme:FClaim";
      volatile_resource is_good = "is_good";
      [RuleR10]: 
        (?clm01 rdf:type acme:Claim).
        ->
        (?clm01 is_good true).
      ;
      [RuleR20]: 
        (?s rdf:type acme:Claim).
        (?s rdf:type acme:PClaim).
        (?s rdf:type acme:EClaim).
        ->
        (?s is_good true).
      ;
      [RuleR30]: 
        (?w1 rdf:type acme:Claim).
        (?w1 rdf:type acme:PClaim).
        (?w1 rdf:type acme:FClaim).
        ->
        (?w1 is_good true).
      ;
    """)
    jetrule_ctx = self._get_augmented_data(data)
    data.close()

    # Augment with rete markups
    rete = JetRuleRete(jetrule_ctx)
    rete.addReteMarkup()

    rete_data = jetrule_ctx.jetRules
    rete_expected = """{"literals": [], "resources": [{"type": "resource", "id": "rdf:type", "value": "rdf:type"}, {"type": "resource", "id": "acme:Claim", "value": "acme:Claim"}, {"type": "resource", "id": "acme:PClaim", "value": "acme:PClaim"}, {"type": "resource", "id": "acme:EClaim", "value": "acme:EClaim"}, {"type": "resource", "id": "acme:FClaim", "value": "acme:FClaim"}, {"type": "volatile_resource", "id": "is_good", "value": "is_good"}], "lookup_tables": [], "jet_rules": [{"name": "RuleR10", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim)", "label": "(?clm01 rdf:type acme:Claim)", "vertex": 1, "parent_vertex": 0}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x1 is_good true)", "label": "(?clm01 is_good true)", "vertex": 1}], "normalizedLabel": "[RuleR10]:(?x1 rdf:type acme:Claim) -> (?x1 is_good true);", "label": "[RuleR10]:(?clm01 rdf:type acme:Claim) -> (?clm01 is_good true);"}, {"name": "RuleR20", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?s"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim)", "label": "(?s rdf:type acme:Claim)", "vertex": 1, "parent_vertex": 0}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?s"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:PClaim"}], "normalizedLabel": "(?x1 rdf:type acme:PClaim)", "label": "(?s rdf:type acme:PClaim)", "vertex": 2, "parent_vertex": 1}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?s"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:EClaim"}], "normalizedLabel": "(?x1 rdf:type acme:EClaim)", "label": "(?s rdf:type acme:EClaim)", "vertex": 3, "parent_vertex": 2}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "id": "?x1", "label": "?s"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x1 is_good true)", "label": "(?s is_good true)", "vertex": 3}], "normalizedLabel": "[RuleR20]:(?x1 rdf:type acme:Claim).(?x1 rdf:type acme:PClaim).(?x1 rdf:type acme:EClaim) -> (?x1 is_good true);", "label": "[RuleR20]:(?s rdf:type acme:Claim).(?s rdf:type acme:PClaim).(?s rdf:type acme:EClaim) -> (?s is_good true);"}, {"name": "RuleR30", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?w1"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim)", "label": "(?w1 rdf:type acme:Claim)", "vertex": 1, "parent_vertex": 0}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?w1"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:PClaim"}], "normalizedLabel": "(?x1 rdf:type acme:PClaim)", "label": "(?w1 rdf:type acme:PClaim)", "vertex": 2, "parent_vertex": 1}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?w1"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:FClaim"}], "normalizedLabel": "(?x1 rdf:type acme:FClaim)", "label": "(?w1 rdf:type acme:FClaim)", "vertex": 4, "parent_vertex": 2}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "id": "?x1", "label": "?w1"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x1 is_good true)", "label": "(?w1 is_good true)", "vertex": 4}], "normalizedLabel": "[RuleR30]:(?x1 rdf:type acme:Claim).(?x1 rdf:type acme:PClaim).(?x1 rdf:type acme:FClaim) -> (?x1 is_good true);", "label": "[RuleR30]:(?w1 rdf:type acme:Claim).(?w1 rdf:type acme:PClaim).(?w1 rdf:type acme:FClaim) -> (?w1 is_good true);"}]}"""
    # print()
    # print('OPTIMIZED GOT:',json.dumps(rete_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(rete_data))
    self.assertEqual(json.dumps(rete_data), rete_expected)


if __name__ == '__main__':
  absltest.main()
