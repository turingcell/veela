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

package util

// TODO: overflow detect
func Int32ToIntAssert(i32 int32) int {
	return int(i32)
}

// TODO: overflow detect
func Int32ToUint64Assert(i32 int32) uint64 {
	AssertTrue(i32 >= 0)
	return uint64(i32)
}

// TODO: overflow detect
func IntToUint32Assert(i int) uint32 {
	AssertTrue(i >= 0)
	return uint32(i)
}

// TODO: overflow detect
func Uint32ToIntAssert(u32 uint32) int {
	return int(u32)
}

// TODO: overflow detect
func Uint64AddAssert(u1, u2 uint64) uint64 {
	return u1 + u2
}

func AssertNoErr(err error) {
	if err != nil {
		panic("unexpected")
	}
}

func AssertTrue(b bool) {
	if b {
	} else {
		panic("unexpected")
	}
}

func U32SetBs(bs []byte, u32 uint32) {
	bs[0] = byte(u32 >> 24)
	bs[1] = byte(u32 >> 16)
	bs[2] = byte(u32 >> 8)
	bs[3] = byte(u32 >> 0)
}

func BsReadU32(bs []byte) (u32 uint32) {
	u32 = uint32(bs[0])<<24 + uint32(bs[1])<<16 + uint32(bs[2])<<8 + uint32(bs[3])<<0
	return
}

func U64SetBs(bs []byte, u64 uint64) {
	bs[0] = byte(u64 >> 56)
	bs[1] = byte(u64 >> 48)
	bs[2] = byte(u64 >> 40)
	bs[3] = byte(u64 >> 32)
	bs[4] = byte(u64 >> 24)
	bs[5] = byte(u64 >> 16)
	bs[6] = byte(u64 >> 8)
	bs[7] = byte(u64 >> 0)
}

func BsReadU64(bs []byte) (u64 uint64) {
	u64 = uint64(bs[0])<<56 + uint64(bs[1])<<48 + uint64(bs[2])<<40 + uint64(bs[3])<<32 +
		uint64(bs[4])<<24 + uint64(bs[5])<<16 + uint64(bs[6])<<8 + uint64(bs[7])<<0
	return
}
