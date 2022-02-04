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
import queue
from jet_listener import JetListener
from jetrule_context import JetRuleContext
from jetrule_validator import JetRuleValidator
from jet_listener_postprocessing import JetRulesPostProcessor
from jetrule_optimizer import JetRuleOptimizer
from jetrule_rete import JetRuleRete
from JetRuleParser import JetRuleParser
from JetRuleLexer import JetRuleLexer
from antlr4.error.ErrorListener import *

FLAGS = flags.FLAGS
# flags.DEFINE_string("jr", "default useful if required", "JetRule file", required=True)
flags.DEFINE_string("in_file", None, "JetRule file")
flags.DEFINE_string("base_path", None, "Base path for in_file, out_file ad all imported files")
flags.DEFINE_string("out_file", None, "JetRule Rete Configuration")
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
  def getRuleFile(self, fname: str) -> str:
    path = os.path.join(self.base_path, fname)
    with open(path, 'rt', encoding='utf-8') as f:
      data = f.read()
    return data

class JetRuleCompiler:

  def __init__(self):
    self.jetrule_ctx = None
    self.global_line_nbr = 0
    self.imported_rule_files = None
    self.imported_rule_file_names = None
    self.processing_rule_files_q = None
    self.a4_err_pat = re.compile(r'line\s+(\d+):(\d+)\s+(.*)')

  # =====================================================================================
  # Internal methods
  # -------------------------------------------------------------------------------------
  def _readInput(self, in_data: str, in_provider: InputProvider, pat, fout: io.StringIO) -> None:
    for line in in_data.splitlines(keepends=True):
      m = pat.match(line)
      if m:
          fname = m.group(1)
          # Pause the current file
          file_info = self.processing_rule_files_q.get()
          current_file_name = file_info['fname']
          file_info['end_pos'] = self.global_line_nbr
          file_offset = self.global_line_nbr - file_info['start_pos'] + file_info['file_offset'] + 1

          if fname in self.imported_rule_file_names:
            # print('File already imported:', fname, ', skipping it')

            # Put another entry for the current file for the remaining statements
            file_info = {'fname': current_file_name, 'start_pos': self.global_line_nbr, 'file_offset': file_offset}
            self.imported_rule_files.append(file_info)
            self.processing_rule_files_q.put(file_info)

          else:
            # print('Importing file: ', fname)

            # Put a new entry to track new file to import
            file_info = {'fname': fname, 'start_pos': self.global_line_nbr, 'file_offset': 0}
            self.imported_rule_files.append(file_info)
            self.imported_rule_file_names.add(fname)
            self.processing_rule_files_q.put(file_info)
            self._readInput(in_provider.getRuleFile(fname), in_provider, pat, fout)

            # Put another entry for the current file for the remaining statements
            file_info = {'fname': current_file_name, 'start_pos': self.global_line_nbr, 'file_offset': file_offset}
            self.imported_rule_files.append(file_info)
            self.processing_rule_files_q.put(file_info)
      else:
        fout.write(line)
        # keep track of line nbr
        self.global_line_nbr += 1

    # Done with current file
    file_info = self.processing_rule_files_q.get()
    file_info['end_pos'] = self.global_line_nbr


  # =====================================================================================
  # compileJetRuleFile
  # -------------------------------------------------------------------------------------
  # All-in-one processing of jetrule file
  # initalize self.jetrule_ctx
  # Compile file fname and process import statement recursively
  # return the jetrule data structure
  # ---------------------------------------------------------------------------------------
  def compileJetRuleFile(self, fname: str, in_provider: InputProvider) -> Dict[str, object]:
    self.processJetRuleFile(fname, in_provider)
    return self._compileReminderTasks()

  # compile the input jetrule buffer
  # return the jetrule data structure for convenience
  # ---------------------------------------------------------------------------------------
  def compileJetRule(self, input: str) -> Dict[str, object]:
    self.processJetRule(input)
    return self._compileReminderTasks()

  # Compile the JetRule beyond the initial processing
  def _compileReminderTasks(self) -> Dict[str, object]:
    if not self.jetrule_ctx.ERROR:
      self.postprocessJetRule()
      self.validateJetRule()
      self.optimizeJetRule()
      self.addReteMarkingJetRule()      
    return self.jetrule_ctx.jetRules


  # =====================================================================================
  # processJetRuleFile
  # -------------------------------------------------------------------------------------
  # Read input jetrule file and returns the initial jetrule data structure
  # return the jetrule data structure for convenience
  # ---------------------------------------------------------------------------------------
  def processJetRuleFile(self, fname: str, in_provider: InputProvider) -> Dict[str, object]:
    pat = re.compile(r'import\s*"([a-zA-Z0-9_\/.-]*)"')
    fout = io.StringIO()

    # keep track of the imports for error reporting
    #   'start_pos' is the first line of the rule file (incl)
    #   'end_pos' is the last line of the rule file (excl), ie. +1
    self.global_line_nbr = 1
    self.processing_rule_files_q = queue.LifoQueue()
    file_info = {'fname': fname, 'start_pos': 1, 'file_offset': 0}
    self.imported_rule_files = [file_info]
    self.imported_rule_file_names = set([fname])
    self.processing_rule_files_q.put(file_info)

    # read recursively the input file and it's imports
    in_file = in_provider.getRuleFile(fname)
    self._readInput(in_file, in_provider, pat, fout)
    fout.seek(0)
    data = fout.read()
    fout.close()
    self.processJetRule(data)
    return self.jetrule_ctx.jetRules

  # process the input jetrule buffer
  # return the jetrule data structure for convenience
  # ---------------------------------------------------------------------------------------
  def processJetRule(self, input: str) -> Dict[str, object]:
    # reset
    ERRORS.clear()

    # input data
    input_data =  a4.InputStream(input)

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
      # post process antlr4 errors to put reference to the included file
      # print('** got err:',err)
      if self.imported_rule_files:
        m = self.a4_err_pat.match(err)
        if m:
          line_nbr = int(m.group(1))
          col_nbr = int(m.group(2))
          err_msg = m.group(3)
          # print('** matched on err, line',line_nbr, 'col', col_nbr)

          # find which file this error falls into
          fname = None
          for file_info in self.imported_rule_files:
            if line_nbr >= file_info['start_pos'] and line_nbr < file_info['end_pos']:
              fname = file_info['fname']
              break

          if fname:
            errors.append("Error in file '{0}' line {1}:{2} {3}".format(fname, line_nbr-file_info['start_pos']+file_info['file_offset']+1, col_nbr+1, err_msg))
          else:
            raise Exception('Oops something is wrong with the import file in JetRuleCompiler')
      else:
        errors.append(err)

    self.jetrule_ctx = JetRuleContext(listener.jetRules, errors)
    self.jetrule_ctx.state = JetRuleContext.STATE_PROCESSED
    return self.jetrule_ctx.jetRules


  # post-process the input jetrule buffer
  # return the jetrule data structure for convenience
  # ---------------------------------------------------------------------------------------
  def postprocessJetRule(self) -> Dict[str, object]:
    assert self.jetrule_ctx, 'Must have a valid jetrule context, '
    'call processJetRule() or processJetRuleFile() first'
    assert self.jetrule_ctx.state==JetRuleContext.STATE_PROCESSED, 'Must call processJetRule() first'

    if self.jetrule_ctx.ERROR:
      return self.jetrule_ctx

    # augment the output with post processor
    postProcessor = JetRulesPostProcessor(self.jetrule_ctx)
    postProcessor.createResourcesForLookupTables()
    postProcessor.mapVariables()
    postProcessor.addNormalizedLabels()
    postProcessor.addLabels()
    self.jetrule_ctx.state = JetRuleContext.STATE_POSTPROCESSED
    return self.jetrule_ctx.jetRules


  # validate the input jetrule buffer
  # ---------------------------------------------------------------------------------------
  def validateJetRule(self) -> bool:
    assert self.jetrule_ctx, 'Must have a valid jetrule context, '
    'call processJetRule() or processJetRuleFile() first'
    assert self.jetrule_ctx.state == JetRuleContext.STATE_POSTPROCESSED, 'Must call postprocessJetRule() first'

    # augment the output with post processor
    validator = JetRuleValidator(self.jetrule_ctx)
    self.jetrule_ctx.state = JetRuleContext.STATE_VALIDATED
    return validator.validateJetRule()


  # Optimize the jetrules
  # ---------------------------------------------------------------------------------------
  def optimizeJetRule(self) -> Dict[str, object]:
    assert self.jetrule_ctx, 'Must have a valid jetrule context, '
    'call processJetRule() or processJetRuleFile() first'
    assert self.jetrule_ctx.state >= JetRuleContext.STATE_POSTPROCESSED, 'Must call at least postprocessJetRule() first'

    optimizer = JetRuleOptimizer(self.jetrule_ctx)
    optimizer.optimizeJetRules()
    self.jetrule_ctx.state = JetRuleContext.STATE_OPTIMIZED
    return self.jetrule_ctx.jetRules


  # Annotate the jetrules for Rete Network
  # ---------------------------------------------------------------------------------------
  def addReteMarkingJetRule(self) -> Dict[str, object]:
    assert self.jetrule_ctx, 'Must have a valid jetrule context, '
    'call processJetRule() or processJetRuleFile() first'
    assert self.jetrule_ctx.state >= JetRuleContext.STATE_POSTPROCESSED, 'Must call at least postprocessJetRule() first'

    rete = JetRuleRete(self.jetrule_ctx)
    rete.addReteMarkup()
    self.jetrule_ctx.state = JetRuleContext.STATE_RETE_MARKINGS
    return self.jetrule_ctx.jetRules


# =======================================================================================
# command line invocation
# ---------------------------------------------------------------------------------------
def main(argv):
  del argv  # Unused.

  
  in_fname = Path(FLAGS.in_file)
  base_path_name = Path(FLAGS.base_path)
  out_fname = Path(FLAGS.out_file)

  base_path = Path(base_path_name)
  path = os.path.join(base_path, in_fname)
  path = os.path.abspath(path)
  if not os.path.exists(path) or not os.path.isfile(path):
    print('ERROR: JetRule file {0} does not exist or is not a file'.format(path))
    sys.exit('ERROR: JetRule file does not exist or is not a file') 

  in_provider = InputProvider(base_path)
  compiler = JetRuleCompiler()
  jetrules = compiler.compileJetRuleFile(in_fname, in_provider)

  # Save the JetRule data structure
  with open(str(path)+'.json', 'wt', encoding='utf-8') as f:
    f.write(json.dumps(jetrules, indent=4))

  print('Result saved to {0}.json'.format(path))


if __name__ == '__main__':
  app.run(main)