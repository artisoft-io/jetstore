"""Tests for bridge.py"""

import sys
import json
from absl import flags
from absl.testing import absltest

import bridge as api

FLAGS = flags.FLAGS

class BridgeTest(absltest.TestCase):

  def test_load_jetstore(self):

    jetstore_hdlr = api.createJetStoreHandle("jetrule_rete_test.db")
    self.assertIsNotNone(jetstore_hdlr)

    print('Starting ReteSession')
    rete_session = api.createReteSession(jetstore_hdlr, 'ms_factory_test1.jr')
    self.assertIsNotNone(rete_session)

    # print("Creating resources")
    s = api.createResource(rete_session, 'iclaim')
    p = api.createResource(rete_session, 'rdf:type')
    o = api.createResource(rete_session, 'hc:Claim')

    self.assertEqual('iclaim',   api.getResourceName(s))
    self.assertEqual('rdf:type', api.getResourceName(p))
    self.assertEqual('hc:Claim', api.getResourceName(o))

    # print('Verifying that the triple is NOT in the rete session YET')
    self.assertFalse(api.containsTriple(rete_session, s, p, o))

    # print('inserting triple in session')
    self.assertEqual(api.insertTriple(rete_session, s, p, o), 1)

    # print('Verifying that the triple IS in the rete session')
    self.assertTrue(api.containsTriple(rete_session, s, p, o))

    # print("Let's infer!!")
    api.executeRules(rete_session)

    # print('Check we have the expected inferred triple')
    self.assertTrue(api.containsTriple(rete_session, s, p, api.createResource(rete_session, 'hc:BaseClaim')))

    print('Iterate over all triples:')
    itor = api.findAll(rete_session)
    while not api.isEnd(itor):
      s = api.getSubject(itor)
      p = api.getPredicate(itor)
      o = api.getObject(itor)
      print('    (', api.getResourceName(s), ', ', api.getResourceName(p), ', ', api.getResourceName(o), ')')
      api.next(itor)
    
    # print('Releasing the iterator')
    api.disposeIterator(itor)

    # print('Releasing rete session')
    api.deleteReteSession(rete_session)

    # print('Releasing memory')
    api.deleteJetStoreHandle(jetstore_hdlr)

    print("That's all for now!")

if __name__ == '__main__':
  absltest.main()
