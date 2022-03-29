"""JetRuleValidator tests"""

import sys
import json
from typing import Dict
from absl import flags
from absl.testing import absltest
import io

from jetrule_compiler import JetRuleCompiler, InputProvider
from jetrule_context import JetRuleContext
from jetrule_validator import JetRuleValidator

FLAGS = flags.FLAGS

class JetRulesValidatorTest(absltest.TestCase):

  def _get_augmented_data(self, input_data: str) -> JetRuleContext:
    compiler = JetRuleCompiler()
    compiler.processJetRule(input_data)
    compiler.postprocessJetRule()
    jetrule_ctx = compiler.jetrule_ctx
    return jetrule_ctx

  def test_validate_var1(self):
    data = """
      # =======================================================================================
      # Defining Constants Resources and Literals
      # ---------------------------------------------------------------------------------------
      # That should not create any error since no rules are declared
    """
    jetrule_ctx = self._get_augmented_data(data)
    self.assertEqual(jetrule_ctx.ERROR, False)
    postprocessed_data = jetrule_ctx.jetRules

    # validate that empty rule file is ok
    expected = """{"literals": [], "resources": [], "lookup_tables": [], "jet_rules": []}"""
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

  def test_validate_var2(self):
    data = """
      # =======================================================================================
      # Simplest rule that is valid
      # ---------------------------------------------------------------------------------------
      resource rdf:type = "rdf:type";
      resource acme:Claim = "acme:Claim";
      volatile_resource is_good = "is_good";
      [RuleV1]: 
        (?clm01 rdf:type acme:Claim).
        ->
        (?clm01 is_good true).
      ;
    """
    jetrule_ctx = self._get_augmented_data(data)
    self.assertEqual(jetrule_ctx.ERROR, False)
    postprocessed_data = jetrule_ctx.jetRules

    # validate that empty rule file is ok
    expected = """{"literals": [], "resources": [{"type": "resource", "id": "rdf:type", "value": "rdf:type", "source_fname": "predefined"}, {"type": "resource", "id": "acme:Claim", "value": "acme:Claim"}, {"type": "volatile_resource", "id": "is_good", "value": "is_good"}], "lookup_tables": [], "jet_rules": [{"name": "RuleV1", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim)", "label": "(?clm01 rdf:type acme:Claim)"}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x1 is_good true)", "label": "(?clm01 is_good true)"}], "optimization": true, "salience": 100, "normalizedLabel": "[RuleV1]:(?x1 rdf:type acme:Claim) -> (?x1 is_good true);", "label": "[RuleV1]:(?clm01 rdf:type acme:Claim) -> (?clm01 is_good true);"}]}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))
    self.assertEqual(json.dumps(postprocessed_data), expected)

    # Validate variables
    validator = JetRuleValidator(jetrule_ctx)
    is_valid = validator.validateJetRule()
    self.assertEqual(is_valid, True)
    self.assertEqual(len(jetrule_ctx.errors), 0)

  def test_validate_var3(self):
    data = """
      # =======================================================================================
      # Simplest rule that is NOT valid
      # ---------------------------------------------------------------------------------------
      resource rdf:type = "rdf:type";
      resource acme:Claim = "acme:Claim";
      [RuleV2]: 
        (?clm01 rdf:type acme:Claim).
        ->
        (?clm02 is_good false).
      ;
    """
    jetrule_ctx = self._get_augmented_data(data)
    self.assertEqual(jetrule_ctx.ERROR, False)
    postprocessed_data = jetrule_ctx.jetRules

    # validate that empty rule file is ok
    expected = """{"literals": [], "resources": [{"type": "resource", "id": "rdf:type", "value": "rdf:type", "source_fname": "predefined"}, {"type": "resource", "id": "acme:Claim", "value": "acme:Claim"}], "lookup_tables": [], "jet_rules": [{"name": "RuleV2", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim)", "label": "(?clm01 rdf:type acme:Claim)"}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?clm02", "id": "?x2", "label": "?clm02"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "false"}], "normalizedLabel": "(?x2 is_good false)", "label": "(?clm02 is_good false)"}], "optimization": true, "salience": 100, "normalizedLabel": "[RuleV2]:(?x1 rdf:type acme:Claim) -> (?x2 is_good false);", "label": "[RuleV2]:(?clm01 rdf:type acme:Claim) -> (?clm02 is_good false);"}]}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))
    self.assertEqual(json.dumps(postprocessed_data), expected)

    # Validate variables
    validator = JetRuleValidator(jetrule_ctx)
    is_valid = validator.validateJetRule()
    self.assertEqual(is_valid, False)
    # print('*** Errors?',jetrule_ctx.errors)
    self.assertEqual(jetrule_ctx.errors[0], "Error rule RuleV2: Variable '?clm02' is not binded in this context '(?clm02 is_good false)' and must be for the rule to be valid.")
    self.assertEqual(jetrule_ctx.errors[1], "Error rule RuleV2: Identifier 'is_good' is not defined in this context '(?clm02 is_good false)', it must be defined.")
    self.assertEqual(len(jetrule_ctx.errors), 2)

  def test_validate_var5(self):
    data = """
      # =======================================================================================
      # Simplest rule that is NOT valid
      # ---------------------------------------------------------------------------------------
      resource rdf:type = "rdf:type";
      resource acme:Claim = "acme:Claim";
      [RuleV4]: 
        (?clm01 rdf:type acme:Claim).[?clm02]
        ->
        (?clm01 is_good false).
      ;
    """
    jetrule_ctx = self._get_augmented_data(data)
    self.assertEqual(jetrule_ctx.ERROR, False)
    postprocessed_data = jetrule_ctx.jetRules

    # validate that empty rule file is ok
    expected = """{"literals": [], "resources": [{"type": "resource", "id": "rdf:type", "value": "rdf:type", "source_fname": "predefined"}, {"type": "resource", "id": "acme:Claim", "value": "acme:Claim"}], "lookup_tables": [], "jet_rules": [{"name": "RuleV4", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "filter": {"type": "var", "value": "?clm02", "id": "?x2", "label": "?clm02"}, "normalizedLabel": "(?x1 rdf:type acme:Claim).[?x2]", "label": "(?clm01 rdf:type acme:Claim).[?clm02]"}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "false"}], "normalizedLabel": "(?x1 is_good false)", "label": "(?clm01 is_good false)"}], "optimization": true, "salience": 100, "normalizedLabel": "[RuleV4]:(?x1 rdf:type acme:Claim).[?x2] -> (?x1 is_good false);", "label": "[RuleV4]:(?clm01 rdf:type acme:Claim).[?clm02] -> (?clm01 is_good false);"}]}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))
    self.assertEqual(json.dumps(postprocessed_data), expected)

    # Validate variables 
    validator = JetRuleValidator(jetrule_ctx)
    is_valid = validator.validateJetRule()
    self.assertEqual(is_valid, False)
    self.assertEqual(jetrule_ctx.ERROR, True)
    # print('*** Errors?',jetrule_ctx.errors)
    self.assertEqual(jetrule_ctx.errors[0], "Error rule RuleV4: Variable '?clm02' is not binded in this context '(?clm01 rdf:type acme:Claim).[?clm02]' and must be for the rule to be valid.")
    self.assertEqual(jetrule_ctx.errors[1], "Error rule RuleV4: Identifier 'is_good' is not defined in this context '(?clm01 is_good false)', it must be defined.")
    self.assertEqual(len(jetrule_ctx.errors), 2)


  def test_validate_lookup1(self):
    data = """
      # Testing name mapping
      lookup_table MSK_DRG_TRIGGER {
        $table_name = acme__msk_trigger_drg_codes,         # main table
        $key = ["DRG", "DRG2"],                            # composite Lookup key

        # Using column names that need fixing to become resource name
        $columns = ["MSK (9)" as text, "$TAG(3)" as text, "TRIGGER+" as text, "DRG" as text, "123" as text, "#%%" as text, "#%#" as text]
      };
    """
    jetrule_ctx = self._get_augmented_data(data)
    postprocessed_data = jetrule_ctx.jetRules

    # Error on generate resources
    self.assertEqual(jetrule_ctx.ERROR, False)

    # Validate the output
    expected = """{"literals": [], "resources": [{"id": "MSK_DRG_TRIGGER", "type": "resource", "value": "MSK_DRG_TRIGGER"}, {"id": "DRG", "type": "resource", "value": "DRG"}], "lookup_tables": [{"type": "lookup", "name": "MSK_DRG_TRIGGER", "key": ["DRG", "DRG2"], "columns": [{"name": "MSK (9)", "type": "text", "as_array": "false"}, {"name": "$TAG(3)", "type": "text", "as_array": "false"}, {"name": "TRIGGER+", "type": "text", "as_array": "false"}, {"name": "DRG", "type": "text", "as_array": "false"}, {"name": "123", "type": "text", "as_array": "false"}, {"name": "#%%", "type": "text", "as_array": "false"}, {"name": "#%#", "type": "text", "as_array": "false"}], "table": "acme__msk_trigger_drg_codes", "resources": ["DRG"]}], "jet_rules": []}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))
    self.assertEqual(json.dumps(postprocessed_data), expected)


  def test_validate_keyword1(self):
    data = """
      # =======================================================================================
      # Simplest rule that is NOT valid
      # ---------------------------------------------------------------------------------------
      resource acme:Claim = "acme:Claim";
      [RuleV5]: 
        (true ?clm01 acme:Claim)
        ->
        (?clm01 false false).
      ;
    """
    jetrule_ctx = self._get_augmented_data(data)
    self.assertEqual(jetrule_ctx.ERROR, True)

    # print('GOT')
    # for k in jetrule_ctx.errors:
    #   print(k)
    # print()
    # print()

    self.assertEqual(jetrule_ctx.errors[0], "line 7:9 extraneous input 'true' expecting {'?', Identifier}")
    self.assertEqual(jetrule_ctx.errors[1], "line 7:31 mismatched input ')' expecting {'?', 'int', 'uint', 'long', 'ulong', 'double', 'text', 'date', 'datetime', 'bool', 'true', 'false', 'null', '+', '-', Identifier, DIGITS, STRING}")
    self.assertEqual(jetrule_ctx.errors[2], "line 9:16 mismatched input 'false' expecting {'?', Identifier}")
    self.assertEqual(jetrule_ctx.errors[3], "line 9:22 extraneous input 'false' expecting ')'")
    self.assertEqual(len(jetrule_ctx.errors), 4)

if __name__ == '__main__':
  absltest.main()
