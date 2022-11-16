# ip2region golang maker makefile
all: build
.PHONY: all

test:
	go test -v ./...


build:
	go build -o xdb_maker ./maker
gen:
	./xdb_maker gen --src=./data/ip.merge.txt --dst=./data/igr.xdb
search:
	./xdb_maker search --db=./data/igr.xdb
bench:
	./xdb_maker bench --db=./data/igr.xdb --src=./data/ip.merge.txt
clean:
	find ./ -name xdb_maker | xargs rm -f
