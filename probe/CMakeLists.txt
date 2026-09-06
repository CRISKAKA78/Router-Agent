cmake_minimum_required(VERSION 3.10)
project(router_probe LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 11)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

find_package(Threads REQUIRED)

add_library(rmp_probe_core STATIC
    src/client.cpp
    src/tunnel.cpp
    src/frame.cpp
    src/json.cpp
    src/task.cpp
    src/file_wire.cpp
    src/file_manager.cpp
    src/sha256.cpp
    src/task_manager.cpp
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
    add_executable(tunnel_tests tests/tunnel_tests.cpp)
    target_link_libraries(tunnel_tests PRIVATE rmp_probe_core)
    add_test(NAME tunnel_tests COMMAND tunnel_tests)
    set_tests_properties(tunnel_tests PROPERTIES TIMEOUT 10)
    add_executable(file_tests tests/file_tests.cpp)
    target_link_libraries(file_tests PRIVATE rmp_probe_core)
    add_test(NAME file_tests COMMAND file_tests)
    set_tests_properties(file_tests PROPERTIES TIMEOUT 30)
    add_executable(probe_tests tests/probe_tests.cpp)
    target_link_libraries(probe_tests PRIVATE rmp_probe_core)
    add_test(NAME probe_tests COMMAND probe_tests)
    set_tests_properties(probe_tests PROPERTIES TIMEOUT 30)
    add_executable(task_manager_tests tests/task_manager_tests.cpp)
    target_link_libraries(task_manager_tests PRIVATE rmp_probe_core)
    add_test(NAME task_manager_tests COMMAND task_manager_tests)
    set_tests_properties(task_manager_tests PROPERTIES TIMEOUT 30)
endif()
