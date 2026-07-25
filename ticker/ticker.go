package ticker

import "time"

func Start(period time.Duration, action func()) {
	if period <= 0 {
		panic("ticker period must be greater than zero")
	}

	if action == nil {
		panic("ticker action must not be nil")
	}

	action()

	timer := time.NewTicker(period)
	defer timer.Stop()

	for range timer.C {
		action()
	}
}
