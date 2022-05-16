#include <cstddef>
#include <iostream>
#include <string>
#include <memory>

#include <gtest/gtest.h>

#include "../rdf/rdf_types.h"
#include "../rete/rete_types.h"
#include "../rete/rete_meta_store_factory.h"
#include "../rete/lookup_sql_helper.h"
#include "../rete/expr_op_others.h"

namespace jets::rete {
namespace {
class LookupSqlHelperTest : public ::testing::Test {
 protected:
  LookupSqlHelperTest() : factory(), rdf_session(), rete_session() 
  {
    int res = factory.load_database("test_data/lookup_helper_test_workspace.db", "test_data/lookup_helper_test_data.db");
    EXPECT_EQ(res, 0);

    // Get the Rete Meta Store 
    meta_store = factory.get_rete_meta_store("lookup_helper_test_workspace.jr");  
    EXPECT_TRUE(meta_store);

    // Create the rdf_session and the rete_session and initialize them
    // Initialize the rete_session now that the rule base is ready
    this->rdf_session = rdf::create_rdf_session(factory.get_meta_graph());
    this->rete_session = create_rete_session(meta_store, this->rdf_session.get());
    this->rete_session->initialize();

    // std::cout<<"Rete Session Initialize Completed!"<<std::endl;
  }

