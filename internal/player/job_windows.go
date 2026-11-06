//go:build windows

package player

import (
	"os"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	killJobOnce sync.Once
	killJob     windows.Handle
	killJobErr  error
)

// newKillOnCloseJob creates a job object whose members are terminated when the
// last handle to the job is closed. This process keeps that handle for its
// whole lifetime, so mpv is killed even when the TUI is force-closed or
// crashes and defer-based cleanup never runs.
func newKillOnCloseJob() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		_ = windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
}

func assignToJob(job windows.Handle, proc *os.Process) error {
	h, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false,
		uint32(proc.Pid),
	)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	return windows.AssignProcessToJobObject(job, h)
}

// assignToKillJob registers a child process with the process-lifetime job.
func assignToKillJob(proc *os.Process) error {
	if proc == nil {
		return nil
	}
	killJobOnce.Do(func() {
		killJob, killJobErr = newKillOnCloseJob()
	})
	if killJobErr != nil {
		return killJobErr
	}
	return assignToJob(killJob, proc)
}
