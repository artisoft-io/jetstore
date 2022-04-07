"""Tests for jets.py"""

from absl.testing import absltest
import jets

class JetsTest(absltest.TestCase):

  def test_load_jetstore(self):

    print('Create and load JetStore Factory')
    jets_factory = jets.create_jetstore_factory('jetrule_rete_test.db', 'jetrule_rete_test.db')

    print('Create ReteSession')
    rete_session = jets_factory.create_rete_session("ms_factory_test1.jr")

    print('Create Resources')
    s = rete_session.create_resource('iclaim')
    p = rete_session.create_resource('rdf:type')
    o = rete_session.create_resource('hc:Claim')
    self.assertEqual(s.get_name(), 'iclaim')
    self.assertEqual(p.get_name(), 'rdf:type')
    self.assertEqual(o.get_name(), 'hc:Claim')

    # Check it's not yet in the session
    self.assertFalse(rete_session.contains_triple(s, p, o))

    # Insert in the session
    self.assertTrue(rete_session.insert_triple(s, p, o))

    # So now, it should be there
    self.assertTrue(rete_session.contains_triple(s, p, o))

    # Execute rule
    rete_session.execute_rules()

    # Iterate over all triples
    print('The ReteSession contains, after execute_rules:')
    itor = rete_session.find_all()
    while(not itor.is_end()):
      print('    (', itor.get_subject(), ', ', itor.get_predicate(), ', ', itor.get_object(), ')')
      itor.next()

    # Check that we have the expected inferred triple
    self.assertTrue(rete_session.contains_triple(s, p, rete_session.create_resource('hc:BaseClaim')))

    print("That's All Folks!")

if __name__ == '__main__':
  absltest.main()

