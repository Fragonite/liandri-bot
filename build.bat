@echo off

for /f %%i in ('git describe --dirty --broken --tags --always --abbrev^=1000 --long') do set GIT_COMMIT_INFO_LONG=%%i

for /f %%i in ('git describe --dirty --broken --tags --always') do set GIT_COMMIT_INFO_SHORT=%%i

go build -ldflags="-X 'main.gVersionLong=%GIT_COMMIT_INFO_LONG%' -X 'main.gVersionShort=%GIT_COMMIT_INFO_SHORT%'" || goto :eof

liandri-bot.exe %1 %2 %3 %4 %5 %6 %7 %8

:eof