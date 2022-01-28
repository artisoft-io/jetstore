"""JetListener test with rule file"""

import sys
import json
from absl import flags
from absl.testing import absltest
import antlr4 as a4

from jet_listener import JetListener
from jet_listener_postprocessing import JetRulesPostProcessor
from JetRuleParser import JetRuleParser
from JetRuleLexer import JetRuleLexer

FLAGS = flags.FLAGS

class JetRulesPostProcessorTest2(absltest.TestCase):

  def _get_augmented_data(self, data) -> JetListener:
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

    # augment the output with post processor
    postProcessor = JetRulesPostProcessor(listener.jetRules)
    postProcessor.mapVariables()
    postProcessor.addNormalizedLabels()
    postProcessor.addLabels()

    return postProcessor.jetRules

  # Test data file are accessible using the path relative to the root of the workspace
  def test_rule_file1(self):
    data = a4.FileStream("jetstore-tools/jetrule-grammar/jet_listener_postprocessing_test_data.jr", encoding='utf-8')
    postprocessed_data = self._get_augmented_data(data)
    # print('GOT:',json.dumps(postprocessed_data, indent=2))

    with open("jetstore-tools/jetrule-grammar/jet_listener_postprocessing_test_data.jr.json", 'rt', encoding='utf-8') as f:
      expected = json.loads(f.read())

    self.assertEqual(json.dumps(postprocessed_data), json.dumps(expected))


if __name__ == '__main__':
  absltest.main()
