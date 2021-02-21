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

package veela

import (
	"fmt"
	"net"
	"sync"

	"github.com/turingcell/veela/dummy/log"
	"github.com/turingcell/veela/dummy/logdb"
	vpb "github.com/turingcell/veela/proto/veela"
	"github.com/turingcell/veela/util"
)

var (
	vlog = log.GetGlobalSharedLogger()
)

// 2**64/24./3600./365./100000000. == 5849.424173550719...
type Epoch uint64

func (e *Epoch) ToUint64() uint64 {
	return uint64(*e)
}

func (e *Epoch) SetFromUint64(u64 uint64) {
	*e = Epoch(u64)
}

func (e *Epoch) SetFromEpoch(fromE Epoch) {
	*e = fromE
}

func (e *Epoch) Incr1() {
	u64 := e.ToUint64()
	u64++
	if u64 == 0 {
		panic("unexpected")
	}
	e.SetFromUint64(u64)
}

type PaxosGroup struct {
	// read only
	groupName string

	mux           sync.Mutex
	acceptorMap   map[Epoch]*Acceptor
	learner       *Learner
	acceptorProxy *AcceptorProxy
}

type ElectionResult struct {
	termLen           uint64
	startFromInstE    Epoch
	acceptorIDs       []Epoch
	acceptorAddrHints []NetworkAddr
}

type NetworkAddr struct {
	flag string // tcp, udp ...
	ip   net.IP
	port uint16
}

type AcceptValue struct {
	// id of accept value, uniq inside one paxos instance
	id Epoch

	// 1st idx is always the election result idx
	//    if len == 0 means the elecion result is invalid
	memberIdxs vpb.AcceptValueMemberIdxs
	bodyBs     []byte
}

func (v *AcceptValue) GetMemberByIdx(idx vpb.AcceptValueMemberIdx) []byte {
	if idx.Len == 0 {
		return nil
	}
	startFrom := util.Int32ToIntAssert(idx.Offset)
	endAt := util.Int32ToIntAssert(idx.Offset) + util.Int32ToIntAssert(idx.Len) - 1
	if startFrom < 0 || startFrom >= len(v.bodyBs) || endAt < 0 || endAt >= len(v.bodyBs) || endAt < startFrom {
		panic("unexpected")
	}
	return v.bodyBs[startFrom : endAt+1]
}

// bs: uint32(ver:0) uint64(id) uint32(lenOfMemberIdxsBs) uint32(lenOfBodyBs) MemberIdxsBs BodyBs
func (v *AcceptValue) Marshal() []byte {
	memberIdxsBs, err := v.memberIdxs.Marshal()
	util.AssertNoErr(err)
	util.AssertTrue(len(memberIdxsBs) > 0)
	lenOfBs := 4 + 8 + 4 + 4 + len(memberIdxsBs) + len(v.bodyBs)
	util.AssertTrue(lenOfBs > 0)
	bs := make([]byte, lenOfBs)
	retBs := bs
	// ver 0
	util.U32SetBs(bs, 0)
	bs = bs[4:]
	// id
	util.U64SetBs(bs, v.id.ToUint64())
	bs = bs[8:]
	// lenOfMemberIdxsBs
	util.U32SetBs(bs, util.IntToUint32Assert(len(memberIdxsBs)))
	bs = bs[4:]
	// lenOfBodyBs
	util.U32SetBs(bs, util.IntToUint32Assert(len(v.bodyBs)))
	bs = bs[4:]
	// MemberIdxsBs
	copy(bs, memberIdxsBs)
	bs = bs[len(memberIdxsBs):]
	// BodyBs
	copy(bs, v.bodyBs)
	bs = bs[len(v.bodyBs):]
	util.AssertTrue(len(bs) == 0)
	return retBs
}

// bs: uint32(ver:0) uint64(id) uint32(lenOfMemberIdxsBs) uint32(lenOfBodyBs) MemberIdxsBs BodyBs
func (v *AcceptValue) GetMinMarshalBufSize() int {
	return 4 + 8 + 4 + 4
}

