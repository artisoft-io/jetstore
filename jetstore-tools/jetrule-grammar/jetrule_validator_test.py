"""JetRuleValidator tests"""

import sys
import json
from typing import Dict
from absl import flags
from absl.testing import absltest
import io

import jetrule_compiler as compiler
from jetrule_context import JetRuleContext

FLAGS = flags.FLAGS

class JetRulesValidatorTest(absltest.TestCase):

  def _get_from_file(self, fname: str) -> Dict[str, object]:
    in_provider = compiler.InputProvider('jetstore-tools/jetrule-grammar')
    jetRulesSpec =  compiler.readJetRuleFile(fname, in_provider)
    ctx = compiler.postprocessJetRule(jetRulesSpec)
    return ctx.jetRules

  def _get_augmented_data(self, data: io.StringIO) -> JetRuleContext:
    jetRulesSpec =  compiler.processJetRule(data)
    return compiler.postprocessJetRule(jetRulesSpec)

  def test_import1(self):
    postprocessed_data = self._get_from_file("import_test1.jr")

    # validate the whole result
    expected = """{"literals": [{"type": "int", "id": "isTrue", "value": "1"}, {"type": "text", "id": "NOT_IN_CONTRACT", "value": "NOT COVERED IN CONTRACT"}, {"type": "text", "id": "EXCLUDED_STATE", "value": "STATE"}], "resources": [{"id": "usi:ProcedureLookup", "type": "resource", "value": "usi:ProcedureLookup"}, {"id": "cPROC_RID", "type": "resource", "value": "PROC_RID"}, {"id": "cPROC_MID", "type": "resource", "value": "PROC_MID"}, {"id": "cPROC_DESC", "type": "resource", "value": "PROC_DESC"}], "lookup_tables": [{"name": "usi:ProcedureLookup", "table": "usi__cm_proc_codes", "key": ["PROC_CODE"], "columns": ["PROC_RID", "PROC_MID", "PROC_DESC"], "resources": ["cPROC_RID", "cPROC_MID", "cPROC_DESC"]}], "jet_rules": []}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))

    # validate the whole result
    self.assertEqual(json.dumps(postprocessed_data), expected)

  def test_validate_var1(self):
    data = io.StringIO("""
      # =======================================================================================
      # Defining Constants Resources and Literals
      # ---------------------------------------------------------------------------------------
      # That should not create any error since no rules are declared
    """)
    jetrule_ctx = self._get_augmented_data(data)
    postprocessed_data = jetrule_ctx.jetRules

    # validate that empty rule file is ok
    expected = """{"literals": [], "resources": [], "lookup_tables": [], "jet_rules": []}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))

    # validate that empty rule file is ok
    self.assertEqual(json.dumps(postprocessed_data), expected)

    # Validate variables
    is_valid = compiler.validateJetRule(jetrule_ctx, False)
    self.assertEqual(is_valid, True)
    self.assertEqual(len(jetrule_ctx.errors), 0)

  def test_validate_var2(self):
    data = io.StringIO("""
      # =======================================================================================
      # Simplest rule that is valid
      # ---------------------------------------------------------------------------------------
      resource rdf:type = "rdf:type";
      resource usi:Claim = "usi:Claim";
      volatile_resource is_good = "is_good";
      [Rule1]: 
        (?clm01 rdf:type usi:Claim).
        ->
        (?clm01 is_good true).
      ;
    """)
    jetrule_ctx = self._get_augmented_data(data)
    postprocessed_data = jetrule_ctx.jetRules

    # validate that empty rule file is ok
    expected = """{"literals": [], "resources": [{"type": "resource", "id": "rdf:type", "value": "rdf:type"}, {"type": "resource", "id": "usi:Claim", "value": "usi:Claim"}, {"type": "volatile_resource", "id": "is_good", "value": "is_good"}], "lookup_tables": [], "jet_rules": [{"name": "Rule1", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "usi:Claim"}], "normalizedLabel": "(?x1 rdf:type usi:Claim)", "label": "(?clm01 rdf:type usi:Claim)"}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x1 is_good true)", "label": "(?clm01 is_good true)"}], "normalizedLabel": "[Rule1]:(?x1 rdf:type usi:Claim) -> (?x1 is_good true);", "label": "[Rule1]:(?clm01 rdf:type usi:Claim) -> (?clm01 is_good true);"}]}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))

    # validate that empty rule file is ok
    self.assertEqual(json.dumps(postprocessed_data), expected)

    # Validate variables
    is_valid = compiler.validateJetRule(jetrule_ctx, False)
    self.assertEqual(is_valid, True)
    self.assertEqual(len(jetrule_ctx.errors), 0)

  def test_validate_var3(self):
    data = io.StringIO("""
      # =======================================================================================
      # Simplest rule that is NOT valid
      # ---------------------------------------------------------------------------------------
      resource rdf:type = "rdf:type";
      resource usi:Claim = "usi:Claim";
      [Rule2]: 
        (?clm01 rdf:type usi:Claim).
        ->
        (?clm02 is_good false).
      ;
    """)
    jetrule_ctx = self._get_augmented_data(data)
    postprocessed_data = jetrule_ctx.jetRules

    # validate that empty rule file is ok
    expected = """{"literals": [], "resources": [{"type": "resource", "id": "rdf:type", "value": "rdf:type"}, {"type": "resource", "id": "usi:Claim", "value": "usi:Claim"}], "lookup_tables": [], "jet_rules": [{"name": "Rule2", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "usi:Claim"}], "normalizedLabel": "(?x1 rdf:type usi:Claim)", "label": "(?clm01 rdf:type usi:Claim)"}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "id": "?x2", "label": "?clm02"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "false"}], "normalizedLabel": "(?x2 is_good false)", "label": "(?clm02 is_good false)"}], "normalizedLabel": "[Rule2]:(?x1 rdf:type usi:Claim) -> (?x2 is_good false);", "label": "[Rule2]:(?clm01 rdf:type usi:Claim) -> (?clm02 is_good false);"}]}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))

    # validate that empty rule file is ok
    self.assertEqual(json.dumps(postprocessed_data), expected)

    # Validate variables
    is_valid = compiler.validateJetRule(jetrule_ctx, False)
    self.assertEqual(is_valid, False)
    # print('*** Errors?',jetrule_ctx.errors)
    self.assertEqual(jetrule_ctx.errors[0], "Error rule Rule2: Variable '?clm02' is not binded in this context '(?clm02 is_good false)' and must be for the rule to be valid.")
    self.assertEqual(len(jetrule_ctx.errors), 1)

  def test_validate_var4(self):
    data = io.StringIO("""
      # =======================================================================================
      # Simplest rule that is NOT valid
      # ---------------------------------------------------------------------------------------
      resource rdf:type = "rdf:type";
      resource usi:Claim = "usi:Claim";
      [Rule3]: 
        (?clm01 rdf:type usi:Claim).
        ->
        (?clm02 is_good false).
      ;
    """)
    jetrule_ctx = self._get_augmented_data(data)
    postprocessed_data = jetrule_ctx.jetRules

    # validate that empty rule file is ok
    expected = """{"literals": [], "resources": [{"type": "resource", "id": "rdf:type", "value": "rdf:type"}, {"type": "resource", "id": "usi:Claim", "value": "usi:Claim"}], "lookup_tables": [], "jet_rules": [{"name": "Rule3", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "usi:Claim"}], "normalizedLabel": "(?x1 rdf:type usi:Claim)", "label": "(?clm01 rdf:type usi:Claim)"}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "id": "?x2", "label": "?clm02"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "false"}], "normalizedLabel": "(?x2 is_good false)", "label": "(?clm02 is_good false)"}], "normalizedLabel": "[Rule3]:(?x1 rdf:type usi:Claim) -> (?x2 is_good false);", "label": "[Rule3]:(?clm01 rdf:type usi:Claim) -> (?clm02 is_good false);"}]}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))

    # validate that empty rule file is ok
    self.assertEqual(json.dumps(postprocessed_data), expected)

    # Validate variables in preflight mode
    is_valid = compiler.validateJetRule(jetrule_ctx, True)
    self.assertEqual(is_valid, False)
    # print('*** Errors?',jetrule_ctx.errors)
    self.assertEqual(len(jetrule_ctx.errors), 0)

  def test_validate_var5(self):
    data = io.StringIO("""
      # =======================================================================================
      # Simplest rule that is NOT valid
      # ---------------------------------------------------------------------------------------
      resource rdf:type = "rdf:type";
      resource usi:Claim = "usi:Claim";
      [Rule4]: 
        (?clm01 rdf:type usi:Claim).[?clm02]
        ->
        (?clm01 is_good false).
      ;
    """)
    jetrule_ctx = self._get_augmented_data(data)
    postprocessed_data = jetrule_ctx.jetRules

    # validate that empty rule file is ok
    expected = """{"literals": [], "resources": [{"type": "resource", "id": "rdf:type", "value": "rdf:type"}, {"type": "resource", "id": "usi:Claim", "value": "usi:Claim"}], "lookup_tables": [], "jet_rules": [{"name": "Rule4", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "usi:Claim"}], "filter": {"type": "var", "id": "?x2", "label": "?clm02"}, "normalizedLabel": "(?x1 rdf:type usi:Claim).[?x2]", "label": "(?clm01 rdf:type usi:Claim).[?clm02]"}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "false"}], "normalizedLabel": "(?x1 is_good false)", "label": "(?clm01 is_good false)"}], "normalizedLabel": "[Rule4]:(?x1 rdf:type usi:Claim).[?x2] -> (?x1 is_good false);", "label": "[Rule4]:(?clm01 rdf:type usi:Claim).[?clm02] -> (?clm01 is_good false);"}]}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))

    # validate that rule is as expected
    self.assertEqual(json.dumps(postprocessed_data), expected)

    # Validate variables 
    is_valid = compiler.validateJetRule(jetrule_ctx, False)
    self.assertEqual(is_valid, False)
    # print('*** Errors?',jetrule_ctx.errors)
    self.assertEqual(jetrule_ctx.errors[0], "Error rule Rule4: Variable '?clm02' is not binded in this context '(?clm01 rdf:type usi:Claim).[?clm02]' and must be for the rule to be valid.")
    self.assertEqual(len(jetrule_ctx.errors), 1)

if __name__ == '__main__':
  absltest.main()
