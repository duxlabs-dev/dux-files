package ipc

import (
	"cmp"
	"slices"

	dankipc "github.com/duxlabs-dev/dux-go/ipc"
)

const APIVersion = 1

type (
	Request    = dankipc.Request
	ConnWriter = dankipc.ConnWriter
	Subscriber = dankipc.Subscriber
	EventBus   = dankipc.EventBus
)

var (
	NewEventBus   = dankipc.NewEventBus
	NewConnWriter = dankipc.NewConnWriter
	Dial          = dankipc.Dial
)

func FindRunningSocket() (string, error) { return dankipc.FindRunningSocket("dfiles") }

func newestFirst[T any](items []T, key func(T) (stamp int64, name string)) {
	slices.SortStableFunc(items, func(a, b T) int {
		aStamp, aName := key(a)
		bStamp, bName := key(b)
		return cmp.Or(cmp.Compare(bStamp, aStamp), cmp.Compare(aName, bName))
	})
}
