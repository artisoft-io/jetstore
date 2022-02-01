from typing import Dict
from absl import app
from absl import flags
import antlr4 as a4
from pathlib import Path
import json
import io
import os
import re
import sys
from jet_listener import JetListener
from jetrule_context import JetRuleContext
from jetrule_validator import JetRuleValidator
from jet_listener_postprocessing import JetRulesPostProcessor
from JetRuleParser import JetRuleParser
from JetRuleLexer import JetRuleLexer
from antlr4.error.ErrorListener import *

FLAGS = flags.FLAGS
# flags.DEFINE_string("jr", "default useful if required", "JetRule file", required=True)
flags.DEFINE_string("jr", None, "JetRule file")
# flags.DEFINE_integer("num_times", 1,
#                      "Number of times to print greeting.")

#TODO Remove this global variable and move it to JetRuleErrorListener class
ERRORS = []
class JetRuleErrorListener(ErrorListener):

  def syntaxError(self, recognizer, offendingSymbol, line, column, msg, e):
      ERRORS.append("line {0}:{1} {2}".format(line, column, msg))


class InputProvider:

  def __init__(self, base_path: Path):
    self.base_path = base_path

  # default provider
  def getRuleFile(self, fname: str):
    path = os.path.join(self.base_path, fname)
    return getInput(path)

def getInput(fname: str) -> io.StringIO:
  return open(fname, 'rt', encoding='utf-8')

def readInput(fin: io.StringIO, in_provider: InputProvider, pat, fout: io.StringIO) -> None:
    while True:
      jline = fin.readline()
      if not jline:
        fin.close()
        break

      m = pat.match(jline)
      if m:
          print('Match found: ', m.group(1))
          readInput(in_provider.getRuleFile(m.group(1)), in_provider, pat, fout)
      else:
        fout.write(jline)


# read and process the rule file
# ---------------------------------------------------------------------------------------
def readJetRuleFile(fname: str, in_provider: InputProvider) -> Dict[str, object]:
  pat = re.compile(r'import\s*"([a-zA-Z0-9_\/.-]*)"')
  fout = io.StringIO()

  # read recursively the input file and it's imports
  readInput(in_provider.getRuleFile(fname), in_provider, pat, fout)
  fout.seek(0)

  jetrule_ctx = processJetRule(fout)
  fout.close()
  return jetrule_ctx.jetRules


# process the input jetrule buffer
# ---------------------------------------------------------------------------------------
def processJetRule(input: io.StringIO) -> JetRuleContext:
  # reset
  ERRORS.clear()

  # input data
  input_data =  a4.InputStream(input.read())

  # lexer
  lexer = JetRuleLexer(input_data)
  stream = a4.CommonTokenStream(lexer)
  
  # parser
  parser = JetRuleParser(stream)
  parser.removeErrorListeners() 

  errorListener = JetRuleErrorListener()
  parser.addErrorListener(errorListener)  

  # build the tree
  tree = parser.jetrule()

  # evaluator
  listener = JetListener()
  walker = a4.ParseTreeWalker()
  walker.walk(listener, tree)

  errors = []
  for err in ERRORS:
    errors.append(err)

  ctx = JetRuleContext(listener.jetRules, errors)
  return ctx


# post-process the input jetrule buffer
# ---------------------------------------------------------------------------------------
def postprocessJetRule(jetrule_ctx: JetRuleContext) -> JetRuleContext:

  if jetrule_ctx.ERROR:
    return jetrule_ctx

  # augment the output with post processor
  postProcessor = JetRulesPostProcessor(jetrule_ctx)
  postProcessor.createResourcesForLookupTables()
  postProcessor.mapVariables()
  postProcessor.addNormalizedLabels()
  postProcessor.addLabels()
  return jetrule_ctx


# validate the input jetrule buffer
# ---------------------------------------------------------------------------------------
def validateJetRule(jetrule_ctx: JetRuleContext) -> bool:

  # augment the output with post processor
  validator = JetRuleValidator(jetrule_ctx)
  return validator.validateJetRule()


# command line invocation
# ---------------------------------------------------------------------------------------
def main(argv):
  del argv  # Unused.

  path = Path(FLAGS.jr)
  if not os.path.exists(path) or not os.path.isfile(path):
    print('ERROR: JetRule file {0} does not exist or is not a file'.format(path))
    sys.exit('ERROR: JetRule file does not exist or is not a file') 

  jetrules = readJetRuleFile(path)

  # Save the JetRule data structure
  with open(str(path)+'.json', 'wt', encoding='utf-8') as f:
    f.write(json.dumps(jetrules, indent=4))

  print('Result saved to {0}.json'.format(path))


if __name__ == '__main__':
  app.run(main)