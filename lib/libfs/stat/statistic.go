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

package stat

import (
	"time"
)

const maxTimeoutLevel = 3

type Statistic struct {
}

func NewStatistic(logBase string, logMaxSize int64, logMaxNum int,
	timeOutUs [maxTimeoutLevel]uint32, useMutex bool) *Statistic {
	st := &Statistic{}

	return st
}

func (st *Statistic) CloseStat() {
	//TODO:
}

func (st *Statistic) BeginStat() (bgTime *time.Time) {
	bg := time.Now()
	return &bg
}

func (st *Statistic) EndStat(typeName string, err error, bgTime *time.Time, statCount uint32) error {
	//TODO:
	return nil
}

func (st *Statistic) WriteStat() error {
	//TODO:
	return nil
}

func (st *Statistic) ClearStat() error {
	//TODO:
	return nil
}

func (st *Statistic) StatBandWidth(typeName string, Size uint32) {
	//TODO:
}
