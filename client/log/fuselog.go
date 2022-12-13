package log

import "github.com/cubefs/cubefs-for-android/lib/log"

var FuseLog = DefaultLogger()

func DefaultLogger() *log.Logger {

	logCfg := log.Config{
		LogFile:    "./cfa-fuse.log",
		LogLevel:   "debug",
		MaxSize:    256,
		MaxBackups: 10,
		MaxAge:     10,
		Compress:   false,
	}

	logger := log.NewLogger(&logCfg)
	return logger
}
