"""JetRuleValidator tests"""

import sys
import json
from typing import Dict
from absl import flags
from absl.testing import absltest
import io

from jetrule_compiler import JetRuleCompiler, InputProvider
from jetrule_context import JetRuleContext
from jetrule_optimizer import JetRuleOptimizer
from jetrule_validator import JetRuleValidator
from jetrule_rete import JetRuleRete

FLAGS = flags.FLAGS

class JetRulesReteTest(absltest.TestCase):

  def _get_augmented_data(self, input_data: str) -> JetRuleContext:
    compiler = JetRuleCompiler()
    compiler.processJetRule(input_data)
    compiler.postprocessJetRule()
    jetrule_ctx = compiler.jetrule_ctx
    return jetrule_ctx


  def test_rete1(self):
    data = """
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
    """
    jetrule_ctx = self._get_augmented_data(data)
    self.assertEqual(jetrule_ctx.ERROR, False)

    # Augment with rete markups
    rete = JetRuleRete(jetrule_ctx)
    rete.addReteMarkup()

    rete_data = jetrule_ctx.jetRules
    rete_expected = """{"literals": [], "resources": [{"type": "resource", "id": "rdf:type", "value": "rdf:type", "source_fname": "predefined"}, {"type": "resource", "id": "acme:Claim", "value": "acme:Claim"}, {"type": "resource", "id": "acme:PClaim", "value": "acme:PClaim"}, {"type": "volatile_resource", "id": "is_good", "value": "is_good"}], "lookup_tables": [], "jet_rules": [{"name": "RuleR1", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim)", "label": "(?clm01 rdf:type acme:Claim)", "vertex": 1, "parent_vertex": 0}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x1 is_good true)", "label": "(?clm01 is_good true)", "vertex": 1}], "optimization": true, "salience": 100, "normalizedLabel": "[RuleR1]:(?x1 rdf:type acme:Claim) -> (?x1 is_good true);", "label": "[RuleR1]:(?clm01 rdf:type acme:Claim) -> (?clm01 is_good true);"}, {"name": "RuleR2", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim)", "label": "(?clm01 rdf:type acme:Claim)", "vertex": 1, "parent_vertex": 0}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:PClaim"}], "normalizedLabel": "(?x1 rdf:type acme:PClaim)", "label": "(?clm01 rdf:type acme:PClaim)", "vertex": 2, "parent_vertex": 1}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x1 is_good true)", "label": "(?clm01 is_good true)", "vertex": 2}], "optimization": true, "salience": 100, "normalizedLabel": "[RuleR2]:(?x1 rdf:type acme:Claim).(?x1 rdf:type acme:PClaim) -> (?x1 is_good true);", "label": "[RuleR2]:(?clm01 rdf:type acme:Claim).(?clm01 rdf:type acme:PClaim) -> (?clm01 is_good true);"}]}"""
    # print()
    # print('OPTIMIZED GOT:',json.dumps(rete_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(rete_data))
    self.assertEqual(json.dumps(rete_data), rete_expected)


  def test_rete2(self):
    data = """
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
    """
    jetrule_ctx = self._get_augmented_data(data)
    self.assertEqual(jetrule_ctx.ERROR, False)

    # Augment with rete markups
    rete = JetRuleRete(jetrule_ctx)
    rete.addReteMarkup()

    rete_data = jetrule_ctx.jetRules
    rete_expected = """{"literals": [], "resources": [{"type": "resource", "id": "rdf:type", "value": "rdf:type", "source_fname": "predefined"}, {"type": "resource", "id": "acme:Claim", "value": "acme:Claim"}, {"type": "resource", "id": "acme:PClaim", "value": "acme:PClaim"}, {"type": "resource", "id": "acme:EClaim", "value": "acme:EClaim"}, {"type": "resource", "id": "acme:FClaim", "value": "acme:FClaim"}, {"type": "volatile_resource", "id": "is_good", "value": "is_good"}], "lookup_tables": [], "jet_rules": [{"name": "RuleR10", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim)", "label": "(?clm01 rdf:type acme:Claim)", "vertex": 1, "parent_vertex": 0}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x1 is_good true)", "label": "(?clm01 is_good true)", "vertex": 1}], "optimization": true, "salience": 100, "normalizedLabel": "[RuleR10]:(?x1 rdf:type acme:Claim) -> (?x1 is_good true);", "label": "[RuleR10]:(?clm01 rdf:type acme:Claim) -> (?clm01 is_good true);"}, {"name": "RuleR20", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?s", "id": "?x1", "label": "?s"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim)", "label": "(?s rdf:type acme:Claim)", "vertex": 1, "parent_vertex": 0}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?s", "id": "?x1", "label": "?s"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:PClaim"}], "normalizedLabel": "(?x1 rdf:type acme:PClaim)", "label": "(?s rdf:type acme:PClaim)", "vertex": 2, "parent_vertex": 1}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?s", "id": "?x1", "label": "?s"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:EClaim"}], "normalizedLabel": "(?x1 rdf:type acme:EClaim)", "label": "(?s rdf:type acme:EClaim)", "vertex": 3, "parent_vertex": 2}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?s", "id": "?x1", "label": "?s"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x1 is_good true)", "label": "(?s is_good true)", "vertex": 3}], "optimization": true, "salience": 100, "normalizedLabel": "[RuleR20]:(?x1 rdf:type acme:Claim).(?x1 rdf:type acme:PClaim).(?x1 rdf:type acme:EClaim) -> (?x1 is_good true);", "label": "[RuleR20]:(?s rdf:type acme:Claim).(?s rdf:type acme:PClaim).(?s rdf:type acme:EClaim) -> (?s is_good true);"}, {"name": "RuleR30", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?w1", "id": "?x1", "label": "?w1"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim)", "label": "(?w1 rdf:type acme:Claim)", "vertex": 1, "parent_vertex": 0}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?w1", "id": "?x1", "label": "?w1"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:PClaim"}], "normalizedLabel": "(?x1 rdf:type acme:PClaim)", "label": "(?w1 rdf:type acme:PClaim)", "vertex": 2, "parent_vertex": 1}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?w1", "id": "?x1", "label": "?w1"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:FClaim"}], "normalizedLabel": "(?x1 rdf:type acme:FClaim)", "label": "(?w1 rdf:type acme:FClaim)", "vertex": 4, "parent_vertex": 2}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?w1", "id": "?x1", "label": "?w1"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x1 is_good true)", "label": "(?w1 is_good true)", "vertex": 4}], "optimization": true, "salience": 100, "normalizedLabel": "[RuleR30]:(?x1 rdf:type acme:Claim).(?x1 rdf:type acme:PClaim).(?x1 rdf:type acme:FClaim) -> (?x1 is_good true);", "label": "[RuleR30]:(?w1 rdf:type acme:Claim).(?w1 rdf:type acme:PClaim).(?w1 rdf:type acme:FClaim) -> (?w1 is_good true);"}]}"""
    # print()
    # print('OPTIMIZED GOT:',json.dumps(rete_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(rete_data))
    self.assertEqual(json.dumps(rete_data), rete_expected)


  def test_rete3(self):
    data = """
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
        (?w1 rdf:type acme:FClaim).
        (?w1 rdf:type acme:PClaim).
        ->
        (?w1 is_good true).
      ;
    """
    jetrule_ctx = self._get_augmented_data(data)
    self.assertEqual(jetrule_ctx.ERROR, False)

    # Augment with rete markups
    rete = JetRuleRete(jetrule_ctx)
    rete.addReteMarkup()

    rete_data = jetrule_ctx.jetRules
    rete_expected = """{"literals": [], "resources": [{"type": "resource", "id": "rdf:type", "value": "rdf:type", "source_fname": "predefined"}, {"type": "resource", "id": "acme:Claim", "value": "acme:Claim"}, {"type": "resource", "id": "acme:PClaim", "value": "acme:PClaim"}, {"type": "resource", "id": "acme:EClaim", "value": "acme:EClaim"}, {"type": "resource", "id": "acme:FClaim", "value": "acme:FClaim"}, {"type": "volatile_resource", "id": "is_good", "value": "is_good"}], "lookup_tables": [], "jet_rules": [{"name": "RuleR10", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim)", "label": "(?clm01 rdf:type acme:Claim)", "vertex": 1, "parent_vertex": 0}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x1 is_good true)", "label": "(?clm01 is_good true)", "vertex": 1}], "optimization": true, "salience": 100, "normalizedLabel": "[RuleR10]:(?x1 rdf:type acme:Claim) -> (?x1 is_good true);", "label": "[RuleR10]:(?clm01 rdf:type acme:Claim) -> (?clm01 is_good true);"}, {"name": "RuleR20", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?s", "id": "?x1", "label": "?s"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim)", "label": "(?s rdf:type acme:Claim)", "vertex": 1, "parent_vertex": 0}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?s", "id": "?x1", "label": "?s"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:PClaim"}], "normalizedLabel": "(?x1 rdf:type acme:PClaim)", "label": "(?s rdf:type acme:PClaim)", "vertex": 2, "parent_vertex": 1}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?s", "id": "?x1", "label": "?s"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:EClaim"}], "normalizedLabel": "(?x1 rdf:type acme:EClaim)", "label": "(?s rdf:type acme:EClaim)", "vertex": 3, "parent_vertex": 2}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?s", "id": "?x1", "label": "?s"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x1 is_good true)", "label": "(?s is_good true)", "vertex": 3}], "optimization": true, "salience": 100, "normalizedLabel": "[RuleR20]:(?x1 rdf:type acme:Claim).(?x1 rdf:type acme:PClaim).(?x1 rdf:type acme:EClaim) -> (?x1 is_good true);", "label": "[RuleR20]:(?s rdf:type acme:Claim).(?s rdf:type acme:PClaim).(?s rdf:type acme:EClaim) -> (?s is_good true);"}, {"name": "RuleR30", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?w1", "id": "?x1", "label": "?w1"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim)", "label": "(?w1 rdf:type acme:Claim)", "vertex": 1, "parent_vertex": 0}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?w1", "id": "?x1", "label": "?w1"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:FClaim"}], "normalizedLabel": "(?x1 rdf:type acme:FClaim)", "label": "(?w1 rdf:type acme:FClaim)", "vertex": 4, "parent_vertex": 1}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?w1", "id": "?x1", "label": "?w1"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:PClaim"}], "normalizedLabel": "(?x1 rdf:type acme:PClaim)", "label": "(?w1 rdf:type acme:PClaim)", "vertex": 5, "parent_vertex": 4}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?w1", "id": "?x1", "label": "?w1"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x1 is_good true)", "label": "(?w1 is_good true)", "vertex": 5}], "optimization": true, "salience": 100, "normalizedLabel": "[RuleR30]:(?x1 rdf:type acme:Claim).(?x1 rdf:type acme:FClaim).(?x1 rdf:type acme:PClaim) -> (?x1 is_good true);", "label": "[RuleR30]:(?w1 rdf:type acme:Claim).(?w1 rdf:type acme:FClaim).(?w1 rdf:type acme:PClaim) -> (?w1 is_good true);"}]}"""
    # print()
    # print('OPTIMIZED GOT:',json.dumps(rete_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(rete_data))
    self.assertEqual(json.dumps(rete_data), rete_expected)

  def test_beta_relation1(self):
    data = """
      # =======================================================================================
      # Simplest rules that are valid
      # ---------------------------------------------------------------------------------------
      resource rdf:type = "rdf:type";
      resource acme:Claim = "acme:Claim";
      resource acme:PClaim = "acme:PClaim";
      resource acme:EClaim = "acme:EClaim";
      resource acme:FClaim = "acme:FClaim";
      resource relatedTo = "relatedTo";
      volatile_resource is_good = "is_good";
      [RuleB10]: 
        (?clm01 rdf:type acme:Claim).[?clm01]
        (?clm01 relatedTo ?clm02).
        (?clm02 relatedTo ?clm03).
        (?clm01 acme:PClaim acme:Claim)
        ->
        (?clm01 is_good true).
        (?clm03 is_good true).
      ;
    """
    jetrule_ctx = self._get_augmented_data(data)
    self.assertEqual(jetrule_ctx.ERROR, False)

    # Augment with rete markups
    rete = JetRuleRete(jetrule_ctx)
    rete.addReteMarkup()
    rete.addBetaRelationMarkup()

    rete_data = jetrule_ctx.jetReteNodes
    rete_expected = """{"main_rule_file_name": null, "support_rule_file_names": null, "resources": [], "lookup_tables": [], "rete_nodes": [{"vertex": 0, "parent_vertex": 0, "label": "Head node", "antecedent_node": null, "consequent_nodes": [], "children_vertexes": [1]}, {"vertex": 1, "parent_vertex": 0, "label": "(?x1 rdf:type acme:Claim).[?x1]", "rules": [], "salience": [], "antecedent_node": {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01", "is_binded": false, "var_pos": 0}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "filter": {"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01", "is_binded": true}, "normalizedLabel": "(?x1 rdf:type acme:Claim).[?x1]", "label": "(?clm01 rdf:type acme:Claim).[?clm01]", "vertex": 1, "parent_vertex": 0, "beta_relation_vars": ["?x1"], "pruned_var": [], "beta_var_nodes": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01", "is_binded": false, "var_pos": 0}]}, "consequent_nodes": [], "children_vertexes": [2]}, {"vertex": 2, "parent_vertex": 1, "label": "(?x1 relatedTo ?x2)", "rules": [], "salience": [], "antecedent_node": {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01", "is_binded": true}, {"type": "identifier", "value": "relatedTo"}, {"type": "var", "value": "?clm02", "id": "?x2", "label": "?clm02", "is_binded": false, "var_pos": 2}], "normalizedLabel": "(?x1 relatedTo ?x2)", "label": "(?clm01 relatedTo ?clm02)", "vertex": 2, "parent_vertex": 1, "beta_relation_vars": ["?x1", "?x2"], "pruned_var": [], "beta_var_nodes": [{"type": "var", "id": "?x1", "is_binded": true, "var_pos": 0, "vertex": 2}, {"type": "var", "value": "?clm02", "id": "?x2", "label": "?clm02", "is_binded": false, "var_pos": 2}]}, "consequent_nodes": [], "children_vertexes": [3]}, {"vertex": 3, "parent_vertex": 2, "label": "(?x2 relatedTo ?x3)", "rules": [], "salience": [], "antecedent_node": {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm02", "id": "?x2", "label": "?clm02", "is_binded": true}, {"type": "identifier", "value": "relatedTo"}, {"type": "var", "value": "?clm03", "id": "?x3", "label": "?clm03", "is_binded": false, "var_pos": 2}], "normalizedLabel": "(?x2 relatedTo ?x3)", "label": "(?clm02 relatedTo ?clm03)", "vertex": 3, "parent_vertex": 2, "beta_relation_vars": ["?x1", "?x3"], "pruned_var": ["?x2"], "beta_var_nodes": [{"type": "var", "id": "?x1", "is_binded": true, "var_pos": 0, "vertex": 3}, {"type": "var", "value": "?clm03", "id": "?x3", "label": "?clm03", "is_binded": false, "var_pos": 2}]}, "consequent_nodes": [], "children_vertexes": [4]}, {"vertex": 4, "parent_vertex": 3, "label": "(?x1 acme:PClaim acme:Claim)", "rules": ["RuleB10"], "salience": [100], "antecedent_node": {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01", "is_binded": true}, {"type": "identifier", "value": "acme:PClaim"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 acme:PClaim acme:Claim)", "label": "(?clm01 acme:PClaim acme:Claim)", "vertex": 4, "parent_vertex": 3, "beta_relation_vars": ["?x1", "?x3"], "pruned_var": ["?x2"], "beta_var_nodes": [{"type": "var", "id": "?x1", "is_binded": true, "var_pos": 0, "vertex": 4}, {"type": "var", "id": "?x3", "is_binded": true, "var_pos": 1, "vertex": 4}]}, "consequent_nodes": [{"type": "consequent", "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01", "is_binded": true}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x1 is_good true)", "label": "(?clm01 is_good true)", "vertex": 4, "consequent_seq": 0, "consequent_for_rule": "RuleB10", "consequent_salience": 100}, {"type": "consequent", "triple": [{"type": "var", "value": "?clm03", "id": "?x3", "label": "?clm03", "is_binded": true}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x3 is_good true)", "label": "(?clm03 is_good true)", "vertex": 4, "consequent_seq": 1, "consequent_for_rule": "RuleB10", "consequent_salience": 100}], "children_vertexes": []}]}"""
    # print()
    # print('OPTIMIZED GOT:',json.dumps(rete_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(rete_data))
    self.assertEqual(json.dumps(rete_data), rete_expected)


if __name__ == '__main__':
  absltest.main()
