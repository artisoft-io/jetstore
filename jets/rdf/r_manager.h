#ifndef JETS_RDF_R_MANAGER_H
#define JETS_RDF_R_MANAGER_H

#include <string>
#include <memory>
#include <list>
#include <unordered_set>
#include <unordered_map>

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
      r_null_ptr_(std::make_shared<RdfAstType>(RDFNull())),
      lmap_(),
      root_mgr_p_(),
      jets_resources_()
  {}

  inline RManager(RManagerPtr root_mgr_p) 
    : is_locked_(false),
      last_bnode_key_(0),
      r_null_ptr_(std::make_shared<RdfAstType>(RDFNull())),
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
    return r_null_ptr_.get();
  }

  template<class T>
  inline r_index 
  get_literal(T v) const
  {
    Rptr lptr = mkLiteral(v);
    return get_item(lptr);
  }

  template<class T>
  inline r_index 
  create_literal(T v)
  {
    Rptr lptr = mkLiteral(v);
    return insert_item(lptr);
  }

  template<class T>
  inline r_index 
  get_resource(T v) const
  {
    Rptr lptr = mkResource(v);
    return get_item(lptr);
  }

  template<class T>
  inline r_index 
  create_resource(T v)
  {
    Rptr lptr = mkResource(v);
    return insert_item(lptr);
  }

  inline r_index 
  create_uuid_resource()
  {
    Rptr lptr = mkResource(create_uuid());
    return insert_item(lptr);
  }

  inline r_index 
  get_bnode(int v) const
  {
    Rptr lptr = mkBlankNode(v);
    return get_item(lptr);
  }

  inline r_index 
  create_bnode(int v)
  {
    Rptr lptr = mkBlankNode(v);
    return insert_item(lptr);
  }

  inline r_index 
  create_bnode()
  {
    return create_bnode(get_next_key());
  }

  inline r_index 
  insert_item(Rptr lptr)
  {
    if(is_locked_) throw rdf_exception("Accessing meta_graph to rdf index -- must use session graph instead");
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

 protected:
  inline int 
  get_next_key()
  {
    return ++last_bnode_key_;
  }

 private:
  bool          is_locked_;
  int           last_bnode_key_;
  Rptr          r_null_ptr_;
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
