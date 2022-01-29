"""JetListenerPostProcessor tests"""

import json
from typing import Dict
from absl import flags
from absl.testing import absltest
import io

import jetrule_compiler as compiler

FLAGS = flags.FLAGS

class JetRulesPostProcessorTest(absltest.TestCase):

  def _get_augmented_data(self, data: io.StringIO) -> Dict[str, object]:
    jetRulesSpec =  compiler.processJetRule(data)
    ctx = compiler.postprocessJetRule(jetRulesSpec)
    return ctx.jetRules

  def test_lookup_table1(self):
    data = io.StringIO("""
      # =======================================================================================
      # Defining Lookup Tables
      # ---------------------------------------------------------------------------------------
      # lookup example based on USI: *include-lookup* "CM/Procedure CM.trd"
      # Note: Legacy trd lookup table will have to be converted to csv
      # Assuming here the csv would have these columns: "PROC_CODE, PROC_RID, PROC_MID, PROC_DESC"
      lookup_table usi:ProcedureLookup {
        $table_name = usi__cm_proc_codes,       # Table name where the data reside (loaded from trd file)
        $key = ["PROC_CODE"],                   # Key columns, resource PROC_CODE automatically created

        # Value columns, corresponding resource automatically created
        $columns = ["PROC_RID", "PROC_MID", "PROC_DESC"]
      };
    """)
    postprocessed_data = self._get_augmented_data(data)

    # validate the whole result
    expected = """{"literals": [], "resources": [{"id": "usi:ProcedureLookup", "type": "resource", "value": "usi:ProcedureLookup"}, {"id": "cPROC_RID", "type": "resource", "value": "PROC_RID"}, {"id": "cPROC_MID", "type": "resource", "value": "PROC_MID"}, {"id": "cPROC_DESC", "type": "resource", "value": "PROC_DESC"}], "lookup_tables": [{"name": "usi:ProcedureLookup", "table": "usi__cm_proc_codes", "key": ["PROC_CODE"], "columns": ["PROC_RID", "PROC_MID", "PROC_DESC"], "resources": ["cPROC_RID", "cPROC_MID", "cPROC_DESC"]}], "jet_rules": []}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))
    self.assertEqual(json.dumps(postprocessed_data), expected)

  def test_lookup_table2(self):
    data = io.StringIO("""
      lookup_table MSK_DRG_TRIGGER {
        $table_name = usi__msk_trigger_drg_codes,         # main table
        $key = ["DRG"],                                   # Lookup key

        # Value columns, corresponding resource automatically created
        # Data type based on columns type
        $columns = ["MSK_AREA_DRG_TRIGGER_ONLY", "MSK_TAG", "TRIGGER_TAG_DRG_ONLY", "DRG", "OVERLAP", "USE_ANESTHESIA"]
      };
    """)
    postprocessed_data = self._get_augmented_data(data)

    # validate the whole result
    expected = """{"literals": [], "resources": [{"id": "MSK_DRG_TRIGGER", "type": "resource", "value": "MSK_DRG_TRIGGER"}, {"id": "cMSK_AREA_DRG_TRIGGER_ONLY", "type": "resource", "value": "MSK_AREA_DRG_TRIGGER_ONLY"}, {"id": "cMSK_TAG", "type": "resource", "value": "MSK_TAG"}, {"id": "cTRIGGER_TAG_DRG_ONLY", "type": "resource", "value": "TRIGGER_TAG_DRG_ONLY"}, {"id": "cDRG", "type": "resource", "value": "DRG"}, {"id": "cOVERLAP", "type": "resource", "value": "OVERLAP"}, {"id": "cUSE_ANESTHESIA", "type": "resource", "value": "USE_ANESTHESIA"}], "lookup_tables": [{"name": "MSK_DRG_TRIGGER", "table": "usi__msk_trigger_drg_codes", "key": ["DRG"], "columns": ["MSK_AREA_DRG_TRIGGER_ONLY", "MSK_TAG", "TRIGGER_TAG_DRG_ONLY", "DRG", "OVERLAP", "USE_ANESTHESIA"], "resources": ["cMSK_AREA_DRG_TRIGGER_ONLY", "cMSK_TAG", "cTRIGGER_TAG_DRG_ONLY", "cDRG", "cOVERLAP", "cUSE_ANESTHESIA"]}], "jet_rules": []}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))
    self.assertEqual(json.dumps(postprocessed_data), expected)

  def test_lookup_table3(self):
    data = io.StringIO("""
      # Testing name mapping
      lookup_table MSK_DRG_TRIGGER {
        $table_name = usi__msk_trigger_drg_codes,         # main table
        $key = ["DRG", "DRG2"],                               # composite Lookup key

        # Using column names that need fixing to become resource name
        $columns = ["MSK (9)", "$TAG(3)", "TRIGGER+", "DRG", "123", "#%%"]
      };
    """)
    postprocessed_data = self._get_augmented_data(data)

    # validate the whole result
    expected = """{"literals": [], "resources": [{"id": "MSK_DRG_TRIGGER", "type": "resource", "value": "MSK_DRG_TRIGGER"}, {"id": "cMSK__9_", "type": "resource", "value": "MSK (9)"}, {"id": "c_TAG_3_", "type": "resource", "value": "$TAG(3)"}, {"id": "cTRIGGER_", "type": "resource", "value": "TRIGGER+"}, {"id": "cDRG", "type": "resource", "value": "DRG"}, {"id": "c123", "type": "resource", "value": "123"}, {"id": "c___", "type": "resource", "value": "#%%"}], "lookup_tables": [{"name": "MSK_DRG_TRIGGER", "table": "usi__msk_trigger_drg_codes", "key": ["DRG", "DRG2"], "columns": ["MSK (9)", "$TAG(3)", "TRIGGER+", "DRG", "123", "#%%"], "resources": ["cMSK__9_", "c_TAG_3_", "cTRIGGER_", "cDRG", "c123", "c___"]}], "jet_rules": []}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))
    self.assertEqual(json.dumps(postprocessed_data), expected)

  def test_jetrule1(self):
    data = io.StringIO("""
      # =======================================================================================
      # Defining Jet Rules
      # ---------------------------------------------------------------------------------------
      # property s: salience, o: optimization, tag: label
      # optimization is true by default
      [Rule1, s=+100, o=false, tag="USI"]: 
        (?clm01 rdf:type usi:Claim).
        not(?clm01 usi:hasDRG ?drg).[(?clm01 + ?drg) + int(1) ]
        ->
        (?clm01 rdf:type usi:SpecialClaim).
        (?clm01 xyz ?drg)
      ;
    """)
    postprocessed_data = self._get_augmented_data(data)
    rule_label = postprocessed_data['jet_rules'][0]['label']

    # reprocess the rule_label to ensure to get the same result
    data = io.StringIO(rule_label)
    postprocessed_data = self._get_augmented_data(data)
    self.assertEqual(rule_label, postprocessed_data['jet_rules'][0]['label'])

    # validate the whole result
    expected = """{"literals": [], "resources": [], "lookup_tables": [], "jet_rules": [{"name": "Rule1", "properties": {"s": "+100", "o": "false", "tag": "\\"USI\\""}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "usi:Claim"}], "normalizedLabel": "(?x1 rdf:type usi:Claim)", "label": "(?clm01 rdf:type usi:Claim)"}, {"type": "antecedent", "isNot": true, "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "usi:hasDRG"}, {"type": "var", "id": "?x2", "label": "?drg"}], "filter": {"type": "binary", "lhs": {"type": "binary", "lhs": {"type": "var", "id": "?x1", "label": "?clm01"}, "op": "+", "rhs": {"type": "var", "id": "?x2", "label": "?drg"}}, "op": "+", "rhs": {"type": "int", "value": "1"}}, "normalizedLabel": "not(?x1 usi:hasDRG ?x2).[(?x1 + ?x2) + int(1)]", "label": "not(?clm01 usi:hasDRG ?drg).[(?clm01 + ?drg) + int(1)]"}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "usi:SpecialClaim"}], "normalizedLabel": "(?x1 rdf:type usi:SpecialClaim)", "label": "(?clm01 rdf:type usi:SpecialClaim)"}, {"type": "consequent", "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "xyz"}, {"type": "var", "id": "?x2", "label": "?drg"}], "normalizedLabel": "(?x1 xyz ?x2)", "label": "(?clm01 xyz ?drg)"}], "normalizedLabel": "[Rule1, s=+100, o=false, tag=\\"USI\\"]:(?x1 rdf:type usi:Claim).not(?x1 usi:hasDRG ?x2).[(?x1 + ?x2) + int(1)] -> (?x1 rdf:type usi:SpecialClaim).(?x1 xyz ?x2);", "label": "[Rule1, s=+100, o=false, tag=\\"USI\\"]:(?clm01 rdf:type usi:Claim).not(?clm01 usi:hasDRG ?drg).[(?clm01 + ?drg) + int(1)] -> (?clm01 rdf:type usi:SpecialClaim).(?clm01 xyz ?drg);"}]}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))
    self.assertEqual(json.dumps(postprocessed_data), expected)

  def test_jetrule2(self):
    data = io.StringIO("""
      [Rule2, s=100, o=true, tag="USI"]: 
        (?clm01 rdf:type usi:Claim).
        not(?clm01 usi:hasDRG ?drg).[true and false]
        ->
        (?clm01 rdf:type usi:SpecialClaim)
      ;
    """)
    postprocessed_data = self._get_augmented_data(data)
    rule_label = postprocessed_data['jet_rules'][0]['label']

    # reprocess the rule_label to ensure to get the same result
    data = io.StringIO(rule_label)
    postprocessed_data = self._get_augmented_data(data)
    self.assertEqual(rule_label, postprocessed_data['jet_rules'][0]['label'])

    # validate the whole result
    expected = """{"literals": [], "resources": [], "lookup_tables": [], "jet_rules": [{"name": "Rule2", "properties": {"s": "100", "o": "true", "tag": "\\"USI\\""}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "usi:Claim"}], "normalizedLabel": "(?x1 rdf:type usi:Claim)", "label": "(?clm01 rdf:type usi:Claim)"}, {"type": "antecedent", "isNot": true, "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "usi:hasDRG"}, {"type": "var", "id": "?x2", "label": "?drg"}], "filter": {"type": "binary", "lhs": {"type": "keyword", "value": "true"}, "op": "and", "rhs": {"type": "keyword", "value": "false"}}, "normalizedLabel": "not(?x1 usi:hasDRG ?x2).[true and false]", "label": "not(?clm01 usi:hasDRG ?drg).[true and false]"}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "usi:SpecialClaim"}], "normalizedLabel": "(?x1 rdf:type usi:SpecialClaim)", "label": "(?clm01 rdf:type usi:SpecialClaim)"}], "normalizedLabel": "[Rule2, s=100, o=true, tag=\\"USI\\"]:(?x1 rdf:type usi:Claim).not(?x1 usi:hasDRG ?x2).[true and false] -> (?x1 rdf:type usi:SpecialClaim);", "label": "[Rule2, s=100, o=true, tag=\\"USI\\"]:(?clm01 rdf:type usi:Claim).not(?clm01 usi:hasDRG ?drg).[true and false] -> (?clm01 rdf:type usi:SpecialClaim);"}]}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))
    self.assertEqual(json.dumps(postprocessed_data), expected)

  def test_jetrule3(self):
    data = io.StringIO("""
      [Rule3]: 
        (?clm01 rdf:type usi:Claim).[(?a1 + b1) * (?a2 + b2)].
        (?clm01 rdf:type usi:Claim).[(?a1 or b1) and ?a2].
        ->
        (?clm01 rdf:type usi:SpecialClaim).
        (?clm02 rdf:type usi:SpecialClaim)
      ;
    """)
    postprocessed_data = self._get_augmented_data(data)
    rule_label = postprocessed_data['jet_rules'][0]['label']

    # reprocess the rule_label to ensure to get the same result
    data = io.StringIO(rule_label)
    postprocessed_data = self._get_augmented_data(data)
    self.assertEqual(rule_label, postprocessed_data['jet_rules'][0]['label'])

    # validate the whole result
    expected = """{"literals": [], "resources": [], "lookup_tables": [], "jet_rules": [{"name": "Rule3", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "usi:Claim"}], "filter": {"type": "binary", "lhs": {"type": "binary", "lhs": {"type": "var", "id": "?x2", "label": "?a1"}, "op": "+", "rhs": {"type": "identifier", "value": "b1"}}, "op": "*", "rhs": {"type": "binary", "lhs": {"type": "var", "id": "?x3", "label": "?a2"}, "op": "+", "rhs": {"type": "identifier", "value": "b2"}}}, "normalizedLabel": "(?x1 rdf:type usi:Claim).[(?x2 + b1) * (?x3 + b2)]", "label": "(?clm01 rdf:type usi:Claim).[(?a1 + b1) * (?a2 + b2)]"}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "usi:Claim"}], "filter": {"type": "binary", "lhs": {"type": "binary", "lhs": {"type": "var", "id": "?x2", "label": "?a1"}, "op": "or", "rhs": {"type": "identifier", "value": "b1"}}, "op": "and", "rhs": {"type": "var", "id": "?x3", "label": "?a2"}}, "normalizedLabel": "(?x1 rdf:type usi:Claim).[(?x2 or b1) and ?x3]", "label": "(?clm01 rdf:type usi:Claim).[(?a1 or b1) and ?a2]"}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "usi:SpecialClaim"}], "normalizedLabel": "(?x1 rdf:type usi:SpecialClaim)", "label": "(?clm01 rdf:type usi:SpecialClaim)"}, {"type": "consequent", "triple": [{"type": "var", "id": "?x4", "label": "?clm02"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "usi:SpecialClaim"}], "normalizedLabel": "(?x4 rdf:type usi:SpecialClaim)", "label": "(?clm02 rdf:type usi:SpecialClaim)"}], "normalizedLabel": "[Rule3]:(?x1 rdf:type usi:Claim).[(?x2 + b1) * (?x3 + b2)].(?x1 rdf:type usi:Claim).[(?x2 or b1) and ?x3] -> (?x1 rdf:type usi:SpecialClaim).(?x4 rdf:type usi:SpecialClaim);", "label": "[Rule3]:(?clm01 rdf:type usi:Claim).[(?a1 + b1) * (?a2 + b2)].(?clm01 rdf:type usi:Claim).[(?a1 or b1) and ?a2] -> (?clm01 rdf:type usi:SpecialClaim).(?clm02 rdf:type usi:SpecialClaim);"}]}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))
    self.assertEqual(json.dumps(postprocessed_data), expected)

  def test_jetrule4(self):
    data = io.StringIO("""
      [Rule4]: 
        (?clm01 has_code ?code).[not(?a1 or b1) and (not ?a2)]
        ->
        (?clm01 value (?a1 + ?b2)).
        (?clm01 value2 ?a1 + ?b2).
        (?clm01 value2 (not ?b2))
      ;
    """)
    postprocessed_data = self._get_augmented_data(data)
    rule_label = postprocessed_data['jet_rules'][0]['label']

    # reprocess the rule_label to ensure to get the same result
    data = io.StringIO(rule_label)
    postprocessed_data = self._get_augmented_data(data)
    self.assertEqual(rule_label, postprocessed_data['jet_rules'][0]['label'])

    # validate the whole result
    expected = """{"literals": [], "resources": [], "lookup_tables": [], "jet_rules": [{"name": "Rule4", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "has_code"}, {"type": "var", "id": "?x2", "label": "?code"}], "filter": {"type": "binary", "lhs": {"type": "unary", "op": "not", "arg": {"type": "binary", "lhs": {"type": "var", "id": "?x3", "label": "?a1"}, "op": "or", "rhs": {"type": "identifier", "value": "b1"}}}, "op": "and", "rhs": {"type": "unary", "op": "not", "arg": {"type": "var", "id": "?x4", "label": "?a2"}}}, "normalizedLabel": "(?x1 has_code ?x2).[(not (?x3 or b1)) and (not ?x4)]", "label": "(?clm01 has_code ?code).[(not (?a1 or b1)) and (not ?a2)]"}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "value"}, {"type": "binary", "lhs": {"type": "var", "id": "?x3", "label": "?a1"}, "op": "+", "rhs": {"type": "var", "id": "?x5", "label": "?b2"}}], "normalizedLabel": "(?x1 value ?x3 + ?x5)", "label": "(?clm01 value ?a1 + ?b2)"}, {"type": "consequent", "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "value2"}, {"type": "binary", "lhs": {"type": "var", "id": "?x3", "label": "?a1"}, "op": "+", "rhs": {"type": "var", "id": "?x5", "label": "?b2"}}], "normalizedLabel": "(?x1 value2 ?x3 + ?x5)", "label": "(?clm01 value2 ?a1 + ?b2)"}, {"type": "consequent", "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "value2"}, {"type": "unary", "op": "not", "arg": {"type": "var", "id": "?x5", "label": "?b2"}}], "normalizedLabel": "(?x1 value2 not ?x5)", "label": "(?clm01 value2 not ?b2)"}], "normalizedLabel": "[Rule4]:(?x1 has_code ?x2).[(not (?x3 or b1)) and (not ?x4)] -> (?x1 value ?x3 + ?x5).(?x1 value2 ?x3 + ?x5).(?x1 value2 not ?x5);", "label": "[Rule4]:(?clm01 has_code ?code).[(not (?a1 or b1)) and (not ?a2)] -> (?clm01 value ?a1 + ?b2).(?clm01 value2 ?a1 + ?b2).(?clm01 value2 not ?b2);"}]}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))
    self.assertEqual(json.dumps(postprocessed_data), expected)

  def test_jetrule5(self):
    data = io.StringIO("""
      [Rule5]: 
        (?clm01 has_code ?code).
        ->
        (?clm01 usi:"lookup_table" true)
      ;
    """)
    postprocessed_data = self._get_augmented_data(data)
    rule_label = postprocessed_data['jet_rules'][0]['label']

    # reprocess the rule_label to ensure to get the same result
    data = io.StringIO(rule_label)
    postprocessed_data = self._get_augmented_data(data)
    self.assertEqual(rule_label, postprocessed_data['jet_rules'][0]['label'])

    # validate the whole result
    expected = """{"literals": [], "resources": [], "lookup_tables": [], "jet_rules": [{"name": "Rule5", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "has_code"}, {"type": "var", "id": "?x2", "label": "?code"}], "normalizedLabel": "(?x1 has_code ?x2)", "label": "(?clm01 has_code ?code)"}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "id": "?x1", "label": "?clm01"}, {"type": "identifier", "value": "usi:lookup_table"}, {"type": "keyword", "value": "true"}], "normalizedLabel": "(?x1 usi:\\"lookup_table\\" true)", "label": "(?clm01 usi:\\"lookup_table\\" true)"}], "normalizedLabel": "[Rule5]:(?x1 has_code ?x2) -> (?x1 usi:\\"lookup_table\\" true);", "label": "[Rule5]:(?clm01 has_code ?code) -> (?clm01 usi:\\"lookup_table\\" true);"}]}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))
    self.assertEqual(json.dumps(postprocessed_data), expected)

  def test_jetrule6(self):
    data = io.StringIO("""
      [Rule6]: 
        (?clm01 has_code r1).
        (?clm01 has_str r2).
        ->
        (?clm01 usi:lookupTbl "valueX").
        (?clm01 usi:market "MERGED \\"MARKET\\" CHARGE BACK").
        (?clm01 usi:market text("MERGED \\"MARKET\\" CHARGE BACK"))

      ;
    """)
    postprocessed_data = self._get_augmented_data(data)
    rule_label = postprocessed_data['jet_rules'][0]['label']

    # reprocess the rule_label to ensure to get the same result
    data = io.StringIO(rule_label)
    postprocessed_data = self._get_augmented_data(data)
    self.assertEqual(rule_label, postprocessed_data['jet_rules'][0]['label'])

    # validate the whole result
    with open('jetstore-tools/jetrule-grammar/rule6_test.json', 'rt', encoding='utf-8') as f:
      expected = json.loads(f.read())
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))
    # print('EXPECTED:',json.dumps(expected))
    self.assertEqual(json.dumps(postprocessed_data), json.dumps(expected))

  def test_jetrule7(self):
    data = io.StringIO("""
      [Rule7]: 
        (?clm01 has_code int(1)).
        (?clm01 has_str "value").
        (?clm01 hasTrue true).
        ->
        (?clm01 usi:lookupTbl true).
        (?clm01 has_literal int(1)).
        (?clm01 has_expr (int(1) + long(4)))
      ;
    """)
    postprocessed_data = self._get_augmented_data(data)
    rule_label = postprocessed_data['jet_rules'][0]['label']

    # reprocess the rule_label to ensure to get the same result
    data = io.StringIO(rule_label)
    postprocessed_data = self._get_augmented_data(data)
    self.assertEqual(rule_label, postprocessed_data['jet_rules'][0]['label'])

    # validate the whole result
    with open('jetstore-tools/jetrule-grammar/rule7_test.json', 'rt', encoding='utf-8') as f:
      expected = json.loads(f.read())
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))
    self.assertEqual(json.dumps(postprocessed_data), json.dumps(expected))

if __name__ == '__main__':
  absltest.main()
