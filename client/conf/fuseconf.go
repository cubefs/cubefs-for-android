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

package conf

import (
	"fmt"
	slog "log"
	"strings"
	"time"

	"github.com/cubefs/cubefs-for-android/lib/config"
	"github.com/cubefs/cubefs-for-android/lib/libfs/conf"
	"github.com/cubefs/cubefs-for-android/lib/libfs/consts"
	"github.com/cubefs/cubefs-for-android/lib/log"

	fs "github.com/hanwen/go-fuse/v2/fuse"
	"github.com/hanwen/go-fuse/v2/fuse/nodefs"
	"github.com/hanwen/go-fuse/v2/fuse/pathfs"
)

type LogCfg log.Config

type FuseConf struct {
	Cfg     conf.Config              `json:"cfa_cfg"`
	NodeOpt nodefs.Options           `json:"node_opt"`
	MntOpt  fs.MountOptions          `json:"mnt_opt"`
	PathOpt pathfs.PathNodeFsOptions `json:"path_opt"`

	MountPoint   string `json:"mnt_point"`
	ReadDirCount int    `json:"read_dir_cnt"`
}

func LoadFuseCfg(path string) *FuseConf {
	fuseCfg := &FuseConf{}
	fuseCfg.MntOpt.IgnoreSecurityLabels = true
	//fuseCfg.NodeOpt.LookupKnownChildren = true
	fuseCfg.PathOpt.ClientInodes = true // support hard link operate

	err := config.LoadConfig(&fuseCfg, path)
	if err != nil {
		slog.Fatalf("load config from %s fail, err %v", path, err)
	}

	if fuseCfg.NodeOpt.EntryTimeout != 0 {
		fuseCfg.NodeOpt.EntryTimeout *= time.Second
	}

	if fuseCfg.NodeOpt.AttrTimeout != 0 {
		fuseCfg.NodeOpt.AttrTimeout *= time.Second
	}

	if fuseCfg.NodeOpt.NegativeTimeout != 0 {
		fuseCfg.NodeOpt.NegativeTimeout *= time.Second
	}

	fuseCfg.Cfg[consts.ExportPath] = fuseCfg.MountPoint

	fuseCfg.MntOpt.Name = fmt.Sprintf("cfa")
	fuseCfg.MntOpt.FsName = "cfa"
	fuseCfg.MntOpt.EnableLocks = true
	fuseCfg.MntOpt.AllowOther = true
	fuseCfg.MntOpt.DirectMount = true

	if fuseCfg.Cfg[consts.CfgClientTag] == "" {
		fuseCfg.Cfg[consts.CfgClientTag] = consts.DefClientTag
	}

	host := fuseCfg.Cfg[consts.CfgProxyHosts]
	if !strings.HasPrefix(host, "http://") {
		host = "http://" + host
		fuseCfg.Cfg[consts.CfgProxyHosts] = host
	}
	if strings.HasSuffix(host, "/") {
		host = host[:len(host)-1]
		fuseCfg.Cfg[consts.CfgProxyHosts] = host
	}

	return fuseCfg
}

func (cfg *FuseConf) GetPath() string {
	return cfg.Cfg[consts.CfgPath]
}
