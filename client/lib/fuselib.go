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

package lib

import (
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/hanwen/go-fuse/v2/fuse/nodefs"
)

type FuseLib struct {
	fuse.RawFileSystem
}

func Mount(mountpoint string, root nodefs.Node, mountOptions *fuse.MountOptions, nodefsOptions *nodefs.Options) (*fuse.Server, *nodefs.FileSystemConnector, error) {
	conn := nodefs.NewFileSystemConnector(root, nodefsOptions)

	fuseLib := FuseLib{
		RawFileSystem: conn.RawFS(),
	}

	s, err := fuse.NewServer(&fuseLib, mountpoint, mountOptions)
	if err != nil {
		return nil, nil, err
	}
	return s, conn, nil
}

func (f *FuseLib) SetLkw(cancel <-chan struct{}, input *fuse.LkIn) (code fuse.Status) {
	return fuse.OK
}
