#ifndef JETS_RDF_R_MANAGER_H
#define JETS_RDF_R_MANAGER_H

#include <cstdint>
#include <string>
#include <string_view>
#include <memory>
#include <list>
#include <unordered_set>
#include <unordered_map>

#include <glog/logging.h>
#include "absl/hash/hash.h"

#include "../rdf/rdf_err.h"
#include "../rdf/rdf_ast.h"
#include "../rdf/uuid.h"
#include "../rdf/containers_type.h"
#include "../rdf/r_resources.h"

// Component to manage all the rdf resources and literals of a graph
namespace jets::rdf {
/////////////////////////////////////////////////////////////////////////////////////////
// RManager manage and allocate all resources and literals used in a RDFGraph
class RManager {
 public:
  using ResourceList = std::list<Rptr>;
  using DataMap = LiteralDataMap;

 protected:
  // constructors are private to ensure using create_rmanager()
  // is used so that the root_mgr is locked
  inline RManager() 
    : is_locked_(false),
      last_bnode_key_(0),
      lmap_(),
      root_mgr_p_(),
      jets_resources_()
  {}

  inline RManager(RManagerPtr root_mgr_p) 
    : is_locked_(false),
      last_bnode_key_(0),
      lmap_(),
      root_mgr_p_(root_mgr_p),
      jets_resources_()
  {}

 public:
  static RManagerPtr create()
  {
    struct make_shared_enabler: public RManager {};
    return std::make_shared<make_shared_enabler>();
  }

  static RManagerPtr create(RManagerPtr root_mgr_p)
  {
    if(root_mgr_p) {
      root_mgr_p->set_locked();
    }
    struct make_shared_enabler: public RManager {
      make_shared_enabler(RManagerPtr rm): RManager(rm){}
    };
    return std::make_shared<make_shared_enabler>(root_mgr_p);
  }

  inline bool
  is_initialized()const
  {
    if(this->root_mgr_p_) return this->root_mgr_p_->is_initialized();
    return this->jets_resources_.is_initialized();
  }

  inline void
  initialize()
  {
    if(this->root_mgr_p_) {
      if(not this->root_mgr_p_->is_initialized()) {
        RDF_EXCEPTION("ERROR RManager::root_mgr_p_ Resources are not initialized!")
      }
      return;
    }
    this->jets_resources_.initialize(this);
  }

  inline JetsResources const*
  jets()const
  {
    if(this->root_mgr_p_) return this->root_mgr_p_->jets();
    return &this->jets_resources_;
  }

  /**
   * @return size_t the nbr of resources excluding nulls and resources in metamap.
   */
  inline size_t
  size() const
  {
    return lmap_.size();
  }

  inline bool
  is_locked() const
  {
    return is_locked_;
  }

  inline void 
  set_locked()
  {
    this->is_locked_ = true;
  }

  inline r_index 
  get_null() const
  {
    return gnull();
  }

  // The lookups below find a value without allocating, and build its Rptr only when
  // the value is new (LiteralDataMap, containers_type.h).
  template<class T>
  inline r_index 
  get_literal(T const& v) const
  {
    return get_by(literal_key(v));
  }

  template<class T>
  inline r_index 
  create_literal(T v)
  {
    return insert_by(literal_key(v), [&v]() { return mkLiteral(std::move(v)); });
  }

  // A std::string is not copied to look it up: only a new one is copied, or moved
  inline r_index 
  create_literal(std::string const& v)
  {
    return insert_by(literal_key(v), [&v]() { return mkLiteral(v); });
  }

  inline r_index 
  create_literal(std::string && v)
  {
    return insert_by(literal_key(v), [&v]() { return mkLiteral(std::move(v)); });
  }

  template<class T>
  inline r_index 
  get_resource(T const& v) const
  {
    return get_by(resource_key(v));
  }

  template<class T>
  inline r_index 
  create_resource(T v)
  {
    return insert_by(resource_key(v), [&v]() { return mkResource(std::move(v)); });
  }

  inline r_index 
  create_resource(std::string const& v)
  {
    return insert_by(resource_key(v), [&v]() { return mkResource(v); });
  }

  inline r_index 
  create_resource(std::string && v)
  {
    return insert_by(resource_key(v), [&v]() { return mkResource(std::move(v)); });
  }

  inline r_index 
  create_uuid_resource()
  {
    return create_resource(create_uuid());
  }

  inline r_index 
  get_bnode(int v) const
  {
    return get_by(RdfAstType(BlankNode(v)));
  }

  inline r_index 
  create_bnode(int v)
  {
    return insert_by(RdfAstType(BlankNode(v)), [v]() { return mkBlankNode(v); });
  }

  inline r_index 
  create_bnode()
  {
    return create_bnode(get_next_key());
  }

  inline r_index
  insert_rdfast(RdfAstType v)
  {
    return insert_by(v, [&v]() { return std::make_shared<RdfAstType>(std::move(v)); });
  }

  inline r_index
  get_rdfast(RdfAstType v)
  {
    return get_by(v);
  }

