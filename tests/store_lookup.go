package main

import (
	"context"
	"net"
	"fmt"

	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
	"github.com/testground/sdk-go/network"
	"github.com/libp2p/go-libp2p"
	manet "github.com/multiformats/go-multiaddr-net"

	"github.com/ipfs/go-log/v2"
)

type NodeInfo struct
{
	IP     *net.IP
}

var node_info_topic = sync.NewTopic("nodeinfo", &NodeInfo{})

func getSubnetAddr(runenv *runtime.RunEnv) (*net.TCPAddr, error) {
	log.SetAllLoggers(log.LevelWarn)
	subnet := runenv.TestSubnet
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	for _, addr := range addrs {
		if ip, ok := addr.(*net.IPNet); ok {
			if subnet.Contains(ip.IP) {
				tcpAddr := &net.TCPAddr{IP: ip.IP}
				return tcpAddr, nil
			}
		} else {
			panic(fmt.Sprintf("%T", addr))
		}
	}
	return nil, fmt.Errorf("no network interface found. Addrs: %v", addrs)
}

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
	tcpAddr, err := getSubnetAddr(runenv)
	addr, err := manet.FromNetAddr(tcpAddr)
	host, err := libp2p.New(ctx,
		libp2p.ListenAddrs(addr),
	)
	if err != nil {
		panic(err)
	}
	runenv.RecordMessage("Host created. We are:", host.ID())

	runenv.RecordMessage("Ending experiment")
	return nil
}