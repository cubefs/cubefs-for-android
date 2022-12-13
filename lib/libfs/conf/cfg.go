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
	"strconv"
	"strings"
	"time"

	"github.com/cubefs/cubefs-for-android/lib/libfs/consts"
	"github.com/cubefs/cubefs-for-android/lib/libfs/util"
	log2 "github.com/cubefs/cubefs-for-android/lib/log"
)

type Config map[string]string

type UserInfo struct {
	// Unique user id. The third-party vendor must ensure that the ID is globally unique
	Uid string

	// User access token, used for authentication when accessing the server.
	// The third-party vendor determines the token generation rules and validity period.
	// The minimum validity period must be more than 10s. If it is too small,
	// the token may be expired before the request reaches the server
	Token string
}

func (cfg Config) BuildUserInfo(Uid, Token string) (u *UserInfo) {
	return &UserInfo{
		Uid:   Uid,
		Token: Token,
	}
}

func (cfg Config) BuildLogCfg() *log2.Config {
	logCfg := log2.Config{}

	if cfg[consts.CfgLogPath] == "" {
		logCfg.LogFile = "./client"
	} else {
		logCfg.LogFile = cfg[consts.CfgLogPath]
	}

	if cfg[consts.CfgLogLevel] == "" {
		logCfg.LogLevel = "warn"
	} else {
		logCfg.LogLevel = cfg[consts.CfgLogLevel]
	}

	maxSize, err := strconv.ParseInt(cfg[consts.CfgLogMaxSize], 10, 32)
	if err != nil || maxSize == 0 {
		logCfg.MaxSize = 4 // default size 4M
	} else {
		logCfg.MaxSize = int(maxSize)
	}

	backUp, err := strconv.ParseInt(cfg[consts.CfgLogBackUp], 10, 32)
	if err != nil || backUp == 0 {
		logCfg.MaxBackups = 10 // default keep 10 log files
	} else {
		logCfg.MaxBackups = int(backUp)
	}

	maxAge, err := strconv.ParseInt(cfg[consts.CfgLogMaxAge], 10, 32)
	if err != nil || maxAge == 0 {
		logCfg.MaxAge = 10 // default keep logs for 10 days
	} else {
		logCfg.MaxAge = int(maxAge)
	}

	compress, err := strconv.ParseBool(cfg[consts.CfgLogCompress])
	if err != nil {
		logCfg.Compress = false
	} else {
		logCfg.Compress = compress
	}

	return &logCfg
}

type HttpCfg struct {
	Hosts           []string `json:"hosts"`              // The destination service address list, must not be empty
	TryTimes        uint32   `json:"try_times"`          // Maximum attempts for a request, TryTimes = RetryTimes + 1
	ClientTimeoutMS int      `json:"client_timeout_ms"`  // Timeout duration of the http client，see http.Client
	MaxFailsPeriodS int64    `json:"max_fails_period_s"` // Penalty time for failures

	ShouldRetry func(code int, err error) bool `json:"-"`
}

func (cfg Config) BuildHttpCfg(log *util.ClientLogger) *HttpCfg {
	hostStr := cfg[consts.CfgProxyHosts]
	if len(hostStr) == 0 || len(strings.Split(hostStr, ",")) == 0 {
		log.Fatal("host can't be empty", log2.String("hosts", hostStr))
	}
	hosts := strings.Split(hostStr, ",")

	tryTimes, err := strconv.ParseUint(cfg[consts.CfgGroupTryTimes], 10, 64)
	if err != nil {
		log.Info("try_times is invalid, use default 0", log2.String("tryTime", cfg[consts.CfgGroupTryTimes]), log2.Any("err", err))
		tryTimes = 0
	}

	timeOutMs, err := strconv.ParseUint(cfg[consts.CfgClientTimeOutMs], 10, 64)
	if err != nil {
		log.Info("client_timeout_ms is invalid, use default 0", log2.String("timeOuts", cfg[consts.CfgClientTimeOutMs]), log2.Any("err", err))
		timeOutMs = 0
	}

	periodS, err := strconv.ParseUint(cfg[consts.CfgPeriodS], 10, 64)
	if err != nil {
		log.Info("max_fails_period_s is invalid, use default 0", log2.String("periods", cfg[consts.CfgPeriodS]), log2.Any("err", err))
		periodS = 0
	}

	cliCfg := HttpCfg{
		Hosts:           hosts,
		TryTimes:        uint32(tryTimes),
		ClientTimeoutMS: int(timeOutMs),
		MaxFailsPeriodS: int64(periodS),
	}

	return &cliCfg
}

type RetryCfg struct {
	RetryTimes  int           //  maximum attempts
	RetryGap    time.Duration //  retry interval
	RetryFactor time.Duration //  retry penalty factor
}

func (cfg Config) BuildRetryCfg(log *util.ClientLogger) *RetryCfg {
	return &RetryCfg{
		RetryTimes:  cfg.GetIntValue(consts.CfgRetryTimes, 3, log),
		RetryGap:    time.Duration(cfg.GetIntValue(consts.CfgRetryGap, 5, log)) * time.Second,
		RetryFactor: time.Duration(cfg.GetIntValue(consts.CfgRetryFactor, 10, log)) * time.Second,
	}
}

func (cfg Config) GetIntValue(cfgName string, defaultVal int, log *util.ClientLogger) int {
	val, err := strconv.ParseInt(cfg[cfgName], 10, 32)
	if err != nil {
		log.Info("GetIntValue",
			log2.String("cfgName", cfgName),
			log2.Int("defaultVal", defaultVal),
			log2.Any("err", err))
		val = int64(defaultVal)
	}

	return int(val)
}

func (cfg Config) GetStringValue(cfgName string, defaultVal string, log *util.ClientLogger) string {
	val, ok := cfg[cfgName]
	if !ok {
		log.Warn("GetStringValue",
			log2.String("cfgName", cfgName),
			log2.String("defaultVal", defaultVal))
		val = defaultVal
	}
	return val
}
