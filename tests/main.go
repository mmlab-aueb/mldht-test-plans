package main

import (
	"github.com/testground/sdk-go/run"
)

var testCases = map[string]interface{}{
	"store-lookup":   StoreLookup,
}

func main() {
	run.InvokeMap(testCases)
}
