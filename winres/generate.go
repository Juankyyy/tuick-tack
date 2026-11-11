// Package winres carries the go-winres directives for the executable icon and
// version metadata. The generated .syso files are committed so a plain
// `go build ./...` keeps working without the tool installed.
package winres

//go:generate go tool go-winres make --in winres.json --out ../cmd/tuick-tack/rsrc --arch amd64
//go:generate go tool go-winres make --in winres-tray.json --out ../cmd/tuick-tack-tray/rsrc --arch amd64
