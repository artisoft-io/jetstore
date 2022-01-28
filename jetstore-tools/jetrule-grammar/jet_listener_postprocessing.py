import sys
from typing import Dict
import json

class JetRulesPostProcessor:

  def __init__(self, data: object):
    self.jetRules = data
    self.varMapping = {}

  # visit jetRules data structure to map variables
  def mapVariables(self):
    if not self.jetRules: raise Exception("Invalid jetRules structure: ",self.jetRules)
    rules = self.jetRules.get('jet_rules')
    # if not rules: raise Exception("Invalid jetRules structure: ",self.jetRules)
    for rule in rules:
      # print('Processing Rule:', rule['name'])
      self.varMapping = {}

      for item in rule.get('antecedents', []):
        triple = item['triple']
        for i in range(3):
          self.processElm(triple[i])

        filter = item.get('filter')
        if filter:
          self.processElm(filter)

      for item in rule.get('consequents', []):
        triple = item['triple']
        for i in range(3):
          self.processElm(triple[i])

  # Process recursively elm according to it's type
  def processElm(self, elm: Dict[str, object]):
    if not elm: return None
    type = elm.get('type')
    if type is None: raise Exception("Invalid jetRules elm: ", elm)

    if type == 'var':
      id = elm['id']
      mappedVar = self.varMapping.get(id)
      if mappedVar is None:
        mappedVar = '?x' + str(len(self.varMapping)+1)
        self.varMapping[id] = mappedVar
      elm['id'] = mappedVar
      elm['label'] = id

    if type == 'binary':
      self.processElm(elm['lhs'])
      self.processElm(elm['rhs'])

    if type == 'unary':
      self.processElm(elm['arg'])

  # Augment rule's antecedents and consequents with
  # a label using the normalized variables
  def addLabels(self):
    return self._addLabels('label', False)

  def addNormalizedLabels(self):
    return self._addLabels('normalizedLabel', True)

  def _addLabels(self, label_name: str, useNormalizedVar: bool):
    if not self.jetRules: raise Exception("Invalid jetRules structure: ",self.jetRules)
    rules = self.jetRules.get('jet_rules')

    for rule in rules:
      name = rule.get('name')
      if not name: raise Exception("Invalid jetRules structure: ",self.jetRules)
      props = rule.get('properties')
      ptxt = ''
      if props:
        ptxt = ', '.join(['{0}={1}'.format(k, v) for k, v in props.items()])
        ptxt = ', ' + ptxt

      label = '[{0}{1}]:'.format(name, ptxt)
      isFirst = True
      for item in rule.get('antecedents', []):
        normalizedLabel = self.makeLabel(item, useNormalizedVar)
        item[label_name] = normalizedLabel
        if not isFirst:
          label += '.'
        isFirst = False
        label += normalizedLabel

      label += ' -> '

      isFirst = True
      for item in rule.get('consequents', []):
        normalizedLabel = self.makeLabel(item, useNormalizedVar)
        item[label_name] = normalizedLabel
        if not isFirst:
          label += '.'
        isFirst = False
        label += normalizedLabel

      label += ';'

      rule[label_name] = label

  def escapeText(self, txt: str):
    if len(txt) < 3: return txt
    result = ''
    sz = len(txt)
    for i in range(sz):
      if txt[i] == '"':
        result += '\\'
      result += txt[i]
    return result

  # Recursive function to build the label of the rule antecedent or consequent
  def makeLabel(self, elm: Dict[str, object], useNormalizedVar: bool) -> str:
    if not elm: return ''
    type = elm.get('type')
    if type is None: raise Exception("Invalid jetRules elm: ", elm)

    if type == 'antecedent':
      label = ''
      isNot = elm.get('isNot')
      if isNot: label += 'not'

      triple = elm['triple']
      s = self.makeLabel(triple[0], useNormalizedVar)
      p = self.makeLabel(triple[1], useNormalizedVar)
      o = self.makeLabel(triple[2], useNormalizedVar)
      label += '({0} {1} {2})'.format(s, p, o)
      filter = elm.get('filter')
      if filter:
        label += '.[{0}]'.format(self.makeLabel(filter, useNormalizedVar))
      return label

    if type == 'consequent':
      label = ''
      triple = elm['triple']
      s = self.makeLabel(triple[0], useNormalizedVar)
      p = self.makeLabel(triple[1], useNormalizedVar)
      o = self.makeLabel(triple[2], useNormalizedVar)
      label += '({0} {1} {2})'.format(s, p, o)
      return label

    if type == 'binary':
      lhs = elm['lhs']
      rhs = elm['rhs']
      if lhs['type'] in ['binary', 'unary']:
        lhs_label = '({0})'.format(self.makeLabel(lhs, useNormalizedVar))
      else:
        lhs_label = self.makeLabel(lhs, useNormalizedVar)

      if rhs['type'] in ['binary', 'unary']:
        rhs_label = '({0})'.format(self.makeLabel(rhs, useNormalizedVar))
      else:
        rhs_label = self.makeLabel(rhs, useNormalizedVar)

      label = '{0} {1} {2}'.format(lhs_label, elm['op'], rhs_label)
      return label

    if type == 'unary':
      arg = elm['arg']
      if arg['type'] in ['binary', 'unary']:
        arg_label = '({0})'.format(self.makeLabel(arg, useNormalizedVar))
      else:
        arg_label = self.makeLabel(arg, useNormalizedVar)

      label = '{0} {1}'.format(elm['op'], arg_label)
      return label

    if type == 'var':
      if useNormalizedVar:
        return elm['id']
      else:
        return elm['label']

    if type == 'text':
      return '"{0}"'.format(self.escapeText(elm['id']))
      # return '"{0}"'.format(elm['id'])
    if type == 'int': return 'int({0})'.format(elm['value'])
    if type == 'uint': return 'uint({0})'.format(elm['value'])
    if type == 'long': return 'long({0})'.format(elm['value'])
    if type == 'ulong': return 'ulong({0})'.format(elm['value'])

    if type in ['identifier', 'keyword']:
      return elm['value']
      
if __name__ == "__main__":
  
  # Load the JetRules structure
  with open('test.jr.json', 'rt', encoding='utf-8') as f:
    data = json.loads(f.read())
  postProcessor = JetRulesPostProcessor(data)
  postProcessor.mapVariables()
  postProcessor.addNormalizedLabels()
  postProcessor.addLabels()

  # Save the updated JetRule structure
  with open('post_processed_test.jr.json', 'wt', encoding='utf-8') as f:
    f.write(json.dumps(postProcessor.jetRules, indent=4))

  print('Result saved to post_processed_test.jr.json')
