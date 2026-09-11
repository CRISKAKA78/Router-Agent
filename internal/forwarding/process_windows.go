package forwarding

import (
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"
)

func configureProcess(c *exec.Cmd) { c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }

// Kill-on-close Job keeps GOST from surviving a crashed Windows management Server.
func ownProcess(c *exec.Cmd) (func(), error) {
	kernel := syscall.NewLazyDLL("kernel32.dll")
	create := kernel.NewProc("CreateJobObjectW")
	set := kernel.NewProc("SetInformationJobObject")
	assign := kernel.NewProc("AssignProcessToJobObject")
	job, _, err := create.Call(0, 0)
	if job == 0 {
		return nil, err
	}
	closeJob := func() { syscall.CloseHandle(syscall.Handle(job)) }
	type basic struct {
		ProcessTime, JobTime int64
		Flags                uint32
		Min, Max             uintptr
		Active               uint32
		Affinity             uintptr
		Priority, Scheduling uint32
	}
	type extended struct {
		Basic                                                      basic
		IO                                                         [6]uint64
		ProcessMemory, JobMemory, PeakProcessMemory, PeakJobMemory uintptr
	}
	limits := extended{}
	limits.Basic.Flags = 0x2000
	ok, _, e := set.Call(job, 9, uintptr(unsafe.Pointer(&limits)), unsafe.Sizeof(limits))
	if ok == 0 {
		closeJob()
		return nil, e
	}
	process, e := syscall.OpenProcess(0x0100|0x0001, false, uint32(c.Process.Pid))
	if e != nil {
		closeJob()
		return nil, e
	}
	defer syscall.CloseHandle(process)
	ok, _, e = assign.Call(job, uintptr(process))
	if ok == 0 {
		closeJob()
		return nil, fmt.Errorf("assign GOST job: %w", e)
	}
	return closeJob, nil
}
