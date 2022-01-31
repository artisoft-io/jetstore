from typing import Dict
from jetrule_context import JetRuleContext
from absl import flags

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
  
  def validateIdentifier(self, var: str) -> bool:
    # print('*** Validate Identifier for rule', self.rule_name, 'visiting elm type', self.elm_type, 'validating identifier', var)
    defined = var in self.jetrule_ctx.defined_resources
    if not defined:
      self.err(
        "Error rule {0}: Identifier '{1}' is not defined in this context '{2}', "
        "it must be define.".format(
          self.rule_name, var, self.term_label)
      )
    return defined

  def has_errors(self):
    return self.jetrule_ctx.ERROR
