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

# The sanitizer build (rules_benchmark Phase 7, I-69), off by default:
#   -DJETS_SANITIZE=ON  compiles and links everything here, abseil included when
#                       it is built here, with AddressSanitizer (leak detection on)
#                       and UndefinedBehaviorSanitizer, every report fatal. Three
#                       UBSan checks are left out, null, nonnull-attribute and
#                       returns-nonnull-attribute: under them GCC 13 rejects a
#                       constexpr comparison in abseil's hash_policy_traits.h.
# abseil must be built with the same flags: under AddressSanitizer its headers
# change the flat tables' layout, so an uninstrumented library does not match.
#   -DJETS_ABSEIL_SOURCE=<abseil-cpp source tree>  builds that abseil here, with
#                       these flags, instead of the installed one; otherwise point
#                       CMAKE_PREFIX_PATH at an abseil installed with them.
# The gate is one command, from the repository root (jets/rete/README.md):
#   cmake -S . -B build-san -DCMAKE_BUILD_TYPE=Release -DJETS_SANITIZE=ON \
#     -DJETS_ABSEIL_SOURCE=/path/to/abseil-cpp-20260107.1 \
#     && make -C build-san -j8 jets_test && (cd build-san/jets && ctest)
option(JETS_SANITIZE "AddressSanitizer and UndefinedBehaviorSanitizer on the jets libraries and tests" OFF)
set(JETS_ABSEIL_SOURCE "" CACHE PATH "An abseil-cpp source tree to build here instead of find_package(absl)")
if(JETS_SANITIZE)
  message(STATUS "jets: AddressSanitizer and UndefinedBehaviorSanitizer on")
  set(JETS_SANITIZE_FLAGS
    -fsanitize=address,undefined
    -fno-sanitize=null,nonnull-attribute,returns-nonnull-attribute
    -fno-sanitize-recover=all
    -fno-omit-frame-pointer)
  add_compile_options(${JETS_SANITIZE_FLAGS})
  add_link_options(-fsanitize=address,undefined)
endif()

# Abseil through its CMake package, so that each library brings the abseil
# libraries it needs (absl::raw_hash_set needs hashtablez_sampler and
# raw_logging_internal, among others). Naming archives such as absl_hash by hand
# links them without those.
if(JETS_ABSEIL_SOURCE)
  message(STATUS "jets: abseil built from ${JETS_ABSEIL_SOURCE}")
  set(ABSL_PROPAGATE_CXX_STD ON)
  set(ABSL_BUILD_TESTING OFF)
  set(JETS_SAVED_BUILD_SHARED_LIBS ${BUILD_SHARED_LIBS})
  set(BUILD_SHARED_LIBS OFF)
  set(CMAKE_POSITION_INDEPENDENT_CODE ON)
  add_subdirectory(${JETS_ABSEIL_SOURCE} ${CMAKE_BINARY_DIR}/abseil EXCLUDE_FROM_ALL)
  set(BUILD_SHARED_LIBS ${JETS_SAVED_BUILD_SHARED_LIBS})
else()
  find_package(absl CONFIG REQUIRED)
endif()

# Build variants, for measuring the engine (rules_benchmark Phase 3, C5). Both
# are off by default, and a build that does not set them compiles and links with
# exactly the flags above.
#   -DJETS_MARCH=x86-64-v3  adds -march=x86-64-v3 to every C and C++ compile here
#   -DJETS_LTO=ON           link-time optimisation of objlib, jets, jets_static
#                           and jets_test (CMake's INTERPROCEDURAL_OPTIMIZATION)
# A build with either runs only on what it was built for: -march on a CPU with
# that instruction set, and an LTO jets_static only through an LTO-aware link.
set(JETS_MARCH "" CACHE STRING "Target instruction set, passed as -march=<value>; empty passes none")
option(JETS_LTO "Link-time optimisation of the jets libraries and tests" OFF)
if(JETS_MARCH)
  message(STATUS "jets: -march=${JETS_MARCH}")
  add_compile_options(-march=${JETS_MARCH})
endif()
if(JETS_LTO)
  include(CheckIPOSupported)
  check_ipo_supported(RESULT JETS_IPO_SUPPORTED OUTPUT JETS_IPO_ERROR LANGUAGES C CXX)
  if(NOT JETS_IPO_SUPPORTED)
    message(FATAL_ERROR "JETS_LTO=ON, and the toolchain does not support it: ${JETS_IPO_ERROR}")
  endif()
  message(STATUS "jets: link-time optimisation on")
endif()

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
  gflags
  glog
  absl::raw_hash_set
  absl::hash
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
  rete/lookup_rand_test.cc
  rete/lookup_row_id_test.cc
  rete/truth_test.cc
  rete/node_vertex_test.cc
  rete/rete_meta_store_test.cc
  rete/rete_session_test.cc
  rete/rete_meta_store_factory_test0.cc
  rete/expr_op_test.cc
  rete/expr_op_specialty_test.cc
  rete/expr_operator_factory_test.cc
  rete/operator_list_test.cc
  rete/cwrapper_test.cc
  rete/lookup_db_open_test.cc
  rete/meta_store_sharing_test.cc
  rete/operator_writes_graph_test.cc
  rete/no_truth_main_on_exist_test.cc
  rete/retract_recorded_consequents_test.cc
  rete/loop_through_negation_test.cc
)

target_link_libraries(jets_test
  gflags
  glog
  absl::raw_hash_set
  absl::hash
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

# operator_list_test.cc reads jets/compilerv2/compiler/operators.txt and
# jets/rete/expr_operator_factory.h from the source tree
target_compile_definitions(jets_test PRIVATE JETS_SOURCE_DIR="${CMAKE_SOURCE_DIR}")

# copy test files
add_custom_command(TARGET jets_test POST_BUILD
  COMMAND cp -r  ${CMAKE_SOURCE_DIR}/jets/rete/test_data $<TARGET_FILE_DIR:jets_test>
)

if(JETS_LTO)
  set_property(TARGET objlib ${C_LIBRARY_NAME} ${C_LIBRARY_NAME}_static jets_test
    PROPERTY INTERPROCEDURAL_OPTIMIZATION TRUE)
endif()

include(GoogleTest)
if(JETS_SANITIZE)
  # Leak detection is LeakSanitizer's default on Linux; it is named so that the
  # gate does not depend on an ASAN_OPTIONS left in the environment
  set(JETS_SANITIZE_ENV "ASAN_OPTIONS=detect_leaks=1:abort_on_error=0" "UBSAN_OPTIONS=print_stacktrace=1")
  gtest_discover_tests(jets_test PROPERTIES ENVIRONMENT "${JETS_SANITIZE_ENV}")
else()
  gtest_discover_tests(jets_test)
endif()



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
