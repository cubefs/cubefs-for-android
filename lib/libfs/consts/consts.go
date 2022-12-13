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

package consts

var CaseMap = map[string]int{}

const (
	CfgProxyHosts      = "proxy_hosts"
	CfgPath            = "path" // mount path
	CfgGroupTryTimes   = "group_try_times"
	CfgClientTimeOutMs = "client_timeout_ms"
	CfgPeriodS         = "fails_period_s"

	// log related
	CfgLogPath     = "log_file"  // log file path
	CfgLogLevel    = "log_level" // log level
	CfgLogMaxSize  = "max_size"  // max size of log files
	CfgLogBackUp   = "back_up"   // number of retained files
	CfgLogMaxAge   = "max_age"   // retention days
	CfgLogCompress = "compress"  // if compress log files

	// statistics log related
	CfgStatLogPath    = "stat_log_file"     // statistics log file path
	CfgStatLogNum     = "stat_log_num"      // number of statistics files
	CfgStatLogMaxSize = "stat_log_max_size" // max size of statistics log files
	CfgStatGap        = "stat_gap"          // statistics interval

	// retry mechanism
	CfgRetryTimes  = "retry_times"  // max attempts
	CfgRetryGap    = "retry_gap"    // retry interval
	CfgRetryFactor = "retry_factor" // penalty factor

	// configuration of third party Vendors
	CfgThirdPartyUidPullType            = "uid_pull_type" // way to fetch user_id and user_token. "1":by uid_pull_url; "2":Passed in by the config file.
	UidPullTypeByUrl                    = "1"
	UidPullTypeByConf                   = "2"
	CfgUserId                           = "user_id"
	CfgUserToken                        = "user_token"
	CfgThirdPartyUidPullIntervalMillSec = "uid_pull_interval_ms" // Account pulling interval, unit ms
	CfgThirdPartyUidPullAccessKey       = "uid_pull_access_key"  // The Access key used for account pulling, issued by the third-party vendor
	CfgThirdPartyUidPullUrl             = "uid_pull_url"         // address for account pulling, eg：http://127.0.0.1:10122/thirdparty/api/user/pullaccount, issued by the third-party vendor

	CfgClientTag   = "client_tag"
	DefClientTag   = "CFA"
	CfgAppId       = "app_id"
	CfgDevId       = "dev_id"
	CfgPackageName = "package_name"

	// others
	CfgOpenFileMax = "open_file_max" // maximum number of files that can be opened
	CfgLsSize      = "ls_size"       // maximum item number of single 'ls' operation

	CfgDentryCacheSize        = "dentry_cache_size"         // size of meta data dentry cache
	CfgDentryCacheExpireMs    = "dentry_cache_expire_ms"    // timeout duration of meta data dentry cache，in milli-sec
	CfgRwCacheCacheSize       = "rw_cache_size"             // write cache size
	CfgRwCacheCheckGap        = "rw_cache_check_gap"        // Timeout check period of write cache
	CfgRwCacheExpireMs        = "rw_cache_expire_ms"        // timeout duration of write cache, in milli-sec
	CfgRwCacheSyncRoutine     = "rw_cache_sync_routine"     // number of asynchronous write I/O coroutines
	CfgRwCachePrefetchRoutine = "rw_cache_prefetch_routine" // number of pre-read coroutines
	CfgRwCachePrefetchTimes   = "rw_cache_prefetch_times"   // Prefetch buffer multiple

	// push monitor dat cfg
	PushCluster = "cluster" // the push cluster
	// default [0, 65536,131072, 262144, 524288, 1048576, 4194304, 8388608, 16777216, 33554432]
	PushSizeBkt = "push_size_bkts"
	ConusulAddr = "cfs_consul_addr"
	CounsulMeta = "cfs_consul_meta"
	ExportPort  = "cfs_export_port" // default empty
	ExportPath  = "cfs_export_path"
)
