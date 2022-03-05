cmake_minimum_required(VERSION 3.5.1)
project(jetstore)

# # For pg_extension
# set(CMAKE_MODULE_PATH ${CMAKE_MODULE_PATH} ${CMAKE_SOURCE_DIR}/cmake)
# find_package(PostgreSQL REQUIRED)

set(BUILD_SHARED_LIBS ON)
#set(CMAKE_BUILD_TYPE Release)
set(CMAKE_CXX_STANDARD 20)

set(JETS_VERSION $ENV{JETS_VERSION})
if(JETS_VERSION)
    message("Building version ${JETS_VERSION}")
else()
    message("Building, no version specified")
    set(JETS_VERSION "no-version-specified")
endif()

string(TIMESTAMP JETS_VERSION_DATE UTC)

add_subdirectory(jets)
add_subdirectory(test_cgo)