  ReteMetaStoreFactory factory;
  rdf::RDFSessionPtr rdf_session;
  ReteSessionPtr rete_session;
  ReteMetaStorePtr meta_store;
};

TEST_F(LookupSqlHelperTest, LookupTest1) {

  rdf::RdfAstType out;
  auto helper = meta_store->get_lookup_sql_helper();

  // Lookup
  EXPECT_EQ(helper->lookup(rete_session.get(), "acme:ProcedureLookup", "100", &out), 0);

  // std::cout<<"Lookup GOT: "<<rdf::get_name(&out)<<std::endl;

  // Verifying we pull stuff correctly from lookup table
  rdf::r_index s = rdf_session->rmgr()->create_resource("jets:acme:ProcedureLookup:100");
  auto itor = rdf_session->find(s, rdf::make_any(), rdf::make_any());
  while(not itor.is_end()) {
    auto predicate = rdf::get_name(itor.get_predicate());
    if(predicate == "PROC_RID") {
      EXPECT_EQ(std::string(rdf::get_type_name(itor.get_object())), std::string("long"));
      EXPECT_EQ(boost::get<rdf::LInt64>(*itor.get_object()).data, 12345678901L);
    } else if(predicate == "FROM_DATE") {
      EXPECT_EQ(std::string(rdf::get_type_name(itor.get_object())), std::string("date"));
    } else if(predicate == "EXCL") {
      EXPECT_EQ(std::string(rdf::get_type_name(itor.get_object())), std::string("int"));
    } else if(predicate == "EVENT_DURATION") {
      EXPECT_EQ(std::string(rdf::get_type_name(itor.get_object())), std::string("int"));
    } else {
      FAIL();
    }
    itor.next();
  }
  EXPECT_EQ(helper->terminate(), 0);
}

TEST_F(LookupSqlHelperTest, LookupTest2) {

  rdf::RdfAstType out;
  auto helper = meta_store->get_lookup_sql_helper();

  // Lookup
  EXPECT_EQ(helper->lookup(rete_session.get(), "acme:ProcedureLookup", "100", &out), 0);

  // std::cout<<"Lookup GOT: "<<rdf::get_name(&out)<<std::endl;

  // Verifying we pull stuff correctly from lookup table
  rdf::r_index s = rdf_session->rmgr()->create_resource("jets:acme:ProcedureLookup:100");
  auto itor = rdf_session->find(s, rdf::make_any(), rdf::make_any());
  while(not itor.is_end()) {
    auto predicate = rdf::get_name(itor.get_predicate());
    if(predicate == "PROC_RID") {
      EXPECT_EQ(std::string(rdf::get_type_name(itor.get_object())), std::string("long"));
      EXPECT_EQ(boost::get<rdf::LInt64>(*itor.get_object()).data, 12345678901L);
    } else if(predicate == "FROM_DATE") {
      EXPECT_EQ(std::string(rdf::get_type_name(itor.get_object())), std::string("date"));
      EXPECT_EQ(boost::get<rdf::LDate>(*itor.get_object()).data, rdf::parse_date("01-04-2022"));
    } else if(predicate == "EXCL") {
      EXPECT_EQ(std::string(rdf::get_type_name(itor.get_object())), std::string("int"));
      EXPECT_EQ(rdf::to_bool(*itor.get_object()), true);
    } else if(predicate == "EVENT_DURATION") {
      EXPECT_EQ(std::string(rdf::get_type_name(itor.get_object())), std::string("int"));
      EXPECT_EQ(boost::get<rdf::LInt32>(*itor.get_object()).data, 100);
    } else {
      FAIL();
    }
    itor.next();
  }
  EXPECT_EQ(helper->terminate(), 0);
}

TEST_F(LookupSqlHelperTest, MultiLookupTest1) {

  auto helper = meta_store->get_lookup_sql_helper();

  rdf::RdfAstType out;

  // Multi Lookup
  EXPECT_EQ(helper->multi_lookup(rete_session.get(), "acme:ProcedureLookup", "200", &out), 0);

  // std::cout<<"MULTI Lookup GOT: "<<rdf::get_name(&out)<<std::endl;

  // Verifying we pull stuff correctly from lookup table
  rdf::r_index s = rdf_session->rmgr()->create_resource("jets:acme:ProcedureLookup:200");
  rdf::r_index p = rdf_session->rmgr()->create_resource("jets:lookup_multi_rows");
  int row_count = 0;
  auto itor = rdf_session->find(s, p, rdf::make_any());  
  while(not itor.is_end()) {
    auto jtor = rdf_session->find(itor.get_object(), rdf::make_any(), rdf::make_any());  
    while(not jtor.is_end()) {
      std::cout<<"   "<<jtor.as_triple()<<std::endl;
      auto predicate = rdf::get_name(jtor.get_predicate());
      if(predicate == "PROC_RID") {
        EXPECT_EQ(std::string(rdf::get_type_name(jtor.get_object())), std::string("long"));
        EXPECT_EQ(boost::get<rdf::LInt64>(*jtor.get_object()).data, 12345678901L);
      } else if(predicate == "FROM_DATE") {
        EXPECT_EQ(std::string(rdf::get_type_name(jtor.get_object())), std::string("date"));
        EXPECT_EQ(boost::get<rdf::LDate>(*jtor.get_object()).data, rdf::parse_date("01-04-2022"));
      } else if(predicate == "EXCL") {
        EXPECT_EQ(std::string(rdf::get_type_name(jtor.get_object())), std::string("int"));
      } else if(predicate == "EVENT_DURATION") {
        EXPECT_EQ(std::string(rdf::get_type_name(jtor.get_object())), std::string("int"));
        EXPECT_EQ(boost::get<rdf::LInt32>(*jtor.get_object()).data, 200);
      } else {
        FAIL();
      }
      ++row_count;
      jtor.next();
    }
    itor.next();
  }

  EXPECT_EQ(row_count, 4*2);
  EXPECT_EQ(helper->terminate(), 0);
}

TEST_F(LookupSqlHelperTest, ToTypeOfTest1) {

  auto helper = meta_store->get_lookup_sql_helper();


  // Type Of Test
  EXPECT_EQ(helper->type_of(rete_session.get(), "claim_number"),         5);
  EXPECT_EQ(helper->type_of(rete_session.get(), "date_of_service"),      9);
  EXPECT_EQ(helper->type_of(rete_session.get(), "point_in_time"),        10);
  EXPECT_EQ(helper->type_of(rete_session.get(), "primary_diagnosis"),    8);
  EXPECT_EQ(helper->type_of(rete_session.get(), "secondary_diagnosis"),  8);
  EXPECT_EQ(helper->type_of(rete_session.get(), "count"),                3);
  EXPECT_EQ(helper->type_of(rete_session.get(), "size"),                 4);
  EXPECT_EQ(helper->type_of(rete_session.get(), "amount"),               7);
  EXPECT_EQ(helper->type_of(rete_session.get(), "timestamp"),            6);


  EXPECT_EQ(helper->terminate(), 0);
}

TEST_F(LookupSqlHelperTest, ToTypeOfTest2) {

  // Type Of Test ToTypeOfOperator
  std::cout<<"ToTypeOfOperator Test "<<std::endl;

  // lhs is int 1 (value)
  rdf::RdfAstType lhs(rdf::LInt32(121));
  // rhs is type to cast to, text is 8
  rdf::RdfAstType rhs(rdf::LInt32(8));

  ToTypeOfOperator op(this->rete_session.get(), nullptr, &lhs, &rhs);
  rdf::RdfAstType out = op();
  EXPECT_EQ(rdf::get_text(&out), "121");
}

TEST_F(LookupSqlHelperTest, ToTypeOfTest3) {
  // lhs is value
  rdf::RdfAstType lhs;
  // rhs is type to cast to, 
  rdf::RdfAstType rhs(rdf::LInt32(8));

  ToTypeOfOperator op(this->rete_session.get(), nullptr, &lhs, &rhs);
  rdf::RdfAstType out = op();
  EXPECT_EQ(rdf::get_type(&out), 0);
}

TEST_F(LookupSqlHelperTest, ToTypeOfTest4) {
  // lhs is value
  rdf::RdfAstType lhs(rdf::LString("-123.45"));
  // rhs is type to cast to, 
  rdf::RdfAstType rhs(rdf::LInt32(7)); // cast to double
  ToTypeOfOperator op(this->rete_session.get(), nullptr, &lhs, &rhs);
  rdf::RdfAstType out = op();
  EXPECT_EQ(boost::get<rdf::LDouble>(out).data, -123.45);
  EXPECT_EQ(rdf::get_type(&out), 7);
}

TEST_F(LookupSqlHelperTest, ToTypeOfTest5) {
  // lhs is value
  rdf::RdfAstType lhs(rdf::LString("-123.45"));
  // rhs is type to cast to, 
  rdf::RdfAstType rhs(rdf::LString("double")); // cast to double
  ToTypeOfOperator op(this->rete_session.get(), nullptr, &lhs, &rhs);
  rdf::RdfAstType out = op();
  EXPECT_EQ(boost::get<rdf::LDouble>(out).data, -123.45);
  EXPECT_EQ(rdf::get_type(&out), 7);
}

TEST_F(LookupSqlHelperTest, ToTypeOfTest6) {
  // lhs is value
  rdf::RdfAstType lhs(rdf::LString("01/22/2022"));
  // rhs is type to cast to, 
  rdf::RdfAstType rhs(rdf::LString("date")); 
  ToTypeOfOperator op(this->rete_session.get(), nullptr, &lhs, &rhs);
  rdf::RdfAstType out = op();
  // std::cout<<"*** The Date is: "<<out<<std::endl;
  EXPECT_EQ(boost::get<rdf::LDate>(out).data, rdf::parse_date("01/22/2022"));
  EXPECT_EQ(rdf::get_type(&out), 9);
}

TEST_F(LookupSqlHelperTest, ToTypeOfTest61) {
  // lhs is value
  rdf::RdfAstType lhs(rdf::LString("01/22/2022"));
  // rhs is type to cast to, 
  rdf::RdfAstType rhs(rdf::LInt32(9)); 
  ToTypeOfOperator op(this->rete_session.get(), nullptr, &lhs, &rhs);
  rdf::RdfAstType out = op();
  // std::cout<<"*** The Date is: "<<out<<std::endl;
  EXPECT_EQ(boost::get<rdf::LDate>(out).data, rdf::parse_date("01/22/2022"));
  EXPECT_EQ(rdf::get_type(&out), 9);
}

TEST_F(LookupSqlHelperTest, ToTypeOfTest7) {
  // lhs is value
  rdf::RdfAstType lhs(rdf::LString("01/22/2022"));
  // rhs is type to cast to, 
  rdf::RdfAstType rhs(rdf::NamedResource("date_of_service")); 
  ToTypeOfOperator op(this->rete_session.get(), nullptr, &lhs, &rhs);
  rdf::RdfAstType out = op();
  // std::cout<<"*** The Date from Named Resource is: "<<out<<std::endl;
  EXPECT_EQ(boost::get<rdf::LDate>(out).data, rdf::parse_date("01/22/2022"));
  EXPECT_EQ(rdf::get_type(&out), 9);
}

TEST_F(LookupSqlHelperTest, ToTypeOfTest8) {
  // lhs is value
  rdf::RdfAstType lhs(rdf::LString("123"));
  // rhs is type to cast to, 
  rdf::RdfAstType rhs(rdf::NamedResource("count")); 
  ToTypeOfOperator op(this->rete_session.get(), nullptr, &lhs, &rhs);
  rdf::RdfAstType out = op();
  // std::cout<<"*** The count from Named Resource is: "<<out<<std::endl;
  EXPECT_EQ(boost::get<rdf::LInt32>(out).data, 123);
  EXPECT_EQ(rdf::get_type(&out), 3);
}

}   // namespace
}   // namespace jets::rete