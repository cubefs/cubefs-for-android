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

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/cubefs/cubefs-for-android/client/cfafuse"
	"github.com/cubefs/cubefs-for-android/client/conf"
	"github.com/cubefs/cubefs-for-android/client/lib"
	fuselog "github.com/cubefs/cubefs-for-android/client/log"
	"github.com/cubefs/cubefs-for-android/lib/libfs"
	"go.uber.org/zap"

	fs "github.com/hanwen/go-fuse/v2/fuse"
	"github.com/hanwen/go-fuse/v2/fuse/pathfs"
)

var (
	configFile = flag.String("c", "", "FUSE client config file")
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	flag.Parse()
	fuseCfg := conf.LoadFuseCfg(*configFile)

	server, mountInfo, err := mount(fuseCfg)
	if err != nil {
		log.Fatalf("mount fail, %v", err)
	}

	signalChan := make(chan os.Signal)
	signal.Notify(signalChan,
		syscall.SIGKILL,
		syscall.SIGSEGV,
		syscall.SIGABRT,
		syscall.SIGTERM,
		syscall.SIGUSR1,
		syscall.SIGUSR2)

	signal.Ignore(syscall.SIGPIPE)
	signal.Ignore(syscall.SIGHUP)

	idleConnsClosed := make(chan struct{})
	go exitSign(signalChan, server, mountInfo, idleConnsClosed)

	server.Serve()
	<-idleConnsClosed
}

// do unmount when exit
func exitSign(signalChan chan os.Signal, se *fs.Server, mountInfo *libfs.CfaMountInfo, closeChan chan struct{}) {

	for {
		sig := <-signalChan
		log.Printf("Receive Sign[%s]\n", sig)

		switch sig {
		case syscall.SIGTERM, syscall.SIGKILL, syscall.SIGABRT, syscall.SIGSEGV:
			se.Unmount()
			mountInfo.DelMount()
			fmt.Println("del mount success")
			close(closeChan)
			return
		case syscall.SIGUSR1: // print fuse log
			level := fuselog.FuseLog.Level()
			if level > zap.DebugLevel {
				fuselog.FuseLog.SetLevel(level - 1)
			}
			se.SetDebug(true)

			mountInfo.SetLogLevel(-1)
		case syscall.SIGUSR2: // not print fuse log
			level := fuselog.FuseLog.Level()
			if level < zap.PanicLevel {
				fuselog.FuseLog.SetLevel(level + 1)
			}
			se.SetDebug(false)

			mountInfo.SetLogLevel(1)
		}
	}
}

func mount(fuseCfg *conf.FuseConf) (*fs.Server, *libfs.CfaMountInfo, error) {
	mountInfo, err := libfs.NewMount(fuseCfg.Cfg)
	if err != nil {
		log.Fatalf("new mount fail, err %v", err)
	}

	err = mountInfo.Mount()
	if err != nil {
		return nil, nil, err
	}

	fs, err := cfafuse.NewCfaFS(fuseCfg, mountInfo)
	if err != nil {
		log.Fatalf("cannot NewCfaFS, err (%v)", err)
	}

	nodeFs := pathfs.NewPathNodeFs(fs, &fuseCfg.PathOpt)

	server, _, err := lib.Mount(fuseCfg.MountPoint, nodeFs.Root(), &fuseCfg.MntOpt, &fuseCfg.NodeOpt)

	return server, mountInfo, err
}
