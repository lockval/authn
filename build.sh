#!/bin/bash

build(){
    echo "build $*"

    GOOS=${2} GOARCH=${1} go build -trimpath -o ../lockv/3rd/dist/${1}/${2}/login${3}   ./internal/login
    GOOS=${2} GOARCH=${1} go build -trimpath -o ../lockv/3rd/dist/${1}/${2}/guest${3} ./internal/guest

}

build arm64 linux
build amd64 linux
build arm64 darwin
build amd64 darwin
build arm64 windows .exe
build amd64 windows .exe

