// Copyright 2016 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/pingcap/errcode"
	log "github.com/pingcap/log"
	"github.com/pingcap/pd/pkg/apiutil"
	"github.com/pkg/errors"
	"github.com/unrolled/render"
)

// dialClient used to dail http request.
var dialClient = &http.Client{
	Transport: &http.Transport{
		DisableKeepAlives: true,
	},
}

// Respond to the client about the given error, integrating with errcode.ErrorCode.
//
// Important: if the `err` is just an error and not an errcode.ErrorCode (given by errors.Cause),
// then by default an error is assumed to be a 500 Internal Error.
//
// If the error is nil, this also responds with a 500 and logs at the error level.
func errorResp(rd *render.Render, w http.ResponseWriter, err error) {
	if err == nil {
		log.Error("nil is given to errorResp")
		rd.JSON(w, http.StatusInternalServerError, "nil error")
		return
	}
	if errCode := errcode.CodeChain(err); errCode != nil {
		w.Header().Set("TiDB-Error-Code", errCode.Code().CodeStr().String())
		rd.JSON(w, errCode.Code().HTTPCode(), errcode.NewJSONFormat(errCode))
	} else {
		rd.JSON(w, http.StatusInternalServerError, err.Error())
	}
}

// Write json into data.
// On error respond with a 400 Bad Request
func readJSONRespondError(rd *render.Render, w http.ResponseWriter, body io.ReadCloser, data interface{}) error {
	err := apiutil.ReadJSON(body, data)
	if err == nil {
		return nil
	}
	var errCode errcode.ErrorCode
	if jsonErr, ok := errors.Cause(err).(apiutil.JSONError); ok {
		errCode = errcode.NewInvalidInputErr(jsonErr.Err)
	} else {
		errCode = errcode.NewInternalErr(err)
	}
	errorResp(rd, w, errCode)
	return err
}

func readJSON(r io.ReadCloser, data interface{}) error {
	defer r.Close()

	b, err := ioutil.ReadAll(r)
	if err != nil {
		return errors.WithStack(err)
	}
	err = json.Unmarshal(b, data)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func postJSON(url string, data []byte, checkOpts ...func(res []byte) bool) error {
	resp, err := dialClient.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return errors.WithStack(err)
	}
	res, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return errors.New(string(res))
	}
	for _, opt := range checkOpts {
		if !opt(res) {
			return errors.New("check failed")
		}
	}
	return nil
}

func doDelete(url string) error {
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	res, err := dialClient.Do(req)
	if err != nil {
		return err
	}
	res.Body.Close()
	return nil
}

func doGet(url string) (*http.Response, error) {
	resp, err := dialClient.Get(url)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("http get url %s return code %d", url, resp.StatusCode)
	}
	return resp, nil
}
