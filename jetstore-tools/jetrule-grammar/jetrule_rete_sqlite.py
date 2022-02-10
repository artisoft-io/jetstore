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

class JetRuleReteSQLite:
  def __init__(self, ctx: JetRuleContext):
    self.ctx = ctx

  # =====================================================================================
  # saveReteConfig
  # -------------------------------------------------------------------------------------
  def saveReteConfig(self, workspace_db: str) -> None:
    assert workspace_db, "Must provide file name for workspace_db"
    workspace_connection = None

    # Opening/creating database
    try:
      workspace_connection = apsw.Connection(workspace_db)
    except (Exception) as error:
      print("Error:", error)
      return str(error)
    finally:
      pass

    # Saving the ctx.jetReteNodes
    try:
      # Create the workspace schema if new db
      cursor = workspace_connection.cursor()
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
        -- resources table
        -- --------------------
        CREATE TABLE IF NOT EXISTS resources (
          key                INTEGER PRIMARY KEY,
          type               STRING NOT NULL,
          id                 STRING,
          value              STRING,
          is_binded          BOOL,
          inline             BOOL,
          source_file_key    INTEGER NOT NULL
        );

        -- --------------------
        -- lookup_tables table
        -- --------------------
        CREATE TABLE IF NOT EXISTS lookup_tables (
          name               STRING PRIMARY KEY ASC,
          table_name         STRING,
          lookup_key         STRING,
          lookup_columns     STRING,
          lookup_resources   STRING,
          source_file_key    INTEGER NOT NULL
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
          vertex             INTEGER NOT NULL,
          type               STRING NOT NULL,
          subject_key        INTEGER,
          predicate_key      INTEGER,
          object_key         INTEGER,
          obj_expr_key       INTEGER,
          label              STRING,
          normalizedLabel    STRING,
          parent_vertex      INTEGER,
          beta_relation_vars STRING,
          pruned_var         STRING,
          source_file_key    INTEGER NOT NULL,
          PRIMARY KEY (vertex, source_file_key)
        );

      """)
      cursor = None

        # Word index
        word_map: Mapping[str, int] = {}
        word_last_key = 0

        with pg.connect(rdb_uri) as rdb_connection:

            #-----------------------------------------
            # City names indexing
            #-----------------------------------------
            #*
            print('reading from zipcode_city_lookup')

            city_names_rowid = 0
            city_names_rows = []
            city_names_words_lk_rows = []
            max_name_lenght = 0

            with rdb_connection.cursor() as rdb_cursor:

                if shard_id:
                    rdb_cursor.execute("SELECT zipcode, city, city_alias FROM zipcode_city_lookup WHERE shard_id = %s", (shard_id,))
                else:
                    rdb_cursor.execute("SELECT zipcode, city, city_alias FROM zipcode_city_lookup")

                for zipcode, city, city_alias in rdb_cursor:

                    city_alias_words = city_alias.split()

                    n_words = len(city_alias_words)
                    max_name_lenght = max(max_name_lenght, n_words)
                    if n_words > CITY_ALIAS_MAX_WORDS:
                        print("City alias name '{0}' for city '{1}' has too many tokens (max is {2}), skipping".format(city_alias, city, CITY_ALIAS_MAX_WORDS))
                        continue

                    # Map the alias name words
                    city_alias_seq = []
                    for word in city_alias_words:

                        key = word_map.get(word)
                        if key is None:
                            word_last_key += 1
                            key = word_last_key
                            word_map[word] = key

                        city_alias_seq.append(key)


                    # city_name_row
                    city_names_rowid += 1
                    zipcode_i = int(zipcode)
                    city_name_row = [city_names_rowid, zipcode_i, city, None, None, None, None, None, None]

                    pos = 0
                    for k in reversed(city_alias_seq):
                        city_name_row[3+pos] = k
                        pos += 1
                    
                    city_names_rows.append(city_name_row)
                    
                    # #*
                    # print('city_name_row',city_name_row)
                        
                    # Insert into the join table
                    for w in city_alias_seq:
                        city_names_words_lk_rows.append( (city_names_rowid, zipcode_i, w) )


            #-----------------------------------------
            # Insert in the city_names table
            #-----------------------------------------
            #*
            print('City alias name max lenght is',max_name_lenght)
            print('inserting into city_names', len(city_names_rows), 'records, and in city_names_words_lk', len(city_names_words_lk_rows), 'records')

            write_cursor = workspace_connection.cursor()
            write_cursor.execute('BEGIN')
            write_cursor.executemany("INSERT INTO city_names (rowid, zipcode, name, w0, w1, w2, w3, w4, w5) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)", city_names_rows)

            # Insert in the join table
            write_cursor.executemany("INSERT INTO city_names_words_lk (city_names_rowid, zipcode, w) VALUES (?, ?, ?)", city_names_words_lk_rows)
            write_cursor.execute('COMMIT')
            write_cursor.close()
            write_cursor = None


            #-----------------------------------------
            # Street names indexing
            #-----------------------------------------
            #*
            print('reading from zipcode_street_lookup')

            street_names_rowid = 0
            street_names_rows = []
            street_names_words_lk_rows = []
            max_name_lenght = 0

            with rdb_connection.cursor() as rdb_cursor:

                if shard_id:
                    rdb_cursor.execute("SELECT zipcode, street_name, street_name_suffix FROM zipcode_street_lookup WHERE shard_id = %s", (shard_id,))
                else:
                    rdb_cursor.execute("SELECT zipcode, street_name, street_name_suffix FROM zipcode_street_lookup")
                for zipcode, street_name, street_name_suffix in rdb_cursor:

                    street_name_words = street_name.split()
                    if street_name_suffix:
                        street_name_words.append(street_name_suffix)

                    n_words = len(street_name_words)
                    max_name_lenght = max(max_name_lenght, n_words)
                    if n_words > STREET_NAME_MAX_WORDS:
                        print("Street name '{0}' has too many tokens (max is {1}, skipping)".format(' '.join(street_name_words), STREET_NAME_MAX_WORDS))
                        continue

                    # Map the alias name words
                    street_name_seq = []
                    for word in street_name_words:

                        key = word_map.get(word)
                        if key is None:
                            word_last_key += 1
                            key = word_last_key
                            word_map[word] = key

                        street_name_seq.append(key)

                    # street_names_row
                    street_names_rowid += 1
                    zipcode_i = int(zipcode)
                    has_suffix = street_name_suffix is not None
                    street_names_row = [street_names_rowid, zipcode_i, ' '.join(street_name_words), has_suffix, None, None, None, None, None, None, None]

                    pos = 0
                    for k in reversed(street_name_seq):
                        street_names_row[4+pos] = k
                        pos += 1
                    
                    street_names_rows.append(street_names_row)
                        
                    # Insert into the join table
                    for w in street_name_seq:
                        street_names_words_lk_rows.append( (street_names_rowid, zipcode_i, w) )


            #-----------------------------------------
            # Insert in the street_names table
            #-----------------------------------------
            #*
            print('Street name max lenght is',max_name_lenght)
            print('inserting into street_names', len(street_names_rows), 'records, and in street_names_words_lk', len(street_names_words_lk_rows), 'records')

            write_cursor = workspace_connection.cursor()
            write_cursor.execute('BEGIN')
            write_cursor.executemany("INSERT INTO street_names (rowid, zipcode, name, has_suffix, w0, w1, w2, w3, w4, w5, w6) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", street_names_rows)

            # Insert in the join table
            write_cursor.executemany("INSERT INTO street_names_words_lk (street_names_rowid, zipcode, w) VALUES (?, ?, ?)", street_names_words_lk_rows)
            write_cursor.execute('COMMIT')
            write_cursor.close()
            write_cursor = None


        #-----------------------------------------
        # Words reference table
        #-----------------------------------------
        rows = []
        for w, k in word_map.items():
            rows.append((k, w))

        #*
        print('Inserting',len(word_map),'words in words table')

        write_cursor = workspace_connection.cursor()
        write_cursor.execute('BEGIN')
        write_cursor.executemany("INSERT INTO words VALUES (?, ?)", rows)
        write_cursor.execute('COMMIT')


    except (Exception) as error:

        print("Error:", error)
        return str(error)

    finally:
        if workspace_connection:
            workspace_connection.close(True)

    
    # All good here!
    return None
