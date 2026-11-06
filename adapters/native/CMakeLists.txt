cmake_minimum_required(VERSION 3.20)
project(snap7_interop_native LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

if(NOT SNAP7_ROOT)
  message(FATAL_ERROR "SNAP7_ROOT must point at a built Snap7 source tree")
endif()

find_path(SNAP7_INCLUDE_DIR
  NAMES snap7.h
  HINTS
    "${SNAP7_ROOT}/release/Wrappers/c-cpp"
    "${SNAP7_ROOT}/examples/plain-c"
    "${SNAP7_ROOT}/examples/cpp"
  REQUIRED
)

find_library(SNAP7_LIBRARY
  NAMES snap7
  HINTS
    "${SNAP7_ROOT}/build/bin/x86_64-linux"
    "${SNAP7_ROOT}/build/bin/aarch64-linux"
    "${SNAP7_LIB_DIR}"
  REQUIRED
)

find_package(nlohmann_json 3.11 REQUIRED)

add_executable(snap7-interop-native
  src/main.cpp
  src/fixture.cpp
  src/log.cpp
  src/state.cpp
)

target_include_directories(snap7-interop-native PRIVATE
  "${SNAP7_INCLUDE_DIR}"
  "${CMAKE_CURRENT_SOURCE_DIR}/src"
)

target_link_libraries(snap7-interop-native PRIVATE
  "${SNAP7_LIBRARY}"
  nlohmann_json::nlohmann_json
  pthread
)

install(TARGETS snap7-interop-native RUNTIME DESTINATION bin)
