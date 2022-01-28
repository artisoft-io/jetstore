"""JetListener test with rule file"""

import sys
import json
from absl import flags
from absl.testing import absltest
import antlr4 as a4

from jet_listener import JetListener
from JetRuleParser import JetRuleParser
from JetRuleLexer import JetRuleLexer

FLAGS = flags.FLAGS

class JetListenerTest2(absltest.TestCase):

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

  # Test data file are accessible using the path relative to the root of the workspace
  def test_rule_file1(self):
    data = a4.FileStream("jetstore-tools/jetrule-grammar/jet_listerner_test_data.jr", encoding='utf-8')
    listener = self._get_listener(data)

    with open("jetstore-tools/jetrule-grammar/jet_listerner_test_data.jr.json", 'rt', encoding='utf-8') as f:
      expected = json.loads(f.read())
    # print('GOT:',json.dumps(listener.jetRules, indent=4))
    # print()
    # print('COMPACT:',json.dumps(listener.jetRules))
    self.assertEqual(json.dumps(listener.jetRules), json.dumps(expected))


if __name__ == '__main__':
  absltest.main()
