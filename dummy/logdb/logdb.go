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

package logdb

// The path should be non-exist yet and logdb would create it by itself.
// But you may not expected logdb would create intermediate directories as required.
// That is just like a simple `mkdir` without `-p` option.
func CreateDB(dirPathStr string) (DB, error) {
	return nil, nil
}

// If the db is invalid yet, error would be returned.
func OpenDBIfExist(dirPathStr string) (DB, error) {
	return nil, nil
}

// Limitations: One writer and multi reader at the same time.
// The key inside logdb is always incremental positive integers thus we named it - `idx`.
type DB interface {
	// Return the range of idx which already set a value: [leftIdx, toAppendIdx).
	// Length of already set idx range == toAppendIdx - leftIdx.
	// toAppendIdx >= leftIdx &&
	// leftIdx > 0
	GetCurrentIdxRange() (leftIdx, toAppendIdx uint64)
	GetValueByIdx(idx uint64) (v []byte, e error)
	// Zero value of deleteAllIdxLessThan means ignore this input arg.
	// For a positive deleteAllIdxLessThan, this call would mark all the idx between (0, deleteAllIdxLessThan)
	// `will-be-deleted` state, and the deleting operation could be asynchrous.
	// Zero len of vArray means do not append any new value.
	// `appendAtIdx` should be exactly equal with the toAppendIdx returned from `GetCurrentIdxRange`, otherwise this function call
	// would be failed.
	AppendAndSync3(appendAtIdx uint64, vArray [][]byte, deleteAllIdxLessThan uint64) (e error)
	AppendAndSync(appendAtIdx uint64, vArray [][]byte) (e error)
	// Close the db handler
	Close() error
}
