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
    jetrule_ctx =  JetRuleContext(jetRulesSpec, [])
    compiler.postprocessJetRule(jetrule_ctx)
    return jetrule_ctx.jetRules

  def _get_augmented_data(self, data: io.StringIO) -> JetRuleContext:
    jetrule_ctx =  compiler.processJetRule(data)
    return compiler.postprocessJetRule(jetrule_ctx)

  def test_import1(self):
    postprocessed_data = self._get_from_file("import_test1.jr")

    # validate the whole result
    expected = """{"literals": [{"type": "int", "id": "isTrue", "value": "1"}, {"type": "text", "id": "NOT_IN_CONTRACT", "value": "NOT COVERED IN CONTRACT"}, {"type": "text", "id": "EXCLUDED_STATE", "value": "STATE"}], "resources": [{"id": "acme:ProcedureLookup", "type": "resource", "value": "acme:ProcedureLookup"}, {"id": "cPROC_RID", "type": "resource", "value": "PROC_RID"}, {"id": "cPROC_MID", "type": "resource", "value": "PROC_MID"}, {"id": "cPROC_DESC", "type": "resource", "value": "PROC_DESC"}], "lookup_tables": [{"name": "acme:ProcedureLookup", "table": "acme__cm_proc_codes", "key": ["PROC_CODE"], "columns": ["PROC_RID", "PROC_MID", "PROC_DESC"], "resources": ["cPROC_RID", "cPROC_MID", "cPROC_DESC"]}], "jet_rules": []}"""
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
    data.close()
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
      resource acme:Claim = "acme:Claim";
      volatile_resource is_good = "is_good";
      [RuleV1]: 
        (?clm01 rdf:type acme:Claim).
        ->
        (?clm01 is_good true).
      ;
    """)
    jetrule_ctx = self._get_augmented_data(data)
    data.close()
    postprocessed_data = jetrule_ctx.jetRules

    # validate that empty rule file is ok
    expected = """{"literals": [], "resources": [{"type": "resource", "id": "rdf:type", "value": "rdf:type"}, {"type": "resource", "id": "acme:Claim", "value": "acme:Claim"}, {"type": "volatile_resource", "id": "is_good", "value": "is_good"}], "lookup_tables": [], "jet_rules": [{"name": "RuleV1", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim)", "label": "(?clm01 rdf:type acme:Claim)"}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x1 is_good true)", "label": "(?clm01 is_good true)"}], "normalizedLabel": "[RuleV1]:(?x1 rdf:type acme:Claim) -> (?x1 is_good true);", "label": "[RuleV1]:(?clm01 rdf:type acme:Claim) -> (?clm01 is_good true);"}]}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))

    # validate that empty rule file is ok
    self.assertEqual(json.dumps(postprocessed_data), expected)

    # Validate variables
    is_valid = compiler.validateJetRule(jetrule_ctx, False)
    self.assertEqual(is_valid, True)
    self.assertEqual(len(jetrule_ctx.errors), 0)

  def test_validate_var2b(self):
    data = io.StringIO("""
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
    """)
    jetrule_ctx = self._get_augmented_data(data)
    data.close()
    postprocessed_data = jetrule_ctx.jetRules

    # validate that empty rule file is ok
    expected = """{"literals": [], "resources": [{"type": "resource", "id": "rdf:type", "value": "rdf:type"}, {"type": "resource", "id": "acme:Claim", "value": "acme:Claim"}, {"type": "volatile_resource", "id": "is_good", "value": "is_good"}], "lookup_tables": [], "jet_rules": [{"name": "RuleV1", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim)", "label": "(?clm01 rdf:type acme:Claim)"}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x1 is_good true)", "label": "(?clm01 is_good true)"}], "normalizedLabel": "[RuleV1]:(?x1 rdf:type acme:Claim) -> (?x1 is_good true);", "label": "[RuleV1]:(?clm01 rdf:type acme:Claim) -> (?clm01 is_good true);"}]}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))

    # validate that empty rule file is ok
    self.assertEqual(json.dumps(postprocessed_data), expected)

    # Validate variables
    is_valid = compiler.validateJetRule(jetrule_ctx, True)
    self.assertEqual(is_valid, True)
    self.assertEqual(len(jetrule_ctx.errors), 0)

  def test_validate_var3(self):
    data = io.StringIO("""
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
    """)
    jetrule_ctx = self._get_augmented_data(data)
    data.close()
    postprocessed_data = jetrule_ctx.jetRules

    # validate that empty rule file is ok
    expected = """{"literals": [], "resources": [{"type": "resource", "id": "rdf:type", "value": "rdf:type"}, {"type": "resource", "id": "acme:Claim", "value": "acme:Claim"}], "lookup_tables": [], "jet_rules": [{"name": "RuleV2", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim)", "label": "(?clm01 rdf:type acme:Claim)"}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "id": "?x2", "label": "?clm02"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "false"}], "normalizedLabel": "(?x2 is_good false)", "label": "(?clm02 is_good false)"}], "normalizedLabel": "[RuleV2]:(?x1 rdf:type acme:Claim) -> (?x2 is_good false);", "label": "[RuleV2]:(?clm01 rdf:type acme:Claim) -> (?clm02 is_good false);"}]}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))

    # validate that empty rule file is ok
    self.assertEqual(json.dumps(postprocessed_data), expected)

    # Validate variables
    is_valid = compiler.validateJetRule(jetrule_ctx, False)
    self.assertEqual(is_valid, False)
    # print('*** Errors?',jetrule_ctx.errors)
    self.assertEqual(jetrule_ctx.errors[0], "Error rule RuleV2: Variable '?clm02' is not binded in this context '(?clm02 is_good false)' and must be for the rule to be valid.")
    self.assertEqual(jetrule_ctx.errors[1], "Error rule RuleV2: Identifier 'is_good' is not defined in this context '(?clm02 is_good false)', it must be define.")
    self.assertEqual(len(jetrule_ctx.errors), 2)

  def test_validate_var4(self):
    data = io.StringIO("""
      # =======================================================================================
      # Simplest rule that is NOT valid
      # ---------------------------------------------------------------------------------------
      resource rdf:type = "rdf:type";
      resource acme:Claim = "acme:Claim";
      [RuleV3]: 
        (?clm01 rdf:type acme:Claim).
        ->
        (?clm02 is_good false).
      ;
    """)
    jetrule_ctx = self._get_augmented_data(data)
    data.close()
    postprocessed_data = jetrule_ctx.jetRules

    # validate that empty rule file is ok
    expected = """{"literals": [], "resources": [{"type": "resource", "id": "rdf:type", "value": "rdf:type"}, {"type": "resource", "id": "acme:Claim", "value": "acme:Claim"}], "lookup_tables": [], "jet_rules": [{"name": "RuleV3", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim)", "label": "(?clm01 rdf:type acme:Claim)"}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "id": "?x2", "label": "?clm02"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "false"}], "normalizedLabel": "(?x2 is_good false)", "label": "(?clm02 is_good false)"}], "normalizedLabel": "[RuleV3]:(?x1 rdf:type acme:Claim) -> (?x2 is_good false);", "label": "[RuleV3]:(?clm01 rdf:type acme:Claim) -> (?clm02 is_good false);"}]}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))

    # validate that empty rule file is ok
    self.assertEqual(json.dumps(postprocessed_data), expected)

    # Validate variables in preflight mode
    is_valid = compiler.validateJetRule(jetrule_ctx, True)
    self.assertEqual(is_valid, False)           # Indicates that it's not valid but in preflight mode
    # print('*** Errors?',jetrule_ctx.errors)   # in preflight mode, errors are not reported in ctx obj
    self.assertEqual(len(jetrule_ctx.errors), 0)

  def test_validate_var5(self):
    data = io.StringIO("""
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
    """)
    jetrule_ctx = self._get_augmented_data(data)
    data.close()
    postprocessed_data = jetrule_ctx.jetRules

    # validate that empty rule file is ok
    expected = """{"literals": [], "resources": [{"type": "resource", "id": "rdf:type", "value": "rdf:type"}, {"type": "resource", "id": "acme:Claim", "value": "acme:Claim"}], "lookup_tables": [], "jet_rules": [{"name": "RuleV4", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "filter": {"type": "var", "id": "?x2", "label": "?clm02"}, "normalizedLabel": "(?x1 rdf:type acme:Claim).[?x2]", "label": "(?clm01 rdf:type acme:Claim).[?clm02]"}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "is_good"}, {"type": "keyword", "value": "false"}], "normalizedLabel": "(?x1 is_good false)", "label": "(?clm01 is_good false)"}], "normalizedLabel": "[RuleV4]:(?x1 rdf:type acme:Claim).[?x2] -> (?x1 is_good false);", "label": "[RuleV4]:(?clm01 rdf:type acme:Claim).[?clm02] -> (?clm01 is_good false);"}]}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))

    # validate that rule is as expected
    self.assertEqual(json.dumps(postprocessed_data), expected)

    # Validate variables 
    is_valid = compiler.validateJetRule(jetrule_ctx, False)
    self.assertEqual(is_valid, False)
    # print('*** Errors?',jetrule_ctx.errors)
    self.assertEqual(jetrule_ctx.errors[0], "Error rule RuleV4: Variable '?clm02' is not binded in this context '(?clm01 rdf:type acme:Claim).[?clm02]' and must be for the rule to be valid.")
    self.assertEqual(jetrule_ctx.errors[1], "Error rule RuleV4: Identifier 'is_good' is not defined in this context '(?clm01 is_good false)', it must be define.")
    self.assertEqual(len(jetrule_ctx.errors), 2)

  def test_validate_keyword1(self):
    data = io.StringIO("""
      # =======================================================================================
      # Simplest rule that is NOT valid
      # ---------------------------------------------------------------------------------------
      resource acme:Claim = "acme:Claim";
      [RuleV5]: 
        (true ?clm01 acme:Claim)
        ->
        (?clm01 false false).
      ;
    """)
    jetrule_ctx = self._get_augmented_data(data)
    data.close()

    self.assertEqual(jetrule_ctx.ERROR, True)

    # print('GOT')
    # for k in jetrule_ctx.errors:
    #   print(k)
    # print()

    self.assertEqual(jetrule_ctx.errors[0], "line 7:9 extraneous input 'true' expecting {'?', Identifier}")
    self.assertEqual(jetrule_ctx.errors[1], "line 7:31 mismatched input ')' expecting {'?', 'int', 'uint', 'long', 'ulong', 'double', 'text', 'true', 'false', Identifier, String}")
    self.assertEqual(jetrule_ctx.errors[2], "line 9:16 mismatched input 'false' expecting {'?', Identifier}")
    self.assertEqual(jetrule_ctx.errors[3], "line 9:22 extraneous input 'false' expecting ')'")
    self.assertEqual(len(jetrule_ctx.errors), 4)

if __name__ == '__main__':
  absltest.main()
