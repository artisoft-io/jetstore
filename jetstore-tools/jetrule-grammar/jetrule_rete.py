from jetrule_context import JetRuleContext
from jet_listener_postprocessing import JetRulesPostProcessor
from dataclasses import dataclass, field
from typing import Any, Sequence, Set
from typing import Dict
import queue

class JetRuleRete:
  def __init__(self, ctx: JetRuleContext):
    self.ctx = ctx

  # =====================================================================================
  # addReteMarkup
  # -------------------------------------------------------------------------------------
  # Augmenting JetRule structure with rete markups: 
  #   - Add to antecedent: rete:parent_vertex and rete:vertex
  #   - Add to consequent: rete:vertex
  # rete:parent_vertex and rete:vertex are integers
  # --
  # Approach:
  # Build a rete network with beta nodes corresponding to rule antecedents.
  # Take the next rule to consider based on how frequent the first term of the rule is 
  # common among other rules. Then add the rule's antecedent to the rete network.
  # Connect nodes across rules by matching normalized labels
  def addReteMarkup(self) -> None:

    # rule structure
    rules = self.ctx.jetRules.get('jet_rules')

    # Rete data structure:
    # List of nodes, pos 0 is head vertex and is reserved
    # Node vertex is position in list
    self.ctx.rete_nodes = [{'vertex': 0, 'parent_vertex': 0, 'label': 'Head node'}]

    # For each rule, find the vertex matching a query based on partent_vertex and label
    for rule in rules:
      parent_vertex = 0
      for antecedent in rule['antecedents']:
        node = self.find_vertex(parent_vertex, antecedent['normalizedLabel'])
        if not node:
          node = {'vertex': len(self.ctx.rete_nodes), 'parent_vertex': parent_vertex, 'label': antecedent['normalizedLabel']}
          self.ctx.rete_nodes.append(node)

        antecedent['vertex'] = node['vertex']
        antecedent['parent_vertex'] = node['parent_vertex']
        parent_vertex = node['vertex']

      # Mark the consequets
      for consequent in rule['consequents']:
        consequent['vertex'] = parent_vertex

  def find_vertex(self, parent_vertex: int, label: str) -> object:
    for node in self.ctx.rete_nodes:
      if node['parent_vertex']==parent_vertex and node['label']==label:
        return node
    return None
