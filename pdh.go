package main

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	pdhDLL = windows.NewLazySystemDLL("pdh.dll")

	procPdhOpenQuery                = pdhDLL.NewProc("PdhOpenQueryW")
	procPdhCloseQuery               = pdhDLL.NewProc("PdhCloseQuery")
	procPdhAddEnglishCounter        = pdhDLL.NewProc("PdhAddEnglishCounterW")
	procPdhAddCounter               = pdhDLL.NewProc("PdhAddCounterW")
	procPdhCollectQueryData         = pdhDLL.NewProc("PdhCollectQueryData")
	procPdhGetFormattedCounterValue = pdhDLL.NewProc("PdhGetFormattedCounterValue")
	procPdhExpandWildCardPath       = pdhDLL.NewProc("PdhExpandWildCardPathW")
)

const (
	pdhFmtDouble        = 0x00000200
	pdhMoreData         = 0x800007D2
	pdhCStatusValidData = 0x00000000
	pdhCStatusNewData   = 0x00000001
)

// PdhQuery wraps a PDH query handle.
type PdhQuery struct {
	handle uintptr
}

// PdhCounter wraps a single counter handle within a query.
type PdhCounter struct {
	handle uintptr
}

// PdhOpenQuery creates a new PDH query.
func PdhOpenQuery() (*PdhQuery, error) {
	var handle uintptr
	r, _, _ := procPdhOpenQuery.Call(0, 0, uintptr(unsafe.Pointer(&handle)))
	if r != 0 {
		return nil, fmt.Errorf("PdhOpenQuery: 0x%x", r)
	}
	return &PdhQuery{handle: handle}, nil
}

// Close releases the query.
func (q *PdhQuery) Close() error {
	if q.handle == 0 {
		return nil
	}
	r, _, _ := procPdhCloseQuery.Call(q.handle)
	q.handle = 0
	if r != 0 {
		return fmt.Errorf("PdhCloseQuery: 0x%x", r)
	}
	return nil
}

// AddEnglishCounter adds a counter using the English locale path. Required
// when the counter path contains object/counter names that vary by locale
// (e.g. "Memory" / "Available MBytes" become localized on non-English Windows).
func (q *PdhQuery) AddEnglishCounter(path string) (*PdhCounter, error) {
	pathW, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	var handle uintptr
	r, _, _ := procPdhAddEnglishCounter.Call(
		q.handle,
		uintptr(unsafe.Pointer(pathW)),
		0,
		uintptr(unsafe.Pointer(&handle)),
	)
	if r != 0 {
		return nil, fmt.Errorf("PdhAddEnglishCounter %q: 0x%x", path, r)
	}
	return &PdhCounter{handle: handle}, nil
}

// AddCounter adds a counter using the localized counter path (used when the
// path was already returned by PdhExpandWildCardPath, which yields localized
// names).
func (q *PdhQuery) AddCounter(path string) (*PdhCounter, error) {
	pathW, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	var handle uintptr
	r, _, _ := procPdhAddCounter.Call(
		q.handle,
		uintptr(unsafe.Pointer(pathW)),
		0,
		uintptr(unsafe.Pointer(&handle)),
	)
	if r != 0 {
		return nil, fmt.Errorf("PdhAddCounter %q: 0x%x", path, r)
	}
	return &PdhCounter{handle: handle}, nil
}

// Collect samples all counters in the query.
func (q *PdhQuery) Collect() error {
	r, _, _ := procPdhCollectQueryData.Call(q.handle)
	if r != 0 {
		return fmt.Errorf("PdhCollectQueryData: 0x%x", r)
	}
	return nil
}

// pdhFmtCounterValue mirrors PDH_FMT_COUNTERVALUE for the DOUBLE variant.
type pdhFmtCounterValue struct {
	CStatus     uint32
	_padding    uint32
	DoubleValue float64
}

// Value returns the latest sampled value as float64.
func (c *PdhCounter) Value() (float64, error) {
	var fmtVal pdhFmtCounterValue
	r, _, _ := procPdhGetFormattedCounterValue.Call(
		c.handle,
		uintptr(pdhFmtDouble),
		0,
		uintptr(unsafe.Pointer(&fmtVal)),
	)
	if r != 0 {
		return 0, fmt.Errorf("PdhGetFormattedCounterValue: 0x%x", r)
	}
	if fmtVal.CStatus != pdhCStatusValidData && fmtVal.CStatus != pdhCStatusNewData {
		return 0, fmt.Errorf("counter status: 0x%x", fmtVal.CStatus)
	}
	return fmtVal.DoubleValue, nil
}

// PdhExpandWildCardPath expands a PDH path containing wildcards into the list
// of currently-matching concrete paths. Returns localized counter paths
// suitable for AddCounter (NOT AddEnglishCounter).
func PdhExpandWildCardPath(wildcard string) ([]string, error) {
	wildW, err := syscall.UTF16PtrFromString(wildcard)
	if err != nil {
		return nil, err
	}

	// First call with bufLen=0 to learn required buffer size.
	var bufLen uint32
	r, _, _ := procPdhExpandWildCardPath.Call(
		0,
		uintptr(unsafe.Pointer(wildW)),
		0,
		uintptr(unsafe.Pointer(&bufLen)),
		0,
	)
	if r != 0 && r != pdhMoreData {
		return nil, fmt.Errorf("PdhExpandWildCardPath sizing: 0x%x", r)
	}
	if bufLen == 0 {
		return nil, nil
	}

	buf := make([]uint16, bufLen)
	r, _, _ = procPdhExpandWildCardPath.Call(
		0,
		uintptr(unsafe.Pointer(wildW)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&bufLen)),
		0,
	)
	if r != 0 {
		return nil, fmt.Errorf("PdhExpandWildCardPath: 0x%x", r)
	}

	// Result is a multi-string: sequences of UTF-16 strings each terminated
	// by NUL, with a final empty string (double-NUL) marking the end.
	var paths []string
	start := 0
	for i := 0; i < int(bufLen); i++ {
		if buf[i] == 0 {
			if i == start {
				break // double-NUL terminator
			}
			paths = append(paths, syscall.UTF16ToString(buf[start:i]))
			start = i + 1
		}
	}
	return paths, nil
}
