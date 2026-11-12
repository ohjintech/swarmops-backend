package control

import (
	controlv1 "github.com/swarm-ops/swarmops-contracts/gen/go/swarmops/control/v1"
	dronev1 "github.com/swarm-ops/swarmops-contracts/gen/go/swarmops/drone/v1"
)

type status = controlv1.CommandStatus

const (
	stQueued    = controlv1.CommandStatus_COMMAND_STATUS_QUEUED
	stSent      = controlv1.CommandStatus_COMMAND_STATUS_SENT
	stAcked     = controlv1.CommandStatus_COMMAND_STATUS_ACKED
	stCompleted = controlv1.CommandStatus_COMMAND_STATUS_COMPLETED
	stRejected  = controlv1.CommandStatus_COMMAND_STATUS_REJECTED
	stFailed    = controlv1.CommandStatus_COMMAND_STATUS_FAILED
	stTimedOut  = controlv1.CommandStatus_COMMAND_STATUS_TIMED_OUT
)

func rank(s status) int {
	switch s {
	case stQueued:
		return 1
	case stSent:
		return 2
	case stAcked:
		return 3
	case stCompleted, stRejected, stFailed, stTimedOut:
		return 4
	default:
		return 0
	}
}

// canTransition reports whether a command may move from cur to next.
// Status only moves forward, terminal states are final, and an ack timeout
// only applies to commands the drone never acknowledged. Messages can arrive
// out of order (an ACK can beat the SENT write), so this ordering matters.
func canTransition(cur, next status) bool {
	if next == stTimedOut {
		return cur == stQueued || cur == stSent
	}
	return rank(next) > rank(cur)
}

func statusFromAck(s dronev1.AckStatus) (status, bool) {
	switch s {
	case dronev1.AckStatus_ACK_STATUS_ACCEPTED:
		return stAcked, true
	case dronev1.AckStatus_ACK_STATUS_REJECTED:
		return stRejected, true
	case dronev1.AckStatus_ACK_STATUS_COMPLETED:
		return stCompleted, true
	case dronev1.AckStatus_ACK_STATUS_FAILED:
		return stFailed, true
	default:
		return 0, false
	}
}
