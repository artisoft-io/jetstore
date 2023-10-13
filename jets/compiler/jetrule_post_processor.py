from pydoc import classname
import sys
from typing import Dict, Sequence
from jetrule_context import JetRuleContext
import json

class JetRulesPostProcessor:

  def __init__(self, ctx: JetRuleContext):
    self.ctx = ctx
    self.classes_dict = {}


  # =====================================================================================
  # PROCESS CLASSES
  # -------------------------------------------------------------------------------------
  # Main Function
  def process_classes(self):
    if self.ctx.classes:
      # setup a dict to access the classes
      for cls in self.ctx.classes:
        cls['sub_classes'] = []
        self.classes_dict[cls['name']] = cls
      # add sub classes to classes (needed for domain_table to columns of sub classes)      
      for cls in self.ctx.classes:
        for base_cls_name in cls['base_classes']:
          base_class = self.classes_dict.get(base_cls_name, {'sub_classes':[]})
          base_class['sub_classes'].append(cls['name'])

      self.createResourcesForClasses()
      self.createInherithanceRulesForClasses()
      self.createTablesForClasses()

  # visit classes and create resources
  def createResourcesForClasses(self):
    for item in self.ctx.classes:
      name = item.get('name')
      source_file_name = item.get('source_file_name')
      self.ctx.addResource(name, name, source_file_name)
      for bc in item['base_classes']:
        self.ctx.addResource(bc, bc, source_file_name)
      grouping_properties = item.get('grouping_properties', [])      
      for p in item['data_properties']:
        name = p['name']
        self.ctx.addResource(name, name, source_file_name)
        if name in grouping_properties:
          p['is_grouping'] = True

  # visit classes and create rules for class inheritance axioms
  def createInherithanceRulesForClasses(self):
    rid = 0
    for item in self.ctx.classes:
      if not item.get('base_classes'):
        continue
      name = item.get('name')
      rid += 1
      rule = {
        'name': self.make_name(name+':'+str(rid)),
        'properties': {
          'i': 'true'
        },
        'source_file_name': item.get('source_file_name'),
        'antecedents': [{
          'type': 'antecedent',
          'isNot': False,
          'triple': [
            {
              'type': 'var',
              'value': '?s1'
            },{
              'type': 'identifier',
              'value': 'rdf:type'
            },{
              'type': 'identifier',
              'value': name
            }
          ]
        }],
        'consequents': [{
          'type':'consequent', 
          'triple':[
            {
              'type':'var',
              'value':'?s1'
            },{
              'type':'identifier',
              'value':'rdf:type'
            },{
              'type':'identifier',
              'value':bc
            }
          ]
        } for bc in item['base_classes'] ]
      }
      self.ctx.jet_rules.append(rule)


  # visit classes and create table for as_table is true
  def createTablesForClasses(self):
    for item in self.ctx.classes:
      if item.get('as_table', 'false') == 'true':
        source_file_name = item.get('source_file_name')
        table = {
          'type': 'table',
          'table_name': self.make_name(item['name']),
          'class_name': item['name'],
          'columns': self.make_columns(item)
        }
        if source_file_name:
          table['source_file_name'] = source_file_name
        self.ctx.tables.append(table)


  def make_name(self, class_name: str) -> str:
    # return class_name.replace(':', '__').lower()
    return class_name

  def make_columns(self, class_item: object) -> Sequence[object]:
    columns = {}
    visited_classes = ['owl:Thing']
    self.add_columns(columns, visited_classes, class_item)

    # Flatten the columns into a list
    columns = [v for k,v in sorted(columns.items())]
    # make column name vs property name
    for column in columns:
      column['property_name'] = column['name']
      column['column_name'] = self.make_name(column['name'])
      del column['name']
    return columns

  def add_columns(self, columns, visited_classes, class_item):
    visited_classes.append(class_item['name'])
    for column in class_item['data_properties']:
      columns[column['name']] = column.copy()
    # do base classes recursivelly
    for base_class_name in class_item['base_classes']:
      if base_class_name not in visited_classes:
        self.add_base_classes_columns(columns, visited_classes, self.classes_dict[base_class_name])
    # do sub classes recursivelly
    for sub_class_name in class_item['sub_classes']:
      if sub_class_name not in visited_classes:
        self.add_sub_classes_columns(columns, visited_classes, self.classes_dict[sub_class_name])

  def add_base_classes_columns(self, columns, visited_classes, class_item):
    visited_classes.append(class_item['name'])
    for column in class_item['data_properties']:
      columns[column['name']] = column.copy()
    # do base classes recursivelly
    for base_class_name in class_item['base_classes']:
      if base_class_name not in visited_classes:
        self.add_base_classes_columns(columns, visited_classes, self.classes_dict[base_class_name])

  def add_sub_classes_columns(self, columns, visited_classes, class_item):
    visited_classes.append(class_item['name'])
    for column in class_item['data_properties']:
      columns[column['name']] = column.copy()
    # do sub classes recursivelly
    for sub_class_name in class_item['sub_classes']:
      if sub_class_name not in visited_classes:
        self.add_sub_classes_columns(columns, visited_classes, self.classes_dict[sub_class_name])
    # do base classes of sub class recursivelly
    for base_class_name in class_item['base_classes']:
      if base_class_name not in visited_classes:
        self.add_base_classes_columns(columns, visited_classes, self.classes_dict[base_class_name])

  # =====================================================================================
  # createResourcesForLookupTables
  # -------------------------------------------------------------------------------------
  # visit lookup tables data structure to create resources corresponding to table names
    # "lookup_tables": [
    #     {
    #         "type": "lookup",
    #         "name": "acme:ProcedureLookup",
    #         "key": [
    #             "EVENT_DURATION"
    #         ],
    #         "columns": [
    #             {
    #                 "name": "EVENT_DURATION",
    #                 "type": "int",
    #                 "as_array": "false"
    #             },
    #             {
    #                 "name": "EXCL",
    #                 "type": "text",
    #                 "as_array": "true"
    #             }
    #         ],
  # also create resource for columns' name that is legal and does not exist as a resource
  def is_legal_identifier(self, text: str)-> bool:
    if not text: 
      return False
    if not text[0].isalpha():
      return False
    for c in text:
      if not c.isalnum() and c != '_' and c != ':':
        return False
    return True
  
  def createResourcesForLookupTables(self):
    for item in self.ctx.lookup_tables:
      name = item.get('name')
      source_file_name = item.get('source_file_name')
      self.ctx.addResource(name, name, source_file_name)
      columns = item['columns']
      resources = []
      for column in columns:
        column_name = column['name']
        if self.is_legal_identifier(column_name):
          if not self.ctx.getResource(column_name):
            self.ctx.addResource(column_name, column_name, source_file_name)
            resources.append(column_name)
      item['resources'] = resources
      

  # =====================================================================================
  # mapVariables
  # -------------------------------------------------------------------------------------
  # visit jetRules data structure to map variables
  def mapVariables(self):
    if not self.ctx.jetRules: raise Exception("Invalid jetRules structure: ",self.ctx.jetRules)
    for rule in self.ctx.jet_rules:
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
      # Check to see if it has already a label, if not look at value,
      # then set it as id
      id = elm.get('label')
      if not id:
        id = elm['value']
      elm['id'] = id
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


  # =====================================================================================
  # processRuleProperties
  # -------------------------------------------------------------------------------------
  # Augment rule's antecedents and consequents based on rule properties
  def processRuleProperties(self):
    if not self.ctx.jetRules: raise Exception("Invalid jetRules structure: ",self.ctx.jetRules)
    for rule in self.ctx.jet_rules:
      properties = rule.get('properties', {})
      # rule optimization flag -- always set with default of True
      o = properties.get("o", "true")
      optimization = True
      if o[0] == 'f':
        # print('Rule',rule['name'],'has optimization turned off')
        optimization = False
      rule['optimization'] = optimization

      # rule salience flag -- always set with default of 100
      salience = 100
      s = properties.get("s")
      if s:
        try:
          salience = int(s)
        except (TypeError, ValueError) as e:
          msg = "Rule {0}: Invalid salience in rule property 's': {1}".format(rule['name'],e)
          print(msg)
          self.ctx.err(msg)
        except:
          msg = "Rule {0}: Invalid salience in rule property 's': {1}".format(rule['name'],s)
          print(msg)
          self.ctx.err(msg)
      # Set the salience at the rule, will be moved to the last antecedent node vertex (by Rete)
      rule['salience'] = salience


  # =====================================================================================
  # addLabels
  # -------------------------------------------------------------------------------------
  # Augment rule's antecedents and consequents with
  # a label using the normalized variables
  def addLabels(self):
    return self._addLabels('label', False)

  def addNormalizedLabels(self):
    return self._addLabels('normalizedLabel', True)

  def _addLabels(self, label_name: str, useNormalizedVar: bool):
    if not self.ctx.jetRules: raise Exception("Invalid jetRules structure: ",self.ctx.jetRules)
    for rule in self.ctx.jet_rules:
      name = rule.get('name')
      if not name: raise Exception("Invalid jetRules structure: ",self.ctx.jetRules)
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
      return '"{0}"'.format(self.escapeText(elm['value']))
    if type == 'int': return 'int({0})'.format(elm['value'])
    if type == 'uint': return 'uint({0})'.format(elm['value'])
    if type == 'long': return 'long({0})'.format(elm['value'])
    if type == 'ulong': return 'ulong({0})'.format(elm['value'])
    if type == 'date': return 'date("{0}")'.format(elm['value'])
    if type == 'datetime': return 'datetime("{0}")'.format(elm['value'])
    if type == 'bool': return 'bool("{0}")'.format(elm['value'])

    if type == 'identifier':
      parts = elm['value'].split(':')
      for i in range(1, len(parts)):
        if parts[i] in self.ctx.symbolNames:
          parts[i] = '"{0}"'.format(parts[i])
      
      return ':'.join(parts)

    if type == 'keyword':
      return elm['value']
