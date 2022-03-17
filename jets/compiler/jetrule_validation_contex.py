from typing import Dict
from frozendict import frozendict
from jetrule_context import JetRuleContext
from absl import flags

RC_OP_MAPPING = frozendict({
  '!=':                    '!=',
  '+':                     '+',
  '-':                     '-',
  '/':                     '/',
  '<':                     '<',
  '<=':                    '<=',
  '==':                    '==',
  '>':                     '>',
  '>=':                    '>=',
  'abs':                   'abs',
  'and':                   'and',
  'apply_format':          'apply_format',
  'cast_to_range_of':      'to_type_of',
  'contains':              'contains',
  'create_entity':         'create_entity',
  'create_literal':        'create_literal',
  'create_resource':       'create_resource',
  'create_uuid_resource':  'create_uuid_resource',
  'different_from':        '!=',
  'exist':                 'exist',
  'exist_not':             'exist_not',
  'get_cardinality':       'size_of',
  'is_literal':            'is_literal',
  'is_no_value':           'is_null',
  'length_of':             'length_of',
  'literal_regex':         'literal_regex',
  'lookup':                'lookup',
  'max_multi_value':       'max_of',
  'min_multi_value':       'min_of',
  'multi_lookup':          'multi_lookup',
  'not':                   'not',
  'or':                    'or',
  'parse_usd_currency':    'parse_usd_currency',
  'sorted_head':           'sorted_head',
  'starts_with':           'starts_with',
  'sum_values':            'sum_values',
  'to_int':                'to_int',
  'to_real':               'to_real',
  'to_upper':              'to_upper',



})
class ValidationContext:
  def __init__(self, jetrule_ctx: JetRuleContext):
    self.jetrule_ctx = jetrule_ctx
    self.binded_vars = set()
    self.rule_name = ''
    self.term_label = ''
    self.elm_type = ''

  def setRuleName(self, name: str):
    self.rule_name = name

  def setTermLabel(self, label: str):
    self.term_label = label

  def setElmType(self, type: str):
    self.elm_type = type

  def err(self, msg: str) -> None:
    self.jetrule_ctx.err(msg)

  def validateBinded(self, var: str) -> bool:
    is_binded = var in self.binded_vars
    if not is_binded:
      self.err(
        "Error rule {0}: Variable '{1}' is not binded in this context '{2}' "
        "and must be for the rule to be valid.".format(
          self.rule_name, var, self.term_label)
      )
    return is_binded

  def addBinded(self, var: str) -> bool:
    self.binded_vars.add(var)
    return True
  
  def validateVar(self, var: str) -> bool:
    # print('*** Validate Variable for rule', self.rule_name, 'visiting elm type', self.elm_type, 'validating var', var)
    if self.elm_type in ['filter', 'consequent', 'negated']:
      return self.validateBinded(var)
    return self.addBinded(var)
  
  def validateIdentifier(self, elm: object) -> bool:
    var = elm['value']
    # print('*** Validate Identifier for rule', self.rule_name, 'visiting elm type', self.elm_type, 'validating identifier', var)
    defined = var in self.jetrule_ctx.defined_resources
    if not defined:
      # Check if we extract the identifier from the rules
      name = self.jetrule_ctx.addResourceFromRule(var)
      if not name:
        self.err(
          "Error rule {0}: Identifier '{1}' is not defined in this context '{2}', "
          "it must be defined.".format(
            self.rule_name, var, self.term_label)
        )
        return False
      elm['value'] = name
    return True

  def has_errors(self):
    return self.jetrule_ctx.ERROR
