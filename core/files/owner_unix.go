//go:build !windows

package files

import (
	"io/fs"
	"syscall"

	"github.com/pkg/sftp"
)

// owner returns the numeric owner and group of a local or SFTP file.
func owner(fi fs.FileInfo) (int, int) {
	switch st := fi.Sys().(type) {
	case *syscall.Stat_t:
		return int(st.Uid), int(st.Gid)
	case *sftp.FileStat:
		return int(st.UID), int(st.GID)
	}
	return -1, -1
}
