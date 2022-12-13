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

package http

import (
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/cubefs/cubefs-for-android/lib/libfs/conf"
	"github.com/cubefs/cubefs-for-android/lib/libfs/consts"
	"github.com/cubefs/cubefs-for-android/lib/libfs/util"
	zap "github.com/cubefs/cubefs-for-android/lib/log"
	"github.com/cubefs/cubefs-for-android/lib/proto"
)

type Transport struct {
	Transport http.RoundTripper
	user      *conf.UserInfo
	log       *util.ClientLogger
	cfg       *conf.Config
	clientID  string
}

func newTransport(user *conf.UserInfo, clientID string, log *util.ClientLogger, cfaCfg *conf.Config) *Transport {
	transport := &Transport{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
		},
		user:     user,
		clientID: clientID,
		log:      log,
		cfg:      cfaCfg,
	}

	trans, ok := transport.Transport.(*http.Transport)
	if ok {
		trans.MaxConnsPerHost = 128
		trans.MaxIdleConnsPerHost = 128
	}

	return transport
}

func (t *Transport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	log := util.CloneLoggerAssignReqId(t.log, req.Header.Get(proto.ReqidKey))

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("accept", "application/json")
	req.Header.Set(proto.ReqUid, t.user.Uid)
	req.Header.Set(proto.ReqToken, t.user.Token)
	req.Header.Set(proto.ReqAppid, (*t.cfg)[consts.CfgAppId])
	req.Header.Set(proto.ReqDevid, (*t.cfg)[consts.CfgDevId])
	req.Header.Set(proto.ReqPkgName, (*t.cfg)[consts.CfgPackageName])
	req.Header.Set(proto.ReqClientTag, (*t.cfg)[consts.CfgClientTag])
	req.Header.Set(proto.ReqClientId, t.clientID)
	req.Header.Set(proto.ReqTime, strconv.FormatInt(time.Now().UnixNano()/1e6, 10)) //TS in millisecond

	log.Debug("to send http request",
		zap.String("method", req.Method),
		zap.String("url", req.URL.String()),
		zap.Any("headers", req.Header))

	return t.Transport.RoundTrip(req)
}
