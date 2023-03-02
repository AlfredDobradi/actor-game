protoc --go_out=. --go_opt=paths=source_relative --proto_path=. common.proto
protoc -I=. -I=$GOPATH/src --gograinv2_out=. common.proto

goimports -w .