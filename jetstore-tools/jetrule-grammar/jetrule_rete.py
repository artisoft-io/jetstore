from jetrule_context import JetRuleContext
from typing import Any, Sequence, Set
from typing import Dict

class JetRuleRete:
  def __init__(self, ctx: JetRuleContext):
    self.ctx = ctx

  # =====================================================================================
  # addReteMarkup
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
  def addReteMarkup(self) -> None:

    # Rete data structure:
    # List of nodes, pos 0 is head vertex and is reserved
    # Node vertex is position in list
    self.ctx.rete_nodes = [{'vertex': 0, 'parent_vertex': 0, 'label': 'Head node'}]
    self.ctx.jetReteNodes = {
      'main_rule_file_name': self.ctx.main_rule_fname, 
      'support_rule_file_names': self.ctx.imported.get(self.ctx.main_rule_fname), 
      'resources':[], 
      'lookup_tables': self.ctx.lookup_tables, 
      'rete_nodes': self.ctx.rete_nodes
    }

    # For each rule, find the vertex matching a query based on parent_vertex and label
    for rule in self.ctx.jet_rules:
      parent_vertex = 0
      for antecedent in rule['antecedents']:
        rete_node = self.find_vertex(parent_vertex, antecedent['normalizedLabel'])
        if not rete_node:
          rete_node = {
            'vertex': len(self.ctx.rete_nodes), 
            'parent_vertex': parent_vertex, 
            'label': antecedent['normalizedLabel'],
            'rules': [],
            'salience': []
          }
          self.ctx.rete_nodes.append(rete_node)

        antecedent['vertex'] = rete_node['vertex']
        antecedent['parent_vertex'] = rete_node['parent_vertex']
        parent_vertex = rete_node['vertex']

      # Mark the consequets
      for consequent in rule['consequents']:
        consequent['vertex'] = parent_vertex

  def find_vertex(self, parent_vertex: int, label: str) -> object:
    for rete_node in self.ctx.rete_nodes:
      if rete_node['parent_vertex']==parent_vertex and rete_node['label']==label:
        return rete_node
    return None

  # =====================================================================================
  # AddBetaRelationMarkup
  # -------------------------------------------------------------------------------------
  def addBetaRelationMarkup(self) -> None:
    # Rete data structure:
    # List of nodes, pos 0 is head vertex and is reserved
    # Node vertex is position in list
    # self.ctx.rete_nodes = [{'vertex': 0, 'parent_vertex': 0, 'label': 'Head node'}]
    # Let's add the reverse relationship (children_vertexes)
    for rete_node in self.ctx.rete_nodes:
      rete_node['antecedent_node'] = None
      rete_node['consequent_nodes'] = []
      rete_node['children_vertexes'] = []
      parent_vertex = rete_node['parent_vertex']
      if rete_node['vertex'] > 0:
        self.ctx.rete_nodes[parent_vertex]['children_vertexes'].append(rete_node['vertex'])

    # now we have the tree in place, let's connect to the jetrule json
    # rule structure
    for rule in self.ctx.jet_rules:
      # Attached a copy of the antecedent to the rete_node (to have access to triple elm)
      for antecedent in rule['antecedents']:
        vertex = antecedent['vertex']
        self.ctx.rete_nodes[vertex]['antecedent_node'] = antecedent.copy()

      # Each node may have 0 or more consequents terms attached to them
      for consequent in rule['consequents']:
        vertex = consequent['vertex']
        self.ctx.rete_nodes[vertex]['consequent_nodes'].append(consequent.copy())
      
      # Carry rule's name and saliance to rete_node associated with the last antecedent
      # of the rule
      rete_node = self.ctx.rete_nodes[rule['antecedents'][-1]['vertex']]
      rete_node['rules'].append(rule['name'])
      rete_node['salience'].append(rule['salience'])


    # Now we have the nodes connected to the rules
    # do dfs to collect the bounded variables at each node,
    # do dfs on children and consequent terms to get the var that are needed
    # Assign to each node (therefore to the associated antecedent) the beta row variables
    # prune variable that are not needed by descendent nodes
    for rete_node in self.ctx.rete_nodes:
      parent_vertex = rete_node['parent_vertex']
      if parent_vertex == 0 and rete_node['vertex'] > 0:
        self._set_beta_var(set(), rete_node)

    # Perform validation on jetrule beta relation config
    for rete_node in self.ctx.rete_nodes:
      parent_vertex = rete_node['parent_vertex']
      if parent_vertex == 0 and rete_node['vertex'] > 0:
        self._validate_rete_node(set(), set(), rete_node)


  # -------------------------------------------------------------------------------------
  # Set Beta Variables
  # -------------------------------------------------------------------------------------
  # This work on self.ctx.rete_nodes data structure, argument 'rete_node' is a rete_nodes
  def _set_beta_var(self, binded_vars: Set[str], rete_node: object):

    # while collecting var of antecedent_node and consequent_nodes, add 'is_binded' indicator to var nodes
    # to indicate if the variable is binded to the parent antecedent
    antecedent = rete_node['antecedent_node']
    binded_vars = binded_vars.union(self._add_var(binded_vars, antecedent, check_binded=True))
    for item in rete_node['consequent_nodes']:
      binded_vars = binded_vars.union(self._add_var(binded_vars, item, check_binded=True))

    # collect the downstream var (dependent var)
    dependent_vars = self._add_child_var({'consequent_nodes': rete_node['consequent_nodes'], 'children_vertexes': rete_node['children_vertexes']})

    # let's do it
    pruned_var = binded_vars.difference(dependent_vars)
    beta_relation_vars = binded_vars.difference(pruned_var)

    # let's put that in the antecedent rete_node
    antecedent['beta_relation_vars'] = [ v for v in beta_relation_vars]
    antecedent['beta_relation_vars'].sort()
    antecedent['pruned_var'] = [v for v in pruned_var]
    antecedent['pruned_var'].sort()
    # print('Vertex ', rete_node['vertex'])
    # print('binded_vars', binded_vars)
    # print('pruned_var', pruned_var)
    # print('beta_relation_vars', beta_relation_vars)

    # Now do the children
    for child in rete_node['children_vertexes']:
      self._set_beta_var(binded_vars, self.ctx.rete_nodes[child])


  def _add_var(self, parent_binded_var: Set[str], elm: object, check_binded) -> Set[str]:
    type = elm.get('type')
    if type is None: raise Exception("Invalid jetRules elm: ", elm)

    if type == 'antecedent' or type == 'consequent':
      triple = elm['triple']
      binded_var = self._add_var(parent_binded_var, triple[0], check_binded=check_binded)
      binded_var = binded_var.union(self._add_var(parent_binded_var, triple[1], check_binded=check_binded))
      binded_var = binded_var.union(self._add_var(parent_binded_var, triple[2], check_binded=check_binded))
      # filter shall have only binded var and var of current rete_node are considered binded for filter and consequent nodes
      # Also, since filter does not add new var, only check if check_binded is True
      filter = elm.get('filter')
      if check_binded and filter:
        self._add_var(parent_binded_var.union(binded_var), filter, check_binded=check_binded)
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


  # -------------------------------------------------------------------------------------
  # Validate Rete Node
  # -------------------------------------------------------------------------------------
  # Perform validation on jetrule beta relation config:
  # pruned rete_node must be pruned in descendent nodes (children_nodes)
  # var introduced at rete_node (not in parent rete_node) shall NOT be marked as is_binded = True
  # while var introduced in filter of rete_node (not in parent rete_node) shall be marked as is_binded = True
  def _validate_rete_node(self, parent_vars: Set[str], parent_pruned_vars: Set[str], rete_node: object):
    # print('*** validate RETE NODE',rete_node)
    # check that the pruned var in parent are also pruned var in rete_node
    # meaning parent_pruned_var.issubset(node_prune_var) is True
    if not parent_pruned_vars.issubset(rete_node['antecedent_node']['pruned_var']):
      raise Exception("Invalid rete_node, missing prune var form parent, rete_node:",rete_node,'parent_pruned_vars',parent_pruned_vars)

    #  Validate that var at antecedent rete_node (not in parent rete_node) shall NOT be marked as is_binded = True
    self._validate_var(parent_vars, rete_node['antecedent_node'])

    #  Validate that var at consequent rete_node shall ALWAYS be marked as is_binded = True
    binded_vars = parent_vars.union(rete_node['antecedent_node']['beta_relation_vars'])
    for item in rete_node['consequent_nodes']:
      self._validate_var(binded_vars, item)

    #  Validate that var at filter rete_node shall ALWAYS be marked as is_binded = True
    filter = rete_node['antecedent_node'].get('filter')
    if filter:
      self._validate_var(binded_vars, filter)

    for child_vertex in rete_node['children_vertexes']:
      self._validate_rete_node(set(rete_node['antecedent_node']['beta_relation_vars']), set(rete_node['antecedent_node']['pruned_var']), self.ctx.rete_nodes[child_vertex])  


  def _validate_var(self, parent_binded_var: Set[str], elm: object):
    type = elm.get('type')
    if type is None: raise Exception("Invalid jetRules elm: ", elm)

    if type == 'antecedent' or type == 'consequent':
      triple = elm['triple']
      self._validate_var(parent_binded_var, triple[0])
      self._validate_var(parent_binded_var, triple[1])
      self._validate_var(parent_binded_var, triple[2])
      # To validate the filter, we need to consider the parent_binded_var AND elm binded_var
      filter = elm.get('filter')
      if filter:
        self._validate_var(parent_binded_var.union(elm['beta_relation_vars']), filter)
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
        raise Exception("Invalid rete_node, var marked as binded but is not in expected binded variable, var:",elm,'expected_binded_vars',parent_binded_var)
      if not elm['is_binded'] and elm['id'] in parent_binded_var:
        raise Exception("Invalid rete_node, var NOT marked as binded but IS in expected binded variable, var:",elm,'expected_binded_vars',parent_binded_var)
    return

  # =====================================================================================
  # normalizeReteNodes
  # -------------------------------------------------------------------------------------
  # Perform last manipulation on self.ctx.rete_nodes to produce JetRuleContext.jetReteNodes
  # data structure to normalize the elements and be ready to persist using a sql model using sqlite
  # -------------------------------------------------------------------------------------
  def normalizeReteNodes(self) -> None:
    # cleanup the head rete_node
    head_node = self.ctx.rete_nodes[0]
    head_node['type'] = 'head_node'
    del head_node['label']
    del head_node['antecedent_node']
    del head_node['consequent_nodes']    

    # replace the rete_node with the rete_node['antecedent_node']
    for i in range(1, len(self.ctx.rete_nodes)):
      rete_node = self.ctx.rete_nodes[i]['antecedent_node']
      rete_node['consequent_nodes'] = self.ctx.rete_nodes[i]['consequent_nodes']
      rete_node['children_vertexes'] = self.ctx.rete_nodes[i]['children_vertexes']
      # Carry over rules name and salience, if populated
      rules = self.ctx.rete_nodes[i].get('rules')
      if rules:
        rete_node['rules'] = rules
        rete_node['salience'] = self.ctx.rete_nodes[i].get('salience')
      # remove the label since it use the original var and it's not meaningful in
      # the rete network
      del rete_node['label']
      self.ctx.rete_nodes[i] = rete_node
    
    # Add a copy the consequent nodes at the end of the antecedent nodes
    # We put a copy to leave the original jetRule unchanges
    for i in range(1, len(self.ctx.rete_nodes)):
      for original_node in self.ctx.rete_nodes[i]['consequent_nodes']:
        # remove the label since it use the original var and it's not meaningful in
        # the rete network
        node = original_node.copy()
        del node['label']
        self.ctx.rete_nodes.append(node)
      del self.ctx.rete_nodes[i]['consequent_nodes']

    # Add resources and literals to rete config
    resources = self.ctx.jetReteNodes['resources']
    key = 0
    for k, v in self.ctx.resourceMap.items():
      v['key'] = key
      key += 1
      resources.append(v)

    # Transform the rete_node['triple'] into reference to the resource
    # also transform the rete_node['filter'] into reference to the resources
    # Add var elm type to resources
    for i in range(1, len(self.ctx.rete_nodes)):
      rete_node = self.ctx.rete_nodes[i]
      triple = rete_node['triple']
      del rete_node['triple']
      rete_node['subject_key'] = self._map_elm(resources, triple[0])
      rete_node['predicate_key'] = self._map_elm(resources, triple[1])
      obj_elm = triple[2]
      if obj_elm['type'] in ['binary', 'unary']:
        rete_node['obj_expr'] = self._map_expr(resources, obj_elm)
      else:
        rete_node['object_key'] = self._map_elm(resources, triple[2])
      filter = rete_node.get('filter')
      if filter:
        rete_node['filter'] = self._map_expr(resources, filter)


  # add elm to resources, set key as pos in sequence, return key
  def _add_key(self, resources: Sequence[Dict[str, object]], elm: Dict[str, object]) -> int:
    key = len(resources)
    elm['key'] = key
    elm['source_file_name'] = self.ctx.main_rule_fname
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
      # remove label and value since they reference the original var and it's
      # not of use for the rete network
      del elm['value']
      del elm['label']
      return self._add_key(resources, elm)

    if type == 'identifier':
      return self.ctx.resourceMap[elm['value']]['key']

    if type in ['int','uint','long','ulong','double','text', 'keyword']:
      elm['inline'] = True
      return self._add_key(resources, elm)

    raise Exception('ERROR JetRuleRete._add_elm: unknown type: '+str(type))
