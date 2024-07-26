#!/bin/bash

if [ ! -x bin/xc ] ; then
    mkdir -p bin
    go build -o bin/xc github.com/joerdav/xc/cmd/xc
fi
bin/xc "$@"
