cmake_minimum_required(VERSION 3.15)
project(metrics_daemon LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_EXPORT_COMPILE_COMMANDS ON)

set(gRPC_DIR "/usr/lib/cmake/grpc")

find_package(protobuf CONFIG REQUIRED)
find_package(gRPC CONFIG REQUIRED)
find_program(gRPC_CPP_PLUGIN_EXECUTABLE grpc_cpp_plugin)

set(PROTO_FILE "${CMAKE_CURRENT_SOURCE_DIR}/protobuf/metrics.proto")
set(PROTO_SRC "${CMAKE_CURRENT_BINARY_DIR}/metrics.pb.cc")
set(PROTO_HDR "${CMAKE_CURRENT_BINARY_DIR}/metrics.pb.h")
set(GRPC_SRC "${CMAKE_CURRENT_BINARY_DIR}/metrics.grpc.pb.cc")
set(GRPC_HDR "${CMAKE_CURRENT_BINARY_DIR}/metrics.grpc.pb.h")

add_custom_command(
    OUTPUT ${PROTO_SRC} ${PROTO_HDR} ${GRPC_SRC} ${GRPC_HDR}
    COMMAND protobuf::protoc
    ARGS --grpc_out="${CMAKE_CURRENT_BINARY_DIR}"
         --cpp_out="${CMAKE_CURRENT_BINARY_DIR}"
         -I "${CMAKE_CURRENT_SOURCE_DIR}/protobuf"
         --plugin=protoc-gen-grpc="${gRPC_CPP_PLUGIN_EXECUTABLE}"
         "${PROTO_FILE}"
    DEPENDS "${PROTO_FILE}"
)

add_executable(metrics_daemon
    src/main.cpp
    ${PROTO_SRC}
    ${GRPC_SRC}
)

target_include_directories(metrics_daemon PRIVATE
    ${CMAKE_CURRENT_SOURCE_DIR}/include
    ${CMAKE_CURRENT_BINARY_DIR}
)

target_link_libraries(metrics_daemon PRIVATE
    protobuf::libprotobuf
    gRPC::grpc++
)
