"""JetListener core tests"""

import sys
import json
from absl import flags
from absl.testing import absltest
import antlr4 as a4

from jet_listener import JetListener
from JetRuleParser import JetRuleParser
from JetRuleLexer import JetRuleLexer

FLAGS = flags.FLAGS

class JetListenerTest(absltest.TestCase):

  def _get_listener(self, data) -> JetListener:
    # lexer
    lexer = JetRuleLexer(data)
    stream = a4.CommonTokenStream(lexer)
    
    # parser
    parser = JetRuleParser(stream)
    tree = parser.jetrule()

    # evaluator
    listener = JetListener()
    walker = a4.ParseTreeWalker()
    walker.walk(listener, tree)
    return listener

  def test_literals1(self):
    data = a4.InputStream("""
      # =======================================================================================
      # Defining Constants Resources and Literals
      # ---------------------------------------------------------------------------------------
      # The JetRule language now have true and false already defined as boolean, adding here
      # for illustration:
      int isTrue = 1;     # this is a comment.
      int isFalse = 0;
    """)
    listener = self._get_listener(data)

    expected = """{"literals": [{"type": "int", "id": "isTrue", "value": "1"}, {"type": "int", "id": "isFalse", "value": "0"}], "resources": [], "lookup_tables": [], "jet_rules": []}"""
    # print('GOT:',json.dumps(listener.jetRules, indent=4))
    # print()
    # print('COMPACT:',json.dumps(listener.jetRules))
    self.assertEqual(json.dumps(listener.jetRules), expected)

  def test_literals2(self):
    data = a4.InputStream("""
      # Defining some constants (e.g. Exclusion Types)
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
    """)
    listener = self._get_listener(data)

    expected = """{"literals": [{"type": "text", "id": "NOT_IN_CONTRACT", "value": "NOT COVERED IN CONTRACT"}, {"type": "text", "id": "EXCLUDED_STATE", "value": "STATE"}, {"type": "text", "id": "HH_AUTH", "value": "HH_AUTH"}, {"type": "text", "id": "EXCL_HH_AUTH", "value": "HH AUTH"}, {"type": "text", "id": "EXCLUDED_COUNTY", "value": "COUNTY"}, {"type": "text", "id": "EXCLUDED_TIN", "value": "TIN"}, {"type": "text", "id": "EXCLUDED_TIN_STATE", "value": "TIN/STATE"}, {"type": "text", "id": "EXCL_MER_COM", "value": "MERGED COMPONENTS"}, {"type": "text", "id": "EXCL_AMT_PAID", "value": "MERGED \\"MARKET\\" CHARGE BACK"}, {"type": "text", "id": "EXCLUDED_GROUPID", "value": "GROUPID"}, {"type": "text", "id": "EXCLUDED_MODALITY", "value": "MODALITY"}], "resources": [], "lookup_tables": [], "jet_rules": []}"""
    # print('GOT:',json.dumps(listener.jetRules, indent=4))
    # print()
    # print('COMPACT:',json.dumps(listener.jetRules))
    self.assertEqual(json.dumps(listener.jetRules), expected)

  def test_resource(self):
    data = a4.InputStream("""
      resource medicareRateObjTC1 = "_0:medicareRateObjTC1";  # Support RC legacy
      resource medicareRateObjTC2 = "_0:medicareRateObjTC2";  # Support RC legacy

      resource None  = null;
      resource uuid  = create_uuid_resource();

      # Some special cases
      resource usi:key = "usi:key";
      resource usi:"lookup_table" = "usi:key";  # Escaping keyword 'lookup_table' in resource name
    """)
    listener = self._get_listener(data)

    expected = """{"literals": [], "resources": [{"type": "resource", "id": "medicareRateObjTC1", "value": "_0:medicareRateObjTC1"}, {"type": "resource", "id": "medicareRateObjTC2", "value": "_0:medicareRateObjTC2"}, {"type": "resource", "id": "None", "value": "null"}, {"type": "resource", "id": "uuid", "value": "create_uuid_resource()"}, {"type": "resource", "id": "usi:key", "value": "usi:key"}, {"type": "resource", "id": "usi:lookup_table", "value": "usi:key"}], "lookup_tables": [], "jet_rules": []}"""
    # print('GOT:',json.dumps(listener.jetRules, indent=4))
    # print()
    # print('COMPACT:',json.dumps(listener.jetRules))
    self.assertEqual(json.dumps(listener.jetRules), expected)

  def test_volatile_resource(self):
    data = a4.InputStream("""
      volatile_resource medicareRateObj261     = "medicareRateObj261";
      volatile_resource medicareRateObj262     = "medicareRateObj262";
    """)
    listener = self._get_listener(data)

    expected = """{"literals": [], "resources": [{"type": "volatile_resource", "id": "medicareRateObj261", "value": "medicareRateObj261"}, {"type": "volatile_resource", "id": "medicareRateObj262", "value": "medicareRateObj262"}], "lookup_tables": [], "jet_rules": []}"""
    # print('GOT:',json.dumps(listener.jetRules, indent=4))
    # print()
    # print('COMPACT:',json.dumps(listener.jetRules))
    self.assertEqual(json.dumps(listener.jetRules), expected)

  def test_lookup_table(self):
    data = a4.InputStream("""
      # =======================================================================================
      # Defining Lookup Tables
      # ---------------------------------------------------------------------------------------
      # lookup example based on USI: *include-lookup* "CM/Procedure CM.trd"
      # Note: Legacy trd lookup table will have to be converted to csv
      # Assuming here the csv would have these columns: "PROC_CODE, PROC_RID, PROC_MID, PROC_DESC"
      lookup_table usi:ProcedureLookup {
        $table_name = usi__cm_proc_codes,       # Table name where the data reside (loaded from trd file)
        $key = [PROC_CODE],                     # Key columns, resource PROC_CODE automatically created

        # Value columns, corresponding resource automatically created
        $columns = [PROC_RID, PROC_MID, PROC_DESC]
      };

      # Another example that is already using a csv file 
      # based on USI: *include-lookup* "MSK/MSK_DRG_TRIGGER.lookup"
      lookup_table MSK_DRG_TRIGGER {
        $table_name = usi__msk_trigger_drg_codes,         # main table
        $key = [DRG],                                     # Lookup key

        # Value columns, corresponding resource automatically created
        # Data type based on columns type
        $columns = [MSK_AREA_DRG_TRIGGER_ONLY, MSK_TAG, TRIGGER_TAG_DRG_ONLY, DRG, OVERLAP, USE_ANESTHESIA]
      };
    """)
    listener = self._get_listener(data)

    expected = """{"literals": [], "resources": [], "lookup_tables": [{"name": "usi:ProcedureLookup", "table": "usi__cm_proc_codes", "key": ["PROC_CODE"], "columns": ["PROC_RID", "PROC_MID", "PROC_DESC"]}, {"name": "MSK_DRG_TRIGGER", "table": "usi__msk_trigger_drg_codes", "key": ["DRG"], "columns": ["MSK_AREA_DRG_TRIGGER_ONLY", "MSK_TAG", "TRIGGER_TAG_DRG_ONLY", "DRG", "OVERLAP", "USE_ANESTHESIA"]}], "jet_rules": []}"""
    # print('GOT:',json.dumps(listener.jetRules, indent=2))
    # print()
    # print('COMPACT:',json.dumps(listener.jetRules))
    self.assertEqual(json.dumps(listener.jetRules), expected)

  def test_jetrule1(self):
    data = a4.InputStream("""
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
    listener = self._get_listener(data)

    expected = """{"literals": [], "resources": [], "lookup_tables": [], "jet_rules": [{"name": "Rule1", "properties": {"s": "+100", "o": "false", "tag": "\\\"USI\\\""}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "usi:Claim"}]}, {"type": "antecedent", "isNot": true, "triple": [{"type": "var", "id": "?clm01"}, {"type": "identifier", "value": "usi:hasDRG"}, {"type": "var", "id": "?drg"}], "filter": {"type": "binary", "lhs": {"type": "binary", "lhs": {"type": "var", "id": "?clm01"}, "op": "+", "rhs": {"type": "var", "id": "?drg"}}, "op": "+", "rhs": {"type": "int", "value": "1"}}}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "id": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "usi:SpecialClaim"}]}, {"type": "consequent", "triple": [{"type": "var", "id": "?clm01"}, {"type": "identifier", "value": "xyz"}, {"type": "var", "id": "?drg"}]}]}]}"""
    # print('GOT:',json.dumps(listener.jetRules, indent=2))
    # print()
    # print('COMPACT:',json.dumps(listener.jetRules))
    self.assertEqual(json.dumps(listener.jetRules), expected)

  def test_jetrule2(self):
    data = a4.InputStream("""
      [Rule2, s=100, o=true, tag="USI"]: 
        (?clm01 rdf:type usi:Claim).
        not(?clm01 usi:hasDRG ?drg).[true and false]
        ->
        (?clm01 rdf:type usi:SpecialClaim)
      ;
    """)
    listener = self._get_listener(data)

    expected = """{"literals": [], "resources": [], "lookup_tables": [], "jet_rules": [{"name": "Rule2", "properties": {"s": "100", "o": "true", "tag": "\\\"USI\\\""}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "usi:Claim"}]}, {"type": "antecedent", "isNot": true, "triple": [{"type": "var", "id": "?clm01"}, {"type": "identifier", "value": "usi:hasDRG"}, {"type": "var", "id": "?drg"}], "filter": {"type": "binary", "lhs": {"type": "keyword", "value": "true"}, "op": "and", "rhs": {"type": "keyword", "value": "false"}}}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "id": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "usi:SpecialClaim"}]}]}]}"""
    # print('GOT:',json.dumps(listener.jetRules, indent=2))
    # print()
    # print('COMPACT:',json.dumps(listener.jetRules))
    self.assertEqual(json.dumps(listener.jetRules), expected)

  def test_jetrule3(self):
    data = a4.InputStream("""
      [Rule3]: 
        (?clm01 rdf:type usi:Claim).[(?a1 + b1) * (?a2 + b2)].
        (?clm01 rdf:type usi:Claim).[(?a1 or b1) and ?a2].
        ->
        (?clm01 rdf:type usi:SpecialClaim).
        (?clm02 rdf:type usi:SpecialClaim)
      ;
    """)
    listener = self._get_listener(data)

    expected = """{"literals": [], "resources": [], "lookup_tables": [], "jet_rules": [{"name": "Rule3", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "usi:Claim"}], "filter": {"type": "binary", "lhs": {"type": "binary", "lhs": {"type": "var", "id": "?a1"}, "op": "+", "rhs": {"type": "identifier", "value": "b1"}}, "op": "*", "rhs": {"type": "binary", "lhs": {"type": "var", "id": "?a2"}, "op": "+", "rhs": {"type": "identifier", "value": "b2"}}}}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "usi:Claim"}], "filter": {"type": "binary", "lhs": {"type": "binary", "lhs": {"type": "var", "id": "?a1"}, "op": "or", "rhs": {"type": "identifier", "value": "b1"}}, "op": "and", "rhs": {"type": "var", "id": "?a2"}}}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "id": "?clm01"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "usi:SpecialClaim"}]}, {"type": "consequent", "triple": [{"type": "var", "id": "?clm02"}, {"type": "identifier", "value": "rdf:type"}, {"type": "identifier", "value": "usi:SpecialClaim"}]}]}]}"""
    # print('GOT:',json.dumps(listener.jetRules, indent=2))
    # print()
    # print('COMPACT:',json.dumps(listener.jetRules))
    self.assertEqual(json.dumps(listener.jetRules), expected)

  def test_jetrule4(self):
    data = a4.InputStream("""
      [Rule4]: 
        (?clm01 has_code ?code).[not(?a1 or b1) and (not ?a2)]
        ->
        (?clm01 value (?a1 + ?b2)).
        (?clm01 value2 ?a1 + ?b2).
        (?clm01 value2 (not ?b2))
      ;
    """)
    listener = self._get_listener(data)

    expected = """{"literals": [], "resources": [], "lookup_tables": [], "jet_rules": [{"name": "Rule4", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?clm01"}, {"type": "identifier", "value": "has_code"}, {"type": "var", "id": "?code"}], "filter": {"type": "binary", "lhs": {"type": "unary", "op": "not", "arg": {"type": "binary", "lhs": {"type": "var", "id": "?a1"}, "op": "or", "rhs": {"type": "identifier", "value": "b1"}}}, "op": "and", "rhs": {"type": "unary", "op": "not", "arg": {"type": "var", "id": "?a2"}}}}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "id": "?clm01"}, {"type": "identifier", "value": "value"}, {"type": "binary", "lhs": {"type": "var", "id": "?a1"}, "op": "+", "rhs": {"type": "var", "id": "?b2"}}]}, {"type": "consequent", "triple": [{"type": "var", "id": "?clm01"}, {"type": "identifier", "value": "value2"}, {"type": "binary", "lhs": {"type": "var", "id": "?a1"}, "op": "+", "rhs": {"type": "var", "id": "?b2"}}]}, {"type": "consequent", "triple": [{"type": "var", "id": "?clm01"}, {"type": "identifier", "value": "value2"}, {"type": "unary", "op": "not", "arg": {"type": "var", "id": "?b2"}}]}]}]}"""
    # print('GOT:',json.dumps(listener.jetRules, indent=2))
    # print()
    # print('COMPACT:',json.dumps(listener.jetRules))
    self.assertEqual(json.dumps(listener.jetRules), expected)

  def test_jetrule5(self):
    data = a4.InputStream("""
      [Rule5]: 
        (?clm01 has_code ?code).
        ->
        (?clm01 usi:"lookup_table" true)
      ;
    """)
    listener = self._get_listener(data)

    expected = """{"literals": [], "resources": [], "lookup_tables": [], "jet_rules": [{"name": "Rule5", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?clm01"}, {"type": "identifier", "value": "has_code"}, {"type": "var", "id": "?code"}]}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "id": "?clm01"}, {"type": "identifier", "value": "usi:lookup_table"}, {"type": "keyword", "value": "true"}]}]}]}"""
    # print('GOT:',json.dumps(listener.jetRules, indent=2))
    # print()
    # print('COMPACT:',json.dumps(listener.jetRules))
    self.assertEqual(json.dumps(listener.jetRules), expected)

  def test_jetrule6(self):
    data = a4.InputStream("""
      [Rule6]: 
        (?clm01 has_code r1).
        (?clm01 has_str r2).
        ->
        (?clm01 usi:"lookup_table" "valueX").
        (?clm01 usi:market "MERGED \\"MARKET\\" CHARGE BACK").
        (?clm01 usi:market text("MERGED \\"MARKET\\" CHARGE BACK"))

      ;
    """)
    listener = self._get_listener(data)

    expected = """{"literals": [], "resources": [], "lookup_tables": [], "jet_rules": [{"name": "Rule6", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?clm01"}, {"type": "identifier", "value": "has_code"}, {"type": "identifier", "value": "r1"}]}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?clm01"}, {"type": "identifier", "value": "has_str"}, {"type": "identifier", "value": "r2"}]}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "id": "?clm01"}, {"type": "identifier", "value": "usi:lookup_table"}, {"type": "text", "id": "valueX"}]}, {"type": "consequent", "triple": [{"type": "var", "id": "?clm01"}, {"type": "identifier", "value": "usi:market"}, {"type": "text", "id": "MERGED \\"MARKET\\" CHARGE BACK"}]}, {"type": "consequent", "triple": [{"type": "var", "id": "?clm01"}, {"type": "identifier", "value": "usi:market"}, {"type": "text", "id": "MERGED \\"MARKET\\" CHARGE BACK"}]}]}]}"""
    # print('GOT:',json.dumps(listener.jetRules, indent=2))
    # print()
    # print('COMPACT:',json.dumps(listener.jetRules))
    self.assertEqual(json.dumps(listener.jetRules), expected)

  def test_jetrule7(self):
    data = a4.InputStream("""
      [Rule7]: 
        (?clm01 has_code int(1)).
        (?clm01 has_str "value").
        (?clm01 hasTrue true).
        ->
        (?clm01 usi:"lookup_table" true).
        (?clm01 has_literal int(1)).
        (?clm01 has_expr (int(1) + long(4)))
      ;
    """)
    listener = self._get_listener(data)

    expected = """{"literals": [], "resources": [], "lookup_tables": [], "jet_rules": [{"name": "Rule7", "properties": {}, "antecedents": [{"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?clm01"}, {"type": "identifier", "value": "has_code"}, {"type": "int", "value": "1"}]}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?clm01"}, {"type": "identifier", "value": "has_str"}, {"type": "text", "id": "value"}]}, {"type": "antecedent", "isNot": false, "triple": [{"type": "var", "id": "?clm01"}, {"type": "identifier", "value": "hasTrue"}, {"type": "keyword", "value": "true"}]}], "consequents": [{"type": "consequent", "triple": [{"type": "var", "id": "?clm01"}, {"type": "identifier", "value": "usi:lookup_table"}, {"type": "keyword", "value": "true"}]}, {"type": "consequent", "triple": [{"type": "var", "id": "?clm01"}, {"type": "identifier", "value": "has_literal"}, {"type": "int", "value": "1"}]}, {"type": "consequent", "triple": [{"type": "var", "id": "?clm01"}, {"type": "identifier", "value": "has_expr"}, {"type": "binary", "lhs": {"type": "int", "value": "1"}, "op": "+", "rhs": {"type": "long", "value": "4"}}]}]}]}"""
    # print('GOT:',json.dumps(listener.jetRules, indent=2))
    # print()
    # print('COMPACT:',json.dumps(listener.jetRules))
    self.assertEqual(json.dumps(listener.jetRules), expected)


if __name__ == '__main__':
  absltest.main()
