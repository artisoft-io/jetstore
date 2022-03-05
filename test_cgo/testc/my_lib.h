#ifndef JETS_RDF_UUID_H
#define JETS_RDF_UUID_H

#include <string>
#include <mutex>

#include <boost/uuid/random_generator.hpp>
#include <boost/uuid/uuid_io.hpp>
#include <boost/uuid/uuid.hpp>

namespace my_lib {
/////////////////////////////////////////////////////////////////////////////////////////
// UUID Utility
/////////////////////////////////////////////////////////////////////////////////////////
extern boost::uuids::random_generator global_uuid_generator;
extern std::mutex                     global_uuid_mutex;

inline std::string
create_uuid()
{
  boost::uuids::random_generator::result_type uuid;
  {
    std::lock_guard<std::mutex> lock(global_uuid_mutex);
    uuid = global_uuid_generator();
  }
  return boost::uuids::to_string(uuid);
}

int my_random();


} // namespace jets::rdf
#endif // JETS_RDF_UUID_H
