from absl import app
from absl import flags
from pathlib import Path
import sys
from jetrule_lookup_sqlite import JetRuleLookupSQLite


FLAGS = flags.FLAGS
flags.DEFINE_string("base_path", None, "Base path for lookup_path, out_file and all imported files")


# =======================================================================================
# command line invocation
# ---------------------------------------------------------------------------------------
def main(argv):
  del argv  # Unused.

  base_path = Path(FLAGS.base_path)

  lookup_db_helper = JetRuleLookupSQLite(base_path=base_path)
  err = lookup_db_helper.saveLookups()
  if err:
    print('ERROR while saving Lookup file to lookup_db: {0}.'.format(str(err)))
    sys.exit('ERROR while saving Lookup file to lookup_db: {0}.'.format(str(err))) 

if __name__ == '__main__':
  flags.mark_flag_as_required('base_path')
  app.run(main)