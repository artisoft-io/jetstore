from absl import flags
from pathlib import Path
from typing import Any, Sequence, Set
from typing import Dict
import sqlite3
import traceback
import os
import pandas as pd
import numpy as np
import re

print ("   Using SQLITE3 file",sqlite3.__file__)              
print ("      SQLITE3 version",sqlite3.version)          
print ("       SQLite version",sqlite3.sqlite_version)    
print()

flags.DEFINE_string("lookup_db", 'jetrule_lookup.db', "JetRule lookup")
flags.DEFINE_bool("clear_lookup_db", True, "Clear JetRule lookup if already exists", short_name='d')
flags.DEFINE_string("rete_db", 'jetrule_rete.db', "JetRule rete config")


class JetRuleLookupSQLite:
  def __init__(self, base_path: str=''): 
    # state required during the execution of the function saveReteConfig
    self.workspace_connection = None 
    self.lookup_connection    = None 
    self.base_path            = base_path
  # =====================================================================================
  # saveLookup
  # ------------------------------------------------------------------------------------- 
  def saveLookups(self, lookup_db: str=None,rete_db: str=None) -> None:
    self.workspace_connection = None 
    self.lookup_connection    = None 



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
          key_columns = [x.strip() for x in lk_tbl['lookup_key'].split(',')] 
          table_key   = lk_tbl['key']
          print('Processing: ' + csv_file)

          # retrieve column information for lookup from rete_db
          lk_columns_dicts        = self._get_lookup_table_columns(table_key)

          # Create the lookup table schema in the lookup_db
          self._create_lookup_schema(table_name, lk_columns_dicts)

          return_columns = ['__key__','jets__key']
          return_columns.extend([x['name'] for x in  lk_columns_dicts])
          converters_and_dtypes = self._get_converters_and_dtypes(lk_columns_dicts, key_columns) # {} # converters={'date':pd.to_datetime})


          lookup_df = self._create_lookup_df(csv_file,key_columns,converters_and_dtypes)

          self._validate_df(lookup_df,lk_columns_dicts,key_columns)

          # Load Lookup CSV to Lookup Table in lookup_db 
          self._load_df_lookup(lookup_df, table_name,return_columns)

          

    except (Exception) as error:
      print("Error while saving lookup_db (2):", error)
      print(traceback.format_exc())
      return str(error)

    finally:
      if self.lookup_connection:
        self.lookup_connection.close()  
      if self.workspace_connection:
        self.workspace_connection.close()          
    # All good here!
    return None


  def getLookup(self, table_name: str, lookup_db: str=None) -> list[dict]:
    
    self.lookup_connection    = None 

    self._open_lookup_db(lookup_db)

    lookup_tbl_cursor = self.lookup_connection.cursor()  

    try:
        select_lookup = f'SELECT * FROM {table_name}'

        lookup_tbl_cursor.execute(select_lookup)    
        lookup_table = lookup_tbl_cursor.fetchall()

    except (Exception) as error:
      print("Error while saving lookup_db (2):", error)
      print(traceback.format_exc())
      return str(error)

    finally:
      if self.lookup_connection:
        self.lookup_connection.close()  

    return lookup_table    

  # -------------------------------------------------------------------------------------
  # _get_lookup_tables
  # -------------------------------------------------------------------------------------
  def _get_lookup_tables(self) -> list: 
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

    lookup_tbl_cursor.execute(select_lookups)    
    lookup_tables = lookup_tbl_cursor.fetchall()


    lookup_tbl_cursor = None
    return self._sanitize_rows(lookup_tables)
   

  # -------------------------------------------------------------------------------------
  # _get_lookup_table_columns
  # -------------------------------------------------------------------------------------
  def _get_lookup_table_columns(self, lookup_table_key: str) -> list:
    lookup_tbl_column_cursor = self.workspace_connection.cursor()  
    
    select_lookups = '''
    SELECT 
        lookup_table_key,
        name,
        type,
        as_array
    FROM 
        lookup_columns
    WHERE
        lookup_table_key =:table_key
    '''

    lookup_tbl_column_cursor.execute(select_lookups,{"table_key" : lookup_table_key})    
    lookup_tables_columns = lookup_tbl_column_cursor.fetchall()

    lookup_tbl_column_cursor = None
    return self._sanitize_rows(lookup_tables_columns)       


  def _convert_jetrule_type(self, jr_type: str) -> str:

    if jr_type in  ['text', 'date', 'datetime'] :
        sqlite_type = 'TEXT'
    elif jr_type in ['int','bool','uint', 'long', 'ulong']:
         sqlite_type = 'INTEGER'         
    elif jr_type == 'double':
         sqlite_type = 'REAL'
    else:
        raise Exception('_convert_jetrule_type: Type not supported: ' + jr_type)    
    return sqlite_type

  # -------------------------------------------------------------------------------------
  # get_lookup_column_schema
  # -------------------------------------------------------------------------------------
  # Get column names and types for schema creation
  def _get_lookup_column_schema(self, lookup_table_columns: list[dict]) -> str: 
        column_schema = ',\n'.join([x['name'] + '  ' +  self._convert_jetrule_type(x['type']) for x in  lookup_table_columns])
        return column_schema

  # -------------------------------------------------------------------------------------
  # _sanitize
  # -------------------------------------------------------------------------------------
  # Used to sanitize strings before execution in SQL, if strict is set to True (default) will raise exception if sanitized string differs from input
  def _sanitize(self,to_sanitize:str, strict:bool=True) -> str:
      sanitized = re.sub('[^0-9a-zA-Z./, :]+', '_', to_sanitize)
      if sanitized != to_sanitize:
        if strict:
            raise Exception(f'_sanitize: sanitized string: {sanitized} did not match original string and _sanitize in strict mode')
        else:
            print(f'_sanitize: WARNING sanitized string: {sanitized} did not match original string. Proceeding with {sanitized}')
      return sanitized


