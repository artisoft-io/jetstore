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
    compiler.compileJetRuleFile("jetrule_main_test.jr", provider)

    for k in compiler.jetrule_ctx.errors:
      print(k)
    print()
    self.assertEqual(compiler.jetrule_ctx.ERROR, False)

    with open("jetstore-tools/jetrule-grammar/jetrule_main_test.jr.json", 'rt', encoding='utf-8') as f:
      expected = json.loads(f.read())

    # TEST HERE
    self.assertEqual(json.dumps(compiler.jetrule_ctx.jetRules), json.dumps(expected))

    # print('GOT RULES:',json.dumps(compiler.jetrule_ctx.jetRules, indent=4))
    # print()
    # print('GOT RETE:',json.dumps(compiler.jetrule_ctx.jetReteNodes, indent=4))

    with open("jetstore-tools/jetrule-grammar/jetrule_main_test.jrc.json", 'rt', encoding='utf-8') as f:
      expected = json.loads(f.read())

    # TEST HERE
    self.assertEqual(json.dumps(compiler.jetrule_ctx.jetReteNodes), json.dumps(expected))

  def test_rule_file2(self):
    provider = InputProvider("jets/rete/rete_test_db/")
    compiler = JetRuleCompiler()
    jetRules = compiler.compileJetRuleFile("ms_factory_test2.jr", provider).jetRules

    # print('GOT')
    for k in compiler.jetrule_ctx.errors:
      print(k)
    print()
    self.assertEqual(compiler.jetrule_ctx.ERROR, False)

    expected = 'xx'
    with open("jets/rete/rete_test_db/ms_factory_test2.jrc.json", 'rt', encoding='utf-8') as f:
      expected = json.loads(f.read())

    # print('GOT RETE:',json.dumps(compiler.jetrule_ctx.jetReteNodes, indent=4))
    self.assertEqual(json.dumps(compiler.jetrule_ctx.jetReteNodes), json.dumps(expected))


if __name__ == '__main__':
  absltest.main()