  inline r_index 
  insert_item(Rptr lptr)
  {
    if(is_locked_) {
      // A locked manager still returns what it already holds, as the retired Go
      // engine's ResourceManager did; only a new item is refused.
      auto itor = lmap_.find(lptr);
      if(itor != lmap_.end()) return itor->second;
      throw rdf_exception("Accessing meta_graph to rdf index -- must use session graph instead");
    }
    if(root_mgr_p_) {
      auto itor = root_mgr_p_->lmap_.find(lptr);
      if(itor != root_mgr_p_->lmap_.end()) {
        // std::cout<<"Resource/literal found in meta_mgr: "<<lptr.get()<<std::endl;
        return itor->second;
      }
    }
    auto ret = lmap_.insert(std::make_pair<>(lptr, lptr.get()));
    // if(ret.second) {
    //   std::cout<<"New resource/literal created XX: "<<ret.first->second<<std::endl;
    // } else {
    //   std::cout<<"Resource/Literal was already created: "<<ret.first->second<<std::endl;
    // }
    return ret.first->second;
  }

  inline r_index 
  get_item(Rptr lptr) const
  {
    if(root_mgr_p_) {
      auto itor = root_mgr_p_->lmap_.find(lptr);
      if(itor != root_mgr_p_->lmap_.end()) {
        // std::cout<<"Resource/literal found in meta_mgr: "<<lptr.get()<<std::endl;
        return itor->second;
      }
    }
    auto itor = lmap_.find(lptr);
    if(itor != lmap_.end()) {
      // std::cout<<"Resource/Literal was already created: "<<ret.first->second<<std::endl;
      return itor->second;
    }
    return nullptr;
  }

  void
  dump_resources()const
  {
    VLOG(60)<<"The Session Resources:";
    std::list<std::string> rlist;
    if(this->root_mgr_p_) {
      for(auto r: this->root_mgr_p_->lmap_) {
        rlist.push_back(to_string(r.second) + " ("+rdf::get_type_name(r.second)+")");
      }
    }
    for(auto r: this->lmap_) {
      rlist.push_back(to_string(r.second) + " ("+rdf::get_type_name(r.second)+")");
    }
    rlist.sort();
    for(auto const& item: rlist) {
      VLOG(60)<<item;
    }
  }

 protected:
  // The key to look a value up by, as mkLiteral and mkResource would build it
  static RdfAstType literal_key(std::int32_t v)  { return LInt32(v); }
  static RdfAstType literal_key(std::uint32_t v) { return LUInt32(v); }
  static RdfAstType literal_key(std::int64_t v)  { return LInt64(v); }
  static RdfAstType literal_key(std::uint64_t v) { return LUInt64(v); }
  static RdfAstType literal_key(double v)        { return LDouble(v); }
  static RdfAstType literal_key(date v)          { return v.is_not_a_date() ? LDate() : LDate(v); }
  static RdfAstType literal_key(datetime v)      { return LDatetime(v); }
  static RdfStringKey literal_key(std::string const& v) { return RdfStringKey{rdf_literal_string_t, v}; }
  static RdfStringKey literal_key(std::string_view v)   { return RdfStringKey{rdf_literal_string_t, v}; }
  static RdfStringKey literal_key(const char * v)       { return RdfStringKey{rdf_literal_string_t, v}; }

  static RdfStringKey resource_key(std::string const& v) { return RdfStringKey{rdf_named_resource_t, v}; }
  static RdfStringKey resource_key(std::string_view v)   { return RdfStringKey{rdf_named_resource_t, v}; }
  static RdfStringKey resource_key(const char * v)       { return RdfStringKey{rdf_named_resource_t, v}; }

  // insert_item, by value: make() builds the Rptr, and is called only for a new value.
  // `key` must not be read after make(): the callers' make() moves from the value that
  // `key` views (an RdfStringKey holds a std::string_view of it).
  template<class K, class Make>
  inline r_index
  insert_by(K const& key, Make && make)
  {
    if(is_locked_) {
      auto itor = lmap_.find(key);
      if(itor != lmap_.end()) return itor->second;
      throw rdf_exception("Accessing meta_graph to rdf index -- must use session graph instead");
    }
    if(root_mgr_p_) {
      auto itor = root_mgr_p_->lmap_.find(key);
      if(itor != root_mgr_p_->lmap_.end()) return itor->second;
    }
    auto itor = lmap_.find(key);
    if(itor != lmap_.end()) return itor->second;
    Rptr lptr = make();             // `key` may dangle from here
    r_index r = lptr.get();
    lmap_.emplace(std::move(lptr), r);
    return r;
  }

  // get_item, by value
  template<class K>
  inline r_index
  get_by(K const& key) const
  {
    if(root_mgr_p_) {
      auto itor = root_mgr_p_->lmap_.find(key);
      if(itor != root_mgr_p_->lmap_.end()) return itor->second;
    }
    auto itor = lmap_.find(key);
    if(itor != lmap_.end()) return itor->second;
    return nullptr;
  }

  inline int 
  get_next_key()
  {
    return ++last_bnode_key_;
  }

 private:
  bool          is_locked_;
  int           last_bnode_key_;
  DataMap       lmap_;
  RManagerPtr   root_mgr_p_;
  JetsResources jets_resources_;
};

inline RManagerPtr 
create_rmanager(RManagerPtr meta_mgr = nullptr)
{
  return RManager::create(meta_mgr);
}

} // namespace jets::rdf
#endif // JETS_RDF_R_MANAGER_H
