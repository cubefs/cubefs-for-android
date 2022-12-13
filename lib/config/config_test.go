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

package config

import (
	"io/ioutil"
	"os"
	"testing"
)

type Config01 struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

func TestLoadBytes(t *testing.T) {
	data := []byte(`{
		"name": "tom",
		"id": 123
	}`)
	var cfg01 Config01
	err := LoadBytes(&cfg01, data)
	if err != nil {
		t.Errorf("%v", err)
	}
	t.Logf("cfg=%v", cfg01)

	data = []byte(`{
		"name": "tom", //name
		"id": 123      //增加注释
	}`)
	err = LoadBytes(&cfg01, data)
	if err != nil {
		t.Errorf("%v", err)
	}
	t.Logf("cfg=%v", cfg01)

	data = []byte(`{
		"id": 123      //增加注释 缺少字段
	}`)
	cfg01 = Config01{0, ""}
	err = LoadBytes(&cfg01, data)
	if err != nil {
		t.Errorf("%v", err)
	}
	t.Logf("cfg=%v", cfg01)

	data = []byte(`{
		"id": 123,      #增加注释 
		"name": "tom", #name
		"other": true
	}`)
	cfg01 = Config01{0, ""}
	err = LoadBytes(&cfg01, data)
	if err != nil {
		t.Errorf("%v", err)
	}
	t.Logf("cfg=%v", cfg01)

	data = []byte(`{
		"id": 123,      #增加注释 
		"name": "\"tom", #name
		"other": true
	}`)
	cfg01 = Config01{0, ""}
	err = LoadBytes(&cfg01, data)
	if err != nil {
		t.Errorf("%v", err)
	}
	t.Logf("cfg=%v", cfg01)
}

func TestLoadConfig(t *testing.T) {
	data := []byte(`{
		"id": 123,      #增加注释 
		"name": "\"tom", #name
		"other": true
	}`)
	defer os.Remove("./test.cfg")
	ioutil.WriteFile("./test.cfg", data, 0644)
	var cfg Config01
	err := LoadConfig(&cfg, "./test.cfg")
	if err != nil {
		t.Errorf("load config error: %v", err)
	}
}
