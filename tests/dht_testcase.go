package main

import (
	"context"
	"net"
	"fmt"

	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
	"github.com/testground/sdk-go/network"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/peer"
	manet "github.com/multiformats/go-multiaddr-net"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/ipfs/go-log/v2"
)

type NodeInfo struct
{
	Addr *peer.AddrInfo //<- Be careful, variable name must start with capital
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

func DHTTest(runenv *runtime.RunEnv) error {
	runenv.RecordMessage("Starting test case")
	ctx    := context.Background()
	synClient := sync.MustBoundClient(ctx, runenv)
	defer synClient.Close()

	libp2pInitialized   := sync.State("libp2p-init-completed")
	bootstrapCompleted  := sync.State("bootstrap-completed")
	experimentCompleted := sync.State("experiment-completed")
	totalNodes          := runenv.TestInstanceCount

	// instantiate a network client; see 'Traffic shaping' in the docs.
	netClient := network.NewClient(synClient, runenv)
	runenv.RecordMessage("waiting for network initialization")
	netClient.MustWaitNetworkInitialized(ctx)
	
	/*
	Configure libp2p
	*/
	tcpAddr, err    := getSubnetAddr(runenv)
	mutliAddr, err  := manet.FromNetAddr(tcpAddr)
	libp2pNode, err := libp2p.New(ctx,
		libp2p.ListenAddrs(mutliAddr),
	)
	addrInfo := host.InfoFromHost(libp2pNode)
	runenv.RecordMessage("libp2p initilization complete")
	seq := synClient.MustSignalAndWait(ctx, libp2pInitialized,totalNodes)
	
	/*
	Synchronize nodes
	*/
	if seq==1 { // I am the bootstrap node, publish
		synClient.Publish(ctx, node_info_topic,  &NodeInfo{addrInfo})
	}
	bootstap_info_channel := make(chan *NodeInfo)
	synClient.Subscribe(ctx, node_info_topic,  bootstap_info_channel)
	bootstrap_node := <-bootstap_info_channel
	runenv.RecordMessage("Received from channel %s", bootstrap_node.Addr)

	/*
	Bootstap nodes
	*/	
	if seq == 1 {//the first nodes is assumed to be the bootstrap node
		synClient.MustSignalEntry(ctx, bootstrapCompleted)
	} else{
		//Bootstrap one by one
		<-synClient.MustBarrier(ctx, bootstrapCompleted, int(seq-1)).C
		runenv.RecordMessage("Node %d will bootstrap from %s", seq, bootstrap_node.Addr)
		if err := libp2pNode.Connect(ctx, *bootstrap_node.Addr); err != nil {
			runenv.RecordMessage("Error in connecting %s", err)
		} else {
			runenv.RecordMessage("Connection established")
		}
		synClient.MustSignalEntry(ctx, bootstrapCompleted)	
	}
	if err != nil {
		panic(err)
	}

	synClient.MustSignalAndWait(ctx, experimentCompleted,totalNodes)
	runenv.RecordMessage("Ending test case")
	return nil
}