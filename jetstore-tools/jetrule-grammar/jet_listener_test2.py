"""JetListener test with rule file"""

import sys
import json
from absl import flags
from absl.testing import absltest
import io
import jetrule_compiler as compiler

FLAGS = flags.FLAGS

class JetListenerTest2(absltest.TestCase):

  def _get_listener_data(self):
    data = compiler.getInput("jetstore-tools/jetrule-grammar/jet_listerner_test_data.jr")
    jetrule_ctx =  compiler.processJetRule(data)
    ctx = compiler.postprocessJetRule(jetrule_ctx)
    data.close()
    return ctx.jetRules

  # Test data file are accessible using the path relative to the root of the workspace
  def test_rule_file1(self):
    jetRules = self._get_listener_data()

    with open("jetstore-tools/jetrule-grammar/jet_listerner_test_data.jr.json", 'rt', encoding='utf-8') as f:
      expected = json.loads(f.read())
    # print('GOT:',json.dumps(jetRules, indent=4))
    # print()
    # print('COMPACT:',json.dumps(jetRules))
    self.assertEqual(json.dumps(jetRules), json.dumps(expected))


if __name__ == '__main__':
  absltest.main()
