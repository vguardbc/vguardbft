#!/bin/bash
go clean -x -cache -testcache
go build -x -v -o vginstance ../vguard
./vginstance -h
