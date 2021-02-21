// Copyright 2021 The Veela Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package log

import "sync"

var (
	globalSharedLoggerMux      sync.Mutex
	globalSharedLogger         Logger
	globalSharedLoggerInitFlag bool
)

func New() Logger {
	panic("NIY")
}

func GetGlobalSharedLogger() Logger {
	globalSharedLoggerMux.Lock()
	defer globalSharedLoggerMux.Unlock()
	if globalSharedLoggerInitFlag {
		return globalSharedLogger
	}
	globalSharedLogger = New()
	globalSharedLoggerInitFlag = true
	return globalSharedLogger
}

type Logger interface {
	Debugf(format string, v ...interface{})
	Debugln(v ...interface{})
	Infof(format string, v ...interface{})
	Infoln(v ...interface{})
	Warnf(format string, v ...interface{})
	Warnln(v ...interface{})
	Errorf(format string, v ...interface{})
	Errorln(v ...interface{})
	Fatalf(format string, v ...interface{})
	Fatalln(v ...interface{})
	Panicf(format string, v ...interface{})
	Panicln(v ...interface{})
}
