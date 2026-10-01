package service

import (
	"errors"
	"time"
)

// errUniqueRace is returned inside a transaction when an INSERT loses a race
// against a unique index. The caller must roll back and retry the whole
// operation with a fresh transaction snapshot (under REPEATABLE READ the
// losing snapshot cannot see the winner's just-committed row).
var errUniqueRace = errors.New("unique index race, retry with fresh snapshot")

const (
	// uniqueRaceRetries bounds whole-transaction restarts on unique races.
	uniqueRaceRetries = 10
	// planWaitRetries/planWaitInterval bound how long a losing request waits
	// for the winning device's commit to become readable (~1s total).
	planWaitRetries  = 20
	planWaitInterval = 50 * time.Millisecond
)
