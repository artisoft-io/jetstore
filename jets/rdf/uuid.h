#ifndef JETS_RDF_UUID_H
#define JETS_RDF_UUID_H

#include <string>
#include <mutex>

#include <boost/uuid/random_generator.hpp>
#include <boost/uuid/string_generator.hpp>
#include <boost/uuid/name_generator.hpp>
#include <boost/uuid/name_generator_md5.hpp>
#include <boost/uuid/uuid_io.hpp>
#include <boost/uuid/uuid.hpp>

namespace jets::rdf {
/////////////////////////////////////////////////////////////////////////////////////////
// UUID Utility
/////////////////////////////////////////////////////////////////////////////////////////
extern boost::uuids::random_generator global_uuid_generator;
extern boost::uuids::uuid namespace_uuid;
extern std::mutex global_uuid_mutex;

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

inline std::string
create_md5_uuid(std::string && name)
{
  boost::uuids::name_generator_md5  md5_uuid_generator(namespace_uuid);
  boost::uuids::name_generator_md5::result_type uuid;
  uuid = md5_uuid_generator(name);
  return boost::uuids::to_string(uuid);
}

inline std::string
create_sha1_uuid(std::string && name)
{
  boost::uuids::name_generator_sha1 sha1_uuid_generator(namespace_uuid);
  boost::uuids::name_generator_sha1::result_type uuid;
  uuid = sha1_uuid_generator(name);
  return boost::uuids::to_string(uuid);
}

} // namespace jets::rdf
#endif // JETS_RDF_UUID_H
