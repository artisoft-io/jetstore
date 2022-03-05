
#include <cstdlib>
#include <stdlib.h>
#include <iostream>

#include "cwrapper.h"
#include "my_lib.h"

int my_random( )
{
	std::cout<<"HELLO from cwrapper in testc my_random ..."<<std::endl;
  return my_lib::my_random();
}

// int find_asserted(HJRETE * rete_hdl, HJR * s, HJR * p, HJR * o, HJITERATOR ** handle);
// int find_inferred(HJRETE * rete_hdl, HJR * s, HJR * p, HJR * o, HJITERATOR ** handle);
