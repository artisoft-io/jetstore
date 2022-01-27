"""First test to see if it works!"""

import sys
import json
from absl import flags
from absl.testing import absltest
import antlr4 as a4

from jet_listener import JetListener
from JetRuleParser import JetRuleParser
from JetRuleLexer import JetRuleLexer

FLAGS = flags.FLAGS

class JetRuleListenerTest(absltest.TestCase):

  def _get_listener(self, data: str) -> JetListener:
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


if __name__ == '__main__':
  absltest.main()
