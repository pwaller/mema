cmake_minimum_required(VERSION 2.8)

find_package(LLVM)


include(AddLLVM)

set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -std=c++0x")

add_definitions(${LLVM_DEFINITIONS})
include_directories(${LLVM_INCLUDE_DIRS} ${CMAKE_BINARY_DIR})
link_directories(${LLVM_LIBRARY_DIRS})

#include(protobuf.cmake)
#wrap_proto(PROTO_SRC test.proto)

add_library(memartl SHARED
  mema_rtl.cpp
  lz4.c
  ${PROTO_SRC}
)

add_subdirectory(LLVMMema)
