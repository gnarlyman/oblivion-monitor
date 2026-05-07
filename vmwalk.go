package main

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// VMStats summarizes a single sweep of a target process's virtual address
// space via VirtualQueryEx. Designed for 32-bit Oblivion.exe (LAA gives
// ~3.5GB user VA) but works against any process the caller can OpenProcess.
//
// PrivateCommitMB is the metric that signals a true memory leak when it
// trends up over time. It is the analog of VMMap's "Heap" + "Private Data"
// rows summed together — a superset of the Win32 heap because OSR-style
// allocators reserve their arena via VirtualAlloc, not HeapCreate.
//
// LargestFreeMB is the metric that predicts the next CTD on a 32-bit
// process. When it shrinks below ~100MB the next big contiguous allocation
// (a streamed mesh, a texture, an OBSE plugin buffer) will fail even though
// "Free" memory looks abundant.
//
// FreeMB - LargestFreeMB measures fragmentation directly: a process with
// 800MB free spread across many small holes is in trouble; the same 800MB
// in one contiguous hole is healthy.
type VMStats struct {
	PrivateCommitMB float64 // sum of MEM_COMMIT && MEM_PRIVATE
	MappedCommitMB  float64 // sum of MEM_COMMIT && (MEM_MAPPED || MEM_IMAGE)
	LargestFreeMB   float64 // max region size where State == MEM_FREE
	FreeMB          float64 // sum of MEM_FREE region sizes
	Regions         int     // number of regions walked
}

// Win32 memory state / type constants.
const (
	memCommit  = 0x00001000
	memFree    = 0x00010000
	memPrivate = 0x00020000
	memMapped  = 0x00040000
	memImage   = 0x01000000
)

// Process access rights. PROCESS_QUERY_LIMITED_INFORMATION (0x1000) is
// sufficient for VirtualQueryEx on Vista+ and avoids needing
// PROCESS_QUERY_INFORMATION (which can be denied for protected processes).
const processQueryLimitedInformation = 0x1000

// Lazy syscalls. We use direct syscalls rather than golang.org/x/sys
// wrappers for VirtualQueryEx because the wrapper signature has changed
// across versions; this keeps the binding stable.
var (
	kernel32           = windows.NewLazySystemDLL("kernel32.dll")
	procVirtualQuery   = kernel32.NewProc("VirtualQueryEx")
	procIsWow64Process = kernel32.NewProc("IsWow64Process")
)

// isWow64Process returns true if pid is a 32-bit process running under
// WoW64 on 64-bit Windows. Oblivion.exe is always WoW64 on a modern OS.
func isWow64Process(h windows.Handle) (bool, error) {
	var wow64 int32
	r, _, errno := procIsWow64Process.Call(uintptr(h), uintptr(unsafe.Pointer(&wow64)))
	if r == 0 {
		return false, errno
	}
	return wow64 != 0, nil
}

// memoryBasicInformation mirrors Win32 MEMORY_BASIC_INFORMATION. Field
// order and packing are critical — must match the C struct exactly.
type memoryBasicInformation struct {
	BaseAddress       uintptr
	AllocationBase    uintptr
	AllocationProtect uint32
	_pad              uint32 // pad to 8-byte boundary on x64; absent on x86
	RegionSize        uintptr
	State             uint32
	Protect           uint32
	Type              uint32
	_pad2             uint32 // pad on x64
}

// OpenProcessForVMWalk opens a process handle suitable for VirtualQueryEx
// sweeps. Caller is responsible for windows.CloseHandle on the result.
func OpenProcessForVMWalk(pid uint32) (windows.Handle, error) {
	h, err := windows.OpenProcess(processQueryLimitedInformation, false, pid)
	if err != nil {
		return 0, fmt.Errorf("OpenProcess(pid=%d): %w", pid, err)
	}
	return h, nil
}

// WalkProcessVM walks the user-mode VA of pid (via the supplied handle) and
// returns aggregate stats. Typically takes 1-50ms depending on how fragmented
// the target's address space is.
//
// Caps the walk at 4 GB for WoW64 (32-bit) targets so we don't include the
// huge 64-bit free hole above the target's address space ceiling. For native
// 64-bit targets, walks the full 128 TB user VA.
func WalkProcessVM(h windows.Handle) (VMStats, error) {
	var maxAddr uintptr
	wow64, err := isWow64Process(h)
	if err == nil && wow64 {
		// 32-bit process on 64-bit Windows: user VA ceiling is 4 GB - 64 KB
		// with LAA, 2 GB without. Walk to 4 GB; VirtualQueryEx will return
		// ERROR_INVALID_PARAMETER past the actual limit and we break.
		maxAddr = 0x100000000
	} else {
		// Native 64-bit process: 128 TB user VA on x64.
		maxAddr = 0x7FFFFFFE0000
	}

	mbiSize := unsafe.Sizeof(memoryBasicInformation{})
	var stats VMStats
	addr := uintptr(0)

	for addr < maxAddr {
		var mbi memoryBasicInformation
		ret, _, errno := procVirtualQuery.Call(
			uintptr(h),
			addr,
			uintptr(unsafe.Pointer(&mbi)),
			mbiSize,
		)
		if ret == 0 {
			// VirtualQueryEx returns 0 on failure. ERROR_INVALID_PARAMETER
			// at the end of the address space is normal; treat as terminator.
			if errno == syscall.Errno(0x57) /* ERROR_INVALID_PARAMETER */ {
				break
			}
			return stats, fmt.Errorf("VirtualQueryEx at 0x%X: %w", addr, errno)
		}
		stats.Regions++

		size := float64(mbi.RegionSize) / (1024.0 * 1024.0)
		switch mbi.State {
		case memCommit:
			if mbi.Type == memPrivate {
				stats.PrivateCommitMB += size
			} else if mbi.Type == memMapped || mbi.Type == memImage {
				stats.MappedCommitMB += size
			}
		case memFree:
			stats.FreeMB += size
			if size > stats.LargestFreeMB {
				stats.LargestFreeMB = size
			}
		}

		// Advance. RegionSize is from BaseAddress, not from addr — using
		// BaseAddress + RegionSize handles the case where addr started
		// inside a region.
		next := mbi.BaseAddress + mbi.RegionSize
		if next <= addr {
			// Defensive: prevent infinite loop on a malformed region.
			break
		}
		addr = next
	}

	return stats, nil
}
