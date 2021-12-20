#include <iostream>
#include <memory>

#include <glog/logging.h>
#include <gflags/gflags.h>

#include "boost/variant.hpp"

#include "rdf/rdf_types.h"

#include "jets/rete/beta_relation.h"

DEFINE_bool(big_menu, true, "Include 'advanced' options in the menu listing");
DEFINE_string(languages, "english,french,german",
              "comma-separated list of languages to offer in the 'lang' menu");

namespace jets {
    typedef rdf::BaseGraph<rdf::UMapStl, rdf::VMapStl, rdf::WSetStl, 
        rdf::BaseGraphIterator<rdf::UMapStl, rdf::VMapStl, rdf::WSetStl>> MyGraph;
class my_visitor : public boost::static_visitor<int>
{
public:
    int operator()(int i) const
    {
        return i;
    }
    
    int operator()(const std::string & str) const
    {
        return str.length();
    }
};

inline void
insert(jets::MyGraph &g, rdf::r_index  u, rdf::r_index  v, rdf::r_index  w)
{
    if (g.insert(u, v, w)) std::cout << "insterted: ("<< u << ", "<< v << ", "<< w << std::endl;
}

} // namespace jets
// MAIN
int main(int argc, char** argv) {
  google::SetVersionString("2021.0.1");
  gflags::ParseCommandLineFlags(&argc, &argv, true);
  google::InitGoogleLogging( argv[0] );

  // See if that works!
  LOG(INFO) << "GOT " << FLAGS_languages;

  // ast!!
  boost::variant< int, std::string > u("hello world");
  std::cout << u; // output: hello world

  int result = boost::apply_visitor( jets::my_visitor(), u );
  std::cout << ", which is of lenght " << result << std::endl; // output: 11 (i.e., length of "hello world")

  std::cout << "That's All Folks!" << std::endl;
  return 0;
}
