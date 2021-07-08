package main

import (
	"context"
	"net"
	"fmt"
	"time"
	//"sync"
	//"math/rand"

	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
	"github.com/testground/sdk-go/network"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/peer"
	manet "github.com/multiformats/go-multiaddr-net"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/ipfs/go-log/v2"
	"github.com/ipfs/go-cid"
	u "github.com/ipfs/go-ipfs-util"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-core/crypto"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
	tcp "github.com/libp2p/go-tcp-transport"
	"github.com/ipfs/go-datastore"
)

type NodeInfo struct
{
	Addr *peer.AddrInfo //<- Be careful, variable name must start with capital
}
type ItemInfo struct
{
	ItemCid cid.Cid //<- Be careful, variable name must start with capital
}

var node_info_topic = sync.NewTopic("nodeinfo", &NodeInfo{})
var item_info_topic = sync.NewTopic("iteminfo", &ItemInfo{})

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

	libp2pInitialized      := sync.State("libp2p-init-completed")
	nodeBootstrapCompleted := sync.State("bootstrap-completed")
	dhtBootstrapCompleted  := sync.State("bootstrap-completed")
	experimentCompleted    := sync.State("experiment-completed")
	totalNodes             := runenv.TestInstanceCount

	// instantiate a network client; see 'Traffic shaping' in the docs.
	netClient := network.NewClient(synClient, runenv)
	runenv.RecordMessage("waiting for network initialization")
	netClient.MustWaitNetworkInitialized(ctx)
	
	/*
	Configure libp2p
	*/
	tcpAddr, err    := getSubnetAddr(runenv)
	mutliAddr, err  := manet.FromNetAddr(tcpAddr)
	priv, _, err := crypto.GenerateKeyPair(
		crypto.Ed25519, // Select your key type. Ed25519 are nice short
		-1,             // Select key length when possible (i.e. RSA).
	)
	libp2pNode, err := libp2p.New(ctx,
		libp2p.Identity(priv),		
		libp2p.Transport(func(u *tptu.Upgrader) *tcp.TcpTransport {
			tpt := tcp.NewTCPTransport(u)
			tpt.DisableReuseport = true
			return tpt
		}),		
		libp2p.EnableNATService(), 
		libp2p.ForceReachabilityPublic(),
		libp2p.ListenAddrs(mutliAddr),
	)

	dhtOptions := []dht.Option{
		dht.ProtocolPrefix("/testground"),
		dht.Datastore(datastore.NewMapDatastore()),
	}
	
	kademliaDHT, err := dht.New(ctx, libp2pNode, dhtOptions...)
	if err!= nil {
		runenv.RecordMessage("Error in seting up Kadmlia %s", err)
	}
	
	runenv.RecordMessage("libp2p initilization complete")
	seq := synClient.MustSignalAndWait(ctx, libp2pInitialized,totalNodes)
	
	/*
	Synchronize nodes
	*/
	addrInfo := host.InfoFromHost(libp2pNode)
	if seq==1 { // I am the bootstrap node, publish
		synClient.Publish(ctx, node_info_topic,  &NodeInfo{addrInfo})
	}
	bootstap_info_channel := make(chan *NodeInfo)
	synClient.Subscribe(ctx, node_info_topic,  bootstap_info_channel)
	bootstrap_node := <-bootstap_info_channel
	
	/*
	Bootstap nodes
	*/		
	if seq == 1 {//the first nodes is assumed to be the bootstrap node
		synClient.MustSignalEntry(ctx, nodeBootstrapCompleted)
	} else{
		//Bootstrap one by one
		<-synClient.MustBarrier(ctx, nodeBootstrapCompleted, int(seq-1)).C
		runenv.RecordMessage("Node %d will bootstrap from %s", seq, bootstrap_node.Addr)
		if err := kademliaDHT.Host().Connect(ctx, *bootstrap_node.Addr); err != nil {
			runenv.RecordMessage("Error in connecting %s", err)
		} else {
			runenv.RecordMessage("Connection established")
		}
		synClient.MustSignalEntry(ctx, nodeBootstrapCompleted)	
	}
	synClient.MustSignalAndWait(ctx, dhtBootstrapCompleted,totalNodes)
	

	/*
	Create records and announce that you can provide them
	*/
	time.Sleep(time.Second * 20)
	runenv.RecordMessage("Routing table size %d", kademliaDHT.RoutingTable().Size())
	packet := fmt.Sprintf("Hello from %s", addrInfo)
	cid := cid.NewCidV0(u.Hash([]byte(packet)))
	//Announce in the DHT
	err = kademliaDHT.Provide(ctx, cid, true)
	if err == nil {
		runenv.RecordMessage("Provided CID: %s", cid)
	} else {
		panic(err)
	}
	synClient.Publish(ctx, item_info_topic,  &ItemInfo{cid})
	item_info_channel := make(chan *ItemInfo)
	synClient.Subscribe(ctx, item_info_topic,  item_info_channel)
	//We consider one item per node
	for i := 0; i < totalNodes; i++ {
		item:= <- item_info_channel
        runenv.RecordMessage("Learned Item %s", item.ItemCid)
    }

	/*
	Finish experiment
	*/
	synClient.MustSignalAndWait(ctx, experimentCompleted,totalNodes)
	runenv.RecordMessage("Ending test case")
	return nil
}