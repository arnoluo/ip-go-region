::ip2region golang maker makefile in windows
@echo off

if [%1] == [] goto:build

if %1==clean (
	call:clean
) else if %1==build (
	call:build
) else if %1==gen (
    call:gen
) else if %1==search (
    call:search
) else if %1==bench (
    call:bench
) else if %1==test (
    call:test
)
exit /b 0

:test
	go test -v ./
exit /b 0

:build
go build -o xdb_maker.exe ./maker
exit /b 0

:gen
./xdb_maker.exe gen --src=./data/ip.merge.txt --dst=./data/igr.xdb
exit /b 0

:search
./xdb_maker.exe search --db=./data/igr.xdb
exit /b 0

:bench
./xdb_maker.exe bench --db=./data/igr.xdb --src=./data/ip.merge.txt
exit /b 0

:clean
del/f/s/q xdb_maker.exe
