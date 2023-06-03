from logging import raiseExceptions
from jetrule_context import JetRuleContext
from typing import Any, Sequence, Set
from typing import Dict
from operator import itemgetter

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
    consequent_seq_dict = {}
    for rule in self.ctx.jet_rules:
      # Attached a copy of the antecedent to the rete_node (to have access to triple elm)
      # The problem we have here is that the antecedent_node copy will be set to the last copy
      # made for the same vertex
      for antecedent in rule['antecedents']:
        vertex = antecedent['vertex']
        self.ctx.rete_nodes[vertex]['antecedent_node'] = antecedent.copy()

      # Each node may have 0 or more consequents terms attached to them
      consequent_seq = consequent_seq_dict.get(vertex, 0)
      # A fresh list for this rule's consequent with the new copy of the consequents
      new_consequents = []
      for consequent in rule['consequents']:
        vertex = consequent['vertex']
        consequent_copy = consequent.copy()
        consequent_seq += 1
        consequent_copy['consequent_seq'] = consequent_seq
        consequent_copy['consequent_for_rule'] = rule['name']
        consequent_copy['consequent_salience'] = rule['salience']
        self.ctx.rete_nodes[vertex]['consequent_nodes'].append(consequent_copy)
        new_consequents.append(consequent_copy)
      consequent_seq_dict[vertex] = consequent_seq
      rule['consequents'] = new_consequents

      # Carry rule's name and salience to rete_node:
      #   - associated with the last antecedent of the rule
      #   - associated with the consequent term of the rule (consequent_copy)
      rete_node = self.ctx.rete_nodes[rule['antecedents'][-1]['vertex']]
      rete_node['rules'].append(rule['name'])
      rete_node['salience'].append(rule['salience'])

    # Now that we have copied the antecedent and consequence, use the same copies in jetRules
    for rule in self.ctx.jet_rules:
      rule['antecedents'] = [
        self.ctx.rete_nodes[node['vertex']]['antecedent_node'] for node in rule['antecedents']
      ]


    # Now we have the nodes connected to the rules
    # do dfs to collect the binded variables at each node,
    # do dfs on children and consequent terms to get the var that are needed
    # Assign to each node (therefore to the associated antecedent) the beta row variables
    # prune variable that are not needed by descendent nodes
    for rete_node in self.ctx.rete_nodes:
      parent_vertex = rete_node['parent_vertex']
      if parent_vertex == 0 and rete_node['vertex'] > 0:
        self._set_beta_var(set(), rete_node)

    # Add 'var_pos' to var nodes of antecedents that are NOT binded,
    # Set the var_pos to the triple pos, i.e. 0, 1, 2 for s, p, o resp.
    # Note this applied to var that are needed downstream, otherwise no need
    # to keep them.
    # Also, collect the var nodes into 'beta_var_nodes', take the parent's
    # 'beta_var_nodes' and add the unbinded var of the current antecedent
    # that are NOT pruned.
    # This will be used to construct the beta_row_initializers elms
    # Note: at this point, reteNodes contains only antecedents and the head node
    for vertex in range(1, len(self.ctx.rete_nodes)):
      antecedent_node = self.ctx.rete_nodes[vertex]['antecedent_node']
      parent_vertex = antecedent_node['parent_vertex']
      beta_var_nodes = []
      if parent_vertex:
        parent_beta_var_nodes = self.ctx.rete_nodes[parent_vertex]['antecedent_node']['beta_var_nodes'] 
        for i in range(len(parent_beta_var_nodes)):
          parent_var_node = parent_beta_var_nodes[i]
          if parent_var_node['id'] in antecedent_node['pruned_var']:
            continue
          var_node = parent_var_node.copy()
          var_node.pop('label', None)
          var_node.pop('value', None)
          var_node['is_binded'] = True
          var_node['var_pos'] = i
          var_node['vertex'] = antecedent_node['vertex']
          beta_var_nodes.append(var_node)

      triple = antecedent_node['triple']
      for pos in range(3):
        elm = triple[pos]
        if elm['type'] == 'var' and not elm['is_binded'] \
          and not elm['id'] in antecedent_node['pruned_var']:
          elm['var_pos'] = pos
          beta_var_nodes.append(elm)

      antecedent_node['beta_var_nodes'] = sorted(beta_var_nodes, key=itemgetter('id'))

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
    # Collect var from filter of current rete_node
    dependent_vars = set()
    filter = antecedent.get('filter')
    if filter:
      dependent_vars = dependent_vars.union(self._add_var(set(), filter, check_binded=False))
    dependent_vars = dependent_vars.union(self._add_child_var({'consequent_nodes': rete_node['consequent_nodes'], 'children_vertexes': rete_node['children_vertexes']}))

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


  # Collect variables from elm. If check_binded is True, set the var as binded if it is present
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
      if filter:
        binded_var = binded_var.union(self._add_var(parent_binded_var.union(binded_var), filter, check_binded=check_binded))
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
    
    # Add the consequent nodes at the end of the antecedent nodes
    for i in range(1, len(self.ctx.rete_nodes)):
      for consequent_node in self.ctx.rete_nodes[i]['consequent_nodes']:
        # remove the label since it use the original var and it's not meaningful in
        # the rete network
        del consequent_node['label']
        self.ctx.rete_nodes.append(consequent_node)
      del self.ctx.rete_nodes[i]['consequent_nodes']

    # Add resources and literals to rete config
    resources = self.ctx.jetReteNodes['resources']
    key = 0
    for _, v in self.ctx.resourceMap.items():
      v['key'] = key
      key += 1
      resources.append(v)

    # Transform the antecedent rete_node['triple'] into reference to resources via a key.
    # also transform the consequent 'triple' and antecedent 'filter' into reference to 
    # resources via a key.
    # For Antecedents:
    # The associated resource (via the key):
    #   -  [F_cst functor]: will have a resource type of resource
    #   -  [F_var functor]: for unbinded var, will have a resource type of var, is_binded = False
    #   -  [F_binded functor]: for binded var, will have a resource type of var, is_binded = True, 
    #                          is_antecedent = True, var_pos is the parent antecedent beta row pos.
    # For Consequents and Filters:
    # The associated resource (via the key):
    #   -  [F_cst functor]: will have a resource type of resource
    #   -  [F_binded functor]: for binded var, will have a resource type of var, is_binded = True, 
    #                          is_antecedent = False, var_pos is the current antecedent beta row pos.
    # Add var elm type to resources:
    #   - put var id when not binded [F_var functor]
    #   - put var_pos when binded (and id for convenience) [F_binded functor]
    # NOTE: This has for side effect to modify the filter and obj expr to use
    # resource keys for all resources and variables.
    for i in range(1, len(self.ctx.rete_nodes)):
      rete_node = self.ctx.rete_nodes[i]
      vertex = rete_node['vertex']    # this is the antecedent vertex for consequent
      type = rete_node['type']
      parent_vertex = 0
      if type == 'antecedent':
        parent_vertex = rete_node['parent_vertex']

      triple = rete_node['triple']
      del rete_node['triple']
      state = {
        'resources': resources,
        'vertex': vertex,
        'beta_relation_vars': self.ctx.rete_nodes[vertex].get('beta_relation_vars', []),
        'parent_beta_relation_vars': self.ctx.rete_nodes[parent_vertex].get('beta_relation_vars', []),
      }

      rete_node['subject_key'] = self._map_elm(state, triple[0], type)
      rete_node['predicate_key'] = self._map_elm(state, triple[1], type)
      obj_elm = triple[2]
      if obj_elm['type'] in ['binary', 'unary']:
        rete_node['obj_expr'] = self._map_expr(state, obj_elm)
      else:
        rete_node['object_key'] = self._map_elm(state, triple[2], type)
      filter = rete_node.get('filter')
      if filter:
        rete_node['filter'] = self._map_expr(state, filter)
    
    # Due to the side effect of the previous transformation, replace
    # nodes literals and resources of jetRules for the full list of resources
    # that include the variable definitions
    jetRules = self.ctx.jetRules
    self.ctx.jetRules = {
      'resources': self.ctx.jetReteNodes['resources'],
      'lookup_tables': jetRules['lookup_tables'],
      'jet_rules': jetRules['jet_rules'],
      'imports': jetRules.get('imports', {}),
    }
    if self.ctx.jetstore_config:
      self.ctx.jetRules['jetstore_config'] = self.ctx.jetstore_config
    if self.ctx.rule_sequences:
      self.ctx.jetRules['rule_sequences'] = self.ctx.rule_sequences
    if self.ctx.classes:
      self.ctx.jetRules['classes'] = self.ctx.classes
    if self.ctx.tables:
      self.ctx.jetRules['tables'] = self.ctx.tables


  # add elm to resources, set key as pos in sequence, return key
  def _add_key(self, resources: Sequence[Dict[str, object]], elm: Dict[str, object]) -> int:
    key = len(resources)
    elm['key'] = key
    elm['source_file_name'] = self.ctx.main_rule_fname
    resources.append(elm)
    return key

  # map the expr to itself by replacing the leaf resource to a key
  def _map_expr(self, state, elm: Dict[str, object]) -> Dict[str, object]:
    type = elm['type']
    if type == 'binary':
      elm['lhs'] = self._map_expr(state, elm['lhs'])
      elm['rhs'] = self._map_expr(state, elm['rhs'])
      return elm

    if type == 'unary':
      elm['arg'] = self._map_expr(state, elm['arg'])
      return elm
    
    # type must be literal or var, meaning we can use _map_elm
    # that returns the key
    return self._map_elm(state, elm, type)


  # map elm to an entry in resources based on type
  # May add elm to resources
  # return key,
  def _map_elm(self, state, elm, parent_type) -> int:
    type = elm['type']
    
    if type == 'var':
      if parent_type == 'triple':
        return 0
      # add vertex to var elm to track which vertex this var belongs to
      elm['vertex'] = state['vertex']
      if elm['is_binded']:
        elm['is_antecedent'] = parent_type == 'antecedent'
        if parent_type == 'antecedent':
          # var_pos is pos in parent beta relation's var
          if elm['id'] in state['parent_beta_relation_vars']:
            elm['var_pos'] = state['parent_beta_relation_vars'].index(elm['id'])
          else:
            raise Exception('@@@@ ERROR: var id',elm['id'],', vertex',state['vertex'],', is a binded var in antecedent but it''s not in parent_beta_relation_vars:',state['parent_beta_relation_vars'])
        else:
          # var_pos is pos in beta relation's var
          if elm['id'] in state['beta_relation_vars']:
            elm['var_pos'] = state['beta_relation_vars'].index(elm['id'])
          else:
            raise Exception('@@@@ ERROR: var id',elm['id'],', vertex',state['vertex'],', is a binded var in consequent or filter but its not in beta_relation_vars:',state['beta_relation_vars'])
      # remove label and value since they reference the original var and it's
      # not of use for the rete network
      del elm['value']
      del elm['label']
      return self._add_key(state['resources'], elm)

    if type == 'identifier':
      return self.ctx.resourceMap[elm['value']]['key']

    if type in ['int','uint','long','ulong','double','text','date','datetime','bool', 'keyword']:
      elm['inline'] = True
      return self._add_key(state['resources'], elm)

    raise Exception('ERROR JetRuleRete._add_elm: unknown type: '+str(type))


  # =====================================================================================
  # normalizeTriples
  # -------------------------------------------------------------------------------------
  # Perform manipulation on self.ctx.triples data structure to normalize the elements 
  # and be ready to persist using a sql model using sqlite
  # -------------------------------------------------------------------------------------
  def normalizeTriples(self) -> None:
    if not self.ctx.triples:
      return
    # this is done to reuse _map_elm function
    state = {
      'resources': self.ctx.jetReteNodes['resources']
    }
    self.ctx.jetRules['triples'] = [{
      'type': 'triple',
      'subject_key': self._map_elm(state, t3['subject'], 'triple'),
      'predicate_key': self._map_elm(state, t3['predicate'], 'triple'),
      'object_key': self._map_elm(state, t3['object'], 'triple'),
      'source_file_name': t3['source_file_name']
    } for t3 in self.ctx.triples ]

