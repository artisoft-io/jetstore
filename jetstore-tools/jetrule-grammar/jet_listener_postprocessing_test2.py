"""JetListener test with rule file"""

import sys
import json
from absl import flags
from absl.testing import absltest
import antlr4 as a4

from jet_listener import JetListener
import jetrule_compiler as compiler

FLAGS = flags.FLAGS

class JetRulesPostProcessorTest2(absltest.TestCase):

  def _get_augmented_data(self) -> JetListener:
    data = compiler.getInput("jetstore-tools/jetrule-grammar/jet_listener_postprocessing_test_data.jr")
    jetRulesSpec =  compiler.processJetRule(data)
    ctx = compiler.postprocessJetRule(jetRulesSpec)
    data.close()
    return ctx.jetRules

  # Test data file are accessible using the path relative to the root of the workspace
  def test_rule_file1(self):
    postprocessed_data = self._get_augmented_data()
    # print('GOT:',json.dumps(postprocessed_data, indent=2))

    with open("jetstore-tools/jetrule-grammar/jet_listener_postprocessing_test_data.jr.json", 'rt', encoding='utf-8') as f:
      expected = json.loads(f.read())

    self.assertEqual(json.dumps(postprocessed_data), json.dumps(expected))


if __name__ == '__main__':
  absltest.main()
