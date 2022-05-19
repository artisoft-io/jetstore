#include <memory>
#include <mutex>

#include "rdf_ast.h"

namespace jets::rdf {

std::mutex global_null_mutex_;
Rptr global_null_=nullptr;

Rptr mkNull()
{
  std::lock_guard<std::mutex> lock(global_null_mutex_);
  if(global_null_ == nullptr) {
    global_null_ = std::make_shared<RdfAstType>(RDFNull()); 
  }
  return global_null_; 
}

RdfAstType Null()
{
  return *mkNull();
}

r_index gnull()
{
  return mkNull().get();
}

}  // namespace jets::rdf