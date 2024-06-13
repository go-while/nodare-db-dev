#!/bin/bash
. ./env.sh
go mod tidy && go get -u && go mod vendor
