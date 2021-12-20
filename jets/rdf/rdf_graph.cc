#include <iostream>
#include <memory>

#include <glog/logging.h>
// #include <gflags/gflags.h>

#include "jets/rdf/rdf_types.h"

// DEFINE_bool(big_menu, true, "Include 'advanced' options in the menu listing");
// DEFINE_string(languages, "english,french,german",
//               "comma-separated list of languages to offer in the 'lang' menu");

namespace jets::rdf {
BaseGraphStlPtr create_stl_base_graph(char const spin)
{
  return std::make_shared<BaseGraphStlImpl>(spin);
}

RDFGraphStlPtr create_stl_rdf_graph(RManagerStlPtr meta_mgr)
{
  if(meta_mgr) {
      return std::make_shared<RDFGraphStlImpl>(meta_mgr);
  }
  return std::make_shared<RDFGraphStlImpl>();
}

RDFSessionStlPtr create_stl_rdf_session(RDFGraphStlPtr meta_graph)
{
  return std::make_shared<RDFSessionStlImpl>(meta_graph);
}

}  // namespace jets::rdf