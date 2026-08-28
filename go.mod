module github.com/help660vip/LitePayloadDumper

go 1.20

require (
	github.com/ssut/payload-dumper-go v0.0.0
	google.golang.org/protobuf v1.34.2
)

require (
	github.com/andybalholm/brotli v1.1.1 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/ulikunitz/xz v0.5.12 // indirect
	golang.org/x/sync v0.10.0 // indirect
)

replace github.com/ssut/payload-dumper-go => ./third_party/payload-dumper-go
