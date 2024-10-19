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
from jetrule_post_processor import JetRulesPostProcessor
from jetrule_optimizer import JetRuleOptimizer
from jetrule_rete import JetRuleRete
from jetrule_rete_sqlite import JetRuleReteSQLite
from JetRuleParser import JetRuleParser
from JetRuleLexer import JetRuleLexer
from antlr4.error.ErrorListener import *

FLAGS = flags.FLAGS
# flags.DEFINE_string("jr", "default useful if required", "JetRule file", required=True)
flags.DEFINE_string("in_file", None, "JetRule file")
flags.DEFINE_string("base_path", None, "Base path for in_file, out_file and all imported files")
flags.DEFINE_bool("save_json", False, "Save JetRule json output file", short_name='s')
# flags.DEFINE_integer("num_times", 1,
#                      "Number of times to print greeting.")

class JetRuleErrorListener(ErrorListener):

  def __init__(self):
    self.ERRORS = []

  def syntaxError(self, recognizer, offendingSymbol, line, column, msg, e):
    # print("*** line " + str(line) + ":" + str(column) + " " + msg, file=sys.stderr)
    self.ERRORS.append("line {0}:{1} {2}".format(line, column, msg))


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
    self.main_rule_fname = None     # will be used a rete config key within the workspace
    self.global_line_nbr = 0
    self.imported_file_info_list = None
    self.imported_file_name_set = None
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
          # replace the import statement with a compiler directive
          # put a jet compiler directive to mark the file being imported
          fout.write('@JetCompilerDirective source_file = "{0}";\n'.format(fname))
          self.global_line_nbr += 1

          # Pause the current file
          file_info = self.processing_rule_files_q.get()
          current_file_name = file_info['fname']
          file_info['end_pos'] = self.global_line_nbr
          file_offset = self.global_line_nbr - file_info['start_pos'] + file_info['file_offset']

          if fname in self.imported_file_name_set:
            print('File already imported:', fname, ', skipping it')

            # Put another entry for the current file for the remaining statements
            file_info = {'fname': current_file_name, 'start_pos': self.global_line_nbr, 'file_offset': file_offset}
            self.imported_file_info_list.append(file_info)
            self.processing_rule_files_q.put(file_info)

          else:
            print('Importing file: ', fname)

            # Put a new entry to track new file to import
            file_info = {'fname': fname, 'start_pos': self.global_line_nbr, 'file_offset': 0}
            self.imported_file_info_list.append(file_info)
            self.imported_file_name_set.add(fname)
            self.processing_rule_files_q.put(file_info)
            self._readInput(in_provider.getRuleFile(fname), in_provider, pat, fout)

            # Add a directive to indicate to the parser listener that we're back to including file
            fout.write('@JetCompilerDirective source_file = "{0}";\n'.format(current_file_name))
            self.global_line_nbr += 1

            # Put another entry for the current file for the remaining statements
            file_info = {'fname': current_file_name, 'start_pos': self.global_line_nbr, 'file_offset': file_offset}
            self.imported_file_info_list.append(file_info)
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
  def compileJetRuleFile(self, fname: str, in_provider: InputProvider) -> JetRuleContext:
    self.processJetRuleFile(fname, in_provider)
    return self._compileReminderTasks()

  # compile the input jetrule buffer
  # return the jetrule data structure for convenience
  # ---------------------------------------------------------------------------------------
  def compileJetRule(self, input: str) -> JetRuleContext:
    self.processJetRule(input)
    return self._compileReminderTasks()

  # Compile the JetRule beyond the initial processing
  def _compileReminderTasks(self) -> JetRuleContext:
    if not self.jetrule_ctx.ERROR:
      # print('***@@**')
      # print(json.dumps(self.jetrule_ctx.jetRules, indent=2))
      # print('***@@**')
      self.postprocessJetRule()
      self.validateJetRule()
      if self.jetrule_ctx.ERROR:
        return self.jetrule_ctx
      self.optimizeJetRule()
      self.compileJetRulesToReteNodes()
    return self.jetrule_ctx


  # =====================================================================================
  # processJetRuleFile
  # -------------------------------------------------------------------------------------
  # Read input jetrule file and returns the initial jetrule data structure
  # return the jetrule data structure for convenience
  # ---------------------------------------------------------------------------------------
  def processJetRuleFile(self, fname: str, in_provider: InputProvider) -> JetRuleContext:
    pat = re.compile(r'import\s*"([a-zA-Z0-9_\/.-]*)"')

    # keep the fname as the main rule file of this knowledge base
    self.main_rule_fname = str(fname)

    fout = io.StringIO()
    self.global_line_nbr = 1
    # put jet compiler directive to mark the first file
    fout.write('@JetCompilerDirective source_file = "{0}";\n'.format(fname))
    self.global_line_nbr += 1

    # keep track of the imports for error reporting
    #   'start_pos' is the first line of the rule file (incl)
    #   'end_pos' is the last line of the rule file (excl), ie. +1
    self.processing_rule_files_q = queue.LifoQueue()
    file_info = {'fname': fname, 'start_pos': self.global_line_nbr, 'file_offset': 0}
    self.imported_file_info_list = [file_info]
    self.imported_file_name_set = set([fname])
    self.processing_rule_files_q.put(file_info)

    # read recursively the input file and it's imports
    in_file = in_provider.getRuleFile(fname)
    self._readInput(in_file, in_provider, pat, fout)
    fout.seek(0)
    data = fout.read()
    fout.close()
    self.processJetRule(data)
    return self.jetrule_ctx

  # process the input jetrule buffer
  # return the jetrule context
  # ---------------------------------------------------------------------------------------
  def processJetRule(self, input: str) -> JetRuleContext:
    # set our error handler
    errorListener = JetRuleErrorListener()
    ConsoleErrorListener.INSTANCE = errorListener

    # input data
    input_data =  a4.InputStream(input)

    # lexer
    lexer = JetRuleLexer(input_data)
    stream = a4.CommonTokenStream(lexer)
    
    # parser
    parser = JetRuleParser(stream)
    # parser.removeErrorListeners() 
    # parser.addErrorListener(errorListener)  

    # build the tree
    tree = parser.jetrule()

    # evaluator
    listener = JetListener(self.main_rule_fname)
    walker = a4.ParseTreeWalker()
    walker.walk(listener, tree)

    errors = []
    for err in errorListener.ERRORS:
      # post process antlr4 errors to put reference to the included file
      # print('** got err:',err)
      if self.imported_file_info_list:
        m = self.a4_err_pat.match(err)
        if m:
          line_nbr = int(m.group(1))
          col_nbr = int(m.group(2))
          err_msg = m.group(3)
          # print('** matched on err, line',line_nbr, 'col', col_nbr)

          # find which file this error falls into
          fname = None
          for file_info in self.imported_file_info_list:
            if line_nbr >= file_info['start_pos'] and line_nbr < file_info['end_pos']:
              fname = file_info['fname']
              break

          if fname:
            errors.append("Error in file '{0}' line {1}:{2} {3}".format(fname, line_nbr-file_info['start_pos']+file_info['file_offset']+1, col_nbr+1, err_msg))
          else:
            print('** got err:',err)
            print('** Cannot determine from which file the error came from!')
            raise Exception('Oops something is wrong with the import file in JetRuleCompiler')
      else:
        errors.append(err)

    imported_file_names = None
    if self.main_rule_fname:
      imported_file_names = []
      for item in self.imported_file_name_set:
        item = str(item)                        # to make sure we don't have a Path obj
        if item != self.main_rule_fname:
          imported_file_names.append(item)
    self.jetrule_ctx = JetRuleContext(listener.jetRules, errors, self.main_rule_fname, imported_file_names)
    self.jetrule_ctx.state = JetRuleContext.STATE_PROCESSED
    return self.jetrule_ctx


  # post-process the input jetrule buffer
  # return the jetrule data structure for convenience
  # ---------------------------------------------------------------------------------------
  def postprocessJetRule(self) -> JetRuleContext:
    assert self.jetrule_ctx, 'Must have a valid jetrule context, '
    'call processJetRule() or processJetRuleFile() first'
    assert self.jetrule_ctx.state==JetRuleContext.STATE_PROCESSED, 'Must call processJetRule() first'

    if self.jetrule_ctx.ERROR:
      return self.jetrule_ctx

    # augment the output with post processor
    postProcessor = JetRulesPostProcessor(self.jetrule_ctx)
    postProcessor.process_classes()
    postProcessor.createResourcesForLookupTables()
    # postProcessor.fixRCVariables()
    postProcessor.mapVariables()
    postProcessor.processRuleProperties()
    postProcessor.addNormalizedLabels()
    postProcessor.addLabels()
    self.jetrule_ctx.state = JetRuleContext.STATE_POSTPROCESSED

    # seal the defined resources in a set, this is for validating rules
    self.jetrule_ctx.defined_resources = set(self.jetrule_ctx.resourceMap.keys())

    return self.jetrule_ctx


  # validate the input jetrule buffer
  # ---------------------------------------------------------------------------------------
  def validateJetRule(self) -> bool:
    assert self.jetrule_ctx, 'Must have a valid jetrule context, '
    'call processJetRule() or processJetRuleFile() first'
    assert self.jetrule_ctx.state == JetRuleContext.STATE_POSTPROCESSED, 'Must call postprocessJetRule() first'

    # augment the output with post processor
    validator = JetRuleValidator(self.jetrule_ctx)
    self.jetrule_ctx.state = JetRuleContext.STATE_VALIDATED
    res = validator.validateJetRule()
    res = res and validator.validateTriples()
    return res


  # Optimize the jetrules
  # ---------------------------------------------------------------------------------------
  def optimizeJetRule(self) -> JetRuleContext:
    assert self.jetrule_ctx, 'Must have a valid jetrule context, '
    'call processJetRule() or processJetRuleFile() first'
    assert self.jetrule_ctx.state == JetRuleContext.STATE_VALIDATED, 'Must call validateJetRule() first'

    optimizer = JetRuleOptimizer(self.jetrule_ctx)
    optimizer.optimizeJetRules()
    self.jetrule_ctx.state = JetRuleContext.STATE_OPTIMIZED
    return self.jetrule_ctx


  # Compile jetrules to a Rete Network
  # ---------------------------------------------------------------------------------------
  def compileJetRulesToReteNodes(self) -> JetRuleContext:
    assert self.jetrule_ctx, 'Must have a valid jetrule context, '
    'call processJetRule() or processJetRuleFile() first'
    assert self.jetrule_ctx.state >= JetRuleContext.STATE_VALIDATED, 'Must call at least validateJetRule() first'

    # Perform the final compilation of the rules, create the JetRuleContext.jetReteNodes data structure
    # Leave the JetRuleContext.jetRules structure unchanged
    rete = JetRuleRete(self.jetrule_ctx)
    rete.addReteMarkup()
    rete.addBetaRelationMarkup()
    rete.normalizeReteNodes()
    rete.normalizeTriples()
    self.jetrule_ctx.state = JetRuleContext.STATE_COMPILED_RETE_NODES

    return self.jetrule_ctx


