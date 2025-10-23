#include "hello.h"
#include <string>
#include <cstring>
#include <cstdlib>

extern "C" {
    char* hello_cpp(const char* name) {
        std::string greeting = "Hello from C++, " + std::string(name) + "! 🚀";
        
        char* result = (char*)malloc(greeting.length() + 1);
        if (result) {
            strcpy(result, greeting.c_str());
        }
        return result;
    }
    
    void free_hello_result(char* result) {
        if (result) {
            free(result);
        }
    }
}
