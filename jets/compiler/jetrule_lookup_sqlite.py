from absl import flags
from pathlib import Path
from typing import Any, Sequence, Set
from typing import Dict
import apsw
import sqlite3
import traceback
import os
import pandas as pd

print ("      Using APSW file",apsw.__file__)                # from the extension module
print ("         APSW version",apsw.apswversion())           # from the extension module
print ("   SQLite lib version",apsw.sqlitelibversion())      # from the sqlite library code
print ("SQLite header version",apsw.SQLITE_VERSION_NUMBER)   # from the sqlite header file at compile time
print()
print ("   Using SQLITE3 file",sqlite3.__file__)                # from the extension module
print ("      SQLITE3 version",sqlite3.version)           # from the extension module
print ("       SQLite version",sqlite3.sqlite_version)      # from the sqlite library code
print()

flags.DEFINE_string("lookup_db", 'jetrule_lookup.db', "JetRule lookup")
flags.DEFINE_bool("clear_lookup_db", False, "Clear JetRule lookup if already exists", short_name='d')
flags.DEFINE_string("rete_db", 'jetrule_rete.db', "JetRule rete config")


class JetRuleLookupSQLite:
  def __init__(self): 
     # state required during the execution of the function saveReteConfig
    self.sqlite3Connection = None 
    self.workspace_connection = None 
    self.lookup_connection = None

  # =====================================================================================
  # saveLookup
  # ------------------------------------------------------------------------------------- 
  def saveLookups(self, lookup_db: str=None,rete_db: str=None) -> None:
    self.workspace_connection = None   
    self.lookup_connection = None   

    # Opening Rete database
    self._open_rete_db(rete_db)
  
    # Opening Lookup database
    self._open_lookup_db(lookup_db)

    try:
      # get all lookup table definitions from rete_db  
      lookup_tables = self._get_lookup_tables()

      # For each lookup table definition  
      for lk_tbl in lookup_tables:
          table_name  = lk_tbl['name']
          csv_file    = lk_tbl['csv_file']
          key_columns = lk_tbl['lookup_key'].split(',')

          # retrieve column information for lookup from rete_db
          lk_columns_dict        = self._get_lookup_table_columns(lk_tbl['key'])

          return_columns = ['__key__','jets__key']
          return_columns.extend([x['name'] for x in  lk_columns_dict])

          # Create the lookup table schema in the lookup_db
          self._create_lookup_schema(table_name, lk_columns_dict)

          # Load Lookup CSV to Lookup Table in lookup_db 
          self._load_csv_lookup(table_name, csv_file, key_columns, return_columns)

    except (Exception) as error:
      print("Error while saving lookup_db (2):", error)
      print(traceback.format_exc())
      return str(error)

    finally:
      if self.lookup_connection:
        self.lookup_connection.close(True)
      if self.workspace_connection:
        self.workspace_connection.close(True)
      if self.sqlite3Connection:
        self.sqlite3Connection.close()  
    # All good here!
    return None


 

  # -------------------------------------------------------------------------------------
  # _get_lookup_tables
  # -------------------------------------------------------------------------------------
  def _get_lookup_tables(self): 
    lookup_tbl_cursor = self.workspace_connection.cursor()  

    select_lookups = '''
    SELECT 
      key,
      name,
      table_name,
      csv_file,
      lookup_key,
      lookup_resources,
      source_file_key 
    FROM 
      lookup_tables
    '''
    lookup_tables = []

    for row in lookup_tbl_cursor.execute(select_lookups):
        columns = [t[0] for t in lookup_tbl_cursor.getdescription()]
        lookup_tables.append(dict(zip(columns, row)))

    lookup_tbl_cursor = None
    return lookup_tables
   

  # -------------------------------------------------------------------------------------
  # _get_lookup_table_columns
  # -------------------------------------------------------------------------------------
  def _get_lookup_table_columns(self, lookup_table_key):
    lookup_tbl_column_cursor = self.workspace_connection.cursor()  

    select_lookups = f'''
    SELECT 
        lookup_table_key,
        name,
        type,
        as_array
    FROM 
        lookup_columns
    WHERE
        lookup_table_key = {lookup_table_key}
    '''
    lookup_tables_columns = []

    for row in lookup_tbl_column_cursor.execute(select_lookups):
        columns = [t[0] for t in lookup_tbl_column_cursor.getdescription()]
        lookup_tables_columns.append(dict(zip(columns, row)))

    lookup_tbl_column_cursor = None
    return lookup_tables_columns       


  # -------------------------------------------------------------------------------------
  # get_lookup_column_schema
  # -------------------------------------------------------------------------------------
  # Get column names and types for schema creation
  def _get_lookup_column_schema(self, lookup_table_columns): 
        column_schema = ',\n'.join([x['name'] + '  STRING' for x in  lookup_table_columns])
        return column_schema


  # -------------------------------------------------------------------------------------
  # _create_schema
  # -------------------------------------------------------------------------------------
  # Create lookup_db schema if not already existing
  def _create_lookup_schema(self, table_name, lk_columns) -> None:
    # create part of the CREATE TABLE STATEMENT
    column_schema = self._get_lookup_column_schema(lk_columns)  

    cursor = self.lookup_connection.cursor()
    cursor.execute(f"""
      -- --------------------
      -- workspace_control table
      -- --------------------
      CREATE TABLE IF NOT  EXISTS {table_name} (
        __key__            INTEGER PRIMARY KEY, 
        jets__key          STRING NOT NULL,
        {column_schema}
      );

      CREATE INDEX IF NOT EXISTS {table_name}_idx 
      ON {table_name} (jets__key);

   """)
    cursor = None      


  # -------------------------------------------------------------------------------------
  # _load_csv_lookup
  # -------------------------------------------------------------------------------------
  # Load Lookup CSV file to Lookup Table in lookup_db
  def _load_csv_lookup(self,table_name,csv_file,key_columns, return_columns) -> None:
    csv_path = os.path.join(Path(flags.FLAGS.base_path), csv_file)
    csv_path = os.path.abspath(csv_path)
    if not os.path.exists(csv_path):
        print('Could note locate: ' + str(csv_path))
    else:    
        lookup_df = pd.read_csv(csv_path, dtype=str, skipinitialspace = True)

        if set(key_columns).issubset(set(lookup_df.columns)): 
            lookup_df.insert(0,'jets__key', lookup_df[key_columns].agg(''.join, axis=1))
        else:
            raise Exception(f'Key Columns missing in provided CSV. Expected {str(key_columns)} in header {str(lookup_df.columns)}')    

        # lookup_df['jets__key'] = lookup_df.apply (lambda row: self._create_jets_key(row,key_columns), axis=1)
        lookup_df.insert(0, '__key__', range(0, 0 + len(lookup_df)))

        if set(return_columns).issubset(set(lookup_df.columns)): 
            lookup_df[return_columns].to_sql(table_name, self.sqlite3Connection, if_exists='append', index=False)
        else:
            raise Exception(f'Return Columns missing in provided CSV. Expected {str(return_columns)} in header {str(lookup_df.columns)}')    

 
  # -------------------------------------------------------------------------------------
  # _create_jets_key
  # -------------------------------------------------------------------------------------
  def _create_jets_key(self,row,key_columns):
     composite_key = ''.join([row[x] for x in key_columns])
     return composite_key       


  # -------------------------------------------------------------------------------------
  # _open_rete_db
  # -------------------------------------------------------------------------------------
  def _open_rete_db(self,rete_db) -> None:
    try:
        if rete_db:
            self.workspace_connection = apsw.Connection(rete_db)
        else:
            rete_db_path = flags.FLAGS.rete_db
            if not rete_db_path:
                rete_db_path = 'jetrule_rete.db'
            path = os.path.join(Path(flags.FLAGS.base_path), rete_db_path)
            path = os.path.abspath(path)
            print('*** RETE_DB PATH',path)
            self.workspace_connection = apsw.Connection(path)
    except (Exception) as error:
        print("Error while opening rete_db (1):", error)
        return str(error)
    finally:
        pass       


 # -------------------------------------------------------------------------------------
  # _open_lookup_db
  # -------------------------------------------------------------------------------------
  def _open_lookup_db(self,lookup_db) -> None:
        # Opening/creating Lookup database
        try:
            if lookup_db:
                self.lookup_connection = apsw.Connection(lookup_db)
                self.sqlite3Connection = sqlite3.Connection(lookup_db)
            else:
                lookup_db_path = flags.FLAGS.lookup_db
                if not lookup_db_path:
                    lookup_db_path = 'jetrule_lookup.db'
                path = os.path.join(Path(flags.FLAGS.base_path), lookup_db_path)
                path = os.path.abspath(path)
                print('*** LOOKUP_DB PATH',path)
                if not os.path.exists(path):
                    print('** DB Path does not exist, creating new lookup_db at ',path)
                if flags.FLAGS.clear_lookup_db and os.path.exists(path):
                    print('*** Clearing DB, creating new lookup_db at ',path)
                    os.remove(path)
                self.lookup_connection = apsw.Connection(path)
                self.sqlite3Connection = sqlite3.Connection(path)
        except (Exception) as error:
            print("Error while opening lookup_db (1):", error)
            return str(error)
        finally:
            pass  