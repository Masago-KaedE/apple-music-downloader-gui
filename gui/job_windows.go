package main

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

type processJob struct {
	handle windows.Handle
}

func createProcessJob(pid int) (*processJob, error) {
	handle, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("创建 Job Object 失败: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		handle,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		windows.CloseHandle(handle)
		return nil, fmt.Errorf("配置 Job Object 失败: %w", err)
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		windows.CloseHandle(handle)
		return nil, fmt.Errorf("打开下载进程失败: %w", err)
	}
	defer windows.CloseHandle(process)
	if err := windows.AssignProcessToJobObject(handle, process); err != nil {
		windows.CloseHandle(handle)
		return nil, fmt.Errorf("绑定下载进程失败: %w", err)
	}
	return &processJob{handle: handle}, nil
}

func (j *processJob) Close() {
	if j == nil || j.handle == 0 {
		return
	}
	windows.CloseHandle(j.handle)
	j.handle = 0
}
