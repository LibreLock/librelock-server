// Package version carries the version of the running server.
//
// Version is overwritten at link time; a plain `go build` (development) leaves the default:
//
//	go build -ldflags "-X librelock-server/version.Version=$(git describe --tags)"
package version

var Version = "dev"

type Info struct {
	Version string `json:"version"`
}

func Get() Info {
	return Info{Version: Version}
}
