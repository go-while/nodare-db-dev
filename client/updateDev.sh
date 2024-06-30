#!/bin/bash
test -z "$1" && echo "usage: $0 commit" && exit 1

go get github.com/go-while/nodare-db-dev@$1 && \
go get github.com/go-while/nodare-db-dev/server@$1 && \
go get github.com/go-while/nodare-db-dev/logger@$1 && \
go mod tidy && go mod vendor
