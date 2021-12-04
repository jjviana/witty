//+build darwin freebsd

package witty

import (
	"syscall"

	"golang.org/x/sys/unix"
)

// tcSetBufParams is used by the tty driver on UNIX systems to configure the
// buffering parameters (minimum character count and minimum wait time in msec.)
// This also waits for output to drain first.
func tcSetBufParams(fd int, vMin uint8, vTime uint8) error {
	_ = syscall.SetNonblock(fd, true)
	tio, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		return err
	}
	tio.Cc[unix.VMIN] = vMin
	tio.Cc[unix.VTIME] = vTime
	if err = unix.IoctlSetTermios(fd, unix.TIOCSETAW, tio); err != nil {
		return err
	}
	return nil
}
