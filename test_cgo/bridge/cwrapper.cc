
#include <cstdlib>
#include <stdlib.h>
#include <iostream>

#include "cwrapper.h"
// #include "my_lib.h"

int my_random2( )
{
	std::cout<<"HELLO2 from cwrapper my_random ..."<<std::endl;
  // return my_lib::my_random();
  return 10;
}

// int find_asserted(HJRETE * rete_hdl, HJR * s, HJR * p, HJR * o, HJITERATOR ** handle);
// int find_inferred(HJRETE * rete_hdl, HJR * s, HJR * p, HJR * o, HJITERATOR ** handle);
