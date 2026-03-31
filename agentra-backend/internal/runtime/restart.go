package runtime

var restartCh = make(chan struct{}, 1)

func RequestRestart() {
	select {
	case restartCh <- struct{}{}:
	default:
	}
}

func RestartCh() <-chan struct{} {
	return restartCh
}
