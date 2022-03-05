
#include <iostream>
#include "my_lib.h"

namespace my_lib {
boost::uuids::random_generator global_uuid_generator;
std::mutex                     global_uuid_mutex;

int my_random( )
{
	std::cout<<"OK my_lib::my_random called!!"<<std::endl;
  return random();
}

}  // namespace my_lib