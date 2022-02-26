"""JetListener test with rule file"""

import sys
import json
from absl import flags
from absl.testing import absltest

from jetstore_api import createJetStoreHandle, \
      deleteJetStoreHandle, createReteSession, deleteReteSession, \
      createResource, createText, createInt, \
      getResourceType, getResourceName, getIntValue, getTextValue, \
      insertTriple, containsTriple, executeRules, \
      findAll, isEnd, next, getSubject, getPredicate, getObject, \
      disposeIterator

FLAGS = flags.FLAGS

class JetStoreTest(absltest.TestCase):

  def test_load_jetstore(self):

    jetstore_hdlr = createJetStoreHandle("jets/rete/rete_test_db/jetrule_rete_test.db")
    self.assertIsNotNone(jetstore_hdlr)

    print('Starting ReteSesson')
    rete_session = createReteSession(jetstore_hdlr, 'ms_factory_test1.jr')
    self.assertIsNotNone(rete_session)

    # print("Creating resources")
    s = createResource(rete_session, 'iclaim')
    p = createResource(rete_session, 'rdf:type')
    o = createResource(rete_session, 'hc:Claim')
    # print('Asserting triple (', getResourceName(s), ', ', getResourceName(p), ', ', getResourceName(o), ')')

    self.assertEqual('iclaim', getResourceName(s))
    self.assertEqual('rdf:type', getResourceName(p))
    self.assertEqual('hc:Claim', getResourceName(o))

    # print('Verifying that the triple is NOT in the rete session YET')
    self.assertFalse(containsTriple(rete_session, s, p, o))

    # print('inserting triple in session')
    self.assertEqual(insertTriple(rete_session, s, p, o), 1)

    # print('Verifying that the triple IS in the rete session')
    self.assertTrue(containsTriple(rete_session, s, p, o))

    # print("Let's infer!!")
    executeRules(rete_session)

    # print('Check we have the expected inferred triple')
    self.assertTrue(containsTriple(rete_session, s, p, createResource(rete_session, 'hc:BaseClaim')))

    print('Iterate over all triples:')
    itor = findAll(rete_session)
    while not isEnd(itor):
      s = getSubject(itor)
      p = getPredicate(itor)
      o = getObject(itor)
      print('    (', getResourceName(s), ', ', getResourceName(p), ', ', getResourceName(o), ')')
      next(itor)
    
    # print('Releasing the iterator')
    disposeIterator(itor)

    # print('Releasing rete session')
    deleteReteSession(rete_session)

    # print('Releasing memory')
    deleteJetStoreHandle(jetstore_hdlr)


    print("That's all for now!")


if __name__ == '__main__':
  absltest.main()
