cmake_minimum_required(VERSION 3.10)

set(CMAKE_CXX_FLAGS_DEBUG "${CMAKE_CXX_FLAGS_DEBUG} -O0 -g3 -fPIC -Wall -export-dynamic -fmessage-length=0")
set(CMAKE_C_FLAGS_DEBUG "${CMAKE_C_FLAGS_DEBUG} -O0 -g3 -fPIC -Wall -fmessage-length=0")

set(CMAKE_CXX_FLAGS_RELEASE "${CMAKE_CXX_FLAGS_RELEASE} -O3 -fPIC -export-dynamic  -fmessage-length=0")
set(CMAKE_C_FLAGS_RELEASE "${CMAKE_C_FLAGS_RELEASE} -O3 -fPIC -fmessage-length=0")

# set(CMAKE_CXX_FLAGS_DEBUG "${CMAKE_CXX_FLAGS_DEBUG} -O0 -g3 -fPIC -Wall -export-dynamic -fmessage-length=0 -fno-builtin-malloc -fno-builtin-calloc -fno-builtin-realloc -fno-builtin-free")
# set(CMAKE_C_FLAGS_DEBUG "${CMAKE_C_FLAGS_DEBUG} -O0 -g3 -fPIC -Wall -fmessage-length=0 -fno-builtin-malloc -fno-builtin-calloc -fno-builtin-realloc -fno-builtin-free")

# set(CMAKE_CXX_FLAGS_RELEASE "${CMAKE_CXX_FLAGS_RELEASE} -O3 -fPIC -export-dynamic  -fmessage-length=0 -fno-builtin-malloc -fno-builtin-calloc -fno-builtin-realloc -fno-builtin-free")
# set(CMAKE_C_FLAGS_RELEASE "${CMAKE_C_FLAGS_RELEASE} -O3 -fPIC -fmessage-length=0 -fno-builtin-malloc -fno-builtin-calloc -fno-builtin-realloc -fno-builtin-free")

set(C_LIBRARY_NAME jets)

# configure a header file to pass some of the CMake settings to the source code
configure_file (
  "./version.h.in"
  "./version.h"
)

set(C_INCLUDE_PATH ${PROJECT_SOURCE_DIR}/jets)


# list of source files
set(C_LIBRARY_SRC rdf/base_graph_iterator.h  rdf/base_graph.h  rdf/containers_type.h  
  rdf/graph_callback_mgr.h  rdf/other/fcmp.c  rdf/other/fcmp.h  rdf/r_manager.cc rdf/r_manager.h rdf/r_resources.h rdf/r_resources.cc 
  rdf/rdf_date_time.h  rdf/rdf_date_time.cc  rdf/rdf_ast.h rdf/rdf_ast.cc  rdf/rdf_err.h  rdf/rdf_graph.h  rdf/rdf_session_iterator.h  
  rdf/rdf_session.h  rdf/rdf_types.h  rdf/uuid.cc  rdf/uuid.h  rdf/w_node.h 
  rete/alpha_functors.h  rete/alpha_node_impl.h  rete/alpha_node.h  
  rete/antecedent_query_visitor.h  rete/beta_relation.h  rete/beta_row_initializer.h  
  rete/beta_row_iterator.h  rete/beta_row.h  rete/expr_impl.h  rete/expr_operator_factory.h  
  rete/expr_op_arithmetics.h rete/expr_op_logics.h  rete/expr_op_others.h rete/expr_op_resources.h rete/expr_op_strings.h 
  rete/expr.h  rete/graph_callback_mgr_impl.h  rete/jets_rete_cwrapper.cpp  
  rete/jets_rete_cwrapper.h  rete/lookup_sql_helper.h  rete/lookup_sql_helper.cc  rete/node_vertex.h  rete/rete_err.h  rete/rete_meta_store_factory_helper.h  
  rete/rete_meta_store_factory.cc  rete/rete_meta_store_factory.h  rete/rete_meta_store.h  
  rete/rete_session.cc  rete/rete_session.h  rete/rete_types_impl.h  rete/rete_types.h)

# this is the "object library" target: compiles the sources only once
add_library(objlib OBJECT ${C_LIBRARY_SRC})

# shared libraries need PIC
set_property(TARGET objlib PROPERTY POSITION_INDEPENDENT_CODE 1)

# List all directories: $ find ./ -type d
target_include_directories(objlib PUBLIC
    ${CMAKE_CURRENT_BINARY_DIR}/
    ${CMAKE_CURRENT_SOURCE_DIR}/..
)

# shared and static libraries built from the same object files
add_library(${C_LIBRARY_NAME} SHARED $<TARGET_OBJECTS:objlib>)
add_library(${C_LIBRARY_NAME}_static STATIC $<TARGET_OBJECTS:objlib>)

# Linking
target_link_libraries(${C_LIBRARY_NAME}
  # jemalloc
  # boost_filesystem
  # boost_system
  gflags
  glog
  absl_hash
  absl_city
  absl_low_level_hash
  absl_raw_hash_set
  boost_date_time
  pthread
  sqlite3
)

# TESTS
enable_testing()

add_executable(jets_test
  rdf/base_graph_test.cc
  rdf/rdf_ast_test.cc
  rdf/r_manager_test.cc
  rdf/rdf_graph_test.cc
  rdf/rdf_session_test.cc
  rete/expr_test.cc
  # rete/lookup_sql_helper_test.cc
  rete/node_vertex_test.cc
  rete/rete_meta_store_test.cc
  rete/rete_session_test.cc
  # # rete/rete_meta_store_factory_test.cc
  rete/expr_op_test.cc
  rete/expr_op_specialty_test.cc
)

target_link_libraries(jets_test
  gflags
  glog
  absl_hash
  absl_city
  absl_low_level_hash
  absl_raw_hash_set
  boost_date_time
  gtest
  gtest_main
  pthread
  sqlite3
  ${C_LIBRARY_NAME}_static
)

target_include_directories(jets_test PUBLIC
    ${CMAKE_CURRENT_BINARY_DIR}/
    ${CMAKE_CURRENT_SOURCE_DIR}/..
)

# copy test files
add_custom_command(TARGET jets_test POST_BUILD
  COMMAND cp -r  ${CMAKE_SOURCE_DIR}/jets/rete/test_data $<TARGET_FILE_DIR:jets_test>
)

include(GoogleTest)
gtest_discover_tests(jets_test)



install(TARGETS ${C_LIBRARY_NAME}
        DESTINATION lib)

# find all header files: $ find ./ -type f -name "*.h"
# find directories: $ find ./ -type d -print
install(DIRECTORY
        .
        DESTINATION include/jets
        FILES_MATCHING PATTERN "*.h"
)

# GO Language Section
# set(INFILE ${PROJECT_SOURCE_DIR}/jets/bridge/bridge.go.in)
# set(OUTFILE ${PROJECT_BINARY_DIR}/jets/bridge/bridge.go)
set(C_LIBRARY_PATH ${PROJECT_BINARY_DIR}/jets)

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
set(GOPATH ${PROJECT_SOURCE_DIR})
set(GOBIN "${PROJECT_BINARY_DIR}/jets")

message(STATUS "GOPATH=${GOPATH}")
message(STATUS "GOBIN=${GOBIN}")
message(STATUS "executable file: ${GOBIN}/${CMAKE_PROJECT_NAME}")
