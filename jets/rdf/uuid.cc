#include "../rdf/uuid.h"
namespace jets::rdf {
boost::uuids::random_generator global_uuid_generator;
std::mutex                     global_uuid_mutex;
}  // namespace jets::rdf