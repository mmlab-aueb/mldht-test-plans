package main

import (
	"github.com/testground/sdk-go/run"
)

var testCases = map[string]interface{}{
	"dht-case":   DHTTest,
}

func main() {
	run.InvokeMap(testCases)
}
