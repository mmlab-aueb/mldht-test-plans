package main

import (
	"context"
	//"net"
	"fmt"
	"time"
	//"sync"
	"math/rand"

	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
	"github.com/testground/sdk-go/network"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/peer"
	manet "github.com/multiformats/go-multiaddr-net"
	"github.com/multiformats/go-multiaddr"
	"github.com/libp2p/go-libp2p-core/host"
	//"github.com/ipfs/go-log/v2"
	"github.com/ipfs/go-cid"
	u "github.com/ipfs/go-ipfs-util"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-core/crypto"
	//tptu "github.com/libp2p/go-libp2p-transport-upgrader"
	//tcp "github.com/libp2p/go-tcp-transport"
	"github.com/ipfs/go-datastore"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
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

func DHTTest(runenv *runtime.RunEnv) error {
	timeout     := time.Duration(runenv.IntParam("timeout_secs"))*time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	synClient   := sync.MustBoundClient(ctx, runenv)
	defer cancel()
	defer synClient.Close()

	libp2pInitialized      := sync.State("libp2p-init-completed")
	nodeBootstrapCompleted := sync.State("bootstrap-completed")
	dhtBootstrapCompleted  := sync.State("bootstrap-completed")
	experimentCompleted    := sync.State("experiment-completed")
	totalNodes             := runenv.TestInstanceCount
	totalItems             := totalNodes
	itemsToFind            := runenv.IntParam("items_to_find")

	if !runenv.TestSidecar {
		runenv.RecordMessage("Sidecar is not available, abandoning...")
		return nil
	}
	netClient := network.NewClient(synClient, runenv)
	err := netClient.WaitNetworkInitialized(ctx)
	if err != nil {
		return err
	}
	
	/*
	Configure libp2p
	*/
	ipaddr, err     := netClient.GetDataNetworkIP()                
	mutliAddr, err  := manet.FromIP(ipaddr)
	priv, _, err    := crypto.GenerateKeyPair(
		crypto.Ed25519, // Select your key type. Ed25519 are nice short
		-1,             // Select key length when possible (i.e. RSA).
	)
	libp2pNode, err := libp2p.New(ctx,
		libp2p.Identity(priv),		
		//libp2p.Transport(func(u *tptu.Upgrader) *tcp.TcpTransport {
			//tpt := tcp.NewTCPTransport(u)
			//tpt.DisableReuseport = true
			//return tpt
		//}),
		libp2p.DefaultTransports,		
		libp2p.EnableNATService(), 
		libp2p.ForceReachabilityPublic(),
		libp2p.ListenAddrs(mutliAddr.Encapsulate(multiaddr.StringCast("/tcp/0"))),
		libp2p.ConnectionManager(connmgr.NewConnManager(
			400,         // Lowwater
			600,         // HighWater,
			time.Minute, // GracePeriod
		)),
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
	 Announce boostrap node
	*/
	addrInfo := host.InfoFromHost(libp2pNode)
	if seq==1 { // I am the bootstrap node, publish
		synClient.Publish(ctx, node_info_topic,  &NodeInfo{addrInfo})
	}
	bootstaInfoChannel := make(chan *NodeInfo)
	synClient.Subscribe(ctx, node_info_topic,  bootstaInfoChannel)
	bootstrapNode := <-bootstaInfoChannel
	
	/*
	 Bootstap DHT
	*/		
	if seq == 1 {//the first nodes is assumed to be the bootstrap node
		synClient.MustSignalEntry(ctx, nodeBootstrapCompleted)
	} else{
		//Bootstrap one by one
		<-synClient.MustBarrier(ctx, nodeBootstrapCompleted, int(seq-1)).C
		runenv.RecordMessage("Node %d will bootstrap from %s", seq, bootstrapNode.Addr)
		if err := kademliaDHT.Host().Connect(ctx, *bootstrapNode.Addr); err != nil {
			runenv.RecordMessage("Error in connecting %s", err)
		}
		time.Sleep(time.Second * 3)
		synClient.MustSignalEntry(ctx, nodeBootstrapCompleted)	
	}
	synClient.MustSignalAndWait(ctx, dhtBootstrapCompleted,totalNodes)
	
	/*
	 Create records and announce that you can provide them
	*/
	time.Sleep(time.Second * 20)
	runenv.RecordMessage("Routing table size %d", kademliaDHT.RoutingTable().Size())
	packet := fmt.Sprintf("Hello from %s", addrInfo)
	cid    := cid.NewCidV0(u.Hash([]byte(packet)))
	//Announce in the DHT
	err = kademliaDHT.Provide(ctx, cid, true)
	if err == nil {
		runenv.RecordMessage("Provided CID: %s", cid)
	} else {
		panic(err)
	}
	synClient.Publish(ctx, item_info_topic,  &ItemInfo{cid})
	itemInfoChannel := make(chan *ItemInfo)
	synClient.Subscribe(ctx, item_info_topic,  itemInfoChannel)
	//We consider one item per node
	item:= make([]*ItemInfo, totalItems)
	for i := 0; i < totalItems; i++ {
		item[i] = <- itemInfoChannel
        runenv.RecordMessage("Learned Item %s", item[i].ItemCid)
    }

	/*
	 Find providers of records
	*/
	for i:=0; i< itemsToFind; i++ {
		index := rand.Intn(totalItems)
		provChan := kademliaDHT.FindProvidersAsync(ctx, item[index].ItemCid, 1)
		provider :=<-provChan
		runenv.RecordMessage("Found provider for %s node %s", item[index].ItemCid, provider)
	}
	
	/*
	 Finish experiment
	*/
	synClient.MustSignalAndWait(ctx, experimentCompleted,totalNodes)
	runenv.RecordMessage("Ending test case")
	return nil
}