# =======================================================================================
# command line invocation
# ---------------------------------------------------------------------------------------
def main(argv):
  del argv  # Unused.

  
  in_fname = Path(FLAGS.in_file)
  base_path = Path(FLAGS.base_path)
  save_json = FLAGS.save_json

  path = os.path.join(base_path, in_fname)
  path = os.path.abspath(path)
  if not os.path.exists(path) or not os.path.isfile(path):
    print('ERROR: JetRule file {0} does not exist or is not a file'.format(path))
    sys.exit('ERROR: JetRule file does not exist or is not a file') 

  in_provider = InputProvider(base_path)
  compiler = JetRuleCompiler()
  compiler.compileJetRuleFile(str(in_fname), in_provider)
  error = None
  if compiler.jetrule_ctx.ERROR:
    print('ERROR while compiling JetRule file {0}:'.format(in_fname))
    for err in compiler.jetrule_ctx.errors:
      print('   ',err)
      error = 'ERROR while compiling JetRule file {0}:'.format(in_fname)

  # Save the JetRule data structure
  # path = os.path.join(base_path, out_fname)
  in_tup = os.path.splitext(in_fname)
  # if save_json:
  #   jetrules_path = os.path.join(base_path, in_tup[0]+'.jr.json')
  #   jetrete_path = os.path.join(base_path, in_tup[0]+'.jrc.json')
    
  #   with open(jetrules_path, 'wt', encoding='utf-8') as f:
  #     f.write(json.dumps(compiler.jetrule_ctx.jetRules, indent=4))
  #   print('JetRules saved to {0}'.format(os.path.abspath(jetrules_path)))

  #   # Save the JetRete data structure
  #   with open(jetrete_path, 'wt', encoding='utf-8') as f:
  #     f.write(json.dumps(compiler.jetrule_ctx.jetReteNodes, indent=4))
  #   print('JetRete saved to {0}'.format(os.path.abspath(jetrete_path)))

  if error:
    sys.exit(error)

  rete_db_helper = JetRuleReteSQLite(compiler.jetrule_ctx)
  err = rete_db_helper.saveReteConfig()
  if err:
    print('ERROR while saving JetRule file to rete_db: {0}.'.format(str(err)))
    sys.exit('ERROR while saving JetRule file to rete_db: {0}.'.format(str(err))) 

  # Save the jetrule json, combine both json (.jr and .jrc) into a single json
  # Correct boolean data type that are in the json as text
  # properties: as_array, as_table
  # NOTE: Using extension .jrcc.json to indicate that it's the corrected json
  if save_json:
    jetrules_path = os.path.join(base_path, in_tup[0]+'.jrcc.json')

    # Combine the json, add the following to jetRules:
    #   - main_rule_file_name
    #   - support_rule_file_names (skipped for now 2024/10/18)
    #   - rete_nodes
    compiler.jetrule_ctx.jetRules["main_rule_file_name"] = compiler.jetrule_ctx.jetReteNodes.get("main_rule_file_name", None)
    # compiler.jetrule_ctx.jetRules["support_rule_file_names"] = compiler.jetrule_ctx.jetReteNodes.get("support_rule_file_names", [])
    compiler.jetrule_ctx.jetRules["rete_nodes"] = compiler.jetrule_ctx.jetReteNodes.get("rete_nodes", None)
    # 2024/10/18 - Remove jet_rules from .jrcc.json file
    compiler.jetrule_ctx.jetRules["jet_rules"] = []    

    # Correct the bool data type stored as strings
    for lookupTable in compiler.jetrule_ctx.jetRules.get("lookup_tables", []):
      for column in lookupTable.get("columns", []):
        v = column.get("as_array", "false")
        if(v == "false"):
          column["as_array"] = False
        else:
          column["as_array"] = True
    
    for table in compiler.jetrule_ctx.jetRules.get("tables", []):
      for column in table.get("columns", []):
        v = column.get("as_array", "false")
        if(v == "false"):
          column["as_array"] = False
        else:
          column["as_array"] = True
    
    for classNode in compiler.jetrule_ctx.jetRules.get("classes", []):
      v = classNode.get("as_table", "false")
      if(v == "false"):
        classNode["as_table"] = False
      else:
        classNode["as_table"] = True
      for dataProperty in classNode.get("data_properties", []):
        v = dataProperty.get("as_array", "false")
        if(v == "false"):
          dataProperty["as_array"] = False
        else:
          dataProperty["as_array"] = True

    with open(jetrules_path, 'wt', encoding='utf-8') as f:
      f.write(json.dumps(compiler.jetrule_ctx.jetRules, indent=4))
    print('Corrected JetRules saved to {0}'.format(os.path.abspath(jetrules_path)))

    # Create the "build" files, create the build directory if not exist
    build_path = os.path.join(base_path, 'build')
    
    # Save the .rete.json:
    obj_path = os.path.join(build_path, in_tup[0]+'.rete.json')
    os.makedirs(os.path.dirname(obj_path), exist_ok=True)
    obj = {
      'main_rule_file_name': compiler.jetrule_ctx.jetRules.get("main_rule_file_name", []),
      'resources': compiler.jetrule_ctx.jetRules.get("resources", []),
      'lookup_tables': compiler.jetrule_ctx.jetRules.get("lookup_tables", []),
      'rete_nodes': compiler.jetrule_ctx.jetRules.get("rete_nodes", [])
    }
    with open(obj_path, 'wt', encoding='utf-8') as f:
      f.write(json.dumps(obj, indent=4))
    print('Rete network saved to {0}'.format(os.path.abspath(obj_path)))
    
    # Save the .model.json:
    obj_path = os.path.join(build_path, in_tup[0]+'.model.json')
    obj = {
      'main_rule_file_name': compiler.jetrule_ctx.jetRules.get("main_rule_file_name", []),
      'classes': compiler.jetrule_ctx.jetRules.get("classes", []),
      'tables': compiler.jetrule_ctx.jetRules.get("tables", [])
    }
    with open(obj_path, 'wt', encoding='utf-8') as f:
      f.write(json.dumps(obj, indent=4))
    print('Model saved to {0}'.format(os.path.abspath(obj_path)))
    
    # Save the .triples.json:
    obj_path = os.path.join(build_path, in_tup[0]+'.triples.json')
    obj = {
      'main_rule_file_name': compiler.jetrule_ctx.jetRules.get("main_rule_file_name", []),
      'triples': compiler.jetrule_ctx.jetRules.get("triples", [])
    }
    with open(obj_path, 'wt', encoding='utf-8') as f:
      f.write(json.dumps(obj, indent=4))
    print('Triples saved to {0}'.format(os.path.abspath(obj_path)))

if __name__ == '__main__':
  flags.mark_flag_as_required('in_file')
  app.run(main)