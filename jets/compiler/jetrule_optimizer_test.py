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

FLAGS = flags.FLAGS

class JetRulesOptimizerTest(absltest.TestCase):

  def _get_augmented_data(self, input_data: str) -> JetRuleContext:
    compiler = JetRuleCompiler()
    compiler.processJetRule(input_data)
    compiler.postprocessJetRule()
    jetrule_ctx = compiler.jetrule_ctx
    return jetrule_ctx


  def test_optimize1(self):
    data = """
      # =======================================================================================
      # Simplest rule that is valid
      # ---------------------------------------------------------------------------------------
      resource rdf:type = "rdf:type";
      resource acme:Claim = "acme:Claim";
      volatile_resource is_good = "is_good";
      [RuleO1]: 
        (?clm01 is_good ?good).
        (?clm01 rdf:type acme:Claim).
        ->
        (?clm01 is_good true).
      ;
    """
    jetrule_ctx = self._get_augmented_data(data)
    postprocessed_data = jetrule_ctx.jetRules

    # validate that empty rule file is ok
    expected = """{"literals": [], "resources": [{"type": "resource", "id": "rdf:type", "value": "rdf:type", "source_fname": "predefined"}, {"type": "resource", "id": "acme:Claim", "value": "acme:Claim"}, {"type": "volatile_resource", "id": "is_good", "value": "is_good"}], "lookup_tables": [], "jet_rules": [{"name": "RuleO1", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "is_good"}, {"type": "var", "value": "?good", "id": "?x2", "label": "?good"}], "normalizedLabel": "(?x1 is_good ?x2)", "label": "(?clm01 is_good ?good)"}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim)", "label": "(?clm01 rdf:type acme:Claim)"}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x1 is_good true)", "label": "(?clm01 is_good true)"}], "optimization": true, "salience": 100, "normalizedLabel": "[RuleO1]:(?x1 is_good ?x2).(?x1 rdf:type acme:Claim) -> (?x1 is_good true);", "label": "[RuleO1]:(?clm01 is_good ?good).(?clm01 rdf:type acme:Claim) -> (?clm01 is_good true);"}]}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))

    # validate that empty rule file is ok
    self.assertEqual(json.dumps(postprocessed_data), expected)

    # Validate variables
    validator = JetRuleValidator(jetrule_ctx)
    is_valid = validator.validateJetRule()
    self.assertEqual(is_valid, True)
    self.assertEqual(len(jetrule_ctx.errors), 0)

    # Optimize the rules
    optimizer = JetRuleOptimizer(jetrule_ctx)
    optimizer.optimizeJetRules()

    optimized_data = jetrule_ctx.jetRules
    optimized_expected = """{"literals": [], "resources": [{"type": "resource", "id": "rdf:type", "value": "rdf:type", "source_fname": "predefined"}, {"type": "resource", "id": "acme:Claim", "value": "acme:Claim"}, {"type": "volatile_resource", "id": "is_good", "value": "is_good"}], "lookup_tables": [], "jet_rules": [{"name": "RuleO1", "properties": {}, "optimization": true, "salience": 100, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim)", "label": "(?clm01 rdf:type acme:Claim)"}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "is_good"}, {"type": "var", "value": "?good", "id": "?x2", "label": "?good"}], "normalizedLabel": "(?x1 is_good ?x2)", "label": "(?clm01 is_good ?good)"}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x1 is_good true)", "label": "(?clm01 is_good true)"}], "authoredLabel": "[RuleO1]:(?clm01 is_good ?good).(?clm01 rdf:type acme:Claim) -> (?clm01 is_good true);", "source_file_name": null, "normalizedLabel": "[RuleO1]:(?x1 rdf:type acme:Claim).(?x1 is_good ?x2) -> (?x1 is_good true);", "label": "[RuleO1]:(?clm01 rdf:type acme:Claim).(?clm01 is_good ?good) -> (?clm01 is_good true);"}]}"""
    # print()
    # print('OPTIMIZED GOT:',json.dumps(optimized_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(optimized_data))
    self.assertEqual(json.dumps(optimized_data), optimized_expected)

  def test_optimize2(self):
    data = """
      # =======================================================================================
      # Simplest rule that is valid
      # ---------------------------------------------------------------------------------------
      resource rdf:type = "rdf:type";
      resource acme:Claim = "acme:Claim";
      volatile_resource is_good = "is_good";
      [RuleO2]: 
        (?clm01 ?good true).
        (?clm02 rdf:type acme:Claim).
        ->
        (?clm02 is_good true).
      ;
    """
    jetrule_ctx = self._get_augmented_data(data)

    # Validate variables
    validator = JetRuleValidator(jetrule_ctx)
    is_valid = validator.validateJetRule()
    self.assertEqual(is_valid, True)
    self.assertEqual(len(jetrule_ctx.errors), 0)

    # Optimize the rules
    optimizer = JetRuleOptimizer(jetrule_ctx)
    optimizer.optimizeJetRules()

    optimized_data = jetrule_ctx.jetRules
    optimized_expected = """{"literals": [], "resources": [{"type": "resource", "id": "rdf:type", "value": "rdf:type", "source_fname": "predefined"}, {"type": "resource", "id": "acme:Claim", "value": "acme:Claim"}, {"type": "volatile_resource", "id": "is_good", "value": "is_good"}], "lookup_tables": [], "jet_rules": [{"name": "RuleO2", "properties": {}, "optimization": true, "salience": 100, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm02", "id": "?x1", "label": "?clm02"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim)", "label": "(?clm02 rdf:type acme:Claim)"}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01", "id": "?x2", "label": "?clm01"}, {"type": "var", "value": "?good", "id": "?x3", "label": "?good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x2 ?x3 true)", "label": "(?clm01 ?good true)"}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?clm02", "id": "?x1", "label": "?clm02"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x1 is_good true)", "label": "(?clm02 is_good true)"}], "authoredLabel": "[RuleO2]:(?clm01 ?good true).(?clm02 rdf:type acme:Claim) -> (?clm02 is_good true);", "source_file_name": null, "normalizedLabel": "[RuleO2]:(?x1 rdf:type acme:Claim).(?x2 ?x3 true) -> (?x1 is_good true);", "label": "[RuleO2]:(?clm02 rdf:type acme:Claim).(?clm01 ?good true) -> (?clm02 is_good true);"}]}"""
    # print()
    # print('OPTIMIZED GOT:',json.dumps(optimized_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(optimized_data))
    self.assertEqual(json.dumps(optimized_data), optimized_expected)

  def test_optimize3(self):
    data = """
      # =======================================================================================
      # Simplest rule that is valid
      # ---------------------------------------------------------------------------------------
      resource rdf:type = "rdf:type";
      resource acme:Claim = "acme:Claim";
      volatile_resource is_good = "is_good";
      [RuleO3]: 
        (?clm01 ?good true).
        (?clm02 rdf:type acme:Claim).[?good and (not ?clm01)].
        (?clm03 rdf:type acme:Claim)
        ->
        (?clm02 is_good true).
      ;
    """
    jetrule_ctx = self._get_augmented_data(data)
    # Validate variables
    validator = JetRuleValidator(jetrule_ctx)
    is_valid = validator.validateJetRule()
    self.assertEqual(is_valid, True)
    self.assertEqual(len(jetrule_ctx.errors), 0)

    # Optimize the rules
    optimizer = JetRuleOptimizer(jetrule_ctx)
    optimizer.optimizeJetRules()

    optimized_data = jetrule_ctx.jetRules
    optimized_expected = """{"literals": [], "resources": [{"type": "resource", "id": "rdf:type", "value": "rdf:type", "source_fname": "predefined"}, {"type": "resource", "id": "acme:Claim", "value": "acme:Claim"}, {"type": "volatile_resource", "id": "is_good", "value": "is_good"}], "lookup_tables": [], "jet_rules": [{"name": "RuleO3", "properties": {}, "optimization": true, "salience": 100, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm02", "id": "?x1", "label": "?clm02"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim)", "label": "(?clm02 rdf:type acme:Claim)"}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm03", "id": "?x2", "label": "?clm03"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x2 rdf:type acme:Claim)", "label": "(?clm03 rdf:type acme:Claim)"}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01", "id": "?x3", "label": "?clm01"}, {"type": "var", "value": "?good", "id": "?x4", "label": "?good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x3 ?x4 true).[?x4 and (not ?x3)]", "label": "(?clm01 ?good true).[?good and (not ?clm01)]", "filter": {"type": "binary", "lhs": {"type": "var", "value": "?good", "id": "?x4", "label": "?good"}, "op": "and", "rhs": {"type": "unary", "op": "not", "arg": {"type": "var", "value": "?clm01", "id": "?x3", "label": "?clm01"}}}}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?clm02", "id": "?x1", "label": "?clm02"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x1 is_good true)", "label": "(?clm02 is_good true)"}], "authoredLabel": "[RuleO3]:(?clm01 ?good true).(?clm02 rdf:type acme:Claim).[?good and (not ?clm01)].(?clm03 rdf:type acme:Claim) -> (?clm02 is_good true);", "source_file_name": null, "normalizedLabel": "[RuleO3]:(?x1 rdf:type acme:Claim).(?x2 rdf:type acme:Claim).(?x3 ?x4 true).[?x4 and (not ?x3)] -> (?x1 is_good true);", "label": "[RuleO3]:(?clm02 rdf:type acme:Claim).(?clm03 rdf:type acme:Claim).(?clm01 ?good true).[?good and (not ?clm01)] -> (?clm02 is_good true);"}]}"""
    # print()
    # print('OPTIMIZED GOT:',json.dumps(optimized_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(optimized_data))
    self.assertEqual(json.dumps(optimized_data), optimized_expected)

  def test_optimize4(self):
    data = """
      # =======================================================================================
      resource rdf:type = "rdf:type";
      resource acme:Claim = "acme:Claim";
      volatile_resource is_good = "is_good";
      [RuleO4]: 
        (?clm01 rdf:type acme:Claim).[?clm01]
        (?clm01 is_good ?good).[?clm01 or true]
        ->
        (?clm01 is_good true).
      ;
    """
    jetrule_ctx = self._get_augmented_data(data)

    # Validate variables
    validator = JetRuleValidator(jetrule_ctx)
    is_valid = validator.validateJetRule()
    self.assertEqual(is_valid, True)
    self.assertEqual(len(jetrule_ctx.errors), 0)

    # Optimize the rules
    optimizer = JetRuleOptimizer(jetrule_ctx)
    optimizer.optimizeJetRules()

    optimized_data = jetrule_ctx.jetRules
    optimized_expected = """{"literals": [], "resources": [{"type": "resource", "id": "rdf:type", "value": "rdf:type", "source_fname": "predefined"}, {"type": "resource", "id": "acme:Claim", "value": "acme:Claim"}, {"type": "volatile_resource", "id": "is_good", "value": "is_good"}], "lookup_tables": [], "jet_rules": [{"name": "RuleO4", "properties": {}, "optimization": true, "salience": 100, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim).[(?x1 or true) and ?x1]", "label": "(?clm01 rdf:type acme:Claim).[(?clm01 or true) and ?clm01]", "filter": {"type": "binary", "lhs": {"type": "binary", "lhs": {"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, "op": "or", "rhs": {"type": "keyword", "value": "true"}}, "op": "and", "rhs": {"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}}}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "is_good"}, {"type": "var", "value": "?good", "id": "?x2", "label": "?good"}], "normalizedLabel": "(?x1 is_good ?x2)", "label": "(?clm01 is_good ?good)"}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x1 is_good true)", "label": "(?clm01 is_good true)"}], "authoredLabel": "[RuleO4]:(?clm01 rdf:type acme:Claim).[?clm01].(?clm01 is_good ?good).[?clm01 or true] -> (?clm01 is_good true);", "source_file_name": null, "normalizedLabel": "[RuleO4]:(?x1 rdf:type acme:Claim).[(?x1 or true) and ?x1].(?x1 is_good ?x2) -> (?x1 is_good true);", "label": "[RuleO4]:(?clm01 rdf:type acme:Claim).[(?clm01 or true) and ?clm01].(?clm01 is_good ?good) -> (?clm01 is_good true);"}]}"""
    # print()
    # print('OPTIMIZED GOT:',json.dumps(optimized_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(optimized_data))
    self.assertEqual(json.dumps(optimized_data), optimized_expected)

  def test_optimize5(self):
    data = """
      # =======================================================================================
      resource rdf:type = "rdf:type";
      resource acme:Claim = "acme:Claim";
      volatile_resource is_good = "is_good";
      [RuleO5]: 
        (?clm01 reverse_of ?clm02).
        (?clm03 rdf:type acme:Claim).
        (?clm01 rdf:type acme:Claim).
        (?clm02 rdf:type acme:Claim)
        ->
        (?clm01 is_good true).
      ;
    """

  # Result:
  # "authoredLabel": "[RuleO5]:(?clm01 reverse_of ?clm02).(?clm03 rdf:type acme:Claim).(?clm01 rdf:type acme:Claim).(?clm02 rdf:type acme:Claim) -> (?clm01 is_good true);",
  # "normalizedLabel": "[RuleO5]:(?x1 rdf:type acme:Claim).(?x2 rdf:type acme:Claim).(?x2 reverse_of ?x3).(?x3 rdf:type acme:Claim) -> (?x2 is_good true);",
  # "label": "[RuleO5]:(?clm03 rdf:type acme:Claim).(?clm01 rdf:type acme:Claim).(?clm01 reverse_of ?clm02).(?clm02 rdf:type acme:Claim) -> (?clm01 is_good true);"    

    jetrule_ctx = self._get_augmented_data(data)
    self.assertEqual(jetrule_ctx.ERROR, False)

    # Optimize the rules
    optimizer = JetRuleOptimizer(jetrule_ctx)
    optimizer.optimizeJetRules()

    optimized_data = jetrule_ctx.jetRules
    optimized_expected = """{"literals": [], "resources": [{"type": "resource", "id": "rdf:type", "value": "rdf:type", "source_fname": "predefined"}, {"type": "resource", "id": "acme:Claim", "value": "acme:Claim"}, {"type": "volatile_resource", "id": "is_good", "value": "is_good"}], "lookup_tables": [], "jet_rules": [{"name": "RuleO5", "properties": {}, "optimization": true, "salience": 100, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm03", "id": "?x1", "label": "?clm03"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim)", "label": "(?clm03 rdf:type acme:Claim)"}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01", "id": "?x2", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x2 rdf:type acme:Claim)", "label": "(?clm01 rdf:type acme:Claim)"}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01", "id": "?x2", "label": "?clm01"}, {"type": "identifier", "value": "reverse_of"}, {"type": "var", "value": "?clm02", "id": "?x3", "label": "?clm02"}], "normalizedLabel": "(?x2 reverse_of ?x3)", "label": "(?clm01 reverse_of ?clm02)"}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm02", "id": "?x3", "label": "?clm02"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x3 rdf:type acme:Claim)", "label": "(?clm02 rdf:type acme:Claim)"}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?clm01", "id": "?x2", "label": "?clm01"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x2 is_good true)", "label": "(?clm01 is_good true)"}], "authoredLabel": "[RuleO5]:(?clm01 reverse_of ?clm02).(?clm03 rdf:type acme:Claim).(?clm01 rdf:type acme:Claim).(?clm02 rdf:type acme:Claim) -> (?clm01 is_good true);", "source_file_name": null, "normalizedLabel": "[RuleO5]:(?x1 rdf:type acme:Claim).(?x2 rdf:type acme:Claim).(?x2 reverse_of ?x3).(?x3 rdf:type acme:Claim) -> (?x2 is_good true);", "label": "[RuleO5]:(?clm03 rdf:type acme:Claim).(?clm01 rdf:type acme:Claim).(?clm01 reverse_of ?clm02).(?clm02 rdf:type acme:Claim) -> (?clm01 is_good true);"}]}"""
    # print()
    # print('OPTIMIZED GOT:',json.dumps(optimized_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(optimized_data))
    self.assertEqual(json.dumps(optimized_data), optimized_expected)

  def test_optimize6(self):
    data = """
      # =======================================================================================
      resource rdf:type = "rdf:type";
      resource acme:Claim = "acme:Claim";
      volatile_resource is_good = "is_good";
      [RuleO5,o=false]: 
        (?clm01 reverse_of ?clm02).
        (?clm03 rdf:type acme:Claim).
        (?clm01 rdf:type acme:Claim).
        (?clm02 rdf:type acme:Claim)
        ->
        (?clm01 is_good true).
      ;
    """

  # Result:
  # "authoredLabel": "[RuleO5]:(?clm01 reverse_of ?clm02).(?clm03 rdf:type acme:Claim).(?clm01 rdf:type acme:Claim).(?clm02 rdf:type acme:Claim) -> (?clm01 is_good true);",
  # "normalizedLabel": "[RuleO5]:(?x1 rdf:type acme:Claim).(?x2 rdf:type acme:Claim).(?x2 reverse_of ?x3).(?x3 rdf:type acme:Claim) -> (?x2 is_good true);",
  # "label": "[RuleO5]:(?clm03 rdf:type acme:Claim).(?clm01 rdf:type acme:Claim).(?clm01 reverse_of ?clm02).(?clm02 rdf:type acme:Claim) -> (?clm01 is_good true);"    

    jetrule_ctx = self._get_augmented_data(data)
    for k in jetrule_ctx.errors:
      print(k)

    # Optimize the rules
    optimizer = JetRuleOptimizer(jetrule_ctx)
    optimizer.optimizeJetRules()

    optimized_data = jetrule_ctx.jetRules
    optimized_expected = """{"literals": [], "resources": [{"type": "resource", "id": "rdf:type", "value": "rdf:type", "source_fname": "predefined"}, {"type": "resource", "id": "acme:Claim", "value": "acme:Claim"}, {"type": "volatile_resource", "id": "is_good", "value": "is_good"}], "lookup_tables": [], "jet_rules": [{"name": "RuleO5", "properties": {"o": "false"}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "reverse_of"}, {"type": "var", "value": "?clm02", "id": "?x2", "label": "?clm02"}], "normalizedLabel": "(?x1 reverse_of ?x2)", "label": "(?clm01 reverse_of ?clm02)"}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm03", "id": "?x3", "label": "?clm03"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x3 rdf:type acme:Claim)", "label": "(?clm03 rdf:type acme:Claim)"}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim)", "label": "(?clm01 rdf:type acme:Claim)"}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm02", "id": "?x2", "label": "?clm02"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x2 rdf:type acme:Claim)", "label": "(?clm02 rdf:type acme:Claim)"}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x1 is_good true)", "label": "(?clm01 is_good true)"}], "optimization": false, "salience": 100, "normalizedLabel": "[RuleO5, o=false]:(?x1 reverse_of ?x2).(?x3 rdf:type acme:Claim).(?x1 rdf:type acme:Claim).(?x2 rdf:type acme:Claim) -> (?x1 is_good true);", "label": "[RuleO5, o=false]:(?clm01 reverse_of ?clm02).(?clm03 rdf:type acme:Claim).(?clm01 rdf:type acme:Claim).(?clm02 rdf:type acme:Claim) -> (?clm01 is_good true);", "authoredLabel": "[RuleO5, o=false]:(?clm01 reverse_of ?clm02).(?clm03 rdf:type acme:Claim).(?clm01 rdf:type acme:Claim).(?clm02 rdf:type acme:Claim) -> (?clm01 is_good true);"}]}"""
    self.assertEqual(jetrule_ctx.ERROR, False)
    # print()
    # print('OPTIMIZED GOT:',json.dumps(optimized_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(optimized_data))
    self.assertEqual(json.dumps(optimized_data), optimized_expected)



if __name__ == '__main__':
  absltest.main()
