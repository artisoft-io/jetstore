from absl.testing import absltest
from absl import flags
import io

from pathlib import Path
from jetrule_lookup_sqlite import JetRuleLookupSQLite


FLAGS = flags.FLAGS

base_path = Path("test_data")

lookup_loader = JetRuleLookupSQLite(base_path='/go/jetstore/jets/compiler/test_data')
lookup_loader.saveLookups(lookup_db = "lookup_loader_test_lookup.db", rete_db = "lookup_loader_test_rete.db")



class JetRulesLookupLoaderTest(absltest.TestCase):

  def test_basic_lookup_load(self):

    
    basic_table = lookup_loader.get_lookup(table_name='acme__ba__sic', lookup_db = "lookup_loader_test_lookup.db")

    self.assertEqual(len(basic_table), 11)  
    self.assertEqual(len(basic_table[0].keys()), 10)  
    self.assertEqual(
      {'BASIC_TEST_LONG','BASIC_TEST_DATE','jets:key','BASIC_TEST_BOOL','BASIC_TEST_INT','__key__','BASIC_TEST_TEXT','BASIC_TEST_UINT','BASIC_TEST_DOUBLE','BASIC_TEST_ULONG'},
      set(basic_table[0].keys())
    )  

    bool_list  = []  
    expected_bool_list  = [0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1]  
    key_list = [] 
    expected_key_list = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10] 
    double_list = [] 
    expected_double_list = [0.5, 0.5, 1.0, 1.1, None, 1.1, 1.1, 1.1, 1.1, 1.1, 1.1]   
    for row in basic_table:
      self.assertEqual(row['jets:key'],'0')
      self.assertEqual(row['BASIC_TEST_DATE'],'1-1-2022')
      self.assertEqual(abs(int(row['BASIC_TEST_INT'])),42)
      self.assertEqual(abs(int(row['BASIC_TEST_LONG'])),1549251913000)      
      self.assertEqual(int(row['BASIC_TEST_UINT']),42)
      self.assertEqual(int(row['BASIC_TEST_ULONG']),1549251913000)
      self.assertEqual(row['BASIC_TEST_TEXT'],'BASIC, TEST, "TEXT", VALUE')
      bool_list.append(row['BASIC_TEST_BOOL']) 
      double_list.append(row['BASIC_TEST_DOUBLE']) 
      key_list.append(row['__key__']) 

    self.assertEqual(expected_bool_list, bool_list)
    self.assertEqual(expected_double_list, double_list)
    self.assertEqual(expected_key_list, key_list)


  def test_composite_lookup_load(self):
    
    composite_table = lookup_loader.get_lookup(table_name='acme__composite', lookup_db = "lookup_loader_test_lookup.db")

    self.assertEqual(len(composite_table), 2)  
    self.assertEqual(len(composite_table[0].keys()), 8)  
    self.assertEqual(
      {'jets:key','__key__','COMPOSITE_TEST_KEY_2','COMPOSITE_TEST_INT','COMPOSITE_TEST_TEXT','COMPOSITE_TEST_BOOL','COMPOSITE_TEST_LONG','COMPOSITE_TEST_DATE'},
      set(composite_table[0].keys())
    )  

    bool_list  = []  
    exected_bool_list  = [0, 0]  
    key_list = [] 
    expected_key_list = [0, 1]     
    jets_key_list = [] 
    expected_jets_key_list = ['123', '124'] 
    for row in composite_table:
      self.assertEqual(row['COMPOSITE_TEST_DATE'],'1-1-2022')
      self.assertEqual(row['COMPOSITE_TEST_INT'],42)
      self.assertEqual(row['COMPOSITE_TEST_TEXT'],'COMPOSITE, TEST, TEXT, VALUE')
      self.assertEqual(row['COMPOSITE_TEST_KEY_2'],2)
      bool_list.append(row['COMPOSITE_TEST_BOOL']) 
      key_list.append(row['__key__']) 
      jets_key_list.append(row['jets:key']) 

    self.assertEqual(exected_bool_list, bool_list)
    self.assertEqual(expected_key_list, key_list)   
    self.assertEqual(expected_jets_key_list, jets_key_list)   

if __name__ == '__main__':
  absltest.main()