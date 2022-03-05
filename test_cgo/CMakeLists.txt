cmake_minimum_required(VERSION 3.10)

set(CMAKE_CXX_FLAGS_DEBUG "${CMAKE_CXX_FLAGS_DEBUG} -O0 -g3 -fPIC -Wall -export-dynamic -fmessage-length=0")
set(CMAKE_C_FLAGS_DEBUG "${CMAKE_C_FLAGS_DEBUG} -O0 -g3 -fPIC -Wall -fmessage-length=0")

set(CMAKE_CXX_FLAGS_RELEASE "${CMAKE_CXX_FLAGS_RELEASE} -O3 -fPIC -export-dynamic  -fmessage-length=0")
set(CMAKE_C_FLAGS_RELEASE "${CMAKE_C_FLAGS_RELEASE} -O3 -fPIC -fmessage-length=0")

# set(CMAKE_CXX_FLAGS_DEBUG "${CMAKE_CXX_FLAGS_DEBUG} -O0 -g3 -fPIC -Wall -export-dynamic -fmessage-length=0 -fno-builtin-malloc -fno-builtin-calloc -fno-builtin-realloc -fno-builtin-free")
# set(CMAKE_C_FLAGS_DEBUG "${CMAKE_C_FLAGS_DEBUG} -O0 -g3 -fPIC -Wall -fmessage-length=0 -fno-builtin-malloc -fno-builtin-calloc -fno-builtin-realloc -fno-builtin-free")

# set(CMAKE_CXX_FLAGS_RELEASE "${CMAKE_CXX_FLAGS_RELEASE} -O3 -fPIC -export-dynamic  -fmessage-length=0 -fno-builtin-malloc -fno-builtin-calloc -fno-builtin-realloc -fno-builtin-free")
# set(CMAKE_C_FLAGS_RELEASE "${CMAKE_C_FLAGS_RELEASE} -O3 -fPIC -fmessage-length=0 -fno-builtin-malloc -fno-builtin-calloc -fno-builtin-realloc -fno-builtin-free")

set(C_LIBRARY_NAME testc)

# configure a header file to pass some of the CMake settings to the source code
# configure_file (
#   "./version.h.in"
#   "./version.h"
# )

set(C_INCLUDE_PATH ${PROJECT_SOURCE_DIR}/test_cgo)


# list of source files
set(C_LIBRARY_SRC testc/my_lib.h testc/my_lib.cc testc/cwrapper.h testc/cwrapper.cc )

# this is the "object library" target: compiles the sources only once
add_library(objtestclib OBJECT ${C_LIBRARY_SRC})

# shared libraries need PIC
set_property(TARGET objtestclib PROPERTY POSITION_INDEPENDENT_CODE 1)

# List all directories: $ find ./ -type d
target_include_directories(objtestclib PUBLIC
    ${CMAKE_CURRENT_BINARY_DIR}/
    ${CMAKE_CURRENT_SOURCE_DIR}/..
)

# shared and static libraries built from the same object files
add_library(${C_LIBRARY_NAME} SHARED $<TARGET_OBJECTS:objtestclib>)
add_library(${C_LIBRARY_NAME}_static STATIC $<TARGET_OBJECTS:objtestclib>)

# Linking
target_link_libraries(${C_LIBRARY_NAME}
  # jemalloc
  # boost_filesystem
  # boost_regex
  # boost_system
  # boost_program_options
  # boost_log
  gflags
  glog
  absl_hash
  jets
)
target_link_libraries(${C_LIBRARY_NAME}_static
  # jemalloc
  # boost_filesystem
  # boost_regex
  # boost_system
  # boost_program_options
  # boost_log
  gflags
  glog
  absl_hash
  jets_static
)

# # TESTS
# enable_testing()

# add_executable(
#   jets_test
#   rdf/base_graph_test.cc
#   rdf/rdf_ast_test.cc
#   rdf/r_manager_test.cc
#   rdf/rdf_graph_test.cc
#   rdf/rdf_session_test.cc
#   rete/expr_test.cc
#   rete/node_vertex_test.cc
#   rete/rete_meta_store_test.cc
#   rete/rete_session_test.cc
#   rete/rete_meta_store_factory_test.cc
# )

# target_link_libraries(
#   jets_test
#   gflags
#   glog
#   absl_hash
#   absl_city
#   absl_low_level_hash
#   absl_raw_hash_set
#   gtest
#   gtest_main
#   pthread
#   sqlite3
#   ${C_LIBRARY_NAME}_static
# )

# target_include_directories(jets_test PUBLIC
#     ${CMAKE_CURRENT_BINARY_DIR}/
#     ${CMAKE_CURRENT_SOURCE_DIR}/..
# )

# # copy test files
# add_custom_command(TARGET jets_test POST_BUILD
#   COMMAND cp -r  ${CMAKE_SOURCE_DIR}/test_cgo/test_data $<TARGET_FILE_DIR:cgo_test_data>
# )

# include(GoogleTest)
# gtest_discover_tests(jets_test)



# install(TARGETS ${C_LIBRARY_NAME}
#         DESTINATION lib)

# # find all header files: $ find ./ -type f -name "*.h"
# # find directories: $ find ./ -type d -print
# install(DIRECTORY
#         .
#         DESTINATION include/jets
#         FILES_MATCHING PATTERN "*.h"
# )

# GO Language Section
# set(INFILE ${PROJECT_SOURCE_DIR}/jets/bridge/bridge.go.in)
# set(OUTFILE ${PROJECT_BINARY_DIR}/jets/bridge/bridge.go)
# set(C_LIBRARY_PATH ${PROJECT_BINARY_DIR}/jets)

set(CFLAGS "-I${C_INCLUDE_PATH}")
set(LDFLAGS "-L${C_LIBRARY_PATH} -l${C_LIBRARY_NAME}_static")

# message(STATUS "Creating .go bridge file")
# message(STATUS "Input template:${INFILE}" )
# message(STATUS "Output file   :${OUTFILE}")
# message(STATUS "Output file CFLAGS : ${CFLAGS}")
# message(STATUS "Output file LDFLAGS: ${LDFLAGS}")

# configure_file( ${INFILE} ${OUTFILE} )


# Call 'go build' to generate executable file
# GOPATH / GOBIN
# see https://golang.org/doc/articles/go_command.html
# note: order matters... binary must come first so bridge.go can be found.
# set(GOPATH ${PROJECT_BINARY_DIR}:${PROJECT_SOURCE_DIR})
# set(GOPATH ${PROJECT_SOURCE_DIR})
# set(GOBIN "${PROJECT_BINARY_DIR}/jets")

# message(STATUS "GOPATH=${GOPATH}")
# message(STATUS "GOBIN=${GOBIN}")
# message(STATUS "executable file: ${GOBIN}/${CMAKE_PROJECT_NAME}")
