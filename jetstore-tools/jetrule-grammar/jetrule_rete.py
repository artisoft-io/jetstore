from jetrule_context import JetRuleContext
from typing import Any, Sequence, Set
from typing import Dict
import apsw
import json

print ("      Using APSW file",apsw.__file__)                # from the extension module
print ("         APSW version",apsw.apswversion())           # from the extension module
print ("   SQLite lib version",apsw.sqlitelibversion())      # from the sqlite library code
print ("SQLite header version",apsw.SQLITE_VERSION_NUMBER)   # from the sqlite header file at compile time
print()

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
  def addBetaRelationMarkup(self) -> None:
    # Rete data structure:
    # List of nodes, pos 0 is head vertex and is reserved
    # Node vertex is position in list
    # self.ctx.rete_nodes = [{'vertex': 0, 'parent_vertex': 0, 'label': 'Head node'}]
    # Let's add the reverse relationship
    for node in self.ctx.rete_nodes:
      node['children_vertexes'] = []
      node['consequent_nodes'] = []
      parent_vertex = node['parent_vertex']
      if node['vertex'] > 0:
        self.ctx.rete_nodes[parent_vertex]['children_vertexes'].append(node['vertex'])

    # now we have the tree in place, let's connect to the jetrule json
    # rule structure
    rules = self.ctx.jetRules.get('jet_rules')
    for rule in rules:
      # Each node have one antecedent attached to it
      for antecedent in rule['antecedents']:
        vertex = antecedent['vertex']
        self.ctx.rete_nodes[vertex]['antecedent_node'] = antecedent
      # Each node may have 0 or more consequents terms attached to them
      for consequent in rule['consequents']:
        vertex = consequent['vertex']
        self.ctx.rete_nodes[vertex]['consequent_nodes'].append(consequent)
    # Now we have the nodes connected to the rules
    # do dfs to collect the bounded variables at each node,
    # do dfs on children and consequent terms to get the var that are needed
    # Assign to each node (therefore to the associated antecedent) the beta row variables
    # prune variable that are not needed by descendent nodes
    for node in self.ctx.rete_nodes:
      parent_vertex = node['parent_vertex']
      if parent_vertex == 0 and node['vertex'] > 0:
        bounded_vars = set()
        self._set_beta_var(bounded_vars, node)

    # done, now alter the jetrule data structure to save the rete_nodes rather than the rules
    del self.ctx.jetRules['jet_rules']    
    self.ctx.jetRules['rete_nodes'] = self.ctx.rete_nodes
    # print('*** RETE NODES:')
    # for node in self.ctx.rete_nodes:
    #   print(json.dumps(node, indent=2))


  def _set_beta_var(self, bounded_vars: Set[str], node: object):
    antecedent = node['antecedent_node']
    self._add_var(bounded_vars, antecedent)

    # collect the downstream var (dependent var)
    dependent_vars = set()
    self._add_child_var(dependent_vars, {'consequent_nodes': node['consequent_nodes'], 'children_vertexes': node['children_vertexes']})

    # let's do it
    pruned_var = bounded_vars.difference(dependent_vars)
    beta_relation_vars = bounded_vars.difference(pruned_var)

    # let's put that in the antecedent node
    antecedent['beta_relation_vars'] = [ v for v in beta_relation_vars]
    antecedent['beta_relation_vars'].sort()
    antecedent['pruned_var'] = [v for v in pruned_var]
    antecedent['pruned_var'].sort()
    # print('Vertex ', node['vertex'])
    # print('bounded_vars', bounded_vars)
    # print('pruned_var', pruned_var)
    # print('beta_relation_vars', beta_relation_vars)

    # Now do the children
    for child in node['children_vertexes']:
      self._set_beta_var(bounded_vars, self.ctx.rete_nodes[child])


  def _add_var(self, bounded_var: Set[str], elm: object) -> None:
    type = elm.get('type')
    if type is None: raise Exception("Invalid jetRules elm: ", elm)

    if type == 'antecedent' or type == 'consequent':
      triple = elm['triple']
      self._add_var(bounded_var, triple[0])
      self._add_var(bounded_var, triple[1])
      self._add_var(bounded_var, triple[2])
      filter = elm.get('filter')
      if filter:
        self._add_var(bounded_var, filter)
      return

    if type == 'binary':
      self._add_var(bounded_var, elm['lhs'])
      self._add_var(bounded_var, elm['rhs'])
      return

    if type == 'unary':
      self._add_var(bounded_var, elm['arg'])
      return

    if type == 'var':
      bounded_var.add(elm['id'])
      return

  # Add child var recursively (antecedents and consequents alike)
  def _add_child_var(self, dependent_vars: Set[str], rete_node: object):
    
    antecedent_node = rete_node.get('antecedent_node')
    if antecedent_node:
      self._add_var(dependent_vars, antecedent_node)

    for consequent in rete_node['consequent_nodes']:
      self._add_var(dependent_vars, consequent)

    for child_vertex in rete_node['children_vertexes']:
      self._add_child_var(dependent_vars, self.ctx.rete_nodes[child_vertex])

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
