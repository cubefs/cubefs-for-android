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

package log

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Other struct {
	name  []byte
	test  string
	table map[string]string
}

type mapMarshaler map[string]string

func (kv mapMarshaler) MarshalLogObject(encode zapcore.ObjectEncoder) error {
	for k, v := range kv {
		encode.AddString(k, v)
	}
	return nil
}

func (other Other) MarshalLogObject(encode zapcore.ObjectEncoder) error {
	encode.AddByteString("name", other.name)
	encode.AddString("test", other.test)
	encode.AddObject("table", mapMarshaler(other.table))
	return nil
}

func TestNewLogger(t *testing.T) {
	cfg := Config{
		LogFile:    "./test.log",
		LogLevel:   "debug",
		MaxSize:    1,
		MaxBackups: 1,
		MaxAge:     1,
		Compress:   false,
	}

	os.Remove(cfg.LogFile)
	logger := NewLogger(&cfg)
	logger.Debug("test", Int("i", 10))
	logger.Info("test", String("iur", "jdaurokdua"))
	logger.Warn("test", Time("time", time.Now()))
	logger.Error("test", NamedError("error", errors.New("dkjsahirueiuh")))
	//logger.Panic("test", zap.String("str", "isaurihkhgiu"))
	//logger.Fatal("test", zap.String("str", "isaurihkhgiu"))
	other := Other{
		name:  []byte("1234"),
		test:  "abcde",
		table: make(map[string]string),
	}
	other.table["udiarhu"] = "oairwq"
	other.table["kujhdsaiur"] = "oairwq"
	other.table["dakur"] = "oairwq"
	logger.Info("cfg print", Any("cfg", other))

	logger.SetLevel(zap.ErrorLevel)
	logger.Info("cfg print", Any("cfg", other))
	logger.Close()

	cfg.LogFile = "audit.log"
	os.Remove(cfg.LogFile)
	logger = NewAuditLogger(&cfg)
	logger.Debug("test", Int("i", 10))
	logger.Info("test", String("iur", "jdaurokdua"))
	logger.Warn("test", Time("time", time.Now()))
	logger.Error("test", NamedError("error", errors.New("dkjsahirueiuh")))
	logger.Info("cfg print", Any("cfg", other))
	logger.Close()
}

func TestStrToLevel(t *testing.T) {
	level := strToLevel("debug")
	if level != zap.DebugLevel {
		t.Errorf("level=%d not DebugLevel", int(level))
	}

	level = strToLevel("dEbug")
	if level != zap.DebugLevel {
		t.Errorf("level=%d not DebugLevel", int(level))
	}

	level = strToLevel("dEbuge")
	if level == zap.DebugLevel {
		t.Errorf("level=%d DebugLevel", int(level))
	}

	if level != zap.InfoLevel {
		t.Errorf("level=%d not InfoLevel", int(level))
	}

	level = strToLevel("info")
	if level != zap.InfoLevel {
		t.Errorf("level=%d not InfoLevel", int(level))
	}

	level = strToLevel("Info")
	if level != zap.InfoLevel {
		t.Errorf("level=%d not InfoLevel", int(level))
	}

	level = strToLevel("Infoe") //默认info级别
	if level != zap.InfoLevel {
		t.Errorf("level=%d not InfoLevel", int(level))
	}

	level = strToLevel("warn")
	if level != zap.WarnLevel {
		t.Errorf("level=%d not WarnLevel", int(level))
	}

	level = strToLevel("Warn")
	if level != zap.WarnLevel {
		t.Errorf("level=%d not WarnLevel", int(level))
	}

	level = strToLevel("Warne") //默认info级别
	if level != zap.InfoLevel {
		t.Errorf("level=%d not InfoLevel", int(level))
	}

	level = strToLevel("error")
	if level != zap.ErrorLevel {
		t.Errorf("level=%d not ErrorLevel", int(level))
	}

	level = strToLevel("Error")
	if level != zap.ErrorLevel {
		t.Errorf("level=%d not ErrorLevel", int(level))
	}

	level = strToLevel("Errorr") //默认info级别
	if level != zap.InfoLevel {
		t.Errorf("level=%d not InfoLevel", int(level))
	}

	level = strToLevel("panic")
	if level != zap.PanicLevel {
		t.Errorf("level=%d not PanicLevel", int(level))
	}

	level = strToLevel("Panic")
	if level != zap.PanicLevel {
		t.Errorf("level=%d not PanicLevel", int(level))
	}

	level = strToLevel("Panicc") //默认info级别
	if level != zap.InfoLevel {
		t.Errorf("level=%d not InfoLevel", int(level))
	}

	level = strToLevel("fatal")
	if level != zap.FatalLevel {
		t.Errorf("level=%d not FatalLevel", int(level))
	}

	level = strToLevel("Fatal")
	if level != zap.FatalLevel {
		t.Errorf("level=%d not FatalLevel", int(level))
	}

	level = strToLevel("Fatall") //默认info级别
	if level != zap.InfoLevel {
		t.Errorf("level=%d not InfoLevel", int(level))
	}
}

func TestHttpSetLevel(t *testing.T) {
	cfg := Config{
		LogFile:    "./http_test.log",
		LogLevel:   "debug",
		MaxSize:    1,
		MaxBackups: 1,
		MaxAge:     1,
		Compress:   false,
	}

	os.Remove(cfg.LogFile)
	logger := NewLogger(&cfg)
	defer func() {
		logger.Close()
	}()
	mux := http.NewServeMux()
	mux.HandleFunc("/log/level", logger.ServeHTTP)

	logger.Info("TestHttpSetLevel")

	reader := strings.NewReader(`{"level":"error"}`)
	r, _ := http.NewRequest(http.MethodPut, "/log/level", reader)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Response code is %v", resp.StatusCode)
	}
	logger.Info("TestHttpSetLevel")
}

func TestLogWith(t *testing.T) {
	cfg := Config{
		LogFile:    "./test.log",
		LogLevel:   "debug",
		MaxSize:    1,
		MaxBackups: 1,
		MaxAge:     1,
		Compress:   false,
	}

	os.Remove(cfg.LogFile)
	logger := NewLogger(&cfg)
	defer func() {
		logger.Close()
	}()

	var l *Logger
	l = logger.With(String("reqid", "djaiuejrhtiu"))
	l.Info("this is a test log with", Int("counter", 5))
	ll := l.With(String("name", "zhangshan"))
	ll.Info("this is a test log with")
}
