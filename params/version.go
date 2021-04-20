package params

import (
	"fmt"
)

// version parts
const (
	VersionMajor = 0  // Major version component of the current release
	VersionMinor = 1  // Minor version component of the current release
	VersionPatch = 0  // Patch version component of the current release
	VersionMeta  = "" // Version metadata to append to the version string
)

// Version holds the textual version string.
var Version = func() string {
	v := fmt.Sprintf("%d.%d.%d", VersionMajor, VersionMinor, VersionPatch)
	if VersionMeta != "" {
		v += "-" + VersionMeta
	}
	return v
}()
