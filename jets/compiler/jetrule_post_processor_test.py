"""JetListenerPostProcessor tests"""

import json
from typing import Dict
from absl import flags
from absl.testing import absltest
import io

from jetrule_compiler import JetRuleCompiler
from jetrule_context import JetRuleContext

FLAGS = flags.FLAGS

class JetRulesPostProcessorTest(absltest.TestCase):

  def _get_augmented_data(self, input_data: str) -> Dict[str, object]:
    compiler = JetRuleCompiler()
    compiler.processJetRule(input_data)
    jetRules = compiler.postprocessJetRule().jetRules
    # not expecting any errors here
    self.assertEqual(compiler.jetrule_ctx.ERROR, False)
    return jetRules

  def _process_data(self, input_data: str) -> JetRuleContext:
    compiler = JetRuleCompiler()
    compiler.processJetRule(input_data)
    compiler.postprocessJetRule()
    return compiler.jetrule_ctx

  def test_lookup_table1(self):
    data = """
      # =======================================================================================
      # Defining Lookup Tables
      # ---------------------------------------------------------------------------------------
      # lookup example based on USI: *include-lookup* "CM/Procedure CM.trd"
      # Note: Legacy trd lookup table will have to be converted to csv
      # Assuming here the csv would have these columns: "PROC_CODE, PROC_RID, PROC_MID, PROC_DESC"
      lookup_table acme:ProcedureLookup {
        $table_name = acme__cm_proc_codes,       # Table name where the data reside (loaded from trd file)
        $key = ["PROC_CODE"],                   # Key columns, resource PROC_CODE automatically created

        # Value columns, corresponding resource automatically created
        $columns = ["PROC_RID" as text, "PROC_MID" as text, "PROC_DESC" as text]
      };
    """
    postprocessed_data = self._get_augmented_data(data)

    # validate the whole result
    expected = """{"literals": [], "resources": [{"id": "acme:ProcedureLookup", "type": "resource", "value": "acme:ProcedureLookup"}, {"id": "PROC_RID", "type": "resource", "value": "PROC_RID"}, {"id": "PROC_MID", "type": "resource", "value": "PROC_MID"}, {"id": "PROC_DESC", "type": "resource", "value": "PROC_DESC"}], "lookup_tables": [{"type": "lookup", "name": "acme:ProcedureLookup", "key": ["PROC_CODE"], "columns": [{"name": "PROC_RID", "type": "text", "as_array": "false"}, {"name": "PROC_MID", "type": "text", "as_array": "false"}, {"name": "PROC_DESC", "type": "text", "as_array": "false"}], "table": "acme__cm_proc_codes", "resources": ["PROC_RID", "PROC_MID", "PROC_DESC"]}], "jet_rules": []}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))
    self.assertEqual(json.dumps(postprocessed_data), expected)

  def test_lookup_table2(self):
    data = """
      lookup_table MSK_DRG_TRIGGER {
        $table_name = acme__msk_trigger_drg_codes,         # main table
        $key = ["DRG"],                                   # Lookup key

        # Value columns, corresponding resource automatically created
        # Data type based on columns type
        $columns = ["MSK_AREA_DRG_TRIGGER_ONLY" as text, "MSK_TAG" as text, "TRIGGER_TAG_DRG_ONLY" as text, "DRG" as text, "OVERLAP" as text, "USE_ANESTHESIA" as text]
      };
    """
    postprocessed_data = self._get_augmented_data(data)

    # validate the whole result
    expected = """{"literals": [], "resources": [{"id": "MSK_DRG_TRIGGER", "type": "resource", "value": "MSK_DRG_TRIGGER"}, {"id": "MSK_AREA_DRG_TRIGGER_ONLY", "type": "resource", "value": "MSK_AREA_DRG_TRIGGER_ONLY"}, {"id": "MSK_TAG", "type": "resource", "value": "MSK_TAG"}, {"id": "TRIGGER_TAG_DRG_ONLY", "type": "resource", "value": "TRIGGER_TAG_DRG_ONLY"}, {"id": "DRG", "type": "resource", "value": "DRG"}, {"id": "OVERLAP", "type": "resource", "value": "OVERLAP"}, {"id": "USE_ANESTHESIA", "type": "resource", "value": "USE_ANESTHESIA"}], "lookup_tables": [{"type": "lookup", "name": "MSK_DRG_TRIGGER", "key": ["DRG"], "columns": [{"name": "MSK_AREA_DRG_TRIGGER_ONLY", "type": "text", "as_array": "false"}, {"name": "MSK_TAG", "type": "text", "as_array": "false"}, {"name": "TRIGGER_TAG_DRG_ONLY", "type": "text", "as_array": "false"}, {"name": "DRG", "type": "text", "as_array": "false"}, {"name": "OVERLAP", "type": "text", "as_array": "false"}, {"name": "USE_ANESTHESIA", "type": "text", "as_array": "false"}], "table": "acme__msk_trigger_drg_codes", "resources": ["MSK_AREA_DRG_TRIGGER_ONLY", "MSK_TAG", "TRIGGER_TAG_DRG_ONLY", "DRG", "OVERLAP", "USE_ANESTHESIA"]}], "jet_rules": []}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))
    self.assertEqual(json.dumps(postprocessed_data), expected)

  def test_lookup_table3(self):
    data = """
      # Testing name mapping
      lookup_table MSK_DRG_TRIGGER {
        $table_name = acme__msk_trigger_drg_codes,         # main table
        $key = ["DRG", "DRG2"],                               # composite Lookup key

        # Using column names that need fixing to become resource name
        $columns = ["MSK (9)" as text, "$TAG(3)" as text, "TRIGGER+" as text, "DRG" as text, "123" as text, "#%%" as text]
      };
    """
    postprocessed_data = self._get_augmented_data(data)

    # validate the whole result
    expected = """{"literals": [], "resources": [{"id": "MSK_DRG_TRIGGER", "type": "resource", "value": "MSK_DRG_TRIGGER"}, {"id": "DRG", "type": "resource", "value": "DRG"}], "lookup_tables": [{"type": "lookup", "name": "MSK_DRG_TRIGGER", "key": ["DRG", "DRG2"], "columns": [{"name": "MSK (9)", "type": "text", "as_array": "false"}, {"name": "$TAG(3)", "type": "text", "as_array": "false"}, {"name": "TRIGGER+", "type": "text", "as_array": "false"}, {"name": "DRG", "type": "text", "as_array": "false"}, {"name": "123", "type": "text", "as_array": "false"}, {"name": "#%%", "type": "text", "as_array": "false"}], "table": "acme__msk_trigger_drg_codes", "resources": ["DRG"]}], "jet_rules": []}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))
    self.assertEqual(json.dumps(postprocessed_data), expected)

  def test_jetrule1(self):
    data = """
      # =======================================================================================
      # Defining Jet Rules
      # ---------------------------------------------------------------------------------------
      # property s: salience, o: optimization, tag: label
      # optimization is true by default
      [Rule1, s=+100, o=true]: 
        (?clm01 rdf:type acme:Claim).
        not(?clm01 acme:hasDRG ?drg).[(?clm01 + ?drg) + int(1) ]
        ->
        (?clm01 rdf:type acme:SpecialClaim).
        (?clm01 xyz ?drg)
      ;
    """
    postprocessed_data = self._get_augmented_data(data)
    rule_label = postprocessed_data['jet_rules'][0]['label']

    # reprocess the rule_label to ensure to get the same result
    postprocessed_data = self._get_augmented_data(rule_label)
    self.assertEqual(rule_label, postprocessed_data['jet_rules'][0]['label'])

    # validate the whole result
    expected = """{"literals": [], "resources": [], "lookup_tables": [], "jet_rules": [{"name": "Rule1", "properties": {"s": "+100", "o": "true"}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim)", "label": "(?clm01 rdf:type acme:Claim)"}, {"type": "antecedent", "isNot": true, "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "acme:hasDRG"}, {"type": "var", "value": "?drg", "id": "?x2", "label": "?drg"}], "filter": {"type": "binary", "lhs": {"type": "binary", "lhs": {"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, "op": "+", "rhs": {"type": "var", "value": "?drg", "id": "?x2", "label": "?drg"}}, "op": "+", "rhs": {"type": "int", "value": "1"}}, "normalizedLabel": "not(?x1 acme:hasDRG ?x2).[(?x1 + ?x2) + int(1)]", "label": "not(?clm01 acme:hasDRG ?drg).[(?clm01 + ?drg) + int(1)]"}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:SpecialClaim"}], "normalizedLabel": "(?x1 rdf:type acme:SpecialClaim)", "label": "(?clm01 rdf:type acme:SpecialClaim)"}, {"type": "consequent", "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "xyz"}, {"type": "var", "value": "?drg", "id": "?x2", "label": "?drg"}], "normalizedLabel": "(?x1 xyz ?x2)", "label": "(?clm01 xyz ?drg)"}], "optimization": true, "salience": 100, "normalizedLabel": "[Rule1, s=+100, o=true]:(?x1 rdf:type acme:Claim).not(?x1 acme:hasDRG ?x2).[(?x1 + ?x2) + int(1)] -> (?x1 rdf:type acme:SpecialClaim).(?x1 xyz ?x2);", "label": "[Rule1, s=+100, o=true]:(?clm01 rdf:type acme:Claim).not(?clm01 acme:hasDRG ?drg).[(?clm01 + ?drg) + int(1)] -> (?clm01 rdf:type acme:SpecialClaim).(?clm01 xyz ?drg);"}]}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))
    self.assertEqual(json.dumps(postprocessed_data), expected)

  def test_jetrule2(self):
    data = """
      [Rule2, s=100, o=true]: 
        (?clm01 rdf:type acme:Claim).
        not(?clm01 acme:hasDRG ?drg).[true and false]
        ->
        (?clm01 rdf:type acme:SpecialClaim)
      ;
    """
    postprocessed_data = self._get_augmented_data(data)
    rule_label = postprocessed_data['jet_rules'][0]['label']

    # reprocess the rule_label to ensure to get the same result
    postprocessed_data = self._get_augmented_data(rule_label)
    self.assertEqual(rule_label, postprocessed_data['jet_rules'][0]['label'])

    # validate the whole result
    expected = """{"literals": [], "resources": [], "lookup_tables": [], "jet_rules": [{"name": "Rule2", "properties": {"s": "100", "o": "true"}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "normalizedLabel": "(?x1 rdf:type acme:Claim)", "label": "(?clm01 rdf:type acme:Claim)"}, {"type": "antecedent", "isNot": true, "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "acme:hasDRG"}, {"type": "var", "value": "?drg", "id": "?x2", "label": "?drg"}], "filter": {"type": "binary", "lhs": {"type": "keyword", "value": "true"}, "op": "and", "rhs": {"type": "keyword", "value": "false"}}, "normalizedLabel": "not(?x1 acme:hasDRG ?x2).[true and false]", "label": "not(?clm01 acme:hasDRG ?drg).[true and false]"}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:SpecialClaim"}], "normalizedLabel": "(?x1 rdf:type acme:SpecialClaim)", "label": "(?clm01 rdf:type acme:SpecialClaim)"}], "optimization": true, "salience": 100, "normalizedLabel": "[Rule2, s=100, o=true]:(?x1 rdf:type acme:Claim).not(?x1 acme:hasDRG ?x2).[true and false] -> (?x1 rdf:type acme:SpecialClaim);", "label": "[Rule2, s=100, o=true]:(?clm01 rdf:type acme:Claim).not(?clm01 acme:hasDRG ?drg).[true and false] -> (?clm01 rdf:type acme:SpecialClaim);"}]}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))
    self.assertEqual(json.dumps(postprocessed_data), expected)

  def test_jetrule3(self):
    data = """
      [Rule3]: 
        (?clm01 rdf:type acme:Claim).[(?a1 + b1) * (?a2 + b2)].
        (?clm01 rdf:type acme:Claim).[(?a1 or b1) and ?a2].
        ->
        (?clm01 rdf:type acme:SpecialClaim).
        (?clm02 rdf:type acme:SpecialClaim)
      ;
    """
    postprocessed_data = self._get_augmented_data(data)
    rule_label = postprocessed_data['jet_rules'][0]['label']

    # reprocess the rule_label to ensure to get the same result
    postprocessed_data = self._get_augmented_data(rule_label)
    self.assertEqual(rule_label, postprocessed_data['jet_rules'][0]['label'])

    # validate the whole result
    expected = """{"literals": [], "resources": [], "lookup_tables": [], "jet_rules": [{"name": "Rule3", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "filter": {"type": "binary", "lhs": {"type": "binary", "lhs": {"type": "var", "value": "?a1", "id": "?x2", "label": "?a1"}, "op": "+", "rhs": {"type": "identifier", "value": "b1"}}, "op": "*", "rhs": {"type": "binary", "lhs": {"type": "var", "value": "?a2", "id": "?x3", "label": "?a2"}, "op": "+", "rhs": {"type": "identifier", "value": "b2"}}}, "normalizedLabel": "(?x1 rdf:type acme:Claim).[(?x2 + b1) * (?x3 + b2)]", "label": "(?clm01 rdf:type acme:Claim).[(?a1 + b1) * (?a2 + b2)]"}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "filter": {"type": "binary", "lhs": {"type": "binary", "lhs": {"type": "var", "value": "?a1", "id": "?x2", "label": "?a1"}, "op": "or", "rhs": {"type": "identifier", "value": "b1"}}, "op": "and", "rhs": {"type": "var", "value": "?a2", "id": "?x3", "label": "?a2"}}, "normalizedLabel": "(?x1 rdf:type acme:Claim).[(?x2 or b1) and ?x3]", "label": "(?clm01 rdf:type acme:Claim).[(?a1 or b1) and ?a2]"}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:SpecialClaim"}], "normalizedLabel": "(?x1 rdf:type acme:SpecialClaim)", "label": "(?clm01 rdf:type acme:SpecialClaim)"}, {"type": "consequent", "triple": [{"type": "var", "value": "?clm02", "id": "?x4", "label": "?clm02"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:SpecialClaim"}], "normalizedLabel": "(?x4 rdf:type acme:SpecialClaim)", "label": "(?clm02 rdf:type acme:SpecialClaim)"}], "optimization": true, "salience": 100, "normalizedLabel": "[Rule3]:(?x1 rdf:type acme:Claim).[(?x2 + b1) * (?x3 + b2)].(?x1 rdf:type acme:Claim).[(?x2 or b1) and ?x3] -> (?x1 rdf:type acme:SpecialClaim).(?x4 rdf:type acme:SpecialClaim);", "label": "[Rule3]:(?clm01 rdf:type acme:Claim).[(?a1 + b1) * (?a2 + b2)].(?clm01 rdf:type acme:Claim).[(?a1 or b1) and ?a2] -> (?clm01 rdf:type acme:SpecialClaim).(?clm02 rdf:type acme:SpecialClaim);"}]}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))
    self.assertEqual(json.dumps(postprocessed_data), expected)

  def test_jetrule4(self):
    data = """
      [Rule4]: 
        (?clm01 has_code ?code).[not(?a1 or b1) and (not ?a2)]
        ->
        (?clm01 value (?a1 + ?b2)).
        (?clm01 value2 ?a1 + ?b2).
        (?clm01 value2 (not ?b2))
      ;
    """
    postprocessed_data = self._get_augmented_data(data)
    rule_label = postprocessed_data['jet_rules'][0]['label']

    # reprocess the rule_label to ensure to get the same result
    postprocessed_data = self._get_augmented_data(rule_label)
    self.assertEqual(rule_label, postprocessed_data['jet_rules'][0]['label'])

    # validate the whole result
    expected = """{"literals": [], "resources": [], "lookup_tables": [], "jet_rules": [{"name": "Rule4", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "has_code"}, {"type": "var", "value": "?code", "id": "?x2", "label": "?code"}], "filter": {"type": "binary", "lhs": {"type": "unary", "op": "not", "arg": {"type": "binary", "lhs": {"type": "var", "value": "?a1", "id": "?x3", "label": "?a1"}, "op": "or", "rhs": {"type": "identifier", "value": "b1"}}}, "op": "and", "rhs": {"type": "unary", "op": "not", "arg": {"type": "var", "value": "?a2", "id": "?x4", "label": "?a2"}}}, "normalizedLabel": "(?x1 has_code ?x2).[(not (?x3 or b1)) and (not ?x4)]", "label": "(?clm01 has_code ?code).[(not (?a1 or b1)) and (not ?a2)]"}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "value"}, {"type": "binary", "lhs": {"type": "var", "value": "?a1", "id": "?x3", "label": "?a1"}, "op": "+", "rhs": {"type": "var", "value": "?b2", "id": "?x5", "label": "?b2"}}], "normalizedLabel": "(?x1 value ?x3 + ?x5)", "label": "(?clm01 value ?a1 + ?b2)"}, {"type": "consequent", "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "value2"}, {"type": "binary", "lhs": {"type": "var", "value": "?a1", "id": "?x3", "label": "?a1"}, "op": "+", "rhs": {"type": "var", "value": "?b2", "id": "?x5", "label": "?b2"}}], "normalizedLabel": "(?x1 value2 ?x3 + ?x5)", "label": "(?clm01 value2 ?a1 + ?b2)"}, {"type": "consequent", "triple": [{"type": "var", "value": "?clm01", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "value2"}, {"type": "unary", "op": "not", "arg": {"type": "var", "value": "?b2", "id": "?x5", "label": "?b2"}}], "normalizedLabel": "(?x1 value2 not ?x5)", "label": "(?clm01 value2 not ?b2)"}], "optimization": true, "salience": 100, "normalizedLabel": "[Rule4]:(?x1 has_code ?x2).[(not (?x3 or b1)) and (not ?x4)] -> (?x1 value ?x3 + ?x5).(?x1 value2 ?x3 + ?x5).(?x1 value2 not ?x5);", "label": "[Rule4]:(?clm01 has_code ?code).[(not (?a1 or b1)) and (not ?a2)] -> (?clm01 value ?a1 + ?b2).(?clm01 value2 ?a1 + ?b2).(?clm01 value2 not ?b2);"}]}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))
    self.assertEqual(json.dumps(postprocessed_data), expected)

  def test_jetrule5(self):
    data = """
      [Rule5]: 
        (?clm01 has_code ?code).
        ->
        (?clm01 acme:"lookup_table" true)
      ;
    """
    postprocessed_data = self._get_augmented_data(data)
    rule_label = postprocessed_data['jet_rules'][0]['label']

    # reprocess the rule_label to ensure to get the same result
    postprocessed_data = self._get_augmented_data(rule_label)
    self.assertEqual(rule_label, postprocessed_data['jet_rules'][0]['label'])

    # validate the whole result
    with open('test_data/rule5_test.json', 'rt', encoding='utf-8') as f:
      expected = json.loads(f.read())
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print('GOT EXPECTED:',json.dumps(expected, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))
    self.assertEqual(json.dumps(postprocessed_data), json.dumps(expected))

  def test_jetrule6(self):
    data = """
      [Rule6]: 
        (?clm01 has_code r1).
        (?clm01 has_str r2).
        ->
        (?clm01 acme:lookupTbl "valueX").
        (?clm01 acme:market "MERGED \\"MARKET\\" CHARGE BACK").
        (?clm01 acme:market text("MERGED \\"MARKET\\" CHARGE BACK"))

      ;
    """
    postprocessed_data = self._get_augmented_data(data)
    rule_label = postprocessed_data['jet_rules'][0]['label']

    # reprocess the rule_label to ensure to get the same result
    postprocessed_data = self._get_augmented_data(rule_label)
    self.assertEqual(rule_label, postprocessed_data['jet_rules'][0]['label'])

    # validate the whole result
    with open('test_data/rule6_test.json', 'rt', encoding='utf-8') as f:
      expected = json.loads(f.read())
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))
    self.assertEqual(json.dumps(postprocessed_data), json.dumps(expected))

  def test_jetrule7(self):
    data = """
      [Rule7]: 
        (?clm01 has_code int(1)).
        (?clm01 has_str "value").
        (?clm01 hasTrue true).
        ->
        (?clm01 acme:lookupTbl true).
        (?clm01 has_literal int(1)).
        (?clm01 has_expr (int(1) + long(4)))
      ;
    """
    postprocessed_data = self._get_augmented_data(data)
    rule_label = postprocessed_data['jet_rules'][0]['label']

    # reprocess the rule_label to ensure to get the same result
    postprocessed_data = self._get_augmented_data(rule_label)
    self.assertEqual(rule_label, postprocessed_data['jet_rules'][0]['label'])

    # validate the whole result
    with open('test_data/rule7_test.json', 'rt', encoding='utf-8') as f:
      expected = json.loads(f.read())
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))
    self.assertEqual(json.dumps(postprocessed_data), json.dumps(expected))

  def test_conflicting_definition1(self):
    data = """
      # Some fine resources
      resource None  = null;
      resource uid  = create_uuid_resource();
      resource uid  = null;
      volatile_resource perfectly_fine  = "perfectly_fine";
      int err_code = 999;

      # No so fine resources...
      resource None = "null";
    """
    # GOT: {
    #   "literals": [
    #     {
    #       "type": "int",
    #       "id": "err_code",
    #       "value": "999"
    #     }
    #   ],
    #   "resources": [
    #     {
    #       "type": "resource",
    #       "id": "None",
    #       "symbol": "null",
    #       "value": null
    #     },
    #     {
    #       "type": "resource",
    #       "id": "uid",
    #       "symbol": "create_uuid_resource()",
    #       "value": null
    #     },
    #     {
    #       "type": "resource",
    #       "id": "uid",
    #       "symbol": "null",
    #       "value": null
    #     },
    #     {
    #       "type": "volatile_resource",
    #       "id": "perfectly_fine",
    #       "value": "perfectly_fine"
    #     },
    #     {
    #       "type": "resource",
    #       "id": "None",
    #       "value": "null"
    #     }
    #   ],
    #   "lookup_tables": [],
    #   "jet_rules": []
    # }
    jetrule_ctx =  self._process_data(data)
    # print('GOT:',json.dumps(jetrule_ctx.jetRules, indent=2))
    self.assertEqual(jetrule_ctx.ERROR, True)
    self.assertEqual(jetrule_ctx.errors[0], 'Error: Resource with id uid is define multiple times, one is a symbol, null, the other is of different type create_uuid_resource()')
    self.assertEqual(jetrule_ctx.errors[1], 'Error: Resource with id None is define multiple times with different values: null and None')
    self.assertEqual(len(jetrule_ctx.errors), 2)
    # print('GOT')
    # for k in jetrule_ctx.errors:
    #   print(k)
    # print()

  def test_conflicting_definition2(self):
    data = """
      # Some fine resources
      resource None  = null;
      volatile_resource perfectly_fine  = "perfectly_fine";
      int err_code = 999;

      # No so fine resources...
      resource rcode = "rcode";
      resource rcode = "my-rcode";
    """

    jetrule_ctx =  self._process_data(data)
    self.assertEqual(jetrule_ctx.ERROR, True)
    # print('GOT')
    # for k in jetrule_ctx.errors:
    #   print(k)
    # print()
    self.assertEqual(jetrule_ctx.errors[0], "Error: Resource with id rcode is define multiple times with different values: my-rcode and rcode")

  def test_conflicting_definition3(self):
    data = """
      # Some fine resources
      resource None  = null;
      volatile_resource perfectly_fine  = "perfectly_fine";
      int err_code = 999;

      # No so fine resources...
      long err_code = 999;
    """

    jetrule_ctx =  self._process_data(data)
    self.assertEqual(jetrule_ctx.ERROR, True)
    # print('GOT')
    # for k in jetrule_ctx.errors:
    #   print(k)
    # print()
    self.assertEqual(jetrule_ctx.errors[0], "Error: Literal with id err_code is define multiple times with different types: long and int")

  def test_conflicting_definition4(self):
    data = """
      # Some fine resources
      resource None  = null;
      volatile_resource perfectly_fine  = "perfectly_fine";
      int err_code = 999;

      # No so fine resources...
      text NAME = "name";
      text NAME = "another_name";
    """

    jetrule_ctx =  self._process_data(data)
    self.assertEqual(jetrule_ctx.ERROR, True)
    # print('GOT')
    # for k in jetrule_ctx.errors:
    #   print(k)
    # print()
    self.assertEqual(jetrule_ctx.errors[0], "Error: Literal with id NAME is define multiple times with different values: another_name and name")

  def test_conflicting_definition5(self):
    data = """
      # Some fine resources
      resource None  = null;
      volatile_resource perfectly_fine  = "perfectly_fine";
      int err_code = 999;

      # No so fine resources...
      int all_wrong = 1;
      resource all_wrong = "all_wrong";
    """
    jetrule_ctx =  self._process_data(data)
    self.assertEqual(jetrule_ctx.ERROR, True)
    self.assertEqual(jetrule_ctx.errors[0], 'Error: Resource with id all_wrong is define multiple times with different types: resource and int')
    self.assertEqual(jetrule_ctx.errors[1], 'Error: Resource with id all_wrong is define multiple times with different values: all_wrong and 1')
    self.assertEqual(len(jetrule_ctx.errors), 2)
    # print('GOT')
    # for k in jetrule_ctx.errors:
    #   print(k)
    # print()

  def test_property_error1(self):
    data = """
      [RulePE1, s=$100]: 
        (?clm01 rdf:type acme:Claim).
        ->
        (?clm01 rdf:type acme:SpecialClaim)
      ;
    """
    jetrule_ctx =  self._process_data(data)
    # print('@@@ GOT',jetrule_ctx.jetRules)
    self.assertEqual(jetrule_ctx.ERROR, True)
    self.assertEqual(jetrule_ctx.errors[0], "line 2:18 token recognition error at: '$1'")
    self.assertEqual(len(jetrule_ctx.errors), 1)
    # print('GOT')
    # for k in jetrule_ctx.errors:
    #   print(k)
    # print()

  def test_property_error2(self):
    data = """
      [RulePE1, s="$100"]: 
        (?clm01 rdf:type acme:Claim).
        ->
        (?clm01 rdf:type acme:SpecialClaim)
      ;
    """
    jetrule_ctx =  self._process_data(data)
    # print('@@@ GOT',jetrule_ctx.jetRules)
    self.assertEqual(jetrule_ctx.ERROR, True)
    self.assertEqual(jetrule_ctx.errors[0], """Rule RulePE1: Invalid salience in rule property 's': invalid literal for int() with base 10: '"$100"'""")
    self.assertEqual(len(jetrule_ctx.errors), 1)
    # print('GOT')
    # for k in jetrule_ctx.errors:
    #   print(k)
    # print()

  def test_property_error3(self):
    data = """
      [RulePE1, s=true, o=1]: 
        (?clm01 rdf:type acme:Claim).
        ->
        (?clm01 rdf:type acme:SpecialClaim)
      ;
    """
    jetrule_ctx =  self._process_data(data)
    # print('@@@ GOT',jetrule_ctx.jetRules)
    self.assertEqual(jetrule_ctx.ERROR, True)
    self.assertEqual(jetrule_ctx.errors[0], """Rule RulePE1: Invalid salience in rule property 's': invalid literal for int() with base 10: 'true'""")
    self.assertEqual(len(jetrule_ctx.errors), 1)
    # print('GOT')
    # for k in jetrule_ctx.errors:
    #   print(k)
    # print()

  def test_class1(self):
    data = """
      # =======================================================================================
      # Jet Rules with class definition
      class hc:Claim {
        $base_classes = [owl:Thing],
        $data_properties = [
          hc:member_key as text
        ],
        $grouping_properties = [
          hc:member_key
        ]
      };
      class hc:MedicalClaim {
        $base_classes = [
          hc:Claim
        ],
        $data_properties = [
          diagnosis as array of text
        ],
        $as_table = true
      };
    """
    jetrule_ctx =  self._process_data(data)
    # print('@@@ GOT jetRules',json.dumps(jetrule_ctx.jetRules, indent=2))
    # print('@@@ GOT jetRules:')
    # print(json.dumps(jetrule_ctx.jetRules))
    expected = """{"literals": [], "resources": [{"id": "hc:Claim", "type": "resource", "value": "hc:Claim"}, {"id": "owl:Thing", "type": "resource", "value": "owl:Thing", "source_file_name": "predefined"}, {"id": "hc:member_key", "type": "resource", "value": "hc:member_key"}, {"id": "hc:MedicalClaim", "type": "resource", "value": "hc:MedicalClaim"}, {"id": "hc:Claim", "type": "resource", "value": "hc:Claim"}, {"id": "diagnosis", "type": "resource", "value": "diagnosis"}], "lookup_tables": [], "jet_rules": [{"name": "hc:Claim:1", "properties": {"i": "true"}, "source_file_name": null, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?s1", "id": "?x1", "label": "?s1"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "hc:Claim"}], "normalizedLabel": "(?x1 rdf:type hc:Claim)", "label": "(?s1 rdf:type hc:Claim)"}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?s1", "id": "?x1", "label": "?s1"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "owl:Thing"}], "normalizedLabel": "(?x1 rdf:type owl:Thing)", "label": "(?s1 rdf:type owl:Thing)"}], "optimization": true, "salience": 100, "normalizedLabel": "[hc:Claim:1, i=true]:(?x1 rdf:type hc:Claim) -> (?x1 rdf:type owl:Thing);", "label": "[hc:Claim:1, i=true]:(?s1 rdf:type hc:Claim) -> (?s1 rdf:type owl:Thing);"}, {"name": "hc:MedicalClaim:2", "properties": {"i": "true"}, "source_file_name": null, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?s1", "id": "?x1", "label": "?s1"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "hc:MedicalClaim"}], "normalizedLabel": "(?x1 rdf:type hc:MedicalClaim)", "label": "(?s1 rdf:type hc:MedicalClaim)"}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?s1", "id": "?x1", "label": "?s1"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "hc:Claim"}], "normalizedLabel": "(?x1 rdf:type hc:Claim)", "label": "(?s1 rdf:type hc:Claim)"}], "optimization": true, "salience": 100, "normalizedLabel": "[hc:MedicalClaim:2, i=true]:(?x1 rdf:type hc:MedicalClaim) -> (?x1 rdf:type hc:Claim);", "label": "[hc:MedicalClaim:2, i=true]:(?s1 rdf:type hc:MedicalClaim) -> (?s1 rdf:type hc:Claim);"}], "classes": [{"type": "class", "name": "hc:Claim", "base_classes": ["owl:Thing"], "data_properties": [{"name": "hc:member_key", "type": "text", "as_array": "false", "is_grouping": true}], "grouping_properties": ["hc:member_key"], "sub_classes": ["hc:MedicalClaim"]}, {"type": "class", "name": "hc:MedicalClaim", "base_classes": ["hc:Claim"], "data_properties": [{"name": "diagnosis", "type": "text", "as_array": "true"}], "as_table": "true", "sub_classes": []}]}"""
    self.assertEqual(jetrule_ctx.ERROR, False)
    self.assertEqual(json.dumps(jetrule_ctx.jetRules), expected)
    # print('GOT ERRORS')
    # for k in jetrule_ctx.errors:
    #   print(k)
    # print()

if __name__ == '__main__':
  absltest.main()
