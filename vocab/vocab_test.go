package vocab_test

import (
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/vocab"
)

type accountType string

const (
	accountAsset     accountType = "asset"
	accountLiability accountType = "liability"
	accountOffLedger accountType = "off_ledger"
)

func TestSet_Contains(t *testing.T) {
	valid := vocab.NewSet(accountAsset, accountLiability)

	if !valid.Contains(accountAsset) {
		t.Errorf("Contains(%q) = false, want true", accountAsset)
	}
	if valid.Contains(accountOffLedger) {
		t.Errorf("Contains(%q) = true, want false", accountOffLedger)
	}
	if valid.Contains("") {
		t.Error("Contains(\"\") = true, want false")
	}
}

type taskStatus string

const (
	taskDraft     taskStatus = "draft"
	taskRunning   taskStatus = "running"
	taskCompleted taskStatus = "completed"
	taskFailed    taskStatus = "failed"
)

func TestTransitions_CanTransitionTo(t *testing.T) {
	trans := vocab.NewTransitions(map[taskStatus][]taskStatus{
		taskDraft:   {taskRunning},
		taskRunning: {taskCompleted, taskFailed},
	})

	cases := []struct {
		from, to taskStatus
		want     bool
	}{
		{taskDraft, taskRunning, true},
		{taskDraft, taskCompleted, false},
		{taskRunning, taskCompleted, true},
		{taskRunning, taskFailed, true},
		{taskCompleted, taskRunning, false},
		{taskFailed, taskRunning, false},
	}

	for _, c := range cases {
		if got := trans.CanTransitionTo(c.from, c.to); got != c.want {
			t.Errorf("CanTransitionTo(%q, %q) = %v, want %v", c.from, c.to, got, c.want)
		}
	}
}