// bs: uint32(ver:0) uint64(id) uint32(lenOfMemberIdxsBs) uint32(lenOfBodyBs) MemberIdxsBs BodyBs
func (v *AcceptValue) UnMarshal(vBs []byte) error {
	*v = AcceptValue{}
	if len(vBs) < v.GetMinMarshalBufSize() {
		return fmt.Errorf("len of vBs is too short: %d", len(vBs))
	}
	ver := util.BsReadU32(vBs)
	vBs = vBs[4:]
	if ver != 0 {
		return fmt.Errorf("only support version 0 of AcceptValue UnMarshal but got %d", ver)
	}
	id := util.BsReadU64(vBs)
	vBs = vBs[8:]
	lenOfMemberIdxsBs := util.Uint32ToIntAssert(util.BsReadU32(vBs))
	vBs = vBs[4:]
	lenOfBodyBs := util.Uint32ToIntAssert(util.BsReadU32(vBs))
	vBs = vBs[4:]
	if lenOfMemberIdxsBs > len(vBs) {
		return fmt.Errorf("lenOfMemberIdxsBs out of bound")
	}
	memberIdxsBs := vBs[:lenOfMemberIdxsBs]
	err := v.memberIdxs.Unmarshal(memberIdxsBs)
	if err != nil {
		return fmt.Errorf("v.memberIdxs.Unmarshal got an err:%v", err)
	}
	vBs = vBs[lenOfMemberIdxsBs:]
	if lenOfBodyBs > len(vBs) {
		return fmt.Errorf("lenOfBodyBs out of bound")
	}
	bodyBs := vBs[:lenOfBodyBs]
	vBs = vBs[lenOfBodyBs:]
	if len(vBs) != 0 {
		return fmt.Errorf("vBs got some unparsed bytes")
	}
	v.bodyBs = bodyBs
	v.id.SetFromUint64(id)
	return nil
}

type Acceptor struct {
	// acceptor id
	id Epoch
	pg *PaxosGroup
	db logdb.DB

	stateSummary vpb.AcceptorStateSummary
}

func (a *Acceptor) CheckAcceptorStateSummary() error {
	if len(a.stateSummary.AcceptorTermStates) <= 0 {
		return fmt.Errorf("len(a.stateSummary.AcceptorTermStates) <= 0")
	}
	var nextStartFromInstEpochShouldBe uint64
	for i, v := range a.stateSummary.AcceptorTermStates {
		if v == nil {
			return fmt.Errorf("got a nil member at a.stateSummary.AcceptorTermStates[%d]", i)
		}
		if v.ElectionResult == nil {
			return fmt.Errorf("got a nil member at a.stateSummary.AcceptorTermStates[%d].ElectionResult", i)
		}
		if nextStartFromInstEpochShouldBe > 0 {
			if v.StartFromInstE == nextStartFromInstEpochShouldBe {
			} else {
				return fmt.Errorf("v.StartFromInstE != nextStartFromInstEpochShouldBe")
			}
		} else {
			if v.StartFromInstE == 0 {
				return fmt.Errorf("v.StartFromInstE == 0")
			}
		}
		nextStartFromInstEpochShouldBe = util.Uint64AddAssert(v.StartFromInstE, util.Int32ToUint64Assert(v.ElectionResult.TermLen))
		util.AssertTrue(nextStartFromInstEpochShouldBe > 0)
		if len(v.AcceptorInOnePaxosInstanceStateArray) != util.Int32ToIntAssert(v.ElectionResult.TermLen) {
			return fmt.Errorf("len(v.AcceptorInOnePaxosInstanceStateArray) != util.Int32ToIntAssert(v.ElectionResult.TermLen)")
		}
		for onePaxosStateI, onePaxosStateK := range v.AcceptorInOnePaxosInstanceStateArray {
			if onePaxosStateK == nil {
				return fmt.Errorf("got a nil AcceptorInOnePaxosInstanceState at %d,%d", i, onePaxosStateI)
			}
		}
	}
	return nil
}

type AcceptStateInOnePaxosInstance struct {
	p_e     Epoch
	a_v_id  Epoch
	a_v_map map[Epoch]AcceptValue
}

