# ip2region golang maker makefile
all: build
.PHONY: all

SRC := ./data/ip.merge.txt
DST := ./data/igr.xdb
MAKER := xdb_maker

test:
	go test -v ./xdb

build:
	go build -o $(MAKER) ./maker

clean:
	find ./ -name $(MAKER) | xargs rm -f

gen: build
	./$(MAKER) gen --src=$(SRC) --dst=$(DST)

search: build
	./$(MAKER) search --db=$(DST)

bench: build
	./$(MAKER) bench --db=$(DST) --src=$(SRC)
