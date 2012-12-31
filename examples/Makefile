.PHONY: all clean
.DEFAULT: all

N?=5000
CXX?=clang++
CXXFLAGS?=
LDFLAGS?=

SOURCES=$(shell echo */*.cpp)
TARGETS=$(SOURCES:.cpp=)

all: $(TARGETS) run

run: $(TARGETS)
	for f in $^; do \
	echo --- $$f; \
	MEMA_OPTIONS=filename=$${f}.mema time ./$$f $(N); \
	done;

clean:
	rm -f $(TARGETS)
	