# -------------------------------------------------------------------------------------
  # _sanitize_rows
  # -------------------------------------------------------------------------------------
  # Used to sanitize rows before execution in SQL, if strict is set to True (default) will raise exception if sanitized string differs from input
  def _sanitize_rows(self,rows_to_sanitize:list[dict], strict:bool=True) -> str:
    sanitized_rows = []
    for row in rows_to_sanitize:
      sanitized_row = {}
      for key in row.keys():
        sanitized_row[key] = self._sanitize(str(row[key]), strict)
      sanitized_rows.append(sanitized_row)  

    return sanitized_rows


  # -------------------------------------------------------------------------------------
  # _create_schema
  # -------------------------------------------------------------------------------------
  # Create lookup_db schema if not already existing
  def _create_lookup_schema(self, table_name: str, lk_columns: list[dict]) -> None:
    # create part of the CREATE TABLE STATEMENT
    column_schema = self._get_lookup_column_schema(lk_columns)  

    cursor = self.lookup_connection.cursor()

    drop_table_statement = f"""
      DROP TABLE IF EXISTS {table_name}; 
   """

    create_table__strict_statement = f"""
      CREATE TABLE {table_name} (
        __key__            INTEGER PRIMARY KEY, 
        jets__key          TEXT NOT NULL,
        {column_schema}
      ) STRICT;
   """ # currently not supported by apsw and sqlite browser

    create_table_statement = f"""
      CREATE TABLE {table_name} (
        __key__            INTEGER PRIMARY KEY, 
        jets__key          TEXT NOT NULL,
        {column_schema}
      );
   """
    create_index_statement = f"""
      CREATE INDEX IF NOT EXISTS {table_name}_idx 
      ON {table_name} (jets__key);
   """
    cursor.execute(drop_table_statement)
    cursor.execute(create_table_statement)
    cursor.execute(create_index_statement)
    cursor = None      



  # -------------------------------------------------------------------------------------
  # _get_converters_and_dtypes
  # -------------------------------------------------------------------------------------
  def _get_converters_and_dtypes(self,lk_columns_dicts: list[dict], key_columns: list) -> tuple[dict,dict]:
      converters =  {}
      dtype_dict = {}
      for col in lk_columns_dicts:
          if col['type'] == 'bool':
              converters[col['name']] = self._convert_to_bool           
          else:
              dtype_dict[col['name']] = str    
      for key_col in key_columns:
          dtype_dict[key_col] = str
      return (converters, dtype_dict)

  # -------------------------------------------------------------------------------------
  # _validate_df
  # -------------------------------------------------------------------------------------
  def _validate_df(self,df,lk_columns_dicts: list[dict], key_columns: list):
        for col in lk_columns_dicts:
          if col['type'] in ['int','double','uint','long','ulong']:
            df[col['name']].apply(self._validate_num)   

  # -------------------------------------------------------------------------------------
  # _validate_num
  # -------------------------------------------------------------------------------------
  def _validate_num(self, val: str) -> str:
    if val and  pd.isnull(val) == False:
      string_val = str(val)
      if string_val.isdigit():
        return string_val

      m = re.match(r"^(-?|\+?)\d*\.?\d+$",string_val)
      if m:
        return string_val 
      else:
        raise Exception(f'_validate_num: {string_val} is not a valid num')
    else:
      return np.nan    

  # -------------------------------------------------------------------------------------
  # _convert_to_bool
  # -------------------------------------------------------------------------------------
  def _convert_to_bool(self, val: str) -> str:
      if val:
          val = str(val)
          value_length = len(val)

          if value_length == 1:
              if val == '0':
                  return '0'
              lower_val = val.lower()
              if lower_val == 'f' or lower_val == 'n':
                 return '0' 
              return '1'
          elif value_length == 5:
              lower_val = val.lower()
              if lower_val == 'false':
                  return '0'
              else:
                  return '1'
          elif value_length == 2:
              lower_val = val.lower()
              if lower_val == 'no':
                  return '0'
              else:
                  return '1'
          else:
              return '1'
      else:
        return '0'   


  # -------------------------------------------------------------------------------------
  # _create_lookup_df
  # -------------------------------------------------------------------------------------
  # Load Lookup CSV file to dataframe
  def _create_lookup_df(self,csv_file: str,key_columns: list[str],converters_and_dtypes: tuple[dict,dict]) -> None:
    csv_path = os.path.join(Path(self.base_path), csv_file)
    csv_path = os.path.abspath(csv_path)

    if not os.path.exists(csv_path):
        raise Exception('_load_csv_lookup: Could note locate: ' + str(csv_path))
    else:    
        lookup_df = pd.read_csv(csv_path, dtype=converters_and_dtypes[1], skipinitialspace = True, converters = converters_and_dtypes[0], escapechar='\\')


        if set(key_columns).issubset(set(lookup_df.columns)): 
            lookup_df.insert(0,'jets__key', lookup_df[key_columns].agg(''.join, axis=1))
        else:
            raise Exception(f'Key Columns missing in provided CSV. Expected {str(key_columns)} in header {str(lookup_df.columns)}')    

        lookup_df.insert(0, '__key__', range(0, len(lookup_df)))
        return lookup_df


  # -------------------------------------------------------------------------------------
  # _load_lookup
  # -------------------------------------------------------------------------------------
  # Load Lookup Dataframe to Lookup Table in lookup_db
  def _load_df_lookup(self,lookup_df, table_name: str,return_columns: list[str]) -> None:

        if set(return_columns).issubset(set(lookup_df.columns)): 
            lookup_df[return_columns].to_sql(table_name, self.lookup_connection, if_exists='append', index=False)
        else:
            raise Exception(f'Return Columns missing in provided CSV. Expected {str(return_columns)} in header {str(lookup_df.columns)}')    

 
  # -------------------------------------------------------------------------------------
  # _create_jets_key
  # -------------------------------------------------------------------------------------
  def _create_jets_key(self,row,key_columns: list[str]):
     composite_key = ''.join([row[x] for x in key_columns])
     return composite_key       


  # -------------------------------------------------------------------------------------
  # _open_rete_db
  # -------------------------------------------------------------------------------------
  def _open_rete_db(self,rete_db: str) -> None:
    try:
        if rete_db:
            print("trying to opendb : " + rete_db)
            self.workspace_connection = sqlite3.Connection(rete_db)
            self.workspace_connection.row_factory = sqlite3.Row
        else:
            rete_db_path = flags.FLAGS.rete_db
            if not rete_db_path:
                rete_db_path = 'jetrule_rete.db'
            path = os.path.join(Path(self.base_path), rete_db_path)
            path = os.path.abspath(path)
            print('*** RETE_DB PATH',path)
            self.workspace_connection = sqlite3.Connection(path)
            print('seeting connection *****')
            self.workspace_connection.row_factory = sqlite3.Row
    except (Exception) as error:
        print("Error while opening rete_db (1):", error)
        return str(error)
    finally:
        pass       


  # -------------------------------------------------------------------------------------
  # _open_lookup_db
  # -------------------------------------------------------------------------------------
  def _open_lookup_db(self,lookup_db:str) -> None:
        # Opening/creating Lookup database
        try:
            if lookup_db:
                self.lookup_connection = sqlite3.Connection(lookup_db)
                self.lookup_connection.row_factory = sqlite3.Row
            else:
                lookup_db_path = flags.FLAGS.lookup_db
                if not lookup_db_path:
                    lookup_db_path = 'jetrule_lookup.db'
                path = os.path.join(Path(self.base_path), lookup_db_path)
                path = os.path.abspath(path)
                print('*** LOOKUP_DB PATH',path)
                if not os.path.exists(path):
                    print('** DB Path does not exist, creating new lookup_db at ',path)
                if flags.FLAGS.clear_lookup_db and os.path.exists(path):
                    print('*** Clearing DB, creating new lookup_db at ',path)
                    os.remove(path)
                self.lookup_connection = sqlite3.Connection(path)
                self.lookup_connection.row_factory = sqlite3.Row
        except (Exception) as error:
            print("Error while opening lookup_db (1):", error)
            return str(error)
        finally:
            pass  