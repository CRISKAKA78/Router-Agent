cmake_minimum_required(VERSION 3.10)
project(router_probe LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 11)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

find_package(Threads REQUIRED)

add_library(rmp_probe_core STATIC
    src/client.cpp
    src/frame.cpp
    src/json.cpp
    src/task.cpp
)
target_include_directories(rmp_probe_core PUBLIC include)
target_link_libraries(rmp_probe_core PUBLIC Threads::Threads)

if(CMAKE_CXX_COMPILER_ID MATCHES "GNU|Clang")
    target_compile_options(rmp_probe_core PRIVATE -Wall -Wextra -Wpedantic)
endif()

add_executable(router-probe src/main.cpp)
target_link_libraries(router-probe PRIVATE rmp_probe_core)

include(CTest)
if(BUILD_TESTING)
    add_executable(probe_tests tests/probe_tests.cpp)
    target_link_libraries(probe_tests PRIVATE rmp_probe_core)
    add_test(NAME probe_tests COMMAND probe_tests)
    set_tests_properties(probe_tests PROPERTIES TIMEOUT 30)
endif()