type AcceptorProxy struct {
	pg *PaxosGroup
}

type Learner struct {
	pg *PaxosGroup
}

type Proposer struct {
	// uniq inside Paxos Group
	id Epoch
	pg *PaxosGroup
}

func New(groupName string) *PaxosGroup {
	pg := PaxosGroup{
		groupName: groupName,
	}
	return &pg
}

func (pg *PaxosGroup) InitAcceptorLogDb(logdbDirPath string, startFromInstE uint64, electionResult vpb.ElectionResult,
	acceptorIDMapToNetworkAddr vpb.AcceptorIDMapToNetworkAddr) error {

	var summary vpb.AcceptorStateSummary
	summary.DeleteInstBeforeEpoch = 0
	var state vpb.AcceptorTermState
	{
		state.StartFromInstE = startFromInstE
		state.ElectionResult = &electionResult
		state.AcceptorIDMapToNetworkAddr = &acceptorIDMapToNetworkAddr
		state.AllChosenFlag = false
		state.AcceptorInOnePaxosInstanceStateArray = make([]*vpb.AcceptorInOnePaxosInstanceState, util.Int32ToIntAssert(electionResult.TermLen))
		for idx := range state.AcceptorInOnePaxosInstanceStateArray {
			var onePaxosAcceptorState vpb.AcceptorInOnePaxosInstanceState
			onePaxosAcceptorState.AcceptValueLogdbIdxMap = make(map[uint64]uint64)
			state.AcceptorInOnePaxosInstanceStateArray[idx] = &onePaxosAcceptorState
		}
		state.LogdbIdxOfLastAcceptorTermState = 0
	}
	summary.AcceptorTermStates = make([]*vpb.AcceptorTermState, 1)
	summary.AcceptorTermStates[0] = &state
	summaryBs, err := summary.Marshal()
	if err != nil {
		return err
	}
	util.AssertTrue(len(summaryBs) > 0)
	db, err := logdb.CreateDB(logdbDirPath)
	if err != nil {
		return err
	}
	leftIdx, toAppendIdx := db.GetCurrentIdxRange()
	util.AssertTrue(leftIdx == 1 && toAppendIdx == 1)
	vArray := make([][]byte, 1)
	vArray = append(vArray, summaryBs)
	err = db.AppendAndSync(1, vArray)
	if err != nil {
		return err
	}
	return db.Close()
}

func (pg *PaxosGroup) LoadAcceptorFromLogDb(logdbDirPath string, acceptorID Epoch) (*Acceptor, error) {
	db, err := logdb.OpenDBIfExist(logdbDirPath)
	if err != nil {
		return nil, err
	}
	leftIdx, toAppendIdx := db.GetCurrentIdxRange()
	util.AssertTrue(toAppendIdx >= leftIdx && leftIdx > 0)
	if leftIdx == toAppendIdx || toAppendIdx-1 == 0 {
		db.Close()
		return nil, fmt.Errorf("there is no valid idx in the logdb which path is:%s", logdbDirPath)
	}
	lastAcceptorSummaryIdx := toAppendIdx - 1
	lastAcceptorSummaryBs, err := db.GetValueByIdx(lastAcceptorSummaryIdx)
	var acceptorSummary vpb.AcceptorStateSummary
	err = acceptorSummary.Unmarshal(lastAcceptorSummaryBs)
	if err != nil {
		db.Close()
		return nil, err
	}
	var a Acceptor
	a.id = acceptorID
	a.pg = pg
	a.db = db
	a.stateSummary = acceptorSummary
	err = a.CheckAcceptorStateSummary()
	if err != nil {
		a.db.Close()
		return nil, err
	}
	return &a, nil
}

func (pg *PaxosGroup) AddAcceptor(a *Acceptor) error {
	if a == nil {
		panic("unexpected")
	}
	pg.mux.Lock()
	defer pg.mux.Unlock()
	id := a.id
	_, existFlag := pg.acceptorMap[id]
	if existFlag {
		return fmt.Errorf("Acceptor %d already exist in acceptorMap", id.ToUint64())
	}
	pg.acceptorMap[id] = a
	return nil
}
