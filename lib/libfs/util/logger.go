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

package util

import (
	"encoding/base64"
	"encoding/binary"
	"os"
	"time"

	"github.com/cubefs/cubefs-for-android/lib/log"
	"github.com/cubefs/cubefs-for-android/lib/proto"

	"go.uber.org/zap"
)

const SUB_REQ_ID = "sub-id"

type ClientLogger struct {
	*log.Logger
	ReqId string
	subId string
}

func NewLogger(logCfg *log.Config) *ClientLogger {
	logger := log.NewLogger(logCfg)

	return &ClientLogger{
		Logger: logger,
		ReqId:  defaultGenReqId(),
	}
}

// use unique loggerId for every specific op
func CloneLogger(log *ClientLogger) *ClientLogger {
	reqId := defaultGenReqId()
	return &ClientLogger{
		ReqId:  reqId,
		Logger: log.Logger.With(zap.String(proto.ReqidKey, reqId)),
	}
}

func CloneLoggerAssignReqId(log *ClientLogger, reqId string) *ClientLogger {
	return &ClientLogger{
		ReqId:  reqId,
		Logger: log.Logger.With(zap.String(proto.ReqidKey, reqId)),
	}
}

func GenSubLogger(log *ClientLogger) *ClientLogger {
	reqId := defaultGenReqId()
	return &ClientLogger{
		ReqId:  log.ReqId,
		subId:  reqId,
		Logger: log.Logger.With(zap.String(SUB_REQ_ID, reqId)),
	}
}

func (c *ClientLogger) GetReqId() string {
	if c.subId != "" {
		return c.subId
	}

	return c.ReqId
}

var pid = uint32(os.Getpid())

func defaultGenReqId() string {
	var b [12]byte
	binary.LittleEndian.PutUint32(b[:], pid)
	binary.LittleEndian.PutUint64(b[4:], uint64(time.Now().UnixNano()))
	return base64.URLEncoding.EncodeToString(b[:])
}
