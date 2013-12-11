package main

import (
	"lantern/config"
	_ "lantern/proxy"
	"lantern/signaling"
	"runtime"
	"time"
)

func main() {
	//runtime.GOMAXPROCS(runtime.NumCPU())
	runtime.GOMAXPROCS(4)

	if false && !config.IsRootNode() {
		go func() {
			signaling.SendMessage(signaling.Message{
				R: "ox@getlantern.org",
				T: 3,
				D: "Hello World",
			})
			time.Sleep(5000)
		}()
	}

	// TODO: there's probably a cleaner way to do this
	time.Sleep(9999999999999999)
}
