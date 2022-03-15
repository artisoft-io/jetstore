from typing import Dict
from jetrule_context import JetRuleContext
from jetrule_validation_contex import ValidationContext

class JetRuleValidator:
  def __init__(self, ctx: JetRuleContext):
    self.ctx = ctx

  # =====================================================================================
  # validateJetRule
  # -------------------------------------------------------------------------------------
  # Validate jetrules antecedent and consequents terms to ensure the correct use of 
  # unbinded variables
  # Returns True when valid, False otherwise
  def validateJetRule(self) -> bool:
    ctx = ValidationContext(self.ctx)

    # for each jetrule validate antecedents and consequents terms
    for rule in self.ctx.jet_rules:
      name = rule.get('name')
      if not name: raise Exception("Invalid jetRules structure: ",self.ctx.jetRules)
      ctx.setRuleName(name)

      for item in rule.get('antecedents', []):
        ctx.setTermLabel(item['label'])
        self.validateElm(item, ctx)

      # print('*** Validate JetRule rule', ctx.rule_name, 'done with antecedents')

      for item in rule.get('consequents', []):
        ctx.setTermLabel(item['label'])
        ctx.setElmType('consequent')
        self.validateElm(item, ctx)
    
    return not ctx.has_errors()

  # Recursive function to build the label of the rule antecedent or consequent
  def validateElm(self, elm: Dict[str, object], ctx: ValidationContext) -> bool:
    type = elm.get('type')
    if type is None: raise Exception("Invalid jetRules elm: ", elm)

    # print('    validateElm', ctx.rule_name, 'elem type', type)

    # Antecedent Term
    if type == 'antecedent':
      ctx.setElmType('antecedent')
      isNot = elm.get('isNot')
      if isNot:
        ctx.setElmType('negated')
      
      # Validate the triple elm
      triple = elm['triple']
      self.validateElm(triple[0], ctx)
      self.validateElm(triple[1], ctx)
      self.validateElm(triple[2], ctx)

      # Validate the filter if any
      filter = elm.get('filter')
      if filter:
        ctx.setElmType('filter')
        self.validateElm(filter, ctx)
      return ctx.has_errors()

    # Consequent Term
    if type == 'consequent':
      ctx.setElmType('consequent')
      triple = elm['triple']
      self.validateElm(triple[0], ctx)

      self.validateElm(triple[1], ctx)
      return self.validateElm(triple[2], ctx)

    # binary operator
    if type == 'binary':
      self.validateElm(elm['lhs'], ctx)
      return self.validateElm(elm['rhs'], ctx)

    if type == 'unary':
      return self.validateElm(elm['arg'], ctx)

    if type == 'var':
      return ctx.validateVar(elm['label'])

    if type == 'identifier':
      return ctx.validateIdentifier(elm['value'])

    if type in ['text','int','uint','long','ulong','date','datetime','keyword']:
      pass

    return ctx.has_errors()
