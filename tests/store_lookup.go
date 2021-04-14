package main

import (
	"context"
	"net"

	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
	"github.com/testground/sdk-go/network"
)

func StoreLookup(runenv *runtime.RunEnv) error {
	ctx := context.Background()
	startedState := sync.State("started")
	client := sync.MustBoundClient(ctx, runenv)
	defer client.Close()
	// instantiate a network client; see 'Traffic shaping' in the docs.
	netclient := network.NewClient(client, runenv)
	runenv.RecordMessage("waiting for network initialization")

	// wait for the network to initialize; this should be pretty fast.
	netclient.MustWaitNetworkInitialized(ctx)
	runenv.RecordMessage("network initilization complete")
	if addrs, err := net.InterfaceAddrs(); err==nil {
		for _, addr := range addrs {
			if ip, ok := addr.(*net.IPNet); ok && ip.IP.To4() != nil{
				runenv.RecordMessage("My IP address is %s", ip.IP.String())
			}
		}
	}

	runenv.RecordMessage("Starting experiment")
	seq := client.MustSignalAndWait(ctx, startedState,4)
	runenv.RecordMessage("my sequence ID: %d", seq)

	runenv.RecordMessage("Ending experiment")
	return nil
}