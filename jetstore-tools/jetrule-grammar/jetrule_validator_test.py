"""JetRuleValidator tests"""

import sys
import json
from typing import Dict
from absl import flags
from absl.testing import absltest
import io

import jetrule_compiler as compiler

FLAGS = flags.FLAGS

class JetRulesValidatorTest(absltest.TestCase):

  def _get_augmented_data(self, fname: str) -> Dict[str, object]:

    in_provider = compiler.InputProvider('jetstore-tools/jetrule-grammar')
    jetRulesSpec =  compiler.readJetRuleFile(fname, in_provider)
    return compiler.postprocessJetRule(jetRulesSpec)

  def test_import1(self):
    data = "import_test1.jr"
    postprocessed_data = self._get_augmented_data(data)

    # validate the whole result
    expected = """{"literals": [{"type": "int", "id": "isTrue", "value": "1"}, {"type": "text", "id": "NOT_IN_CONTRACT", "value": "NOT COVERED IN CONTRACT"}, {"type": "text", "id": "EXCLUDED_STATE", "value": "STATE"}], "resources": [{"id": "usi:ProcedureLookup", "type": "resource", "value": "usi:ProcedureLookup"}, {"id": "cPROC_RID", "type": "resource", "value": "PROC_RID"}, {"id": "cPROC_MID", "type": "resource", "value": "PROC_MID"}, {"id": "cPROC_DESC", "type": "resource", "value": "PROC_DESC"}], "lookup_tables": [{"name": "usi:ProcedureLookup", "table": "usi__cm_proc_codes", "key": ["PROC_CODE"], "columns": ["PROC_RID", "PROC_MID", "PROC_DESC"], "resources": ["cPROC_RID", "cPROC_MID", "cPROC_DESC"]}], "jet_rules": []}"""
    # print('GOT:',json.dumps(postprocessed_data, indent=2))
    # print()
    # print('COMPACT:',json.dumps(postprocessed_data))

    # validate the whole result
    self.assertEqual(json.dumps(postprocessed_data), expected)

if __name__ == '__main__':
  absltest.main()
