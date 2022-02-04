import sys
import enum
from typing import Dict, Sequence
import json
from JetRuleLexer import JetRuleLexer

class JetRuleContext:

  STATE_READY = 1
  STATE_PROCESSED = 2
  STATE_POSTPROCESSED = 3
  STATE_VALIDATED = 4
  STATE_OPTIMIZED = 5
  STATE_RETE_MARKINGS = 6

  def __init__(self, data: Dict[str, object], errors: Sequence[str]):
    self.jetRules = data
    # resourceMap contains literals and resources
    self.resourceMap = {}
    self.errors = errors

    # For rete network
    self.rete_nodes = []

    self.literals = None
    self.resources = None
    self.lookup_tables = None
    self.jet_rules = None
    self.defined_resources = None

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

    if self.literals is None or self.resources is None or self.lookup_tables is None or self.jet_rules is None: 
      raise Exception("Invalid jetRules structure: ",self.jetRules)

    self._initMap(self.resourceMap, self.literals, 'Literal')
    self._initMap(self.resourceMap, self.resources, 'Resource')

    # collect all defined resources and literals for rule validation
    self.defined_resources = frozenset(self.resourceMap.keys())

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

  def _addRL(self, map, tag, name: str, type: str, value):
    assert type is not None
    assert value is not None
    r = map.get(name)
    if r and (r['value'] != value or type != r.get('type')):
      self.err('Error: Creating {0} with id {1} that already exist with a different definition.'.format(tag, name))

    item = {'id': name, 'type': type, 'value': value}
    map[name] = item
    return item

  def addResource(self, name: str, value: str):
    item = self._addRL(self.resourceMap, 'resource', name, 'resource', value)
    self.resources.append(item)

  def addLiteral(self, name: str, type: str, value: str):
    item = self._addRL(self.resourceMap, 'literal', name, type, value)
    self.literals.append(item)

  def err(self, msg: str) -> None:
    self.ERROR = True
    self.errors.append(msg)
