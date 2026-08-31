//go:build windows

package ingestion

import (
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type parserProcessController interface {
	terminate() error
	close() error
}

type windowsParserProcessController struct {
	job windows.Handle
}

func configureParserProcess(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
	return nil
}

func attachParserProcess(cmd *exec.Cmd) (parserProcessController, error) {
	return attachKillOnCloseJob(cmd.Process.Pid)
}

func attachKillOnCloseJob(pid int) (parserProcessController, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	defer windows.CloseHandle(process)
	if err := windows.AssignProcessToJobObject(job, process); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	return windowsParserProcessController{job: job}, nil
}

func (p windowsParserProcessController) terminate() error {
	return windows.TerminateJobObject(p.job, 1)
}

func (p windowsParserProcessController) close() error {
	return windows.CloseHandle(p.job)
}
