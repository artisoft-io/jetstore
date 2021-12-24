#include <iostream>
#include <memory>

#include <glog/logging.h>
// #include <gflags/gflags.h>

#include "beta_row_initializer.h"
#include "jets/rdf/rdf_types.h"
#include "jets/rete/rete_types.h"

// DEFINE_bool(big_menu, true, "Include 'advanced' options in the menu listing");
// DEFINE_string(languages, "english,french,german",
//               "comma-separated list of languages to offer in the 'lang' menu");

namespace jets::rete {

inline int
initialize_beta_row(BetaRow * beta_row, BetaRowInitializer const* beta_row_initializer, BetaRow const* parent_row, rdf::r_index const* triple)
{
  if(not beta_row or not beta_row_initializer or not parent_row or not triple) {
    LOG(ERROR) << "initialize_beta_row: A required param is null: beta_row or beta_row_initializer or parent_row or triple" ;
    return -1;
  }
  auto itor = beta_row_initializer->begin();
  auto end = beta_row_initializer->end();
  int pos = 0;
  for(; itor != end; ++itor) {
    int idx = *itor;
    if(idx & brc_parent_node) {
      beta_row->put(pos, parent_row->get(idx & brc_low_mask));
    } else {
      beta_row->put(pos, triple[idx & brc_low_mask]);
    }
  }
  return 0;
}

}  // namespace jets::rete