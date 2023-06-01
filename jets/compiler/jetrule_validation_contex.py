from typing import Dict
from frozendict import frozendict
from jetrule_context import JetRuleContext
from absl import flags

RC_OP_MAPPING = frozendict({
  # Arthemtic operators
  '+':                     '+',                    # done
  '-':                     '-',                    # done
  '/':                     '/',                    # done
  '*':                     '*',                    # done
  'abs':                   'abs',                  # done
  'max_multi_value':       'max_of',               # done
  'min_multi_value':       'min_of',               # done
  'sorted_head':           'sorted_head',          # done
  'sum_values':            'sum_values',           # done
  'to_int':                'to_int',               # done
  'to_real':               'to_double',            # done

  # Logical operators
  '!=':                    '!=',                   # done
  '==':                    '==',                   # done
  '<':                     '<',                    # done
  '<=':                    '<=',                   # done
  '>':                     '>',                    # done
  '>=':                    '>=',                   # done
  'and':                   'and',                  # done
  'not':                   'not',                  # done
  'or':                    'or',                   # done

  # String operators
  'apply_format':          'apply_format',         # done
  'cast_to_range_of':      'to_type_of',           # done
  'contains':              'contains',             # done
  'length_of':             'length_of',            # done
  'literal_regex':         'literal_regex',        # done
  'parse_usd_currency':    'parse_usd_currency',   # done
  'starts_with':           'starts_with',          # done
  'to_upper':              'to_upper',             # done
  'to_lower':              'to_lower',             # done

  # Resource operators
  'create_entity':         'create_entity',        # done
  'create_literal':        'create_literal',       # done
  'create_resource':       'create_resource',      # done
  'create_uuid_resource':  'create_uuid_resource', # done
  'different_from':        '!=',                   # done
  'exist':                 'exist',                # done
  'exist_not':             'exist_not',            # done
  'get_cardinality':       'size_of',              # done
  'is_literal':            'is_literal',           # done
  'is_resource':           'is_resource',          # done
  'is_no_value':           'is_null',              # done

  # Other operators
  'lookup':                'lookup',               # done
  'multi_lookup':          'multi_lookup',         # done



})
class ValidationContext:
  def __init__(self, jetrule_ctx: JetRuleContext):
    self.jetrule_ctx = jetrule_ctx
    self.entity_type = ''
    self.binded_vars = set()        # applicable to validate rule's identifiers
    self.entity_name = ''
    self.term_label = ''
    self.elm_type = ''

  def setEntityType(self, name: str):
    self.entity_type = name

  def setEntityName(self, name: str):
    self.entity_name = name

  def setTermLabel(self, label: str):
    self.term_label = label

  def setElmType(self, type: str):
    self.elm_type = type

  def err(self, msg: str) -> None:
    self.jetrule_ctx.err(msg)

  # for entity_type == 'rule' only
  def validateBinded(self, var: str) -> bool:
    is_binded = var in self.binded_vars
    if not is_binded:
      self.err(
        "Error {0} {1}: Variable '{2}' is not binded in this context '{3}' "
        "and must be for the rule to be valid.".format(
          self.entity_type, self.entity_name, var, self.term_label)
      )
    return is_binded

  # for entity_type == 'rule' only
  def addBinded(self, var: str) -> bool:
    self.binded_vars.add(var)
    return True
  
  def validateVar(self, var: str) -> bool:
    # print('*** Validate Variable for rule', self.entity_name, 'visiting elm type', self.elm_type, 'validating var', var)
    if self.elm_type == 'triple':
      self.err(
        "Error {0}: Variable '{1}' is not permitted in triple statement".format(
          self.entity_type, var)
      )
      return False

    if self.elm_type in ['filter', 'consequent', 'negated']:
      return self.validateBinded(var)
    return self.addBinded(var)
  
  # for entity_type == 'rule' and 'triple'
  def validateIdentifier(self, elm: object) -> bool:
    var = elm['value']
    # print('*** Validate Identifier for', self.entity_type, self.entity_name, 'visiting elm type', self.elm_type, 'validating identifier', var)
    defined = var in self.jetrule_ctx.defined_resources
    if not defined:
      is_predefined = self.jetrule_ctx.isValidPredefinedResources(var, var)
      if is_predefined is True:
        self.jetrule_ctx.addResource(var, var, 'predefined')
        return True
        
      # Check if we extract the identifier from the rules
      name = self.jetrule_ctx.addResourceFromRule(var)
      if not name:
        self.err(
          "Error {0} {1}: Identifier '{2}' is not defined in this context '{3}', "
          "it must be defined.".format(
            self.entity_type, self.entity_name, var, self.term_label)
        )
        return False
      elm['value'] = name
    return True
  
  # for entity_type == 'triple', applicable to subject and predicate
  # Auto create resource if identifier is not defined
  def validateTripleIdentifier(self, elm: object, source_fname: str) -> bool:
    var = elm['value']
    # print('*** Validate Triple Identifier for', self.entity_type, self.entity_name, 'visiting elm type', self.elm_type, 'validating identifier', var)
    defined = var in self.jetrule_ctx.defined_resources
    if not defined:
      is_predefined = self.jetrule_ctx.isValidPredefinedResources(var, var)
      if is_predefined is True:
        self.jetrule_ctx.addResource(var, var, 'predefined')
        return True
        
      # Add resource automatically
      self.jetrule_ctx.addResource(var, var, source_fname)
    return True

  def has_errors(self):
    return self.jetrule_ctx.ERROR
