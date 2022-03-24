import sys
import enum
from typing import Dict, Sequence
import json
from JetRuleLexer import JetRuleLexer

class JetRuleContext:

  STATE_PARSE_ERROR = 1
  STATE_READY = 2
  STATE_PROCESSED = 3
  STATE_POSTPROCESSED = 4
  STATE_VALIDATED = 5
  STATE_OPTIMIZED = 6
  STATE_COMPILE_ERROR = 7
  STATE_COMPILED_RETE_NODES = 8

  def __init__(self, data: Dict[str, object], errors: Sequence[str], main_rule_fname: str, imported_files: Sequence[str]):
    # Main jetrules data structure - json (without rule compilation)
    # NOTE: jetRule is overriden in JetRuleRete.normalizeReteNodes
    # -----------------------------------------------
    self.jetRules: Dict[str, object] = data
    self.main_rule_fname = main_rule_fname   # kbase key within the workspace

    # keeping track of the file imports
    # Format {'main_file.jr': ['import1.jr','import2.jr']}
    #TODO this will likely change to keep track of the import at file level, not at main file level
    self.imported = {}
    if main_rule_fname:
      self.imported[main_rule_fname] = imported_files
      self.jetRules['imports'] = self.imported


    # resourceMap contains literals and resources
    self.resourceMap = {}
    self.errors = errors

    # For rete network
    # main data structure - json with rule compiled into a rete network
    # NOTE: jetReteNodes['rete_nodes'] is overriden in JetRuleRete.normalizeReteNodes
    # -----------------------------------------------
    # This is filled by JetRuleRete class during the last compiler step 
    self.jetReteNodes: Dict[str, object] = None
    self.rete_nodes = []

    # Shortcuts to elements in self.jetRules elements
    # -----------------------------------------------
    self.literals = None
    self.resources = None
    self.lookup_tables = None
    self.jet_rules = None
    self.defined_resources: set = None # All defined resource, populated after post processing
    self.compiler_directives = None
    self.classes = None
    self.tables = None

    self.ERROR = False
    if self.errors:
      self.ERROR = True
      return

    # Prepare a set of symbol names to be able to escape them in resource names
    self.symbolNames = set([symbolName.replace("'", '') for symbolName in JetRuleLexer.literalNames])

    # initalize the literal and resource map
    if not self.jetRules: 
      raise Exception("Invalid jetRules structure: ",self.jetRules)

    self.literals = self.jetRules.get('literals')
    self.resources = self.jetRules.get('resources')
    self.lookup_tables = self.jetRules.get('lookup_tables')
    self.jet_rules = self.jetRules.get('jet_rules')
    self.compiler_directives = self.jetRules.get('compiler_directives')
    self.classes = self.jetRules.get('classes')
    self.tables = []

    if self.literals is None or self.resources is None or self.lookup_tables is None or self.jet_rules is None: 
      raise Exception("Invalid jetRules structure: ",self.jetRules)

    self._initMap(self.resourceMap, self.literals, 'Literal')
    self._initMap(self.resourceMap, self.resources, 'Resource')

    # init done whew!
    self.state = JetRuleContext.STATE_READY

  def _initMap(self, map: Dict[str, object], items, tag):
    for item in items:
      id = item['id']
      type = item['type']
      value = item['value']
      symbol = item.get('symbol')
      c = map.get(id)
      if c:
        if symbol:    # special case, symbol used to initialize a resource
          cs = c.get('symbol')
          if not cs or cs != symbol:
            ot = cs if cs else c['type']
            self.err('Error: {0} with id {1} is define multiple times, one is a symbol, {2}, the other is of different type {3}'.format(tag, id, symbol, ot))
        else:
          if c['type'] != type:
            self.err('Error: {0} with id {1} is define multiple times with different types: {2} and {3}'.format(tag, id, type, c['type']))
          if c['value'] != value:
            self.err('Error: {0} with id {1} is define multiple times with different values: {2} and {3}'.format(tag, id, value, c['value']))
      map[item['id']] = item

  def _addRL(self, map, tag, name: str, type: str, value, source_fname: str) -> object:
    assert type is not None
    assert value is not None
    r = map.get(name)
    if r:
      if source_fname and source_fname != r.get('source_file_name'):
        self.err('Error: Creating {0} with id {1} in file {2} but it already exist in file {3}.'.format(tag, name, source_fname, r.get('source_file_name')))
      if r['value'] != value or type != r.get('type'):
        self.err('Error: Creating {0} with id {1} that already exist with a different definition.'.format(tag, name))

    item = {'id': name, 'type': type, 'value': value}
    if source_fname:
      item['source_file_name'] = source_fname
    map[name] = item
    return item

  # Used by post processor, do not use after post processing is done (use addResourceFromRule, see jetrule_validation_context.validateIdentifier)
  def addResource(self, name: str, value: str, source_fname: str):
    item = self._addRL(self.resourceMap, 'resource', name, 'resource', value, source_fname)
    self.resources.append(item)

  # not used, added for completeness
  def addLiteral(self, name: str, type: str, value: str, source_fname: str):
    item = self._addRL(self.resourceMap, 'literal', name, type, value, value, source_fname)
    self.literals.append(item)

  def err(self, msg: str) -> None:
    self.ERROR = True
    self.errors.append(msg)

  def get_compiler_directive(self, name: str) -> bool:
    if self.compiler_directives:
      return self.compiler_directives.get(name)
    return None

  # Used by jetrule_validation_context.validateIdentifier to add resources defined implicitly in rules
  def addResourceFromRule(self, name: str) -> str:
    if name and self.get_compiler_directive('extract_resources_from_rules'):
      value = name
      type = 'resource'
      if name.startswith('_0:'):
        name = name[len('_0:'):]
        type = 'volatile_resource'

      if name in self.defined_resources:
        return name
      item = self._addRL(self.resourceMap, 'resource', name, type, value, self.main_rule_fname)
      self.resources.append(item)
      self.defined_resources.add(name)
      return name
    return None

  def getResource(self, name: str) -> (object or None):
    return self.resourceMap.get(name)