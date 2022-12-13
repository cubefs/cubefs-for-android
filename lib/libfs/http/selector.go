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
	"log"
	"math/rand"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/cubefs/cubefs-for-android/lib/libfs/util"

	"go.uber.org/zap"
)

type host struct {
	raw            string
	host           string
	lastFailedTime int64

	punishLock sync.RWMutex
}

func (h *host) SetFail(log *ClientLogger) {
	atomic.StoreInt64(&h.lastFailedTime, time.Now().Unix())
	log.Warn("host.setFail",
		zap.String("host", h.raw),
		zap.Int64("failTime", h.lastFailedTime))
}

type selector struct {
	tryTimes          uint32
	failRetryInterval int64        // Interval between failed retries; no interval when set -1
	hostsMu           sync.RWMutex // protect hosts
	hosts             []*host
	idx               uint32
	defaultHost       *host // host used by request, and will be directly used by subsequent requests
}

func newSelector(hosts []string, tryTimes uint32, failRetryInterval int64) *selector {
	if failRetryInterval == 0 {
		failRetryInterval = 1
	}

	// By default, two retries are performed
	if len(hosts) == 0 && tryTimes == 0 {
		tryTimes = 2
	}

	s := &selector{
		tryTimes:          tryTimes,
		failRetryInterval: failRetryInterval,
	}
	s.SetHosts(hosts)
	s.defaultHost = s.hosts[0]
	return s
}

func (s *selector) SetHosts(hosts []string) {
	if len(hosts) == 0 {
		log.Panic("empty hosts")
	}
	var hs []*host
	for _, h := range hosts {

		if !strings.HasPrefix(h, "http") {
			h = "http://" + h
		}

		_, err := url.Parse(h)
		if err != nil {
			log.Panic("error host", h, err)
		}

		hs = append(hs, &host{raw: h})
	}
	s.hostsMu.Lock()
	s.hosts = hs
	s.hostsMu.Unlock()
}

func (s *selector) Get() (h *host, rs *retrySelector) {

	s.hostsMu.RLock()
	rs = &retrySelector{hosts: s.hosts, failRetryInterval: s.failRetryInterval}
	h = s.defaultHost
	s.hostsMu.RUnlock()
	return
}

func (s *selector) update(h *host) {
	s.hostsMu.Lock()
	s.defaultHost = h
	s.hostsMu.Unlock()
}

func (s *selector) GetTryTimes() int {
	if s.tryTimes != 0 {
		return int(s.tryTimes)
	}
	s.hostsMu.RLock()
	t := len(s.hosts)
	s.hostsMu.RUnlock()
	return t
}

type retrySelector struct {
	hosts             []*host // entire host list
	retryHosts        []*host
	failRetryInterval int64
}

func (s *retrySelector) Get(log *ClientLogger) (h *host) {
	if len(s.retryHosts) == 0 {
		s.retryHosts = make([]*host, 0, len(s.hosts)-1)
		for _, h := range s.hosts {
			if ok := h.IsPunished(s.failRetryInterval); ok {
				log.Debug("host is punished",
					zap.String("host", h.raw))
				continue
			}
			s.retryHosts = append(s.retryHosts, h)
		}
	}

	if len(s.retryHosts) == 0 {
		return
	}
	// doesn't matter if it's in the failure list
	s.retryHosts, h = randomShrink(s.retryHosts)
	return
}

func (h *host) IsPunished(failRetryInterval int64) (ok bool) {
	lastFailedTime := atomic.LoadInt64(&h.lastFailedTime)
	isPunished := lastFailedTime != 0 && time.Now().Unix()-lastFailedTime < failRetryInterval
	return isPunished
}

func randomShrink(ss []*host) ([]*host, *host) {
	n := len(ss)
	if n == 1 {
		return ss[0:0], ss[0]
	}
	i := rand.Intn(n)
	s := ss[i]
	ss[i] = ss[0]
	return ss[1:], s
}
