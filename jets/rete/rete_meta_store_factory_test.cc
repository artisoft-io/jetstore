#include <iostream>
#include <memory>
#include <filesystem>

#include <gtest/gtest.h>

#include "sqlite3.h"

#include "../rdf/rdf_types.h"
#include "../rete/rete_types.h"
#include "../rete/rete_meta_store_factory.h"

namespace fs = std::filesystem;
namespace jets::rete {
namespace {
// Simple test

static int callback(void *data, int argc, char **argv, char **azColName) {
  if (data) {
    std::cout << (const char*)data;
  }
   int i;
   for(i = 0; i<argc; i++) {
      std::cout << azColName[i] << " = " << (argv[i] ? argv[i] : "NULL") << std::endl;
   }
   std::cout << std::endl;
   return 0;
}


class SQLiteTest : public ::testing::Test {
 protected:
  SQLiteTest() : db_name("ms_test.db") {

    sqlite3 *db;
    char *zErrMsg = 0;
    int rc;
    char const* sql;

    /* Open database */
      std::filesystem::path p(db_name);
      std::cout << "Current path is " << fs::current_path() << '\n';
      std::cout << "Absolute path for " << p << " is " 
                << std::filesystem::absolute(p) << '\n';

      std::cout << "Path exist? " << std::filesystem::exists(p)  << '\n';
      if(std::filesystem::exists(p) and std::filesystem::is_regular_file(p)) {
        std::filesystem::remove(p);
      }

    //  rc = sqlite3_open(std::filesystem::absolute(p).c_str(), &db);
    rc = sqlite3_open(this->db_name.c_str(), &db);
    if( rc ) {
        std::cout<< "Can't open database: %s\n" << sqlite3_errmsg(db) << std::endl;
    } else {
        std::cout<< "Open database\n"  << std::endl;
    }
    EXPECT_EQ(rc, SQLITE_OK);

    /* Create SQL statement */
    sql = "CREATE TABLE COMPANY("  \
        "ID INT PRIMARY KEY     NOT NULL," \
        "NAME           TEXT    NOT NULL," \
        "AGE            INT     NOT NULL," \
        "ADDRESS        CHAR(50)," \
        "SALARY         REAL );";

    /* Execute SQL statement */
    rc = sqlite3_exec(db, sql, callback, 0, &zErrMsg);
    if( rc != SQLITE_OK ){
        fprintf(stderr, "SQL CREATE error: %s\n", zErrMsg);
        sqlite3_free(zErrMsg);
    } else {
        fprintf(stdout, "Table created successfully\n");
    }
    EXPECT_EQ(rc, SQLITE_OK);   

   /* Create SQL statement */
   sql = "INSERT INTO COMPANY (ID,NAME,AGE,ADDRESS,SALARY) "  \
         "VALUES (1, 'Paul', 32, 'California', 20000.00 ); " \
         "INSERT INTO COMPANY (ID,NAME,AGE,ADDRESS,SALARY) "  \
         "VALUES (2, 'Allen', 25, 'Texas', 15000.00 ); "     \
         "INSERT INTO COMPANY (ID,NAME,AGE,ADDRESS,SALARY)" \
         "VALUES (3, 'Teddy', 23, 'Norway', 20000.00 );" \
         "INSERT INTO COMPANY (ID,NAME,AGE,ADDRESS,SALARY)" \
         "VALUES (4, 'Mark', 25, 'Rich-Mond ', 65000.00 );";

   /* Execute SQL statement */
   rc = sqlite3_exec(db, sql, callback, 0, &zErrMsg);   
   if( rc != SQLITE_OK ){
      fprintf(stderr, "SQL INSERT error: %s\n", zErrMsg);
      sqlite3_free(zErrMsg);
   } else {
      fprintf(stdout, "Records created successfully\n");
   }
    EXPECT_EQ(rc, SQLITE_OK);   

   rc = sqlite3_close(db);
   if( rc != SQLITE_OK ){
      fprintf(stderr, "CLOSE error: %s\n", zErrMsg);
      sqlite3_free(zErrMsg);
   } else {
      fprintf(stdout, "DB closed successfully\n");
   }
   EXPECT_EQ(rc, SQLITE_OK);   

  }

  std::string db_name;
};

// Define the tests
TEST_F(SQLiteTest, FirstTest) {
  sqlite3 *db;
  char *zErrMsg = 0;
  int rc;
  char const*sql;
  const char* data = "Callback function called";

  /* Open database */
  rc = sqlite3_open(this->db_name.c_str(), &db);
  if( rc ) {
      fprintf(stderr, "Can't open database: %s\n", sqlite3_errmsg(db));
  } else {
      fprintf(stderr, "Opened database successfully\n");
  }
  EXPECT_EQ(rc, SQLITE_OK);   

  /* Create SQL statement */
  sql = "SELECT * from COMPANY";
  /* Execute SQL statement */
  rc = sqlite3_exec(db, sql, callback, (void*)data, &zErrMsg);    
  if( rc != SQLITE_OK ) {
      fprintf(stderr, "SQL error: %s\n", zErrMsg);
      sqlite3_free(zErrMsg);
  } else {
      fprintf(stdout, "Operation done successfully\n");
  }
  EXPECT_EQ(rc, SQLITE_OK);   

  rc = sqlite3_close(db);
  if( rc != SQLITE_OK ){
    fprintf(stderr, "CLOSE error: %s\n", zErrMsg);
    sqlite3_free(zErrMsg);
  } else {
    fprintf(stdout, "DB closed successfully\n");
  }
  EXPECT_EQ(rc, SQLITE_OK);
}
}   // namespace
}   // namespace jets::rete