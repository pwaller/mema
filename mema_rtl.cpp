// TODO: Take license from other project

//#include <iostream>

#include <cassert>
#include <unordered_set>

#include <stdlib.h>
#include <stdio.h>
#include <string.h>

#include <sys/time.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <unistd.h>

#include "lz4.h"

typedef int64_t uptr;

struct Flags {
  // Do nothing
  bool disable;
  // Verbosity level (0 - silent, 1 - a bit of output, 2+ - more output).
  int  verbosity;
  // If set, prints some debugging information and does additional checks.
  bool debug;
  // Disable compression (for debugging)
  bool compression;
  // File to write memory access data to
  const char* filename;
};

static Flags mema_flags;

# define GET_CALLER_PC() (uptr)__builtin_return_address(0)
# define GET_CURRENT_FRAME() (uptr)__builtin_frame_address(0)

#define GET_CALLER_PC_BP_SP \
  uptr bp = GET_CURRENT_FRAME();              \
  uptr pc = GET_CALLER_PC();                  \
  uptr local_stack;                           \
  uptr sp = (uptr)&local_stack
  
Flags *flags() {
  return &mema_flags;
}

static char *GetFlagValue(const char *env, const char *name) {
  if (env == 0)
    return 0;
  const char *pos = strstr(env, name);
  const char *end;
  if (pos == 0)
    return 0;
  pos += strlen(name);
  if (pos[0] != '=') {
    end = pos;
  } else {
    pos += 1;
    if (pos[0] == '"') {
      pos += 1;
      end = strchr(pos, '"');
    } else if (pos[0] == '\'') {
      pos += 1;
      end = strchr(pos, '\'');
    } else {
      end = strchr(pos, ' ');
    }
    if (end == 0)
      end = pos + strlen(pos);
  }
  int len = end - pos;
  char *f = (char*)malloc(len + 1);
  memcpy(f, pos, len);
  f[len] = '\0';
  return f;
}

void ParseFlag(const char *env, bool *flag, const char *name) {
  char *val = GetFlagValue(env, name);
  if (val == 0)
    return;
  if (0 == strcmp(val, "0") ||
      0 == strcmp(val, "no") ||
      0 == strcmp(val, "false"))
    *flag = false;
  if (0 == strcmp(val, "1") ||
      0 == strcmp(val, "yes") ||
      0 == strcmp(val, "true"))
    *flag = true;
  free(val);
}

void ParseFlag(const char *env, int *flag, const char *name) {
  char *val = GetFlagValue(env, name);
  if (val == 0)
    return;
  *flag = atoll(val);
  free(val);
}

void ParseFlag(const char *env, const char **flag, const char *name) {
  const char *val = GetFlagValue(env, name);
  if (val == 0)
    return;
  *flag = val;
}

static void ParseFlagsFromString(Flags *f, const char *str) {
  ParseFlag(str, &f->disable, "disable");
  ParseFlag(str, &f->verbosity, "verbosity");
  ParseFlag(str, &f->debug, "debug");
  ParseFlag(str, &f->compression, "compression");
  ParseFlag(str, &f->filename, "filename");
}

void InitializeFlags(Flags *f, const char *env) {
  memset(f, 0, sizeof(*f));

  f->disable = false;
  f->verbosity = 0;
  f->debug = false;
  f->compression = true;
  f->filename = NULL;

  // Override from command line.
  ParseFlagsFromString(f, env);
}

enum MemaRecordType {
  MEMA_ACCESS = 0,
  MEMA_FUNC_ENTER = 1,
  MEMA_FUNC_EXIT = 2
};

typedef struct {
  MemaRecordType type;
  union {
    struct {
      double time;
      uptr pc, bp, sp, addr;
      bool is_write;
    } acc;
    struct {
      uptr addr;
    } func;
  };
} MemAccess;


static const char *kMemaModuleCtorName = "mema.module_ctor";
static const char *kMemaInitName = "__mema_init";

// Constant chosen to fit inside 10MB
const unsigned int mem_accesses_bufsize = 10*1024*1024 / sizeof(MemAccess);

// TODO: think of threadsafe way of doing this.. Can haz threadlocal?
static MemAccess mem_accesses[mem_accesses_bufsize];
static MemAccess * next_free_mem_access = &mem_accesses[0];

static const MemAccess * first_mem_access = &mem_accesses[0],
                       * last_free_mem_access = 
                                        &mem_accesses[mem_accesses_bufsize - 1];

static int memaccess_fd = -1;

