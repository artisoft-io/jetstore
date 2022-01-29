from typing import Dict
from jetrule_context import JetRuleContext
from absl import flags

class JetRuleValidator:
  def __init__(self, ctx: JetRuleContext):
    self.ctx = ctx

  class ValidationContext:
    def __init__(self, jetrule_ctx: JetRuleContext, preflight: bool):
      self.jetrule_ctx = jetrule_ctx
      self.has_errors = False
      self.preflight = preflight
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
      self.has_errors = True
      if self.preflight:
        return
      self.jetrule_ctx.err(msg)

    def validateBinded(self, var: str) -> bool:
      is_binded = var in self.binded_vars
      # print('*** validateBinded for var',var,'is',is_binded)
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

  # =====================================================================================
  # validateVariables
  # -------------------------------------------------------------------------------------
  # Validate jetrules antecedent and consequents terms to ensure the correct use of 
  # unbinded variables
  # Returns True when valid, False otherwise
  def validateVariables(self, preflight: bool = False) -> bool:
    ctx = JetRuleValidator.ValidationContext(self.ctx, preflight)
    rules = self.ctx.jetRules.get('jet_rules')

    # for each jetrule validate antecedents and consequents terms
    for rule in rules:
      name = rule.get('name')
      if not name: raise Exception("Invalid jetRules structure: ",self.ctx.jetRules)
      ctx.setRuleName(name)

      for item in rule.get('antecedents', []):
        ctx.setTermLabel(item['label'])
        self.validateElm(item, ctx)
        if ctx.preflight and ctx.has_errors: return not ctx.has_errors

      for item in rule.get('consequents', []):
        ctx.setTermLabel(item['label'])
        ctx.setElmType('consequent')
        self.validateElm(item, ctx)
        if ctx.preflight and ctx.has_errors: return not ctx.has_errors
    
    return not ctx.has_errors

  # Recursive function to build the label of the rule antecedent or consequent
  def validateElm(self, elm: Dict[str, object], ctx: ValidationContext) -> bool:
    if not elm: return ctx.has_errors

    type = elm.get('type')
    if type is None: raise Exception("Invalid jetRules elm: ", elm)

    # Antecedent Term
    if type == 'antecedent':
      ctx.setElmType('antecedent')
      isNot = elm.get('isNot')
      if isNot:
        ctx.setElmType('negated')
      
      # Validate the triple elm
      triple = elm['triple']
      self.validateElm(triple[0], ctx)
      if ctx.preflight and ctx.has_errors: return ctx.has_errors

      self.validateElm(triple[1], ctx)
      if ctx.preflight and ctx.has_errors: return ctx.has_errors

      self.validateElm(triple[2], ctx)
      if ctx.preflight and ctx.has_errors: return ctx.has_errors

      # Validate the filter if any
      filter = elm.get('filter')
      if filter:
        ctx.setElmType('filter')
        self.validateElm(filter, ctx)
      return ctx.has_errors

    # Consequent Term
    if type == 'consequent':
      triple = elm['triple']
      self.validateElm(triple[0], ctx)
      if ctx.preflight and ctx.has_errors: return ctx.has_errors

      self.validateElm(triple[1], ctx)
      if ctx.preflight and ctx.has_errors: return ctx.has_errors
      return self.validateElm(triple[2], ctx)

    # binary operator
    if type == 'binary':
      self.validateElm(elm['lhs'], ctx)
      if ctx.preflight and ctx.has_errors: return ctx.has_errors
      return self.validateElm(elm['rhs'], ctx)

    if type == 'unary':
      return self.validateElm(elm['arg'], ctx)

    if type == 'var':
      return ctx.validateVar(elm['label'])

    if type in ['text','int','uint','long','ulong','identifier','keyword']:
      pass

    return ctx.has_errors

