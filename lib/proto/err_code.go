// Copyright 2022 The CubeFS Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package proto

import (
	"strconv"

	"github.com/pkg/errors"
)

const SUCCESS = 0

type Errno int

func (e Errno) IsSuccess() bool {
	return int(e) == SUCCESS
}

func (e Errno) IsPosixErrno() bool {
	return int(e) > E_POSXI_ERRNO_FLAG
}

func (e Errno) Equal(e1 Errno) bool {
	return e == e1
}

func ToErrno(err error) (errno Errno) {
	if err == nil {
		return SUCCESS
	}
	for tmp := err; tmp != nil; tmp = errors.Unwrap(tmp) {
		if res, ok := tmp.(Errno); ok {
			return res
		}
	}
	return E_UTIL_FAULT
}

func TryToErrno(err error) error {
	for tmp := err; tmp != nil; tmp = errors.Unwrap(tmp) {
		if res, ok := tmp.(Errno); ok {
			return res
		}
	}
	return err
}

func (e Errno) Error() string {
	switch {
	case e.IsSuccess():
		return "success"
	case e.IsPosixErrno():
		if int(e) < len(PosixErrnoMsgs) {
			return PosixErrnoMsgs[e]
		}
		return "unknown errno " + strconv.Itoa(int(e))
	default:
		return "unknown errno " + strconv.Itoa(int(e))
	}
}

//util errno
const E_UTIL_ERRNO_FLAG = -500

const (
	E_UTIL_FAULT Errno = E_UTIL_ERRNO_FLAG - iota
)

var UtilErrnoMsg = map[Errno]string{
	E_UTIL_FAULT: "util: unexpected fault happened",
}
