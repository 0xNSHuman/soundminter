package minter

import (
	"sync"

	"github.com/0xNSHuman/dapp-tools/schedule"
)

var once sync.Once = sync.Once{}
var sharedScheduler *schedule.Scheduler

func Scheduler() *schedule.Scheduler {
	once.Do(
		func() {
			sharedScheduler = schedule.NewScheduler()
		},
	)

	return sharedScheduler
}
