"""JetListener core tests"""

import sys
import json
from typing import Dict
from absl import flags
from absl.testing import absltest

from jetrule_compiler import JetRuleCompiler

FLAGS = flags.FLAGS

class JetListenerTest(absltest.TestCase):

  def _get_listener_data(self, data: str) -> Dict[str, object]:
    compiler = JetRuleCompiler()
    jetRules = compiler.processJetRule(data).jetRules
    for err in compiler.jetrule_ctx.errors:
      print('ERROR ::',err)
    self.assertFalse(compiler.jetrule_ctx.ERROR, "Unexpected JetRuleCompiler Errors")
    return jetRules

  def test_directive1(self):
    data = """
      # =======================================================================================
      # Defining Constants Resources and Literals
      # ---------------------------------------------------------------------------------------
      # The JetRule language now have true and false already defined as boolean, adding here
      # for illustration:
      @JetCompilerDirective source_file = "literal.jr";

      int isTrue = 1;     # this is a comment.
      int isFalse = 0;
    """
    jetRules = self._get_listener_data(data)
    
    expected = """{"literals": [{"type": "int", "id": "isTrue", "value": "1", "source_file_name": "literal.jr"}, {"type": "int", "id": "isFalse", "value": "0", "source_file_name": "literal.jr"}], "resources": [], "lookup_tables": [], "jet_rules": []}"""
    # print('GOT:',json.dumps(jetRules, indent=4))
    # print()
    # print('COMPACT:',json.dumps(jetRules))
    self.assertEqual(json.dumps(jetRules), expected)

  def test_literals1(self):
    data = """
      # =======================================================================================
      # Defining Constants Resources and Literals
      # ---------------------------------------------------------------------------------------
      # The JetRule language now have true and false already defined as boolean, adding here
      # for illustration:
      int isTrue = 1;     # this is a comment.
      int isFalse = 0;
    """
    jetRules = self._get_listener_data(data)
    
    expected = """{"literals": [{"type": "int", "id": "isTrue", "value": "1"}, {"type": "int", "id": "isFalse", "value": "0"}], "resources": [], "lookup_tables": [], "jet_rules": []}"""
    # print('GOT:',json.dumps(jetRules, indent=4))
    # print()
    # print('COMPACT:',json.dumps(jetRules))
    self.assertEqual(json.dumps(jetRules), expected)

  def test_literals2(self):
    data = """
      # Defining some constants (e.g. Exclacmeon Types)
      # ---------------------------------------------------------------------------------------
      text NOT_IN_CONTRACT      = "NOT COVERED IN CONTRACT";
      text EXCLUDED_STATE       = "STATE";
      text HH_AUTH              = "HH_AUTH";
      text EXCL_HH_AUTH         = "HH AUTH";
      text EXCLUDED_COUNTY      = "COUNTY";
      text EXCLUDED_TIN         = "TIN";
      text EXCLUDED_TIN_STATE   = "TIN/STATE";
      text EXCL_MER_COM         = "MERGED COMPONENTS";
      text EXCL_AMT_PAID        = "MERGED \\"MARKET\\" CHARGE BACK";
      text EXCLUDED_GROUPID     = "GROUPID";
      text EXCLUDED_MODALITY    = "MODALITY";
    """
    jetRules = self._get_listener_data(data)
    
    expected = """{"literals": [{"type": "text", "id": "NOT_IN_CONTRACT", "value": "NOT COVERED IN CONTRACT"}, {"type": "text", "id": "EXCLUDED_STATE", "value": "STATE"}, {"type": "text", "id": "HH_AUTH", "value": "HH_AUTH"}, {"type": "text", "id": "EXCL_HH_AUTH", "value": "HH AUTH"}, {"type": "text", "id": "EXCLUDED_COUNTY", "value": "COUNTY"}, {"type": "text", "id": "EXCLUDED_TIN", "value": "TIN"}, {"type": "text", "id": "EXCLUDED_TIN_STATE", "value": "TIN/STATE"}, {"type": "text", "id": "EXCL_MER_COM", "value": "MERGED COMPONENTS"}, {"type": "text", "id": "EXCL_AMT_PAID", "value": "MERGED \\"MARKET\\" CHARGE BACK"}, {"type": "text", "id": "EXCLUDED_GROUPID", "value": "GROUPID"}, {"type": "text", "id": "EXCLUDED_MODALITY", "value": "MODALITY"}], "resources": [], "lookup_tables": [], "jet_rules": []}"""
    # print('GOT:',json.dumps(jetRules, indent=4))
    # print()
    # print('COMPACT:',json.dumps(jetRules))
    self.assertEqual(json.dumps(jetRules), expected)

  def test_literals3(self):
    data = """
      # =======================================================================================
      # Defining Constants Resources and Literals
      # ---------------------------------------------------------------------------------------
      double DD = 10.9;
      resource NullResource = null;
      resource TrueResource = true;
      resource FalseResource = false;
    """
    jetRules = self._get_listener_data(data)
    
    expected = """{"literals": [{"type": "double", "id": "DD", "value": "10.9"}], "resources": [{"type": "resource", "id": "NullResource", "symbol": "null", "value": null}, {"type": "resource", "id": "TrueResource", "symbol": "true", "value": null}, {"type": "resource", "id": "FalseResource", "symbol": "false", "value": null}], "lookup_tables": [], "jet_rules": []}"""
    # print('GOT:',json.dumps(jetRules, indent=4))
    # print()
    # print('COMPACT:',json.dumps(jetRules))
    self.assertEqual(json.dumps(jetRules), expected)

  def test_literalRegex1(self):
    data = """
      # =======================================================================================
      # Defining Constants Literals with Regex expression
      # ---------------------------------------------------------------------------------------
      text regex1 = "(-?\\d*,?\\d+(\\.?\\d{1,2})?)";
    """
    jetRules = self._get_listener_data(data)
    
    expected = """{"literals": [{"type": "text", "id": "regex1", "value": "(-?\\\\d*,?\\\\d+(\\\\.?\\\\d{1,2})?)"}], "resources": [], "lookup_tables": [], "jet_rules": []}"""
    # print('GOT:',json.dumps(jetRules, indent=4))
    # print()
    # print('COMPACT:',json.dumps(jetRules))
    self.assertEqual(json.dumps(jetRules), expected)

  def test_literalEscape1(self):
    data = """
      # =======================================================================================
      # Defining Constants Literals with escape character
      # ---------------------------------------------------------------------------------------
      text str1 = "some \\"escaped\\" string";
    """
    jetRules = self._get_listener_data(data)
    
    expected = """{"literals": [{"type": "text", "id": "str1", "value": "some \\"escaped\\" string"}], "resources": [], "lookup_tables": [], "jet_rules": []}"""
    # print('GOT:',json.dumps(jetRules, indent=4))
    # print()
    # print('COMPACT:',json.dumps(jetRules))
    self.assertEqual(json.dumps(jetRules), expected)

  def test_literalEscape2(self):
    data = """
      # =======================================================================================
      # Defining Constants Literals with escape character
      # ---------------------------------------------------------------------------------------
      text str1 = "some string ending with a backslash \\\\";
    """
    jetRules = self._get_listener_data(data)
    
    expected = """{"literals": [{"type": "text", "id": "str1", "value": "some string ending with a backslash \\\\"}], "resources": [], "lookup_tables": [], "jet_rules": []}"""
    # print('GOT:',json.dumps(jetRules, indent=4))
    # print()
    # print('COMPACT:',json.dumps(jetRules))
    self.assertEqual(json.dumps(jetRules), expected)

  def test_resource(self):
    data = """
      resource medicareRateObjTC1 = "_0:medicareRateObjTC1";  # Support RC legacy
      resource medicareRateObjTC2 = "_0:medicareRateObjTC2";  # Support RC legacy

      resource None  = null;
      resource uuid  = create_uuid_resource();

      # Some special cases
      resource acme:key = "acme:key";
      resource acme:"lookup_table" = "acme:key";  # Escaping keyword 'lookup_table' in resource name
    """
    jetRules = self._get_listener_data(data)
    
    expected = """{"literals": [], "resources": [{"type": "resource", "id": "medicareRateObjTC1", "value": "_0:medicareRateObjTC1"}, {"type": "resource", "id": "medicareRateObjTC2", "value": "_0:medicareRateObjTC2"}, {"type": "resource", "id": "None", "symbol": "null", "value": null}, {"type": "resource", "id": "uuid", "symbol": "create_uuid_resource()", "value": null}, {"type": "resource", "id": "acme:key", "value": "acme:key"}, {"type": "resource", "id": "acme:lookup_table", "value": "acme:key"}], "lookup_tables": [], "jet_rules": []}"""
    # print('GOT:',json.dumps(jetRules, indent=4))
    # print()
    # print('COMPACT:',json.dumps(jetRules))
    self.assertEqual(json.dumps(jetRules), expected)

  def test_volatile_resource(self):
    data = """
      volatile_resource medicareRateObj261     = "medicareRateObj261";
      volatile_resource medicareRateObj262     = "medicareRateObj262";
    """
    jetRules = self._get_listener_data(data)
    
    expected = """{"literals": [], "resources": [{"type": "volatile_resource", "id": "medicareRateObj261", "value": "medicareRateObj261"}, {"type": "volatile_resource", "id": "medicareRateObj262", "value": "medicareRateObj262"}], "lookup_tables": [], "jet_rules": []}"""
    # print('GOT:',json.dumps(jetRules, indent=4))
    # print()
    # print('COMPACT:',json.dumps(jetRules))
    self.assertEqual(json.dumps(jetRules), expected)
    
  def test_lookup_table(self):
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
        $columns = ["PROC_RID", "PROC_MID", "PROC_DESC"]
      };

      # Another example that is already acmeng a csv file 
      # based on USI: *include-lookup* "MSK/MSK_DRG_TRIGGER.lookup"
      lookup_table MSK_DRG_TRIGGER {
        $table_name = acme__msk_trigger_drg_codes,         # main table
        $key = ["DRG"],                                   # Lookup key

        # Value columns, corresponding resource automatically created
        # Data type based on columns type
        $columns = ["MSK_AREA_DRG_TRIGGER_ONLY", "MSK_TAG", "TRIGGER_TAG_DRG_ONLY", "DRG", "OVERLAP", "USE_ANESTHESIA"]
      };
    """
    jetRules = self._get_listener_data(data)
    
    expected = """{"literals": [], "resources": [], "lookup_tables": [{"name": "acme:ProcedureLookup", "table": "acme__cm_proc_codes", "key": ["PROC_CODE"], "columns": ["PROC_RID", "PROC_MID", "PROC_DESC"]}, {"name": "MSK_DRG_TRIGGER", "table": "acme__msk_trigger_drg_codes", "key": ["DRG"], "columns": ["MSK_AREA_DRG_TRIGGER_ONLY", "MSK_TAG", "TRIGGER_TAG_DRG_ONLY", "DRG", "OVERLAP", "USE_ANESTHESIA"]}], "jet_rules": []}"""
    # print('GOT:',json.dumps(jetRules, indent=2))
    # print()
    # print('COMPACT:',json.dumps(jetRules))
    self.assertEqual(json.dumps(jetRules), expected)

  def test_jetrule1(self):
    data = """
      # =======================================================================================
      # Defining Jet Rules
      # ---------------------------------------------------------------------------------------
      # property s: salience, o: optimization, tag: label
      # optimization is true by default
      [Rule1, s=+100, o=false, tag="USI"]: 
        (?clm01 rdf:type acme:Claim).
        not(?clm01 acme:hasDRG ?drg).[(?clm01 + ?drg) + int(1) ]
        ->
        (?clm01 rdf:type acme:SpecialClaim).
        (?clm01 xyz ?drg)
      ;
    """
    jetRules = self._get_listener_data(data)
    
    expected = """{"literals": [], "resources": [], "lookup_tables": [], "jet_rules": [{"name": "Rule1", "properties": {"s": "+100", "o": "false", "tag": "\\\"USI\\\""}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}]}, {"type": "antecedent", "isNot": true, "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "acme:hasDRG"}, {"type": "var", "value": "?drg"}], "filter": {"type": "binary", "lhs": {"type": "binary", "lhs": {"type": "var", "value": "?clm01"}, "op": "+", "rhs": {"type": "var", "value": "?drg"}}, "op": "+", "rhs": {"type": "int", "value": "1"}}}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:SpecialClaim"}]}, {"type": "consequent", "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "xyz"}, {"type": "var", "value": "?drg"}]}]}]}"""
    # print('GOT:',json.dumps(jetRules, indent=2))
    # print()
    # print('COMPACT:',json.dumps(jetRules))
    self.assertEqual(json.dumps(jetRules), expected)

  def test_jetrule2(self):
    data = """
      [Rule2, s=100, o=true, tag="USI"]: 
        (?clm01 rdf:type acme:Claim).
        not(?clm01 acme:hasDRG ?drg).[true and false]
        ->
        (?clm01 rdf:type acme:SpecialClaim)
      ;
    """
    jetRules = self._get_listener_data(data)
    
    expected = """{"literals": [], "resources": [], "lookup_tables": [], "jet_rules": [{"name": "Rule2", "properties": {"s": "100", "o": "true", "tag": "\\\"USI\\\""}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}]}, {"type": "antecedent", "isNot": true, "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "acme:hasDRG"}, {"type": "var", "value": "?drg"}], "filter": {"type": "binary", "lhs": {"type": "keyword", "value": "true"}, "op": "and", "rhs": {"type": "keyword", "value": "false"}}}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:SpecialClaim"}]}]}]}"""
    # print('GOT:',json.dumps(jetRules, indent=2))
    # print()
    # print('COMPACT:',json.dumps(jetRules))
    self.assertEqual(json.dumps(jetRules), expected)

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
    jetRules = self._get_listener_data(data)
    
    expected = """{"literals": [], "resources": [], "lookup_tables": [], "jet_rules": [{"name": "Rule3", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "filter": {"type": "binary", "lhs": {"type": "binary", "lhs": {"type": "var", "value": "?a1"}, "op": "+", "rhs": {"type": "identifier", "value": "b1"}}, "op": "*", "rhs": {"type": "binary", "lhs": {"type": "var", "value": "?a2"}, "op": "+", "rhs": {"type": "identifier", "value": "b2"}}}}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}], "filter": {"type": "binary", "lhs": {"type": "binary", "lhs": {"type": "var", "value": "?a1"}, "op": "or", "rhs": {"type": "identifier", "value": "b1"}}, "op": "and", "rhs": {"type": "var", "value": "?a2"}}}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:SpecialClaim"}]}, {"type": "consequent", "triple": [{"type": "var", "value": "?clm02"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:SpecialClaim"}]}]}]}"""
    # print('GOT:',json.dumps(jetRules, indent=2))
    # print()
    # print('COMPACT:',json.dumps(jetRules))
    self.assertEqual(json.dumps(jetRules), expected)

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
    jetRules = self._get_listener_data(data)
    
    expected = """{"literals": [], "resources": [], "lookup_tables": [], "jet_rules": [{"name": "Rule4", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "has_code"}, {"type": "var", "value": "?code"}], "filter": {"type": "binary", "lhs": {"type": "unary", "op": "not", "arg": {"type": "binary", "lhs": {"type": "var", "value": "?a1"}, "op": "or", "rhs": {"type": "identifier", "value": "b1"}}}, "op": "and", "rhs": {"type": "unary", "op": "not", "arg": {"type": "var", "value": "?a2"}}}}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "value"}, {"type": "binary", "lhs": {"type": "var", "value": "?a1"}, "op": "+", "rhs": {"type": "var", "value": "?b2"}}]}, {"type": "consequent", "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "value2"}, {"type": "binary", "lhs": {"type": "var", "value": "?a1"}, "op": "+", "rhs": {"type": "var", "value": "?b2"}}]}, {"type": "consequent", "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "value2"}, {"type": "unary", "op": "not", "arg": {"type": "var", "value": "?b2"}}]}]}]}"""
    # print('GOT:',json.dumps(jetRules, indent=2))
    # print()
    # print('COMPACT:',json.dumps(jetRules))
    self.assertEqual(json.dumps(jetRules), expected)

  def test_jetrule5(self):
    data = """
      [Rule5]: 
        (?clm01 has_code ?code).
        ->
        (?clm01 acme:"lookup_table" true)
      ;
    """
    jetRules = self._get_listener_data(data)
    
    expected = """{"literals": [], "resources": [], "lookup_tables": [], "jet_rules": [{"name": "Rule5", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "has_code"}, {"type": "var", "value": "?code"}]}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "acme:lookup_table"}, {"type": "keyword", "value": "true"}]}]}]}"""
    # print('GOT:',json.dumps(jetRules, indent=2))
    # print()
    # print('COMPACT:',json.dumps(jetRules))
    self.assertEqual(json.dumps(jetRules), expected)

  def test_jetrule6(self):
    data = """
      [Rule6]: 
        (?clm01 has_code r1).
        (?clm01 has_str r2).
        ->
        (?clm01 acme:"lookup_table" "valueX").
        (?clm01 acme:market "MERGED \\"MARKET\\" CHARGE BACK").
        (?clm01 acme:market text("MERGED \\"MARKET\\" CHARGE BACK"))

      ;
    """
    jetRules = self._get_listener_data(data)
    
    expected = """{"literals": [], "resources": [], "lookup_tables": [], "jet_rules": [{"name": "Rule6", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "has_code"}, {"type": "identifier", "value": "r1"}]}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "has_str"}, {"type": "identifier", "value": "r2"}]}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "acme:lookup_table"}, {"type": "text", "value": "valueX"}]}, {"type": "consequent", "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "acme:market"}, {"type": "text", "value": "MERGED \\\"MARKET\\\" CHARGE BACK"}]}, {"type": "consequent", "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "acme:market"}, {"type": "text", "value": "MERGED \\\"MARKET\\\" CHARGE BACK"}]}]}]}"""
    # print('GOT:',json.dumps(jetRules, indent=2))
    # print()
    # print('COMPACT:',json.dumps(jetRules))
    self.assertEqual(json.dumps(jetRules), expected)

  def test_jetrule7(self):
    data = """
      [Rule7]: 
        (?clm01 has_code int(1)).
        (?clm01 has_str "value").
        (?clm01 hasTrue true).
        ->
        (?clm01 acme:"lookup_table" true).
        (?clm01 has_literal int(1)).
        (?clm01 has_expr (int(1) + long(4)))
      ;
    """
    jetRules = self._get_listener_data(data)
    
    expected = """{"literals": [], "resources": [], "lookup_tables": [], "jet_rules": [{"name": "Rule7", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "has_code"}, {"type": "int", "value": "1"}]}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "has_str"}, {"type": "text", "value": "value"}]}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "hasTrue"}, {"type": "keyword", "value": "true"}]}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "acme:lookup_table"}, {"type": "keyword", "value": "true"}]}, {"type": "consequent", "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "has_literal"}, {"type": "int", "value": "1"}]}, {"type": "consequent", "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "has_expr"}, {"type": "binary", "lhs": {"type": "int", "value": "1"}, "op": "+", "rhs": {"type": "long", "value": "4"}}]}]}]}"""
    # print('GOT:',json.dumps(jetRules, indent=2))
    # print()
    # print('COMPACT:',json.dumps(jetRules))
    self.assertEqual(json.dumps(jetRules), expected)

  def test_jetrule10(self):
    data = """
      # =======================================================================================
      # Jet Rules with int and double
      [Rule10]: 
        (?clm01 rdf:type acme:Claim).
        not(?clm01 acme:hasDRG ?drg).[(?clm01 + 1.0) - 10 ]
        ->
        (?clm01 rdf:type acme:SpecialClaim).
        (?clm01 xyz (?drg and -5) or (+2.99 + +3.77))
      ;
    """
    jetRules = self._get_listener_data(data)
    
    expected = """{"literals": [], "resources": [], "lookup_tables": [], "jet_rules": [{"name": "Rule10", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}]}, {"type": "antecedent", "isNot": true, "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "acme:hasDRG"}, {"type": "var", "value": "?drg"}], "filter": {"type": "binary", "lhs": {"type": "binary", "lhs": {"type": "var", "value": "?clm01"}, "op": "+", "rhs": {"type": "double", "value": "1.0"}}, "op": "-", "rhs": {"type": "int", "value": "10"}}}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:SpecialClaim"}]}, {"type": "consequent", "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "xyz"}, {"type": "binary", "lhs": {"type": "binary", "lhs": {"type": "var", "value": "?drg"}, "op": "and", "rhs": {"type": "int", "value": "-5"}}, "op": "or", "rhs": {"type": "binary", "lhs": {"type": "double", "value": "+2.99"}, "op": "+", "rhs": {"type": "double", "value": "+3.77"}}}]}]}]}"""
    # print('GOT:',json.dumps(jetRules, indent=2))
    # print()
    # print('COMPACT:',json.dumps(jetRules))
    self.assertEqual(json.dumps(jetRules), expected)

  def test_jetrule20(self):
    data = """
      # =======================================================================================
      # Jet Rules with identifier for operators for RC RR migration
      [Rule20]: 
        (?clm01 rdf:type acme:Claim).
        not(?clm01 acme:hasDRG ?drg).[(?clm01 random_lookup 1.0) getCardinality 10 ]
        ->
        (?clm01 rdf:type acme:SpecialClaim).
        (?clm01 xyz (?drg and -5) or (+2.99 + +3.77))
      ;
    """
    jetRules = self._get_listener_data(data)
    
    expected = """{"literals": [], "resources": [], "lookup_tables": [], "jet_rules": [{"name": "Rule20", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:Claim"}]}, {"type": "antecedent", "isNot": true, "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "acme:hasDRG"}, {"type": "var", "value": "?drg"}], "filter": {"type": "binary", "lhs": {"type": "binary", "lhs": {"type": "var", "value": "?clm01"}, "op": "random_lookup", "rhs": {"type": "double", "value": "1.0"}}, "op": "getCardinality", "rhs": {"type": "int", "value": "10"}}}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "acme:SpecialClaim"}]}, {"type": "consequent", "triple": [{"type": "var", "value": "?clm01"}, {"type": "identifier", "value": "xyz"}, {"type": "binary", "lhs": {"type": "binary", "lhs": {"type": "var", "value": "?drg"}, "op": "and", "rhs": {"type": "int", "value": "-5"}}, "op": "or", "rhs": {"type": "binary", "lhs": {"type": "double", "value": "+2.99"}, "op": "+", "rhs": {"type": "double", "value": "+3.77"}}}]}]}]}"""
    # print('GOT:',json.dumps(jetRules, indent=2))
    # print()
    # print('COMPACT:',json.dumps(jetRules))
    self.assertEqual(json.dumps(jetRules), expected)

  def test_triples1(self):
    data = """
      # =======================================================================================
      # Jet Rules with triples
        @JetCompilerDirective extract_resources_from_rules = "true";
        date d1 = "03/16/2022";
        datetime dt1 = "03/16/2022 16:08:28.195865";

        triple(s1,top:operator,"<");
        triple(s2,_0:yearDistance, 22);
        triple(s3, jet:"int", uint(2));
        triple(s4, ?dos, date("01/22/2022"));
    """
    jetRules = self._get_listener_data(data)
    
    expected = """{"literals": [{"type": "date", "id": "d1", "value": "03/16/2022"}, {"type": "datetime", "id": "dt1", "value": "03/16/2022 16:08:28.195865"}], "resources": [], "lookup_tables": [], "jet_rules": [], "triples": [{"type": "triple", "subject": {"type": "identifier", "value": "s1"}, "predicate": {"type": "identifier", "value": "top:operator"}, "object": {"type": "text", "value": "<"}}, {"type": "triple", "subject": {"type": "identifier", "value": "s2"}, "predicate": {"type": "identifier", "value": "_0:yearDistance"}, "object": {"type": "int", "value": "22"}}, {"type": "triple", "subject": {"type": "identifier", "value": "s3"}, "predicate": {"type": "identifier", "value": "jet:int"}, "object": {"type": "uint", "value": "2"}}, {"type": "triple", "subject": {"type": "identifier", "value": "s4"}, "predicate": {"type": "var", "value": "?dos"}, "object": {"type": "date", "value": "01/22/2022"}}], "compiler_directives": {"extract_resources_from_rules": "true"}}"""
    # print('GOT:',json.dumps(jetRules, indent=2))
    # print()
    # print('COMPACT:',json.dumps(jetRules))
    self.assertEqual(json.dumps(jetRules), expected)

  def test_classes1(self):
    data = """
      # =======================================================================================
      # Jet Rules with class definition
      class jets:Entity {
        # This is an example of a domain class
        $sub_class_of = owl:Thing,
        $data_property = jets:key as int,
        $data_property = diagnosis as array of text,
        $as_table = true
      };
    """
    jetRules = self._get_listener_data(data)
    
    expected = """{"literals": [], "resources": [], "lookup_tables": [], "jet_rules": [], "classes": [{"type": "class", "name": "jets:Entity", "base_classes": ["owl:Thing"], "data_properties": [{"name": "jets:key", "type": "int", "as_array": "false"}, {"name": "diagnosis", "type": "text", "as_array": "true"}], "as_table": "true"}]}"""
    # print('GOT:',json.dumps(jetRules, indent=2))
    # print()
    # print('COMPACT:',json.dumps(jetRules))
    self.assertEqual(json.dumps(jetRules), expected)

  def test_classes2(self):
    data = """
      # =======================================================================================
      # Jet Rules with class definition
      class jets:Entity {
        $sub_class_of = owl:Thing,
        $data_property = jets:key as int,
        $as_table = false
      };
      class hc:MedicalClaim {
        # This is an example of a domain class
        $sub_class_of = jets:Entity,
        $sub_class_of = hc:Claim,
        $data_property = diagnosis as array of text,
        $as_table = true
      };
    """
    jetRules = self._get_listener_data(data)
    
    expected = """{"literals": [], "resources": [], "lookup_tables": [], "jet_rules": [], "classes": [{"type": "class", "name": "jets:Entity", "base_classes": ["owl:Thing"], "data_properties": [{"name": "jets:key", "type": "int", "as_array": "false"}], "as_table": "false"}, {"type": "class", "name": "hc:MedicalClaim", "base_classes": ["jets:Entity", "hc:Claim"], "data_properties": [{"name": "diagnosis", "type": "text", "as_array": "true"}], "as_table": "true"}]}"""
    # print('GOT:',json.dumps(jetRules, indent=2))
    # print()
    # print('COMPACT:',json.dumps(jetRules))
    self.assertEqual(json.dumps(jetRules), expected)


if __name__ == '__main__':
  absltest.main()
