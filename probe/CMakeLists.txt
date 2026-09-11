cmake_minimum_required(VERSION 3.10)
project(router_probe LANGUAGES C CXX)

set(CMAKE_CXX_STANDARD 11)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

find_package(Threads REQUIRED)

file(GLOB RMP_TLS_SOURCES third_party/mbedtls/library/*.c)
add_library(rmp_tls STATIC ${RMP_TLS_SOURCES})
set_target_properties(rmp_tls PROPERTIES C_STANDARD 99 C_STANDARD_REQUIRED ON)
target_include_directories(rmp_tls PUBLIC third_party/mbedtls/include include)
target_compile_definitions(rmp_tls PUBLIC MBEDTLS_CONFIG_FILE="rmp/tls_config.h")

add_library(rmp_probe_core STATIC
    src/client.cpp
    src/system_info.cpp
    src/telemetry.cpp
    src/neighbors.cpp
    src/egress.cpp
    src/live_config.cpp
    src/switch_probe.cpp
    src/port_counters.cpp
    src/identity.cpp
    src/collection.cpp
    src/forwarding.cpp
    src/tunnel.cpp
    src/frame.cpp
    src/json.cpp
    src/task.cpp
    src/router_config.cpp
    src/file_wire.cpp
    src/file_manager.cpp
    src/sha256.cpp
    src/task_manager.cpp
)
target_include_directories(rmp_probe_core PUBLIC include)
target_link_libraries(rmp_probe_core PUBLIC Threads::Threads rmp_tls)

if(CMAKE_CXX_COMPILER_ID MATCHES "GNU|Clang")
    target_compile_options(rmp_probe_core PRIVATE -Wall -Wextra -Wpedantic)
endif()

add_executable(router-probe src/main.cpp)
target_link_libraries(router-probe PRIVATE rmp_probe_core)
set(RMP_NETWORK_INTERFACES "br0,eth0,eth1,usb0" CACHE STRING "Default exact network interface names, comma separated")
if(NOT RMP_NETWORK_INTERFACES MATCHES "^[A-Za-z0-9_,.-]*$")
    message(FATAL_ERROR "Invalid default interface names")
endif()
target_compile_definitions(router-probe PRIVATE RMP_NETWORK_INTERFACES="${RMP_NETWORK_INTERFACES}")

include(CTest)
if(BUILD_TESTING)
    add_executable(egress_tests tests/egress_tests.cpp)
    target_link_libraries(egress_tests PRIVATE rmp_probe_core)
    add_test(NAME egress_tests COMMAND egress_tests)
    add_executable(port_counter_tests tests/port_counter_tests.cpp)
    target_link_libraries(port_counter_tests PRIVATE rmp_probe_core)
    add_test(NAME port_counter_tests COMMAND port_counter_tests)
    add_executable(monitoring_v2_tests tests/monitoring_v2_tests.cpp)
    target_link_libraries(monitoring_v2_tests PRIVATE rmp_probe_core)
    add_test(NAME monitoring_v2_tests COMMAND monitoring_v2_tests)
    add_executable(managed_config_tests tests/managed_config_tests.cpp)
    target_link_libraries(managed_config_tests PRIVATE rmp_probe_core)
    add_test(NAME managed_config_tests COMMAND managed_config_tests)
    add_executable(telemetry_tests tests/telemetry_tests.cpp)
    target_link_libraries(telemetry_tests PRIVATE rmp_probe_core)
    add_test(NAME telemetry_tests COMMAND telemetry_tests)
    add_executable(system_info_tests tests/system_info_tests.cpp)
    target_link_libraries(system_info_tests PRIVATE rmp_probe_core)
    add_test(NAME system_info_tests COMMAND system_info_tests)
    add_executable(router_config_tests tests/router_config_tests.cpp)
    target_link_libraries(router_config_tests PRIVATE rmp_probe_core)
    add_test(NAME router_config_tests COMMAND router_config_tests)
    set_tests_properties(router_config_tests PROPERTIES TIMEOUT 25)
    add_executable(collection_tests tests/collection_tests.cpp)
    target_link_libraries(collection_tests PRIVATE rmp_probe_core)
    add_test(NAME collection_tests COMMAND collection_tests)
    set_tests_properties(collection_tests PROPERTIES TIMEOUT 15)
    add_executable(identity_tests tests/identity_tests.cpp)
    target_link_libraries(identity_tests PRIVATE rmp_probe_core)
    add_test(NAME identity_tests COMMAND identity_tests)
    set_tests_properties(identity_tests PROPERTIES TIMEOUT 15)
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

add_executable(switch_probe_tests tests/switch_probe_tests.cpp)
target_link_libraries(switch_probe_tests rmp_probe_core)
add_test(NAME switch_probe_tests COMMAND switch_probe_tests)

add_executable(neighbor_tests tests/neighbor_tests.cpp)
target_link_libraries(neighbor_tests rmp_probe_core)
add_test(NAME neighbor_tests COMMAND neighbor_tests)
set_tests_properties(neighbor_tests PROPERTIES TIMEOUT 15)
