"""JetListener test with rule file"""

import sys
import json
from absl import flags
from absl.testing import absltest

from jetstore_api import createJetStoreHandle, \
      deleteJetStoreHandle, createReteSession, deleteReteSession

FLAGS = flags.FLAGS

class JetStoreTest(absltest.TestCase):

  def test_load_jetstore(self):

    jetstore_hdlr = createJetStoreHandle("jets/rete/rete_test_db/jetrule_rete_test.db")

    if jetstore_hdlr:
      print('GOT IT!!!')
    self.assertIsNotNone(jetstore_hdlr)

    print('Starting ReteSesson')
    rete_session = createReteSession(jetstore_hdlr, 'ms_factory_test1.jr')

    print('Releasing rete session')
    deleteReteSession(rete_session)

    print('Releasing memory')
    deleteJetStoreHandle(jetstore_hdlr)


    print("That's all for now!")


if __name__ == '__main__':
  absltest.main()
