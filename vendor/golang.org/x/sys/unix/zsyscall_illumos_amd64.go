// go run mksyscall_solaris.go -illumos -tags illumos,amd64 syscall_illumos.go
// Code generated by the command above; see README.md. DO NOT EDIT.

// +build illumos,amd64

package unix

import (
	"unsafe"
)

//go:cgo_import_dynamic libc_readv readv "libc.so"
//go:cgo_import_dynamic libc_preadv preadv "libc.so"
//go:cgo_import_dynamic libc_writev writev "libc.so"
//go:cgo_import_dynamic libc_pwritev pwritev "libc.so"

//go:linkname procreadv libc_readv
//go:linkname procpreadv libc_preadv
//go:linkname procwritev libc_writev
//go:linkname procpwritev libc_pwritev

var (
	procreadv,
	procpreadv,
	procwritev,
	procpwritev syscallFunc
)

// THIS FILE IS GENERATED BY THE COMMAND AT THE TOP; DO NOT EDIT

func readv(fd int, iovs []Iovec) (n int, err error) {
	var _p0 *Iovec
	if len(iovs) > 0 {
		_p0 = &iovs[0]
	}
	r0, _, e1 := sysvicall6(uintptr(unsafe.Pointer(&procreadv)), 3, uintptr(fd), uintptr(unsafe.Pointer(_p0)), uintptr(len(iovs)), 0, 0, 0)
	n = int(r0)
	if e1 != 0 {
		err = e1
	}
	return
}

// THIS FILE IS GENERATED BY THE COMMAND AT THE TOP; DO NOT EDIT

func preadv(fd int, iovs []Iovec, off int64) (n int, err error) {
	var _p0 *Iovec
	if len(iovs) > 0 {
		_p0 = &iovs[0]
	}
	r0, _, e1 := sysvicall6(uintptr(unsafe.Pointer(&procpreadv)), 4, uintptr(fd), uintptr(unsafe.Pointer(_p0)), uintptr(len(iovs)), uintptr(off), 0, 0)
	n = int(r0)
	if e1 != 0 {
		err = e1
	}
	return
}

// THIS FILE IS GENERATED BY THE COMMAND AT THE TOP; DO NOT EDIT

func writev(fd int, iovs []Iovec) (n int, err error) {
	var _p0 *Iovec
	if len(iovs) > 0 {
		_p0 = &iovs[0]
	}
	r0, _, e1 := sysvicall6(uintptr(unsafe.Pointer(&procwritev)), 3, uintptr(fd), uintptr(unsafe.Pointer(_p0)), uintptr(len(iovs)), 0, 0, 0)
	n = int(r0)
	if e1 != 0 {
		err = e1
	}
	return
}

// THIS FILE IS GENERATED BY THE COMMAND AT THE TOP; DO NOT EDIT

func pwritev(fd int, iovs []Iovec, off int64) (n int, err error) {
	var _p0 *Iovec
	if len(iovs) > 0 {
		_p0 = &iovs[0]
	}
	r0, _, e1 := sysvicall6(uintptr(unsafe.Pointer(&procpwritev)), 4, uintptr(fd), uintptr(unsafe.Pointer(_p0)), uintptr(len(iovs)), uintptr(off), 0, 0)
	n = int(r0)
	if e1 != 0 {
		err = e1
	}
	return
}
