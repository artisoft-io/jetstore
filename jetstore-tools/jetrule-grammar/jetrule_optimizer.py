from jetrule_context import JetRuleContext
from jet_listener_postprocessing import JetRulesPostProcessor
from dataclasses import dataclass, field
from typing import Any, Set
from typing import Dict
import queue

# Utility class for the priority queue, 
# see https://docs.python.org/3/library/queue.html
@dataclass(order=True)
class PrioritizedItem:
    priority: int
    pos: int
    item: Any=field(compare=False)

class JetRuleOptimizer:
  def __init__(self, ctx: JetRuleContext):
    self.ctx = ctx

  # =====================================================================================
  # optimizeJetRules
  # -------------------------------------------------------------------------------------
  # Main method for the class
  # First pass at reordering antecendent terms and identifying possible head terms
  # Select the term occuring the most across all rules to be the head term.
  # Build the rete network and rearrange antecedents to maximize the number of
  # shared beta nodes among the rules
  # -
  # This method perform the first pass optimization, looking at each rule individually
  # This compute a new jet_rules list and replace the original list in self.ctx.jetRules
  def optimizeJetRules(self) -> None:

    # Authored rule structure
    rules = self.ctx.jetRules.get('jet_rules')

    # target rules structure
    optimized_rules = []

    # for each jetrule evaluate the priority of each antecendent and pick the one
    # with the highest priority (lower number)
    filters = None
    for rule in rules:
      name = rule.get('name')
      if not name: raise Exception("Invalid jetRules structure: ",self.ctx.jetRules)

      # prepare the optimized rule
      optimized_rule = {
        'name':name, 
        'properties': rule.get('properties'),
      }

      # The reordered antecedents
      optimized_antecedents = []

      # The antecedent that need to be reordered, starting with the full list of antecedents
      # that will be pruned successively intil that are all placed in optimized_antecedents
      antecedents = rule.get('antecedents')
      if not antecedents: raise Exception("Invalid jetRules structure: ",self.ctx.jetRules)

      # Collect filters from original rule's antecedent so to reallocated them after
      # the antecedent have been reordered
      filters = []
      for item in antecedents:
        filter = item.pop('filter', None)
        if filter:
          filters.append(filter)

      # Iterativelly allocate the antecedent with the top priority
      # Priority is computed as: 1000 - score
      # where score is computed based on triple configuration
      binded_vars = set()

      # ---------------------------------------------------------------------------------
      # optimized_antecedents
      # ---------------------------------------------------------------------------------
      # While we still have antecedent to place
      while antecedents:
        antecedent_q = queue.PriorityQueue()
        # Put the remaining antecedents into a priority queue to see which one is the
        # best one to place
        pos = 0
        for item in antecedents:
          t3 = item['triple']
          p = self.evaluatePriority(binded_vars, t3[0], t3[1], t3[2])
          antecedent_q.put(PrioritizedItem(1000 - p, pos, item))
          pos += 1

        # Get the best priority item
        priority_item: PrioritizedItem = antecedent_q.get()
        # print('  Got Priority item: priority', priority_item.priority,', pos',pos,', antecedent',priority_item.item)
        priority_antecedent = priority_item.item

        # update the binded variables to take in consideration the priority_item
        t3 = priority_antecedent['triple']
        self.updateBindedVar(binded_vars, t3[0], t3[1], t3[2])

        # Add the priority antecedent to the optimized_antecedents
        optimized_antecedents.append(priority_antecedent)

        # Remove it from the antecedent list
        del antecedents[priority_item.pos]


      # ---------------------------------------------------------------------------------
      # place filters on optimized_antecedents
      # ---------------------------------------------------------------------------------
      # Place the filters on the optimized_antecedents
      binded_vars.clear()
      for optimized_antecedent in optimized_antecedents:

        # update the binded variables to take in consideration this optimized_antecedent
        t3 = optimized_antecedent['triple']
        self.updateBindedVar(binded_vars, t3[0], t3[1], t3[2])
        
        # identify the filters that can be attached to this term
        pos = 0
        matched_pos = []
        matched_filters = []
        for filter in filters:
          # print('*** Can FILTER Match?')
          # print('*** binded var',binded_vars)
          # print('*** filter',filter)
          canb = self.canBind(binded_vars, filter)
          # print('*** can bind?',canb)
          # print()
          if canb:
            matched_pos.append(pos)
            matched_filters.append(filter)
          pos += 1
        
        # if match more than one filter, need to combined them into a
        # single filter with 'and' logical operator
        lfilter = None
        while matched_filters:
          if lfilter:
            lfilter = {
              'type': 'binary',
              'lhs': lfilter,
              'op': 'and',
              'rhs': matched_filters.pop()
            }
          else:
            lfilter = matched_filters.pop()
        
        # Add the filter to the antecedent
        if lfilter:
          optimized_antecedent['filter'] = lfilter
        
        # remove the placed filter(s), starting with last
        matched_pos.sort(reverse=True)
        for pos in matched_pos:
          del filters[pos]

      # Put the optimized_antecedents into the rule
      optimized_rule['antecedents'] = optimized_antecedents
      optimized_rule['consequents'] = rule.get('consequents')
      optimized_rule['authoredLabel'] = rule.get('label')

      # Carry-over of @JetCompilerDirective
      fname = rule.get('source_file_name')
      if fname:
        optimized_rule['source_file_name'] = fname

      # Add the rule to the new rule list
      optimized_rules.append(optimized_rule)

    # Update the context with updated rules
    self.ctx.jetRules['jet_rules'] = optimized_rules

    # Re-normalize the variables and add updated labels
    postProcessor = JetRulesPostProcessor(self.ctx)
    postProcessor.mapVariables()
    postProcessor.addNormalizedLabels()
    postProcessor.addLabels()

    # That should do it!
    # verify that we placed all filters
    if filters:
      print('ERROR REMAINING FILTERS:', filters)
      raise Exception('ERROR REMAINING FILTERS!')


  def updateBindedVar(self, binded_vars: Set[str], s: Dict[str, object], p: Dict[str, object], o: Dict[str, object]) -> None:
    if s['type'] == 'var':
      binded_vars.add(s['label'])
    if p['type'] == 'var':
      binded_vars.add(p['label'])
    if o['type'] == 'var':
      binded_vars.add(o['label'])

  # Returns True if elm contains only binded variables
  def canBind(self, binded_vars: Set[str], elm: Dict[str, object]) -> bool:
    type = elm['type']
    if type == 'binary':
      return self.canBind(binded_vars,elm['lhs']) and self.canBind(binded_vars,elm['rhs'])

    if type == 'unary':
      return self.canBind(binded_vars,elm['arg'])

    if type == 'var':
      return elm['label'] in binded_vars

    return True


  # =====================================================================================
  # evaluatePriority
  # -------------------------------------------------------------------------------------
  # Compute a priority number based on number of unbounded variables.
  # Returns int >= 0
  # Recursive function to evaluate the priority of an antecedent term
  def evaluatePriority(self, binded_vars: Set[str], s: Dict[str, object], p: Dict[str, object], o: Dict[str, object]) -> int:
    if not s or not p or not o: return -1
    stype = s['type']
    ptype = p['type']
    otype = o['type']

    sprio = 0
    pprio = 0
    oprio = 0
    # s
    if stype == 'var':
      sbind = s['label'] in binded_vars
      if sbind:
        sprio = 200 + 20
      else:
        sprio = 0 + 40
    elif stype == 'identifier':
      sprio = 100 + 0
    # p
    if ptype == 'var':
      pbind = p['label'] in binded_vars
      if pbind:
        pprio = 200 + 0
      else:
        pprio = 0 + 0
    elif ptype == 'identifier':
      pprio = 100 + 40
      if p['value'] == 'rdf:type':
        pprio += 10
    # o
    if otype == 'var':
      obind = o['label'] in binded_vars
      if obind:
        oprio = 200 + 40
      else:
        oprio = 0 + 20
    elif otype == 'identifier':
      oprio = 100 + 20
  
    return sprio + pprio + oprio

