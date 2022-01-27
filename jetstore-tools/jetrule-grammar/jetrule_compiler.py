from absl import app
from absl import flags
import antlr4 as a4
from pathlib import Path
import json
import os
import sys
from jet_listener import JetListener
from jet_listener_postprocessing import JetRulesPostProcessor
from JetRuleParser import JetRuleParser
from JetRuleLexer import JetRuleLexer
from JetRuleListener import JetRuleListener

FLAGS = flags.FLAGS
flags.DEFINE_string("jr", None, "JetRule file", required=True)
# flags.DEFINE_integer("num_times", 1,
#                      "Number of times to print greeting.")


def main(argv):
  del argv  # Unused.

  path = Path(FLAGS.jr)
  if not os.path.exists(path) or not os.path.isfile(path):
    print('ERROR: JetRule file {0} does not exist or is not a file'.format(path))
    sys.exit('ERROR: JetRule file does not exist or is not a file') 

  data =  a4.FileStream(path, encoding='utf-8')
    
  # lexer
  lexer = JetRuleLexer(data)
  stream = a4.CommonTokenStream(lexer)
  
  # parser
  parser = JetRuleParser(stream)
  tree = parser.jetrule()

  # evaluator
  listener = JetListener()
  walker = a4.ParseTreeWalker()
  walker.walk(listener, tree)

  # Save the JetRule data structure
  with open(str(path)+'.json', 'wt', encoding='utf-8') as f:
    f.write(json.dumps(listener.jetRules, indent=4))

  print('Result saved to {0}.json'.format(path))



if __name__ == '__main__':
  app.run(main)