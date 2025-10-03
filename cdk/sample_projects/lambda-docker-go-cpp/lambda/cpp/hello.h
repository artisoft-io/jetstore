#ifndef HELLO_H
#define HELLO_H

#ifdef __cplusplus
extern "C" {
#endif

char* hello_cpp(const char* name);
void free_hello_result(char* result);

#ifdef __cplusplus
}
#endif

#endif // HELLO_H
