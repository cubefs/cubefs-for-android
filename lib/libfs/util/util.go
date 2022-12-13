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
	"log"
	"net"
	"os"
)

func GetHostName() string {
	host, err := os.Hostname()
	if err != nil {
		log.Panic("get host fail", err)
	}

	return host
}

func GetIp() string {
	var ips []string
	netAddr, err := net.InterfaceAddrs()
	if err != nil {
		log.Panic("get inter addr fail", err.Error())
	}

	for _, a := range netAddr {
		if ipNet, ok := a.(*net.IPNet); ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
			ips = append(ips, ipNet.IP.String())
		}
	}

	if len(ips) <= 0 || ips == nil {
		log.Panic("can't get valid ip")
	}

	return ips[0]
}
