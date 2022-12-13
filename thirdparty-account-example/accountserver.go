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
	"encoding/json"
	"github.com/cubefs/cubefs-for-android/lib/proto"
	"log"
	"net/http"
)

const (
	FixedAccessKey = "testAccessKey"
	FixedUser      = "1234567"
	FixedToken     = "testToken"
)

func main() {

	// eg: return fixed a user, and a  token
	http.HandleFunc("/thirdparty/api/user/pullaccount", func(w http.ResponseWriter, r *http.Request) {
		accessKey := r.Header.Get(proto.ReqAccessKey)

		log.Printf("req fetch tocken, remote: %s, accessKey:%s\n", r.RemoteAddr, accessKey)

		respBody := &proto.FetchResponse{}
		respData := &proto.FetchRespData{
			Uid:   FixedUser,
			Token: FixedToken,
		}

		if accessKey != FixedAccessKey {
			log.Printf("req invalid accessKey:%s\n", accessKey)
			respBody.Code = 1
			respBody.Msg = "invalid access key"
			respBody.Data = nil
		} else {
			log.Printf("resp fetch tocken, remote: %s, user_id:%s, user_token: %s\n",
				r.RemoteAddr, FixedUser, FixedToken)
			respBody.Code = 0
			respBody.Msg = "success"
			respBody.Data = respData
		}

		data, err := json.Marshal(respBody)
		if err != nil {
			panic(err)
		}

		w.Write(data)
	})

	log.Println("account server example run ... \n")
	port := ":10122"
	err := http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatalf("listen %s failed, err %s", port, err.Error())
	}
	log.Println("account server example exit ... \n")
}
