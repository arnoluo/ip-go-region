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
gen:
	./$(MAKER) gen --src=$(SRC) --dst=$(DST)
search:
	./$(MAKER) search --db=$(DST)
bench:
	./$(MAKER) bench --db=$(DST) --src=$(SRC)
clean:
	find ./ -name $(MAKER) | xargs rm -f
