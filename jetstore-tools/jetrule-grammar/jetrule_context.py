import sys
from typing import Dict, Sequence
import json
from JetRuleLexer import JetRuleLexer

class JetRuleContext:

  def __init__(self, data: Dict[str, object]):
    self.jetRules = data
    self.literalMap = {}
    self.resourceMap = {}
    self.errors = []

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

    self._initMap(self.literalMap, self.literals, 'Literal')
    self._initMap(self.resourceMap, self.resources, 'Resource')

  def _initMap(self, map, items, tag):
    for item in items:
      id = item['id']
      type = item['type']
      value = item['value']
      c = map.get(id)
      if c:
        if c['type'] != type or c['value'] != value:
          self.errors.append('Error: {0} with id {1} is define multiple times.'.format(tag, id))
      map[item['id']] = item

  def _addRL(self, map, tag, name: str, type: str, value):
    r = map.get(name)
    if r and (r['value'] != value or type != r.get('type')):
      self.errors.append('Error: Creating {0} with id {1} that already exist with a different definition.'.format(tag, name))

    item = {'id': name, 'type': type, 'value': value}
    map[name] = item
    return item

  def addResource(self, name: str, value: str):
    item = self._addRL(self.resourceMap, 'resource', name, 'resource', value)
    self.resources.append(item)

  def addLiteral(self, name: str, type: str, value: str):
    item = self._addRL(self.literalMap, 'literal', name, type, value)
    self.literals.append(item)

  def err(self, msg: str) -> None:
    self.errors.append(msg)
