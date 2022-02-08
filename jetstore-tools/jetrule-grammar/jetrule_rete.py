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
  def addReteMarkup(self) -> None:

    # rule structure
    rules = self.ctx.jetRules.get('jet_rules')

    # Rete data structure:
    # List of nodes, pos 0 is head vertex and is reserved
    # Node vertex is position in list
    self.ctx.rete_nodes = [{'vertex': 0, 'parent_vertex': 0, 'label': 'Head node'}]
    self.ctx.jetReteNodes = {'resources':[], 'rete_nodes': self.ctx.rete_nodes}

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

  # =====================================================================================
  # AddBetaRelationMarkup
  # -------------------------------------------------------------------------------------
  # Augmenting JetRule structure with rete markups: 
  #   - Add to antecedent: parent_vertex and vertex
  #   - Add to consequent: vertex
  # parent_vertex and vertex are integers
  # --
  # Approach:
  # Build a rete network with beta nodes corresponding to rule antecedents.
  # Add the rule's antecedent to the rete network.
  # Connect nodes across rules by matching normalized labels (merging common antecedents)
  def addBetaRelationMarkup(self) -> None:
    # Rete data structure:
    # List of nodes, pos 0 is head vertex and is reserved
    # Node vertex is position in list
    # self.ctx.rete_nodes = [{'vertex': 0, 'parent_vertex': 0, 'label': 'Head node'}]
    # Let's add the reverse relationship (children_vertexes)
    for node in self.ctx.rete_nodes:
      node['antecedent_node'] = None
      node['consequent_nodes'] = []
      node['children_vertexes'] = []
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
        self.ctx.rete_nodes[vertex]['antecedent_node'] = antecedent.copy()

      # Each node may have 0 or more consequents terms attached to them
      for consequent in rule['consequents']:
        vertex = consequent['vertex']
        self.ctx.rete_nodes[vertex]['consequent_nodes'].append(consequent.copy())

    # # *** Let's not do this -- let's keep rules unchanges
    # # Alter the jet_rules structure to replace antecedents with alpha_nodes
    # for rule in rules:

    #   if self.ctx.verbose:
    #     rule['alpha_nodes'] = []
    #   else:
    #     rule['alpha_node_vertices'] = []
      
    #   for antecedent in rule['antecedents']:
    #     vertex = antecedent['vertex']

    #     # We're puting reference to the whole rete_node if mode verbose
    #     # otherwise put the vertex only
    #     if self.ctx.verbose:
    #       rule['alpha_nodes'].append(self.ctx.rete_nodes[vertex])
    #     else:
    #       rule['alpha_node_vertices'].append(vertex)
      
    #   # remove the antecedents from rule since some are duplicated, rete_nodes have
    #   # the unique list of rete_nodes (unique antecedents)
    #   del rule['antecedents']
    #   # also remove consequents since they are now on the rete_node
    #   del rule['consequents']
    # # *** Let's not do this -- let's keep rules unchanges

    # Now we have the nodes connected to the rules
    # do dfs to collect the bounded variables at each node,
    # do dfs on children and consequent terms to get the var that are needed
    # Assign to each node (therefore to the associated antecedent) the beta row variables
    # prune variable that are not needed by descendent nodes
    for node in self.ctx.rete_nodes:
      parent_vertex = node['parent_vertex']
      if parent_vertex == 0 and node['vertex'] > 0:
        self._set_beta_var(set(), node)

    # LET'S NOT DO THIS
    # done, add to the jetrule data structure the rete_nodes
    # self.ctx.jetRules['rete_nodes'] = self.ctx.rete_nodes
    # print('*** RETE NODES:')
    # for node in self.ctx.rete_nodes:
    #   print(json.dumps(node, indent=2))

    # Perform validation on jetrule beta relation config
    for node in self.ctx.rete_nodes:
      parent_vertex = node['parent_vertex']
      if parent_vertex == 0 and node['vertex'] > 0:
        self._validate_rete_node(set(), set(), node)


  # -------------------------------------------------------------------------------------
  # Validate Rete Node
  # -------------------------------------------------------------------------------------
  # Perform validation on jetrule beta relation config:
  # pruned node must be pruned in descendent nodes (children_nodes)
  # var introduced at node (not in parent node) shall not be marked as is_binded = True
  def _validate_rete_node(self, parent_vars: Set[str], parent_pruned_vars: Set[str], node: object):
    # print('*** validate RETE NODE',node)
    # check that the pruned var in parent are also pruned var in node
    # meaning parent_pruned_var.issubset(node_prune_var) is True
    if not parent_pruned_vars.issubset(node['antecedent_node']['pruned_var']):
      raise Exception("Invalid rete_node, missing prune var form parent, rete_node:",node,'parent_pruned_vars',parent_pruned_vars)

    for child_vertex in node['children_vertexes']:
      self._validate_rete_node(set(node['antecedent_node']['beta_relation_vars']), set(node['antecedent_node']['pruned_var']), self.ctx.rete_nodes[child_vertex])  


  def _validate_var(self, parent_binded_var: Set[str], elm: object):
    type = elm.get('type')
    if type is None: raise Exception("Invalid jetRules elm: ", elm)

    if type == 'antecedent' or type == 'consequent':
      triple = elm['triple']
      self._validate_var(parent_binded_var, triple[0])
      self._validate_var(parent_binded_var, triple[1])
      self._validate_var(parent_binded_var, triple[2])
      filter = elm.get('filter')
      if filter:
        self._validate_var(parent_binded_var, filter)
      return

    if type == 'binary':
      self._validate_var(parent_binded_var, elm['lhs'])
      self._validate_var(parent_binded_var, elm['rhs'])
      return

    if type == 'unary':
      self._validate_var(parent_binded_var, elm['arg'])
      return

    if type == 'var':
        if elm['is_binded'] and not elm['id'] in parent_binded_var:
          raise Exception("Invalid rete_node, var marked as binded but is not in parent beta variable, var:",elm,'parent_binded_vars',parent_binded_var)
    return


  # -------------------------------------------------------------------------------------
  # Set Beta Variables
  # -------------------------------------------------------------------------------------
  # This work on self.ctx.rete_nodes data structure, argument 'node' is a rete_nodes
  def _set_beta_var(self, binded_vars: Set[str], node: object):

    # while collecting var of antecedent_node, add 'is_binded' indicator to var nodes
    # to indicate if the variable is binded to the parent antecedent
    antecedent = node['antecedent_node']
    binded_vars = binded_vars.union(self._add_var(binded_vars, antecedent, check_binded=True))

    # collect the downstream var (dependent var)
    dependent_vars = self._add_child_var({'consequent_nodes': node['consequent_nodes'], 'children_vertexes': node['children_vertexes']})

    # let's do it
    pruned_var = binded_vars.difference(dependent_vars)
    beta_relation_vars = binded_vars.difference(pruned_var)

    # let's put that in the antecedent node
    antecedent['beta_relation_vars'] = [ v for v in beta_relation_vars]
    antecedent['beta_relation_vars'].sort()
    antecedent['pruned_var'] = [v for v in pruned_var]
    antecedent['pruned_var'].sort()
    # print('Vertex ', node['vertex'])
    # print('binded_vars', binded_vars)
    # print('pruned_var', pruned_var)
    # print('beta_relation_vars', beta_relation_vars)

    # Now do the children
    for child in node['children_vertexes']:
      self._set_beta_var(binded_vars, self.ctx.rete_nodes[child])


  def _add_var(self, parent_binded_var: Set[str], elm: object, check_binded) -> Set[str]:
    type = elm.get('type')
    if type is None: raise Exception("Invalid jetRules elm: ", elm)

    if type == 'antecedent' or type == 'consequent':
      triple = elm['triple']
      binded_var = self._add_var(parent_binded_var, triple[0], check_binded=check_binded)
      binded_var = binded_var.union(self._add_var(parent_binded_var, triple[1], check_binded=check_binded))
      binded_var = binded_var.union(self._add_var(parent_binded_var, triple[2], check_binded=check_binded))
      filter = elm.get('filter')
      if filter:
        binded_var = binded_var.union(self._add_var(parent_binded_var, filter, check_binded=check_binded))
      return binded_var

    if type == 'binary':
      binded_var = self._add_var(parent_binded_var, elm['lhs'], check_binded=check_binded)
      binded_var = binded_var.union(self._add_var(parent_binded_var, elm['rhs'], check_binded=check_binded))
      return binded_var

    if type == 'unary':
      binded_var = self._add_var(parent_binded_var, elm['arg'], check_binded=check_binded)
      return binded_var

    if type == 'var':
      if check_binded:
        if elm['id'] in parent_binded_var:
          elm['is_binded'] = True
        else:
          elm['is_binded'] = False
      return set([elm['id']])
    return set()

  # Add child var recursively (antecedents and consequents alike)
  def _add_child_var(self, rete_node: object) -> Set[str]:
    dependent_vars = set()
    antecedent_node = rete_node.get('antecedent_node')
    if antecedent_node:
      dependent_vars = dependent_vars.union(self._add_var(set(), antecedent_node, check_binded=False))

    for consequent in rete_node['consequent_nodes']:
      dependent_vars = dependent_vars.union(self._add_var(set(), consequent, check_binded=False))

    for child_vertex in rete_node['children_vertexes']:
      dependent_vars = dependent_vars.union(self._add_child_var(self.ctx.rete_nodes[child_vertex]))

    return dependent_vars

  # =====================================================================================
  # normalizeReteNodes
  # -------------------------------------------------------------------------------------
  # Perform last manipulation on self.ctx.rete_nodes to produce JetRuleContext.jetReteNodes
  # data structure to normalize the elements and be ready to persist using a sql model using sqlite
  # -------------------------------------------------------------------------------------
  def normalizeReteNodes(self) -> None:
    if self.ctx.verbose:
      print('Warning: JetRuleContext.verbose is True, will not normalize the Rete Nodes')
      return

    # replace the rete_node with the original rete_node['antecedent_node']
    for i in range(1, len(self.ctx.rete_nodes)):
      node = self.ctx.rete_nodes[i]['antecedent_node']
      node['consequent_nodes'] = self.ctx.rete_nodes[i]['consequent_nodes']
      node['children_vertexes'] = self.ctx.rete_nodes[i]['children_vertexes']
      self.ctx.rete_nodes[i] = node
    
    # Add the consequent nodes at the end of the antecedent nodes
    for i in range(1, len(self.ctx.rete_nodes)):
      for node in self.ctx.rete_nodes[i]['consequent_nodes']:
        self.ctx.rete_nodes.append(node)
      del self.ctx.rete_nodes[i]['consequent_nodes']

    # Add resources and literals to rete config
    resources = self.ctx.jetReteNodes['resources']
    key = 0
    for k, v in self.ctx.resourceMap.items():
      v['key'] = key
      key += 1
      resources.append(v)

    # Transform the node['triple'] into reference to the resource
    # Add var elm type to resources
    for i in range(1, len(self.ctx.rete_nodes)):
      node = self.ctx.rete_nodes[i]
      triple = node['triple']
      del node['triple']
      node['subject'] = self._map_elm(resources, triple[0])
      node['predicate'] = self._map_elm(resources, triple[1])
      obj_elm = triple[2]
      if obj_elm['type'] in ['binary', 'unary']:
        node['obj_expr'] = self._map_expr(resources, obj_elm)
      else:
        node['object'] = self._map_elm(resources, triple[2])   


  # add elm to resources, set key as pos in sequence, return key
  def _add_key(self, resources: Sequence[Dict[str, object]], elm: Dict[str, object]) -> int:
    key = len(resources)
    elm['key'] = key
    resources.append(elm)
    return key

  # map the expr to itself by replacing the leaf resource to a key
  def _map_expr(self, resources, elm: Dict[str, object]) -> Dict[str, object]:
    type = elm['type']
    if type == 'binary':
      elm['lhs'] = self._map_expr(resources, elm['lhs'])
      elm['rhs'] = self._map_expr(resources, elm['rhs'])
      return elm

    if type == 'unary':
      elm['arg'] = self._map_expr(resources, elm['arg'])
      return elm
    
    # type must be literal, meaning we can use _map_elm
    # that returns the key
    return self._map_elm(resources, elm)


  # map elm to an entry in resources based on type
  # May add elm to resources
  # return key,
  def _map_elm(self, resources, elm) -> int:
    type = elm['type']
    
    if type == 'var':
      return self._add_key(resources, elm)

    if type == 'identifier':
      return self.ctx.resourceMap[elm['value']]['key']

    if type in ['int','uint','long','ulong','double','text']:
      elm['inline'] = True
      return self._add_key(resources, elm)

    raise Exception('ERROR JetRuleRete._add_elm: unknown type: '+str(type))
