from absl import app
from absl import flags
from pathlib import Path
import sys
from jetrule_lookup_sqlite import JetRuleLookupSQLite


FLAGS = flags.FLAGS
flags.DEFINE_string("base_path", None, "Base path for lookup_path, out_file and all imported files")
flags.DEFINE_string("lookup_db", 'jetrule_lookup.db', "JetRule lookup")
flags.DEFINE_string("rete_db", 'jetrule_rete.db', "JetRule rete config")
flags.DEFINE_bool("append_db", False, "Append new tables to JetRule lookup db if already exists, already existing tables will be dropped and recreated if provided again by rete_db", short_name='a')

# =======================================================================================
# command line invocation
# ---------------------------------------------------------------------------------------
def main(argv):
  del argv  # Unused.

  base_path = Path(FLAGS.base_path)
  lookup_db = FLAGS.lookup_db
  rete_db   = FLAGS.rete_db
  append_db = FLAGS.append_db

  lookup_db_helper = JetRuleLookupSQLite(base_path)
  err = lookup_db_helper.saveLookups(lookup_db, rete_db, append_db)
  if err:
    print('ERROR while saving Lookup file to lookup_db: {0}.'.format(str(err)))
    sys.exit('ERROR while saving Lookup file to lookup_db: {0}.'.format(str(err))) 

if __name__ == '__main__':
  flags.mark_flag_as_required('base_path')
  app.run(main)