void __mema_empty_buffer() {
  if (memaccess_fd == -1) {
    next_free_mem_access = &mem_accesses[0];
    return;
  }

  // TODO: Could write out a summary of the addresses/pages accessed in this 
  // block
  
  const size_t n_records = next_free_mem_access - first_mem_access;
  const size_t uncompressed_size = sizeof(mem_accesses[0]) * n_records;
  
  const char* uncompressed_data = reinterpret_cast<const char*>(first_mem_access);
  
  std::unordered_set<uptr> pages;
  for (size_t i = 0; i < n_records; i++) {
    pages.insert(mem_accesses[i].acc.addr / sysconf(_SC_PAGESIZE));
  }
  
  if (flags()->compression) {
    size_t len = LZ4_compressBound(uncompressed_size);
    char * compressed = new char[len];
    size_t compressed_size = 0;
    //snappy::RawCompress
    
    compressed_size = LZ4_compress(
      uncompressed_data,
      compressed, uncompressed_size);
    
    size_t len1 = LZ4_compressBound(compressed_size);
    char * compressed1 = new char[len1];
    size_t compressed_size1 = 0;
    compressed_size1 = LZ4_compress(compressed, compressed1, compressed_size);

    uptr r = write(memaccess_fd, 
      reinterpret_cast<void *>(&compressed_size1),
      sizeof(compressed_size));
                   
    uptr r1 = write(memaccess_fd, compressed1, compressed_size1);
    
    //Report("memaccess: Compressed size: %d (max %d) %p %p\n", compressed_size, len, r, r1);
    if ((size_t)r1 != compressed_size1) {
      printf("Failure: %zd != %zd\n", r1, compressed_size1);
    }
    
    assert((size_t)r1 == compressed_size1);
    //PrintBytes("  ", (uptr*)(compressed+0*kWordSize));
    //PrintBytes("  ", (uptr*)(compressed+1*kWordSize));
    //PrintBytes("  ", (uptr*)(compressed+2*kWordSize));
    delete compressed1;
    delete compressed;
    
    if (flags()->debug && flags()->verbosity > 0)              
      printf("memaccess: Emptying memaccess buffer, uncompressed = %zd compressed = %zd compressed1 = %zd\n", 
             uncompressed_size, compressed_size, compressed_size1);
  } else {
    
    uptr r = write(memaccess_fd, 
      reinterpret_cast<const void *>(&uncompressed_size),
      sizeof(uncompressed_size));
                   
    uptr r1 = write(memaccess_fd, uncompressed_data, uncompressed_size);
      printf("memaccess: Emptying memaccess buffer, uncompressed = %zd\n", 
             uncompressed_size);
    
  }
  
    
  next_free_mem_access = &mem_accesses[0];
}

extern "C" {

void __mema_function_entry(uptr addr) {
  if (flags()->disable) return;
  
  GET_CALLER_PC_BP_SP;
  
  MemAccess & f = *(next_free_mem_access++);
  f.type = MEMA_FUNC_ENTER;
  f.acc.pc = pc; f.acc.bp = bp; f.acc.sp = sp;
  f.acc.addr = addr;
  
  // Round-robbin buffer
  if (next_free_mem_access == last_free_mem_access) {
      __mema_empty_buffer();
  }
}

void __mema_function_exit(uptr addr) {
  if (flags()->disable) return;
  
  GET_CALLER_PC_BP_SP;
  
  MemAccess & f = *(next_free_mem_access++);
  f.type = MEMA_FUNC_EXIT;
  f.func.addr = addr;
  
  // Round-robbin buffer
  if (next_free_mem_access == last_free_mem_access) {
      __mema_empty_buffer();
  }
}

void __mema_enable()  { if (flags()->verbosity) printf("__mema_enable()\n");  flags()->disable = false; }
void __mema_disable() { if (flags()->verbosity) printf("__mema_disable()\n"); flags()->disable = true ; }

void __mema_access(uptr addr, char size, bool is_write) {
  if (flags()->disable) return;
  // TODO: Size?
  GET_CALLER_PC_BP_SP;
  
  struct timeval tv;
  gettimeofday(&tv, NULL);
  if (flags()->disable || !flags()->filename) return;
  
  MemAccess & f = *(next_free_mem_access++);
  f.type = MEMA_ACCESS;
  f.acc.time = tv.tv_sec + (0.000001 * tv.tv_usec);
  f.acc.pc = pc; f.acc.bp = bp; f.acc.sp = sp;
  f.acc.addr = addr;
  f.acc.is_write = is_write;
  
  // Round-robbin buffer
  if (next_free_mem_access == last_free_mem_access) {
    __mema_empty_buffer();
  }
}


void __mema_finalize() {
  //if (flags()->disable) return;
  printf("mema_finalize()\n");
  __mema_empty_buffer();
  if (memaccess_fd != -1)
    close(memaccess_fd);
}

static bool mema_initialized = false;

void __mema_initialize() {
  //std::cout << "mema_initialize()" << std::endl;
  if (mema_initialized) return;
  mema_initialized = true;
  printf("mema_initialize()\n");
  
  // Initialize flags.
  const char *options = getenv("MEMA_OPTIONS");
  InitializeFlags(flags(), options);
    
  if (!flags()->filename) {
    printf("memaccess_filename not set\n");
    return;
  }
  
  memaccess_fd = open(flags()->filename, O_WRONLY | O_CREAT | O_TRUNC, 0666);
  
  write(memaccess_fd, "MEMACCES", 8); // magic bytes
  
  printf("Will write memaccess data to %s..\n", flags()->filename);
  
  int maps_fd = open("/proc/self/maps", false);
  int bytes_read = 0;
  
  char buf[4096] = {0};
  
  do {
    bytes_read = read(maps_fd, buf, sizeof(buf));
    if (bytes_read > 0)
      write(memaccess_fd, buf, bytes_read);    
  } while (bytes_read > 0);  
  
  write(memaccess_fd, "\0", 1);    
    atexit(__mema_finalize);
}

}

// Taken from ASAN
#if defined(MEMA_USE_PREINIT_ARRAY)
  // On Linux, we force __mema_init to be called before anyone else
  // by placing it into .preinit_array section.
  // FIXME: do we have anything like this on Mac?
  __attribute__((section(".preinit_array")))
    typeof(__mema_initialize) *__mema_preinit =__mema_initialize;
#elif defined(_WIN32) && defined(_DLL)
  // On Windows, when using dynamic CRT (/MD), we can put a pointer
  // to __mema_init into the global list of C initializers.
  // See crt0dat.c in the CRT sources for the details.
  #pragma section(".CRT$XIB", long, read)  // NOLINT
  __declspec(allocate(".CRT$XIB")) void (*__mema_preinit)() = __mema_initialize;
#endif
typedef int64_t uptr;
