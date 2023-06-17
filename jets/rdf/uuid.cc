#include "../rdf/uuid.h"
namespace jets::rdf {
boost::uuids::random_generator    global_uuid_generator;
boost::uuids::uuid namespace_uuid = boost::uuids::string_generator()(std::getenv("JETS_DOMAIN_KEY_HASH_SEED")!=NULL ? std::getenv("JETS_DOMAIN_KEY_HASH_SEED"): "03847036-1ef4-4c24-8815-7aab5064c3ce");
std::mutex                     global_uuid_mutex;

}  // namespace jets::rdf