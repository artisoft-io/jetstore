from jetrule_context import JetRuleContext
from absl import app
from absl import flags
from pathlib import Path
from typing import Any, Sequence, Set
from typing import Dict
import apsw
import traceback
import os
import sys
import json

print ("      Using APSW file",apsw.__file__)                # from the extension module
print ("         APSW version",apsw.apswversion())           # from the extension module
print ("   SQLite lib version",apsw.sqlitelibversion())      # from the sqlite library code
print ("SQLite header version",apsw.SQLITE_VERSION_NUMBER)   # from the sqlite header file at compile time
print()

flags.DEFINE_string("rete_db", 'jetrule_rete.db', "JetRule rete config")
flags.DEFINE_bool("clear_rete_db", False, "Clear JetRule rete config if already exists", short_name='d')

class JetRuleReteSQLite:
  def __init__(self, ctx: JetRuleContext):
    self.ctx = ctx

    # state required during the execution of the function saveReteConfig
    self.rule_file_keys = {}
    self.workspace_connection = None
    self.read_cursor = None
    self.wc_key = None
    self.resources_last_key = None
    self.expr_last_key = None
    self.rete_node_last_key = None
    self.beta_row_config_last_key = None
    self.domain_classes_last_key = None
    self.data_properties_last_key = None
    self.domain_tables_last_key = None
    self.lookup_tables_last_key = None
    self.write_cursor = None
    self.main_rule_file_key = None

  # =====================================================================================
  # saveReteConfig
  # ------------------------------------------------------------------------------------- 
  def saveReteConfig(self, workspace_db: str=None) -> str:
    assert self.ctx, 'Must have a valid JetRuleContext'
    assert self.ctx.jetReteNodes, 'Must have a valid JetRuleContext.jetReteNodes'
    self.workspace_connection = None

    # Opening/creating database
    try:
      if workspace_db:
        self.workspace_connection = apsw.Connection(workspace_db)
      else:
        rete_db_path = flags.FLAGS.rete_db
        if not rete_db_path:
          rete_db_path = 'jetrule_rete.db'
        path = os.path.join(Path(flags.FLAGS.base_path), rete_db_path)
        path = os.path.abspath(path)
        print('*** RETE_DB PATH',path)
        if not os.path.exists(path):
          print('** DB Path does not exist, creating new rete_db at ',path)
        if flags.FLAGS.clear_rete_db and os.path.exists(path):
          print('*** Clearing DB, creating new rete_db at ',path)
          os.remove(path)
        self.workspace_connection = apsw.Connection(path)
    except (Exception) as error:
      print("Error while opening rete_db (1):", error)
      return str(error)
    finally:
      pass

    # Saving the ctx.jetReteNodes
    try:
      # Create the workspace schema if new db
      self._create_schema()
      self.rule_file_keys = {}

      # open a read cursor for looking up ids
      self.read_cursor = self.workspace_connection.cursor()

      # Check that main rule file is not already in workspace_control
      main_rule_file_name = self.ctx.jetReteNodes.get('main_rule_file_name')
      assert main_rule_file_name, 'Invalid json for jetReteNodes'
      if self._get_source_rule_file_key(main_rule_file_name) is not None:
        raise Exception("ERROR: main_rule_file_name '"+str(main_rule_file_name)+"' already exist in rete_db")

      # Get tables last key for insertion of new rows
      self.wc_key                   = self._get_last_key('workspace_control', 'key')
      self.resources_last_key       = self._get_last_key('resources', 'key')
      self.expr_last_key            = self._get_last_key('expressions', 'key')
      self.rete_nodes_last_key      = self._get_last_key('rete_nodes', 'key')
      self.beta_row_config_last_key = self._get_last_key('beta_row_config', 'key')
      self.domain_classes_last_key  = self._get_last_key('domain_classes', 'key')
      self.data_properties_last_key = self._get_last_key('data_properties', 'key')
      self.domain_tables_last_key   = self._get_last_key('domain_tables', 'key')
      self.lookup_tables_last_key   = self._get_last_key('lookup_tables', 'key')

      # Open the self.write_cursor
      self.write_cursor = self.workspace_connection.cursor()
      self.write_cursor.execute('BEGIN')

      # Add main_rule_file to workspace_control table
      # Will need the key for the rete_nodes
      self.main_rule_file_key = self._add_source_rule_file(main_rule_file_name, True)

      # Add support files to workspace_control if not there already
      for support_file in self.ctx.jetReteNodes['support_rule_file_names']:
        if self._get_source_rule_file_key(support_file) is None:
          self._add_source_rule_file(support_file, False)

      # Add all resources to rete_db, will skip source file already in rete_db
      # -------------------------------------------------------------------------
      self._save_resources()

      # Add all domain classes and tables
      # -------------------------------------------------------------------------
      self._save_domain_classes()
      self._save_domain_tables()

      # Add all lookup_table to rete_db, will skip source file already in rete_db
      # -------------------------------------------------------------------------
      self._save_lookup_tables()

      # Add expressions based on filters and object expr
      # -------------------------------------------------------------------------
      self._save_expressions()

      # Add rete_nodes to rete_nodes table
      # -------------------------------------------------------------------------
      self._save_rete_nodes()

      # All done, commiting the work
      # print('done')
      self.write_cursor.execute('COMMIT')
      self.write_cursor.close()
      self.write_cursor = None

    except (Exception) as error:
      print("Error while saving rete_db (2):", error)
      print(traceback.format_exc())
      return str(error)

    finally:
      if self.workspace_connection:
        self.workspace_connection.close(True)

    # All good here!
    return None


  # -------------------------------------------------------------------------------------
  # _get_last_key
  # -------------------------------------------------------------------------------------
  def _get_last_key(self, table_name: str, key_name: str) -> int:
    last_key = None
    for k, in self.read_cursor.execute(f"SELECT max({key_name}) FROM {table_name}"):
      last_key = k
    if last_key is None:
      last_key = 0
    else:
      last_key += 1
    # print('GOT max(self.resources_last_key)',self.resources_last_key)
    return last_key


  # -------------------------------------------------------------------------------------
  # _save_resources
  # -------------------------------------------------------------------------------------
  def _save_resources(self):
    # print('Saving resources. . .')
    for item in self.ctx.jetReteNodes.get('resources',[]):
      skey = self.rule_file_keys.get(item['source_file_name'])
      if skey is not None:
        key = self.resources_last_key
        self.resources_last_key += 1
        item['db_key'] = key                  # keep the globaly unique key for insertion in expressions and rete_nodes tables
        row = [key, item['type'], item.get('id'), item.get('value'), item.get('symbol'),
              item.get('is_binded'), item.get('inline'), skey, item.get('vertex'), item.get('var_pos')]
        self.write_cursor.execute(
          "INSERT INTO resources (key, type, id, value, symbol, is_binded, "
            "inline, source_file_key, vertex, row_pos) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", 
          row)
      else:
        # file with skey already in db, get the resource 'db_key' from the db; column key in resources table
        key = self._get_resource_key(item['id'])
        if key is None:
          raise Exception("Error while getting key for resource with id '{0}', resource not found!".format(item['id']))
        item['db_key'] = key                  # keep the globaly unique key for insertion in expressions and rete_nodes tables


  # -------------------------------------------------------------------------------------
  # _save_domain_classes
  # -------------------------------------------------------------------------------------
  def _save_domain_classes(self):
    for cls in self.ctx.jetRules.get('classes', []):
      skey = self.rule_file_keys.get(cls['source_file_name'])
      if skey is not None:
        key = self.domain_classes_last_key
        self.domain_classes_last_key += 1
        cls['db_key'] = key                  # keep the globaly unique key for insertion in other tables
        row = [key, cls['name'], cls.get('as_table', False), skey]
        self.write_cursor.execute("INSERT INTO domain_classes (key, name, as_table, source_file_key) VALUES (?, ?, ?, ?)", row)

        # save base classes
        for base_cls in cls['base_classes']:
          bckey = self._get_key('domain_classes', 'name', base_cls)
          self.write_cursor.execute("INSERT INTO base_classes (domain_class_key, base_class_key) VALUES (?, ?)", [key, bckey])

        # save domain properties
        for property in cls['data_properties']:
          pkey = self.data_properties_last_key
          self.data_properties_last_key += 1
          property['db_key'] = pkey                  # keep the globaly unique key for insertion in other tables
          row = [pkey, key, property['name'], property['type'], property.get('as_array', False)]
          self.write_cursor.execute("INSERT INTO data_properties (key, domain_class_key, name, type, as_array) VALUES (?, ?, ?, ?, ?)", row)

      else:
        # file with skey already in db, get the resource 'db_key' from the db;
        key = self._get_key('domain_classes', 'name', cls['name'])
        if key is None:
          raise Exception("Error while getting key for domain_classe with name '{0}', not found!".format(cls['name']))
        cls['db_key'] = key                  # keep the globaly unique key for insertion in expressions and rete_nodes tables
        # get the data_properties key as well
        for property in cls['data_properties']:
          key = self._get_key('data_properties', 'name', property['name'])
          if key is None:
            raise Exception("Error while getting key for data_property with name '{0}', not found!".format(property['name']))
          property['db_key'] = key                  # keep the globaly unique key for insertion in other tables


  # -------------------------------------------------------------------------------------
  # _save_domain_tables
  # -------------------------------------------------------------------------------------
  def _save_domain_tables(self):
    # print('Saving domain_tables. . .')
    for tbl in self.ctx.jetRules.get('tables', []):
      skey = self.rule_file_keys.get(tbl['source_file_name'])
      if skey is not None:
        key = self.domain_tables_last_key
        self.domain_tables_last_key += 1
        tbl['db_key'] = key                  # keep the globaly unique key for insertion in other tables
        domain_class_key = self._get_key('domain_classes', 'name', tbl['class_name'])
        row = [key, domain_class_key, tbl['table_name']]
        self.write_cursor.execute("INSERT INTO domain_tables (key, domain_class_key, name) VALUES (?, ?, ?)", row)

        # save domain columns
        for column in tbl['columns']:
          domain_table_key = key
          data_property_key = self._get_key('data_properties', 'name', column['property_name'])
          row = [domain_table_key, data_property_key, column['column_name'], column['type'], column.get('as_array', False)]
          self.write_cursor.execute("INSERT INTO domain_columns (domain_table_key, data_property_key, name, type, as_array) VALUES (?, ?, ?, ?, ?)", row)

      else:
        # file with skey already in db, get the resource 'db_key' from the db;
        key = self._get_key('domain_tables', 'name', tbl['table_name'])
        if key is None:
          raise Exception("Error while getting key for domain_table with name '{0}', not found!".format(tbl['table_name']))
        tbl['db_key'] = key                  # keep the globaly unique key for insertion in expressions and rete_nodes tables

  # -------------------------------------------------------------------------------------
  # _save_lookup_tables
  # -------------------------------------------------------------------------------------
  def _save_lookup_tables(self):
    # print('Saving lookup tables. . .')
    for item in self.ctx.jetReteNodes.get('lookup_tables', []):
      skey = self.rule_file_keys.get(item['source_file_name'])
      if skey is not None:
        key = self.lookup_tables_last_key
        self.lookup_tables_last_key += 1
        item['db_key'] = key                  # keep the globaly unique key for insertion in other tables
        row = [key, item['name'], item.get('table'), item.get('csv_file'), ','.join(item['key']), ','.join(item['resources']), skey]
        self.write_cursor.execute(
          "INSERT INTO lookup_tables (key, name, table_name, csv_file, lookup_key, lookup_resources, source_file_key) VALUES (?, ?, ?, ?, ?, ?, ?)", 
          row)
        
        for column in item['columns']:
          row = [key, column['name'], column['type'], column.get('as_array', False)]
          self.write_cursor.execute(
            "INSERT INTO lookup_columns (lookup_table_key, name, type, as_array) VALUES (?, ?, ?, ?)", 
            row)

  # -------------------------------------------------------------------------------------
  # _save_expressions
  # -------------------------------------------------------------------------------------
  def _save_expressions(self):
    # print('Saving expressions. . .')
    for item in self.ctx.jetReteNodes.get('rete_nodes',[]):
      filter = item.get('filter')
      if filter:
        item['filter_expr_key'] = self._expr_2_key(filter)
      obj_expr = item.get('obj_expr')
      if obj_expr:
        item['obj_expr_key'] = self._expr_2_key(obj_expr)

  # -------------------------------------------------------------------------------------
  # _expr_2_key
  # -------------------------------------------------------------------------------------
  # Add expression to expressions table recursivelly and return the key
  # Put resource entities as well: resource (constant) and var (binded)
  # expr is the resource key, so we can call persist directly.
  def _expr_2_key(self, expr: Dict[str, object]) -> int:
    assert expr, 'Expecting expression'
    # Check if we have a resource key
    if isinstance(expr, int):
      return self._persist_expr(expr)

    type = expr['type']
    if type == 'binary':
      expr['arg0_key'] = self._expr_2_key(expr['lhs'])
      expr['arg1_key'] = self._expr_2_key(expr['rhs'])
      return self._persist_expr(expr)

    if type == 'unary':
      expr['arg0_key'] = self._expr_2_key(expr['arg'])
      return self._persist_expr(expr)
    raise Exception('_expr_2_key: ERROR Expecting only binay or unary as type')

  # -------------------------------------------------------------------------------------
  # _persist_expr
  # -------------------------------------------------------------------------------------
  # Add expr to expressions table
  def _persist_expr(self, expr: Dict[str, object]) -> int:
    assert expr, 'Expecting expression'
    assert self.write_cursor, 'Expecting self.write_cursor'
    key = self.expr_last_key
    self.expr_last_key += 1
    if isinstance(expr, int):
      # Convert the resource key to the db key (global key)
      db_key = self.ctx.jetReteNodes['resources'][expr]['db_key']
      row = [key, 'resource', db_key, None, None, None, None, None, None, self.main_rule_file_key]
    else:
      row = [
        key, expr['type'], expr.get('arg0_key'), expr.get('arg1_key'), expr.get('arg2_key'), 
        expr.get('arg3_key'), expr.get('arg4_key'), expr.get('arg5_key'), expr.get('op'), self.main_rule_file_key
      ]
    self.write_cursor.execute("INSERT INTO expressions (key, type, arg0_key, arg1_key, arg2_key, arg3_key, "
                              "arg4_key, arg5_key, op, source_file_key) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", row)
    return key


  # -------------------------------------------------------------------------------------
  # _save_rete_nodes
  # -------------------------------------------------------------------------------------
  def _save_rete_nodes(self):
    # print('Saving rete_nodes. . .')
    resources = self.ctx.jetReteNodes.get('resources')
    for rete_node in self.ctx.jetReteNodes.get('rete_nodes',[]):
      # Get the db_key for all resources
      subject_key = rete_node.get('subject_key')
      if subject_key is not None:
        subject_key = resources[subject_key]['db_key']

      predicate_key = rete_node.get('predicate_key')
      if predicate_key is not None:
        predicate_key = resources[predicate_key]['db_key']

      object_key = rete_node.get('object_key')
      if object_key is not None:
        object_key = resources[object_key]['db_key']
      
      # Get the salience
      salience = rete_node.get('salience')
      if salience is not None:
        s = set(salience)
        if len(s) > 1:
          raise Exception('ERROR: Multiple rules have same antecedents but different salience:'+str(rete_node.get('rules')))
        salience = salience[0]

      # Check if multiple rules have same antecedents
      rules = rete_node.get('rules')
      if rules and len(rules)>1:
        print('WARNING: Multiple rules have the same antecedents, they will be merges in the rete graph:',rules)

      # Assign key to rete node
      key = self.rete_nodes_last_key
      self.rete_nodes_last_key += 1
      
      row = [
        key, rete_node['vertex'], rete_node['type'], subject_key, predicate_key, object_key, 
        rete_node.get('obj_expr_key'), rete_node.get('filter_expr_key'), 
        rete_node.get('normalizedLabel'), rete_node.get('parent_vertex'), self.main_rule_file_key,
        rete_node.get('isNot'), salience, rete_node.get('consequent_seq', 0)
      ]
      self.write_cursor.execute(
        "INSERT INTO rete_nodes (key, vertex, type, subject_key, predicate_key, object_key, obj_expr_key, filter_expr_key, "
        "normalizedLabel, parent_vertex, source_file_key, is_negation, salience, consequent_seq) "
        "VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", 
        row)

      # Add beta_row_config table
      # -------------------------------------------------------------------------
      self._save_beta_row_config(rete_node)


  # -------------------------------------------------------------------------------------
  # _save_beta_row_config
  # -------------------------------------------------------------------------------------
  def _save_beta_row_config(self, rete_node: object):
    # print('Saving beta_row_configs. . .')
    beta_var_nodes = rete_node.get('beta_var_nodes', [])
    for seq in range(len(beta_var_nodes)):
      bvnode = beta_var_nodes[seq]

      # Assign key to beta_row_config
      key = self.beta_row_config_last_key
      self.beta_row_config_last_key += 1

      beta_row_config = [
        key, bvnode['vertex'], seq, self.main_rule_file_key,
        bvnode['var_pos'], bvnode['is_binded'], bvnode['id'], 
      ]
      self.write_cursor.execute(
        "INSERT INTO beta_row_config (key, vertex, seq, source_file_key, row_pos, is_binded, id)"
        "VALUES (?, ?, ?, ?, ?, ?, ?)", 
        beta_row_config)


  # -------------------------------------------------------------------------------------
  # _add_source_rule_file
  # -------------------------------------------------------------------------------------
  # Add source_file_name to rete_db and return the associated key
  def _add_source_rule_file(self, source_file_name: str, is_main: bool) -> int:
    assert self.write_cursor, "ERROR, write_cursor is not open!"
    key = self.wc_key
    self.wc_key += 1
    self.write_cursor.execute("INSERT INTO workspace_control (key, source_file_name, is_main) VALUES (?, ?, ?)", (key, source_file_name, is_main))
    self.rule_file_keys[source_file_name] = key
    return key

  # -------------------------------------------------------------------------------------
  # _get_source_rule_file_key
  # -------------------------------------------------------------------------------------
  # Get the key associated with source_file_name or None if not in rete_db
  def _get_source_rule_file_key(self, source_file_name: str) -> int:
    for k, in self.read_cursor.execute("SELECT key FROM workspace_control WHERE source_file_name = ?", (source_file_name,)):
      # print('*** Got',source_file_name,'with key',k)
      return k

  # -------------------------------------------------------------------------------------
  # _get_resource_key
  # -------------------------------------------------------------------------------------
  # Get the key associated with resource or literal id, exclude var
  def _get_resource_key(self, id_: str) -> int:
    for k, in self.read_cursor.execute("SELECT key FROM resources WHERE id = ? AND type != 'var'", (id_,)):
      # print('*** Got',source_file_name,'with key',k)
      return k


  # -------------------------------------------------------------------------------------
  # _get_key
  # -------------------------------------------------------------------------------------
  def _get_key(self, table_name: str, column_name: str, column_value: str) -> int:
    # print('GET_KEY:',f"SELECT key FROM {table_name} WHERE {column_name} = '{column_value}'")
    for k, in self.read_cursor.execute(f"SELECT key FROM {table_name} WHERE {column_name} = '{column_value}'"):
      return k

  # -------------------------------------------------------------------------------------
  # _create_schema
  # -------------------------------------------------------------------------------------
  # Create rete_db schema if not already existing
  def _create_schema(self) -> None:
    # Create the workspace schema if new db
    cursor = self.workspace_connection.cursor()
    cursor.execute("""
      -- --------------------
      -- workspace_control table
      -- --------------------
      CREATE TABLE IF NOT  EXISTS workspace_control (
        key                INTEGER PRIMARY KEY,
        source_file_name   STRING,
        is_main            BOOL
      );

      -- --------------------
      -- domain_classes tables
      -- --------------------
      CREATE TABLE IF NOT EXISTS domain_classes (
        key                INTEGER PRIMARY KEY,
        name               STRING NOT NULL,
        as_table           BOOL DEFAULT FALSE,
        source_file_key    INTEGER NOT NULL,
        -- domain class name must be unique in workspace
        UNIQUE (name)
      );
      CREATE TABLE IF NOT EXISTS base_classes (
        domain_class_key   INTEGER NOT NULL,
        base_class_key     INTEGER NOT NULL,
        UNIQUE (domain_class_key, base_class_key)
      );
      CREATE TABLE IF NOT EXISTS data_properties (
        key                INTEGER PRIMARY KEY,
        domain_class_key   INTEGER NOT NULL,
        name               STRING NOT NULL,
        type               STRING NOT NULL,
        as_array           BOOL DEFAULT FALSE,
        -- domain property name must be unique in workspace
        UNIQUE (name)
      );
      INSERT INTO domain_classes (key, name, source_file_key) VALUES (0, 'owl:Thing', -1)
        ON CONFLICT (key) DO NOTHING;

      -- --------------------
      -- domain_tables tables
      -- --------------------
      CREATE TABLE IF NOT EXISTS domain_tables (
        key                INTEGER PRIMARY KEY,
        domain_class_key   INTEGER NOT NULL,
        name               STRING NOT NULL,
        -- domain table name must be unique since domain_class are unique
        UNIQUE (name)
      );
      CREATE TABLE IF NOT EXISTS domain_columns (
        domain_table_key   INTEGER NOT NULL,
        data_property_key  INTEGER NOT NULL,
        name               STRING NOT NULL,
        type               STRING NOT NULL,
        as_array           BOOL DEFAULT FALSE,
        -- a column must appear only once in a table
        UNIQUE (domain_table_key, data_property_key),
        -- a column name must be unique for a table
        UNIQUE (domain_table_key, name)
      );

      -- --------------------
      -- resources table
      -- --------------------
      CREATE TABLE IF NOT EXISTS resources (
        key                INTEGER PRIMARY KEY,
        type               STRING NOT NULL,
        id                 STRING,
        value              STRING,
        symbol             STRING,
        is_binded          BOOL,     -- for var type only
        inline             BOOL,
        source_file_key    INTEGER NOT NULL,
        vertex             INTEGER,  -- for var type only, var for vertex
        row_pos            INTEGER   -- for var type only, pos in beta row
      );

      -- --------------------
      -- lookup_tables table
      -- --------------------
      CREATE TABLE IF NOT EXISTS lookup_tables (
        key                INTEGER PRIMARY KEY,
        name               STRING NOT NULL,
        table_name         STRING,
        csv_file           STRING,
        lookup_key         STRING,
        lookup_resources   STRING,
        source_file_key    INTEGER NOT NULL,
        UNIQUE (name, source_file_key)
      );
      CREATE TABLE IF NOT EXISTS lookup_columns (
        lookup_table_key   INTEGER NOT NULL,
        name               STRING NOT NULL,
        type               STRING NOT NULL,
        as_array           BOOL DEFAULT FALSE,
        -- a column name must be unique for a table
        UNIQUE (lookup_table_key, name)
      );

      -- --------------------
      -- expressions table
      -- --------------------
      -- type = {'binary', 'unary', 'resource', 'function'}
      -- when type == 'resource', arg0_key is resources.key
      CREATE TABLE IF NOT EXISTS expressions (
        key                INTEGER PRIMARY KEY,
        type               STRING NOT NULL,
        arg0_key           INTEGER,
        arg1_key           INTEGER,
        arg2_key           INTEGER,
        arg3_key           INTEGER,
        arg4_key           INTEGER,
        arg5_key           INTEGER,
        op                 STRING,
        source_file_key    INTEGER NOT NULL
      );

      -- --------------------
      -- rete_nodes table
      -- --------------------
      CREATE TABLE IF NOT EXISTS rete_nodes (
        key                INTEGER PRIMARY KEY,
        vertex             INTEGER NOT NULL,
        type               STRING NOT NULL,
        subject_key        INTEGER,
        predicate_key      INTEGER,
        object_key         INTEGER,
        obj_expr_key       INTEGER,
        filter_expr_key    INTEGER,
        normalizedLabel    STRING,
        parent_vertex      INTEGER,
        source_file_key    INTEGER NOT NULL,
        is_negation        INTEGER,
        salience           INTEGER,
        consequent_seq     INTEGER NOT NULL,
        UNIQUE (vertex, type, consequent_seq, source_file_key)
      );

      -- --------------------
      -- beta_row_config table
      -- --------------------
      CREATE TABLE IF NOT EXISTS beta_row_config (
        key                INTEGER PRIMARY KEY,
        vertex             INTEGER NOT NULL,
        seq                INTEGER NOT NULL,
        source_file_key    INTEGER NOT NULL,
        row_pos            INTEGER NOT NULL,
        is_binded          INTEGER,
        id                 STRING,
        UNIQUE (vertex, seq, source_file_key)
      );

      -- --------------------
      -- schema_info table
      -- --------------------
      CREATE TABLE IF NOT EXISTS schema_info (
        version_major      INTEGER NOT NULL,
        version_minor      INTEGER NOT NULL,
        UNIQUE (version_major, version_minor)
      );
      INSERT INTO schema_info (version_major, version_minor) VALUES (1, 0)
        ON CONFLICT (version_major, version_minor) DO NOTHING;
    """)
    cursor = None
