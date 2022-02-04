"""JetListener test with rule file"""

import sys
import json
from absl import flags
from absl.testing import absltest

from jetrule_compiler import JetRuleCompiler, InputProvider

FLAGS = flags.FLAGS

class JetListenerTest2(absltest.TestCase):

  # Test data file are accessible acmeng the path relative to the root of the workspace
  def test_rule_file1(self):
    provider = InputProvider("jetstore-tools/jetrule-grammar/")
    compiler = JetRuleCompiler()
    compiler.processJetRuleFile("jet_listerner_test_data.jr", provider)
    compiler.postprocessJetRule()
    compiler.validateJetRule()
    compiler.optimizeJetRule()
    jetRules = compiler.addReteMarkingJetRule()


    with open("jetstore-tools/jetrule-grammar/jet_listerner_test_data.jr.json", 'rt', encoding='utf-8') as f:
      expected = json.loads(f.read())
    # print('GOT:',json.dumps(jetRules, indent=4))
    # with open("jetstore-tools/jetrule-grammar/jet_listerner_test_data.jr.json", 'wt', encoding='utf-8') as f:
    #   f.write(json.dumps(jetRules, indent=4))
    # print()
    # print('COMPACT:',json.dumps(jetRules))

    self.assertEqual(json.dumps(jetRules), json.dumps(expected))


if __name__ == '__main__':
  absltest.main()
