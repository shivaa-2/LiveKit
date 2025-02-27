// Copyright 2023 LiveKit, Inc.
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

package routing

import (
	"context"
	"sync"
	"time"

	"go.uber.org/atomic"
	"google.golang.org/protobuf/proto"

	"github.com/livekit/protocol/livekit"
	"github.com/livekit/protocol/logger"
)

var _ Router = (*LocalRouter)(nil)

// a router of messages on the same node, basic implementation for local testing
type LocalRouter struct {
	currentNode  LocalNode
	signalClient SignalClient

	lock sync.RWMutex
	// channels for each participant
	requestChannels  map[string]*MessageChannel
	responseChannels map[string]*MessageChannel
	isStarted        atomic.Bool
}

func NewLocalRouter(currentNode LocalNode, signalClient SignalClient) *LocalRouter {
	return &LocalRouter{
		currentNode:      currentNode,
		signalClient:     signalClient,
		requestChannels:  make(map[string]*MessageChannel),
		responseChannels: make(map[string]*MessageChannel),
	}
}

func (r *LocalRouter) GetNodeForRoom(_ context.Context, _ livekit.RoomName) (*livekit.Node, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	node := proto.Clone((*livekit.Node)(r.currentNode)).(*livekit.Node)
	return node, nil
}

func (r *LocalRouter) SetNodeForRoom(_ context.Context, _ livekit.RoomName, _ livekit.NodeID) error {
	return nil
}

func (r *LocalRouter) ClearRoomState(_ context.Context, _ livekit.RoomName) error {
	return nil
}

func (r *LocalRouter) RegisterNode() error {
	return nil
}

func (r *LocalRouter) UnregisterNode() error {
	return nil
}

func (r *LocalRouter) RemoveDeadNodes() error {
	return nil
}

func (r *LocalRouter) GetNode(nodeID livekit.NodeID) (*livekit.Node, error) {
	if nodeID == livekit.NodeID(r.currentNode.Id) {
		return r.currentNode, nil
	}
	return nil, ErrNotFound
}

func (r *LocalRouter) ListNodes() ([]*livekit.Node, error) {
	return []*livekit.Node{
		r.currentNode,
	}, nil
}

func (r *LocalRouter) StartParticipantSignal(ctx context.Context, roomName livekit.RoomName, pi ParticipantInit) (res StartParticipantSignalResults, err error) {
	return r.StartParticipantSignalWithNodeID(ctx, roomName, pi, livekit.NodeID(r.currentNode.Id))
}

func (r *LocalRouter) StartParticipantSignalWithNodeID(ctx context.Context, roomName livekit.RoomName, pi ParticipantInit, nodeID livekit.NodeID) (res StartParticipantSignalResults, err error) {
	connectionID, reqSink, resSource, err := r.signalClient.StartParticipantSignal(ctx, roomName, pi, nodeID)
	if err != nil {
		logger.Errorw("could not handle new participant", err,
			"room", roomName,
			"participant", pi.Identity,
			"connID", connectionID,
		)
	} else {
		return StartParticipantSignalResults{
			ConnectionID:   connectionID,
			RequestSink:    reqSink,
			ResponseSource: resSource,
			NodeID:         nodeID,
		}, nil
	}
	return
}

func (r *LocalRouter) Start() error {
	if r.isStarted.Swap(true) {
		return nil
	}
	go r.statsWorker()
	// go r.memStatsWorker()
	return nil
}

func (r *LocalRouter) Drain() {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.currentNode.State = livekit.NodeState_SHUTTING_DOWN
}

func (r *LocalRouter) Stop() {}

func (r *LocalRouter) GetRegion() string {
	return r.currentNode.Region
}

func (r *LocalRouter) statsWorker() {
	for {
		if !r.isStarted.Load() {
			return
		}
		// update every 10 seconds
		<-time.After(statsUpdateInterval)
		r.lock.Lock()
		r.currentNode.Stats.UpdatedAt = time.Now().Unix()
		r.lock.Unlock()
	}
}

/*
	func (r *LocalRouter) memStatsWorker() {
		ticker := time.NewTicker(time.Second * 30)
		defer ticker.Stop()

		for {
			<-ticker.C

			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			logger.Infow("memstats",
				"mallocs", m.Mallocs, "frees", m.Frees, "m-f", m.Mallocs-m.Frees,
				"hinuse", m.HeapInuse, "halloc", m.HeapAlloc, "frag", m.HeapInuse-m.HeapAlloc,
			)
		}
	}
*/
