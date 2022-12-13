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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/cubefs/cubefs-for-android/lib/libfs/conf"
	"github.com/cubefs/cubefs-for-android/lib/libfs/util"
	zap "github.com/cubefs/cubefs-for-android/lib/log"
	"github.com/cubefs/cubefs-for-android/lib/proto"

	"github.com/pkg/errors"
)

const MAX_REDIRECT_TIMES = 10

type HttpClient struct {
	*conf.HttpCfg
	client   *http.Client
	sel      *selector
	ClientId string
	sync.RWMutex
}

func (h *HttpClient) getRedirectTimes() int {
	return MAX_REDIRECT_TIMES
}

func (h *HttpClient) Close() {
	h.client.CloseIdleConnections()
}

func (h *HttpClient) UpdateHosts(Hosts string) {
	h.Lock()
	defer h.Unlock()

	h.Hosts = []string{Hosts}
	h.sel = newSelector(h.Hosts, h.HttpCfg.TryTimes, -1)
}

func NewHttpClient(cfg *conf.HttpCfg, user *conf.UserInfo, clientID string,
	log *util.ClientLogger, cfaCfg *conf.Config) *HttpClient {
	tr := newTransport(user, clientID, log, cfaCfg)

	if cfg.ShouldRetry == nil {
		cfg.ShouldRetry = ShouldRetry
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   time.Duration(cfg.ClientTimeoutMS) * time.Millisecond,
	}

	return &HttpClient{
		HttpCfg:  cfg,
		client:   client,
		sel:      newSelector(cfg.Hosts, cfg.TryTimes, cfg.MaxFailsPeriodS),
		ClientId: clientID,
	}
}

// type HttpRequest
type HttpRequest struct {
	*http.Request
	Body io.ReaderAt
}

func newRequest(method, urlStr string, body io.ReaderAt) (*HttpRequest, error) {

	var r io.Reader
	if body != nil {
		r = &RetryReader{body, 0}
	}
	httpReq, err := http.NewRequest(method, urlStr, r)
	if err != nil {
		return nil, err
	}
	req := &HttpRequest{httpReq, body}
	return req, nil
}

// result must be an address
var (
	ErrIllegalParam = errors.New("param is illegal")
)

func (p *HttpClient) DoPostWithJson(api_uri string, params interface{}, result interface{}, log *util.ClientLogger) (err error) {
	msg, err := json.Marshal(params)
	if err != nil {
		log.Error("param is not legal",
			zap.Any("param", params),
			zap.Any("err", err))
		return ErrIllegalParam
	}

	req, err := newRequest("POST", api_uri, bytes.NewReader(msg))
	if err != nil {
		log.Error("gen request fail",
			zap.Any("err", err))
		return err
	}

	req.Header.Set(proto.ReqidKey, log.GetReqId())

	resp, err := p.doCtxWithHostRet(req, log)
	if err != nil {
		log.Error("request fail, ",
			zap.Any("param", params),
			zap.Any("err", err.Error()))
		return err
	}

	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(result)
	if err != nil {
		log.Error("decode result fail",
			zap.Any("err", err))
		return err
	}

	return nil
}

var (
	ErrRetryNeeded      = errors.New("req need retry")
	ErrAbnormalRespCode = errors.New("abnormal resp code")
)

func (p *HttpClient) doCtxWithHostRet(cfsReq *HttpRequest, log *util.ClientLogger) (resp *http.Response, err error) {
	req := cfsReq.Request
	reqUri := req.URL.RequestURI()

	h, sel := p.sel.Get()
	if h == nil {
		log.Error("no available service",
			zap.Any("host", p.Hosts))
		return nil, ErrRetryNeeded
	}

	var code int
	retryTimes := p.sel.GetTryTimes()
	redirectTime := p.getRedirectTimes()

	retryIdx := 0
	redirectIdx := 0

	replaceHost := false
	defer func() {
		if replaceHost {
			log.Warn("http reset default host", zap.String("host", h.raw))
			p.sel.update(h)
		}
	}()

	for {
		rHost := h.raw

		if err != nil {
			log.Error("url not legal",
				zap.String("host", rHost),
				zap.String("uri", reqUri))
			return nil, ErrIllegalParam
		}

		if cfsReq.Body != nil {
			r := &RetryReader{cfsReq.Body, 0}
			req.Body = ioutil.NopCloser(r)
		} else {
			req.Body = nil
		}

		req.Host = req.URL.Host
		resp, err = p.client.Do(req)

		if resp == nil && err == nil {
			//if URI or req struct wrong, will encounter no err but reps is nil
			return nil, errors.New("http resp is nil, please check proto")
		}
		if err != nil {
			return
		}

		code = 0
		if resp != nil {
			code = resp.StatusCode
		}

		if code >= 400 {
			log.Error("resp http code is invalid",
				zap.String("url", rHost+reqUri),
				zap.Int("code", code))
			return nil, ErrAbnormalRespCode
		}

		if code == http.StatusPermanentRedirect {
			loc := resp.Header.Get("Location")
			u, err := req.URL.Parse(loc)
			if err != nil {
				log.Error("redirect parse location fail", zap.String("loc", loc), zap.Error(err))
				return nil, err
			}

			h = &host{raw: fmt.Sprintf("%s://%s", u.Scheme, u.Host)}

			log.Debug("redirect", zap.String("old", req.Host), zap.String("loc", u.Host))

			redirectIdx++
			if redirectIdx >= redirectTime {
				log.Warn("over max redirect times", zap.Int("now", redirectIdx), zap.Int("max", redirectTime))
				return nil, errors.New(fmt.Sprintf("stopped after  %d redirects", redirectTime))
			}

			if resp != nil {
				discardAndClose(resp.Body)
			}

			replaceHost = true
			continue
		}

		if p.ShouldRetry(code, err) {
			log.Warn("cfa.DoWithHostRet: retry host, times: ",
				zap.String("retry host", rHost),
				zap.Int("code", code),
				zap.Any("err", err),
				zap.String("url", reqUri))

			h.SetFail(log)
			h = sel.Get(log)
			if h == nil || retryIdx == retryTimes-1 {
				log.Error("simple.DoWithHostRet: no more try",
					zap.Int("idx", retryIdx))
				return nil, err
			}

			if resp != nil {
				discardAndClose(resp.Body)
			}

			replaceHost = true
			continue
		}

		return
	}
	return
}

// retry on timeout only
var ShouldRetry = func(code int, err error) bool {
	urlErr, ok := err.(*url.Error)
	if ok && urlErr.Timeout() {
		return true
	}

	if err == nil {
		return false
	}

	if strings.Contains(err.Error(), "use of closed network connection") {
		return true
	}

	if strings.Contains(err.Error(), "connection reset by peer") {
		return true
	}

	return false
}

// prevents problems when the same host tries
type RetryReader struct {
	Reader io.ReaderAt
	Offset int64
}

func (p *RetryReader) Read(val []byte) (n int, err error) {
	n, err = p.Reader.ReadAt(val, p.Offset)
	p.Offset += int64(n)
	return
}

func (p *RetryReader) Close() error {
	if rc, ok := p.Reader.(io.ReadCloser); ok {
		return rc.Close()
	}
	return nil
}

func discardAndClose(r io.ReadCloser) error {
	io.Copy(ioutil.Discard, r)
	return r.Close()
}
