#include <iostream>
#include <memory>
#include <string>
#include <string_view>

#include <gtest/gtest.h>

#include "../rdf/rdf_types.h"

namespace jets::rdf {
namespace {
// Simple test

TEST(RManagerTest, CreateLiteral) 
{
  // rdf resource manager
  auto rmanager_p = RManager::create();
  auto rmanager = *rmanager_p;

  // literals
  auto five = rmanager.create_literal<int32_t>(5);
  LInt32 tfive(5);
  EXPECT_TRUE(boost::get<LInt32>(*five) == tfive);
  EXPECT_EQ(rmanager.size(), 1);

  auto fivex = rmanager.get_literal<int32_t>(5);
  EXPECT_EQ(five, fivex);

  auto bfalse = rmanager.create_literal<bool>(false);

  auto zero = rmanager.create_literal<int32_t>(0);
  EXPECT_NE(five, zero);
  EXPECT_EQ(rmanager.size(), 2);

  auto bfalse_p = boost::get<LInt32>(bfalse);
  EXPECT_NE(bfalse_p, nullptr);
  EXPECT_FALSE(bfalse_p->data);

  auto zero_p = boost::get<LInt32>(zero);
  auto five_p = boost::get<LInt32>(five);
  EXPECT_EQ(bfalse_p->data, zero_p->data);
  EXPECT_NE(bfalse_p->data, five_p->data);

}

TEST(RManagerTest, CreateBNodes) 
{
  // rdf resource manager
  auto rmanager_p = RManager::create();
  auto rmanager = *rmanager_p;

  // subjects
  auto bn1 = rmanager.create_bnode();
  BlankNode tbn1(1);
  EXPECT_TRUE(boost::get<BlankNode>(*bn1) == tbn1);
  EXPECT_EQ(get_key(bn1), tbn1.key);
  EXPECT_EQ(get_key(nullptr), 0);
  EXPECT_EQ(rmanager.size(), 1);

  auto bn1x = rmanager.create_bnode(get_key(bn1));
  EXPECT_TRUE(bn1x == bn1);
  EXPECT_EQ(rmanager.size(), 1);

  auto bn2 = rmanager.create_bnode();
  EXPECT_FALSE(bn2 == bn1);
  EXPECT_FALSE(boost::get<BlankNode>(*bn2) == boost::get<BlankNode>(*bn1));
  EXPECT_NE(get_key(bn2), get_key(bn1));
  EXPECT_EQ(rmanager.size(), 2);

  // using pointer rather than ref
  auto x = boost::get<BlankNode>(bn2);
  EXPECT_EQ(x->key, get_key(bn2));

  // objects
  auto five = rmanager.create_literal<int32_t>(5);
  LInt32 tfive(5);
  EXPECT_TRUE(boost::get<LInt32>(*five) == tfive);
  EXPECT_EQ(rmanager.size(), 3);
}

TEST(RManagerTest, CreateResources) 
{
  // rdf resource manager
  auto rmanager_p = RManager::create();
  auto rmanager = *rmanager_p;

  // subjects
  std::string s1("r1");
  auto r1 = rmanager.create_resource(s1);
  NamedResource tr1("r1");
  EXPECT_TRUE(boost::get<NamedResource>(*r1) == tr1);
  EXPECT_EQ(get_name(r1), tr1.name);
  EXPECT_EQ(get_name(nullptr), std::string{"NULL"});

  // create again the same resource
  auto r2 = rmanager.create_resource(s1);
  EXPECT_EQ(r2, r1);
}

TEST(RManagerTest, CreateResources2) 
{
  // rdf resource manager
  auto rmanager_p = RManager::create();
  auto rmanager = *rmanager_p;

  // test to find back resource
  auto r1 = rmanager.create_resource("r1");
  auto r2 = rmanager.get_resource("r1");
  EXPECT_EQ(r1, r2);
}

TEST(RManagerTest, CreateNulls) 
{
  // rdf resource manager
  auto rmanager_p = RManager::create();
  auto rmanager = *rmanager_p;

  // subjects
  auto n1 = rmanager.get_null();
  auto n2 = rmanager.get_null();
  EXPECT_EQ(n1, n2);
  EXPECT_TRUE(boost::get<RDFNull>(*n1) == boost::get<RDFNull>(*n2));
  EXPECT_EQ(get_name(nullptr), std::string{"NULL"});

  // create a resource
  auto r1 = rmanager.create_resource("r1");
  EXPECT_NE(r1, n1);
}

TEST(RManagerTest, NullIsASingleton)
{
  // rdf resource manager
  auto rmanager_p = RManager::create();
  auto rmanager = *rmanager_p;
  rmanager.initialize();

  // nulls behave the same
  auto n1 = RdfAstType();
  auto n2 = RdfAstType();
  EXPECT_EQ(n1.which(), rdf_null_t);
  EXPECT_EQ(n2.which(), rdf_null_t);
  EXPECT_TRUE(n1 == n2);
  EXPECT_FALSE(n1 != n2);

  // but are different instance, hence different address
  r_index r1 = &n1;
  r_index r2 = &n2;
  void* vr1 = (void*)r1;
  void* vr2 = (void*)r2;
  EXPECT_TRUE(vr1 != vr2);
  // r_index operator== makes it to be equal
  EXPECT_TRUE(n1 == n2);
  EXPECT_FALSE(n1 != n2);

  // the resource manager resolve it correctly to the singleton
  r1 = rmanager.insert_item(std::make_shared<rdf::RdfAstType>(n1));
  r2 = rmanager.insert_item(std::make_shared<rdf::RdfAstType>(n2));
  EXPECT_TRUE(r1 == r2);
  EXPECT_TRUE(r1 == gnull());
  EXPECT_FALSE(r1 != gnull());
}

TEST(RManagerTest, MetaManager) 
{
  // rdf resource meta manager
  auto meta_mgr = create_rmanager();

  // subjects
  auto r1 = meta_mgr->create_resource("r1");
  EXPECT_EQ(get_name(r1), std::string{"r1"});

  auto r1x = meta_mgr->get_resource("r1");
  EXPECT_EQ(r1, r1x);

  EXPECT_FALSE(meta_mgr->is_locked());
  meta_mgr->set_locked();
  EXPECT_TRUE(meta_mgr->is_locked());

  auto r_mgr = create_rmanager(meta_mgr);
  
  // create again the same resource, should be the same
  auto r2 = r_mgr->create_resource("r1");
  EXPECT_EQ(r2, r1);

  auto r3 = r_mgr->create_resource("r3");
  EXPECT_NE(r3, r2);

  // try to create a resource from meta_mgr which should throw
  EXPECT_THROW(meta_mgr->create_resource("r4"), rdf_exception);

  r1x = meta_mgr->get_resource("r1");
  EXPECT_EQ(r1, r1x);

  // creating what a locked meta manager already holds returns it, as the Go
  // engine's ResourceManager does; it threw until 2026-10-11
  // (rules_unit_test_harness I-15)
  EXPECT_EQ(meta_mgr->create_resource("r1"), r1);
  auto l5 = meta_mgr->get_literal(int32_t(5));
  EXPECT_EQ(l5, nullptr);
  EXPECT_THROW(meta_mgr->create_literal(int32_t(5)), rdf_exception);
}

// A value is found by value, without building an Rptr, and gets the same r_index as
// the Rptr built for it would (LiteralDataMap, containers_type.h).
TEST(RManagerTest, FindByValueHashesAsTheRptr)
{
  RdfAstHash hash;
  RdfAstEq eq;
  auto res = mkResource("x");
  auto txt = mkLiteral("x");
  EXPECT_EQ(hash(res), hash(RdfStringKey{rdf_named_resource_t, "x"}));
  EXPECT_EQ(hash(txt), hash(RdfStringKey{rdf_literal_string_t, std::string("x")}));
  EXPECT_TRUE(eq(res, RdfStringKey{rdf_named_resource_t, "x"}));
  EXPECT_FALSE(eq(res, RdfStringKey{rdf_literal_string_t, "x"}));
  EXPECT_FALSE(eq(txt, RdfStringKey{rdf_literal_string_t, "y"}));
  for(auto const& p: {mkLiteral(int32_t(7)), mkLiteral(uint32_t(7)), mkLiteral(int64_t(7)), 
      mkLiteral(uint64_t(7)), mkLiteral(7.5), mkBlankNode(7), mkNull(),
      mkLiteral(date(2026, 10, 21)), mkLiteral(date(boost::date_time::not_a_date_time)),
      mkLiteral(datetime(date(2026, 10, 21), boost::posix_time::hours(7)))}) {
    RdfAstType v = *p;
    EXPECT_EQ(hash(p), hash(v)) << to_string(p.get());
    EXPECT_TRUE(eq(p, v)) << to_string(p.get());
  }
  EXPECT_FALSE(eq(mkLiteral(int32_t(7)), RdfAstType(LInt64(7))));
}

TEST(RManagerTest, CreateFindsByValueAndBuildsOnlyWhatIsNew)
{
  auto rmgr = RManager::create();
  auto s1 = rmgr->create_literal(std::string("hello"));
  EXPECT_EQ(rmgr->create_literal(std::string_view("hello")), s1);
  EXPECT_EQ(rmgr->create_literal("hello"), s1);
  EXPECT_EQ(rmgr->get_literal("hello"), s1);
  EXPECT_EQ(rmgr->insert_item(mkLiteral("hello")), s1);
  EXPECT_EQ(rmgr->insert_rdfast(LString("hello")), s1);
  EXPECT_EQ(rmgr->size(), 1u);

  auto r1 = rmgr->create_resource("hello");
  EXPECT_NE(r1, s1);                              // a resource is not a string literal
  EXPECT_EQ(rmgr->create_resource(std::string("hello")), r1);
  EXPECT_EQ(rmgr->get_resource(std::string_view("hello")), r1);
  EXPECT_EQ(rmgr->insert_item(mkResource("hello")), r1);
  EXPECT_EQ(rmgr->size(), 2u);

  auto i7 = rmgr->create_literal(int32_t(7));
  EXPECT_NE(rmgr->create_literal(int64_t(7)), i7);  // a different type is a different literal
  EXPECT_EQ(rmgr->insert_rdfast(LInt32(7)), i7);
  EXPECT_EQ(rmgr->get_rdfast(LInt32(7)), i7);
  auto nad = rmgr->create_literal(date(boost::date_time::not_a_date_time));
  EXPECT_EQ(rmgr->insert_item(mkLiteral(date(boost::date_time::not_a_date_time))), nad);
  auto b3 = rmgr->create_bnode(3);
  EXPECT_EQ(rmgr->get_bnode(3), b3);
  EXPECT_EQ(rmgr->size(), 6u);
  EXPECT_EQ(rmgr->get_literal("absent"), nullptr);
  EXPECT_EQ(rmgr->get_rdfast(LDouble(1.5)), nullptr);
  EXPECT_EQ(rmgr->size(), 6u);

  // a session manager finds what its locked root holds, and builds the rest itself
  auto root = RManager::create();
  auto rr = root->create_resource("shared");
  auto sess = RManager::create(root);
  EXPECT_EQ(sess->create_resource("shared"), rr);
  EXPECT_EQ(sess->size(), 0u);
  EXPECT_NE(sess->create_literal("own"), nullptr);
  EXPECT_EQ(sess->size(), 1u);
  EXPECT_THROW(root->create_literal("new"), rdf_exception);
}

}   // namespace
}   // namespace jets::rdf