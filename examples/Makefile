.PHONY: all clean
.DEFAULT: all

N?=10000
CXX?=clang++
CXXFLAGS?=
LDFLAGS?=

SOURCES=$(shell echo */*.cpp)
TARGETS=$(SOURCES:.cpp=)

all: $(TARGETS) run

$(TARGETS): $(SOURCES)
	$(CXX) $(CXXFLAGS) $(LDFLAGS) $@.cpp -o $@

run: $(TARGETS)
	for f in $^; do echo --- $$f && time ./$$f $(N); done

clean:
	rm -f $(TARGETS)
	
