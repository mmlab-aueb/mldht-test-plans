package main

import (
	"github.com/testground/sdk-go/runtime"
)

func StoreLookup(runenv *runtime.RunEnv) error {
	runenv.RecordMessage("Starting experiment")
	runenv.RecordMessage("Ending experiment")
	return nil
}