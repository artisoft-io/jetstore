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
    # sort the imported_files sequence to help testing
    sorted_imported_files = None
    if main_rule_fname:
      sorted_imported_files = []
    if imported_files:
      sorted_imported_files = list(imported_files)
      sorted_imported_files.sort()
    self.imported = {}
    if main_rule_fname:
      self.imported[main_rule_fname] = sorted_imported_files
      self.jetRules['imports'] = self.imported


    # resourceMap contains literals and resources
    self.resourceMap = {}
    self.errors = errors
    self.predefined_resources = set()

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
    self.triples = None

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
    self.jetstore_config = self.jetRules.get('jetstore_config')
    self.rule_sequences = self.jetRules.get('rule_sequences')
    self.classes = self.jetRules.get('classes')
    self.tables = []
    self.triples = self.jetRules.get('triples', [])

    if self.literals is None or self.resources is None or self.lookup_tables is None or self.jet_rules is None: 
      raise Exception("Invalid jetRules structure: ",self.jetRules)

    # initialize resourceMap with pre-defined resources
    # These are standard resources with prefix rdf, rdfs, and owl
    self._initPredefinedResources()

    # Add literal and resources defined in the input rule file
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
      # check if it's a predefined resource
      cpd = self.isValidPredefinedResources(id, value)
      if cpd is not None:
        if type != 'resource':
          self.err('Error: {0} with id {1} is a predefined resource, it must be of type resource.'.format(tag, id))
        if not cpd:
          self.err('Error: {0} with id {1} is a predefined resource, value must be same as id, value is {2}.'.format(tag, id, value))
        if cpd is True:
          item['source_fname'] = 'predefined'
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
    cpd = self.isValidPredefinedResources(name, value)
    if cpd is True:
      source_fname = 'predefined'
    if cpd is False:
      self.err('Error: Resource with id {1} is a predefined resource, value must be same as id, value is {2}.'.format(name, value))

    r = map.get(name)
    if r:
      if source_fname and source_fname != r.get('source_file_name'):
        if r['value'] != value or type != r.get('type'):
          self.err('Error: Creating {0} with id {1} in file {2} but it already exist in file {3} with a different definition.'.format(tag, name, source_fname, r.get('source_file_name')))
        else:
          print('Warning: Creating {0} with id {1} in file {2} but it already exist in file {3}'.format(tag, name, source_fname, r.get('source_file_name')))
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
        value = name  # we don't keep the _0: prefix to the value, it's added when creating the resource in jetrule c++ lib
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

  # returns None if argument is not predefined
  # returns True if argument is predefined and valid
  # return False if argument is predefined and NOT valid (id != value)
  def isValidPredefinedResources(self, id: str, value: str) -> bool:
    if id in self.predefined_resources:
      if id != value:
        return False
      return True
    return None

  def _initPredefinedResources(self):
    self.predefined_resources.add('jets:completed')
    self.predefined_resources.add('jets:entity_property')
    self.predefined_resources.add('jets:exception')
    self.predefined_resources.add('jets:iState')
    self.predefined_resources.add('jets:key')
    self.predefined_resources.add('jets:lookup_multi_rows')
    self.predefined_resources.add('jets:lookup_row')
    self.predefined_resources.add('jets:loop')
    self.predefined_resources.add('jets:State')
    self.predefined_resources.add('jets:source_period_sequence')
    self.predefined_resources.add('jets:value_property')
    self.predefined_resources.add('owl:AllDifferent')
    self.predefined_resources.add('owl:AllDisjointClasses')
    self.predefined_resources.add('owl:AllDisjointProperties')
    self.predefined_resources.add('owl:AnnotationProperty')
    self.predefined_resources.add('owl:AsymmetricProperty')
    self.predefined_resources.add('owl:Class')
    self.predefined_resources.add('owl:DataRange')
    self.predefined_resources.add('owl:DeprecatedClass')
    self.predefined_resources.add('owl:DeprecatedProperty')
    self.predefined_resources.add('owl:DatatypeProperty')
    self.predefined_resources.add('owl:FunctionalProperty')
    self.predefined_resources.add('owl:InverseFunctionalProperty')
    self.predefined_resources.add('owl:IrreflexiveProperty')
    self.predefined_resources.add('owl:NamedIndividual')
    self.predefined_resources.add('owl:Nothing')
    self.predefined_resources.add('owl:ObjectProperty')
    self.predefined_resources.add('owl:Ontology')
    self.predefined_resources.add('owl:OntologyProperty')
    self.predefined_resources.add('owl:ReflexiveProperty')
    self.predefined_resources.add('owl:Restriction')
    self.predefined_resources.add('owl:SymmetricProperty')
    self.predefined_resources.add('owl:Thing')
    self.predefined_resources.add('owl:TransitiveProperty')
    self.predefined_resources.add('owl:allValuesFrom')
    self.predefined_resources.add('owl:backwardCompatibleWith')
    self.predefined_resources.add('owl:cardinality')
    self.predefined_resources.add('owl:complementOf')
    self.predefined_resources.add('owl:deprecated')
    self.predefined_resources.add('owl:differentFrom')
    self.predefined_resources.add('owl:disjointWith')
    self.predefined_resources.add('owl:disjointUnionOf')
    self.predefined_resources.add('owl:distinctMembers')
    self.predefined_resources.add('owl:equivalentClass')
    self.predefined_resources.add('owl:equivalentProperty')
    self.predefined_resources.add('owl:hasSelf')
    self.predefined_resources.add('owl:hasValue')
    self.predefined_resources.add('owl:imports')
    self.predefined_resources.add('owl:incompatibleWith')
    self.predefined_resources.add('owl:intersectionOf')
    self.predefined_resources.add('owl:inverseOf')
    self.predefined_resources.add('owl:maxCardinality')
    self.predefined_resources.add('owl:maxQualifiedCardinality')
    self.predefined_resources.add('owl:minCardinality')
    self.predefined_resources.add('owl:minQualifiedCardinality')
    self.predefined_resources.add('owl:members')
    self.predefined_resources.add('owl:onClass')
    self.predefined_resources.add('owl:onDataRange')
    self.predefined_resources.add('owl:onProperty')
    self.predefined_resources.add('owl:oneOf')
    self.predefined_resources.add('owl:priorVersion')
    self.predefined_resources.add('owl:propertyChainAxiom')
    self.predefined_resources.add('owl:propertyDisjointWith')
    self.predefined_resources.add('owl:qualifiedCardinality')
    self.predefined_resources.add('owl:sameAs')
    self.predefined_resources.add('owl:someValuesFrom')
    self.predefined_resources.add('owl:unionOf')
    self.predefined_resources.add('owl:versionInfo')
    self.predefined_resources.add('owl:versionIRI')
    self.predefined_resources.add('owl:topObjectProperty')
    self.predefined_resources.add('owl:topDataProperty')
    self.predefined_resources.add('rdf:Description')
    self.predefined_resources.add('rdf:Property')
    self.predefined_resources.add('rdf:type')
    self.predefined_resources.add('rdfs:Class')
    self.predefined_resources.add('rdfs:Datatype')
    self.predefined_resources.add('rdfs:comment')
    self.predefined_resources.add('rdfs:domain')
    self.predefined_resources.add('rdfs:label')
    self.predefined_resources.add('rdfs:range')
    self.predefined_resources.add('rdfs:subClassOf')
    self.predefined_resources.add('rdfs:subPropertyOf')
