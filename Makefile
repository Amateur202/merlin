.PHONY: all build clean

all: build

build:
	cd src && go build -o merlin

clean:
	rm -f src/merlin
	rm -f *.c
	rm -f examples/*.c
