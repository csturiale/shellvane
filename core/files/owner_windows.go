//go:build windows

package files

import (
	"io/fs"

	"github.com/pkg/sftp"
)

// owner returns the numeric owner and group of an SFTP file; local Windows
// files have none.
func owner(fi fs.FileInfo) (int, int) {
	if st, ok := fi.Sys().(*sftp.FileStat); ok {
		return int(st.UID), int(st.GID)
	}
	return -1, -1
}
