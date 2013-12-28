#include <stdio.h>
#include <sys/types.h>

#include "interception.h"
typedef int64_t uptr;

DECLARE_REAL(void*, malloc, uptr)
INTERCEPTOR(void*, malloc, uptr size) {
  void* addr = REAL(malloc)(size);
  printf("Called malloc: size=%ld ptr=%p\n", size, addr);
  return addr;
}

DECLARE_REAL(void, free, void*)
INTERCEPTOR(void, free, void* addr) {
  // printf("Calling free: ptr=%p\n", addr);
  REAL(free)(addr);
}

namespace {
  void __attribute__((constructor)) init() {
    INTERCEPT_FUNCTION(malloc);
    INTERCEPT_FUNCTION(free);
  }
};

