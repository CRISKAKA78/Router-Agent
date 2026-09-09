package enrollment

import (
	"os"
	"syscall"
	"unsafe"
)

var kernel32 = syscall.NewLazyDLL("kernel32.dll")
var lockFileEx = kernel32.NewProc("LockFileEx")
var moveFileEx = kernel32.NewProc("MoveFileExW")

func lockDirectory(path string) (*os.File, error) {
	f, e := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	var overlapped syscall.Overlapped
	ok, _, err := lockFileEx.Call(f.Fd(), 3, 0, 1, 0, uintptr(unsafe.Pointer(&overlapped)))
	if ok == 0 {
		f.Close()
		return nil, err
	}
	return f, nil
}
func replaceFile(source, target string) error {
	a, e := syscall.UTF16PtrFromString(source)
	if e != nil {
		return e
	}
	b, e := syscall.UTF16PtrFromString(target)
	if e != nil {
		return e
	}
	ok, _, err := moveFileEx.Call(uintptr(unsafe.Pointer(a)), uintptr(unsafe.Pointer(b)), 9)
	if ok == 0 {
		return err
	}
	return nil
}
