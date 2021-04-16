package main

import (
	"context"
	"net"

	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
	"github.com/testground/sdk-go/network"
)

type NodeInfo struct
{
	IP     *net.IP
}

var node_info_topic = sync.NewTopic("nodeinfo", &NodeInfo{})

func StoreLookup(runenv *runtime.RunEnv) error {
	ctx := context.Background()
	client := sync.MustBoundClient(ctx, runenv)
	defer client.Close()

	startedState := sync.State("started")
	// instantiate a network client; see 'Traffic shaping' in the docs.
	netclient := network.NewClient(client, runenv)
	runenv.RecordMessage("waiting for network initialization")

	// wait for the network to initialize; this should be pretty fast.
	netclient.MustWaitNetworkInitialized(ctx)
	runenv.RecordMessage("network initilization complete")
	ip := netclient.MustGetDataNetworkIP()
	runenv.RecordMessage("IP address: %s", ip)
	//publishing my information
	node_info_channel := make(chan *NodeInfo)
	_, _ = client.MustPublishSubscribe(ctx, node_info_topic, &NodeInfo{&ip}, node_info_channel)
	runenv.RecordMessage("Starting experiment")
	//wait until all nodes have reached this state
	seq := client.MustSignalAndWait(ctx, startedState,4)
	runenv.RecordMessage("my sequence ID: %d", seq)
	//read the entries published in the channel
	for i := 0; i < 4; i++ {
		entry := <-node_info_channel
		runenv.RecordMessage("Received from channel %s", entry.IP.String())
	}

	runenv.RecordMessage("Ending experiment")
	return nil
}