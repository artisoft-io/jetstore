CXX = g++
CXXFLAGS = -Wall -Wextra -O2 -fPIC -std=c++17
LDFLAGS = -shared

TARGET = libhello.so
SOURCES = hello.cpp
OBJECTS = $(SOURCES:.cpp=.o)

all: $(TARGET)

$(TARGET): $(OBJECTS)
	$(CXX) $(LDFLAGS) -o $@ $^

%.o: %.cpp
	$(CXX) $(CXXFLAGS) -c $< -o $@

clean:
	rm -f $(OBJECTS) $(TARGET)

install: $(TARGET)
	sudo cp $(TARGET) /usr/local/lib/
	sudo cp hello.h /usr/local/include/
	sudo ldconfig

.PHONY: all clean install
