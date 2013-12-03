package main

import (
	_ "lantern/proxy"
	_ "lantern/signaling"
	"runtime"
	"time"
)

func main() {
	//runtime.GOMAXPROCS(runtime.NumCPU())
	runtime.GOMAXPROCS(4)
	
	// TODO: there's probably a cleaner way to do this
	time.Sleep(9999999999999999)
}
