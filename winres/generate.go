// Package winres carries the go-winres directives for the executable icon and
// version metadata. The generated .syso file is committed so a plain
// `go build ./...` keeps working without the tool installed.
package winres

//go:generate go tool go-winres make --in winres.json --out ../cmd/tuick-tack/rsrc --arch amd64
