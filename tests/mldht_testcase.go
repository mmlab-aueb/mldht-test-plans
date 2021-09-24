package main

import (
	"context"
	"fmt"
	"time"
	"math/rand"
	gosync "sync"

	"github.com/testground/sdk-go/runtime"
	"github.com/testground/sdk-go/sync"
	"github.com/testground/sdk-go/network"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/peer"
	manet "github.com/multiformats/go-multiaddr-net"
	"github.com/multiformats/go-multiaddr"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/ipfs/go-cid"
	u "github.com/ipfs/go-ipfs-util"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/ipfs/go-datastore"

	logging "github.com/ipfs/go-log"
)

func MLDHTTest(runenv *runtime.RunEnv) error {

	logging.SetLogLevel("dht", "INFO")
	ctx         := context.Background()
	synClient   := sync.MustBoundClient(ctx, runenv)
	defer synClient.Close()

	type NodeInfo struct
	{
		Addr      *peer.AddrInfo //<- Be careful, variable name must start with capital
		ClusterId string
		BootSeq   int64
	}
	type ItemInfo struct
	{
		ItemCid cid.Cid //<- Be careful, variable name must start with capital
	}

	var node_info_topic     = sync.NewTopic("nodeinfo", &NodeInfo{})
	var item_info_topic     = sync.NewTopic("iteminfo", &ItemInfo{})
	libp2pInitialized      := sync.State("libp2p-init-completed")
	nodeBootstrapCompleted := sync.State("bootstrap-completed")
	dhtBootstrapCompleted  := sync.State("bootstrap-completed")
	experimentCompleted    := sync.State("experiment-completed")
	totalNodes             := runenv.TestInstanceCount
	totalItems             := totalNodes
	itemsToFind            := runenv.IntParam("items_to_find")
	clusterId              := runenv.StringParam("cluster_id")
	mapMutex               := gosync.RWMutex{}
	learnedPeersperKey     := make(map[string]map[string]int) //<-user for counting hops per key
	localNodes             := make(map[string]bool) //<- it used for stroring nodes of our cluster
	if !runenv.TestSidecar {
		runenv.RecordMessage("Sidecar is not available, abandoning...")
		return nil
	}
	netClient := network.NewClient(synClient, runenv)
	err := netClient.WaitNetworkInitialized(ctx)
	if err != nil {
		runenv.RecordMessage("Error in seting up network %s", err)
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
	mutliAddr = mutliAddr.Encapsulate(multiaddr.StringCast("/tcp/0"))
	libp2pNode, err := libp2p.New(ctx,
		libp2p.Identity(priv),		
		libp2p.DefaultTransports,		
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
		return err
	}

	kademliaDHT.LocalNodes = localNodes
	runenv.RecordMessage("libp2p initilization complete")
	seq := synClient.MustSignalAndWait(ctx, libp2pInitialized,totalNodes)
	
	/*
	 Announce node
	*/
	addrInfo := host.InfoFromHost(libp2pNode)
	runenv.RecordMessage("Node Id %s Cluster Id %s Bootseq %d", addrInfo, clusterId, seq)
	synClient.Publish(ctx, node_info_topic,  &NodeInfo{addrInfo, clusterId, seq})
	bootstaInfoChannel := make(chan *NodeInfo)
	synClient.Subscribe(ctx, node_info_topic,  bootstaInfoChannel)
	firstInGroup := true
	var bootstrapNode *NodeInfo
	for i := 0; i < totalNodes; i++ {
		node := <-bootstaInfoChannel
		if node.ClusterId == clusterId {//added to local nodes
			runenv.RecordMessage("Adding new local node %s", string(node.Addr.ID))
			localNodes[string(node.Addr.ID)] = true
		}
		if node.BootSeq == 1 && firstInGroup{
			bootstrapNode = node
		}
		if node.ClusterId == clusterId && node.BootSeq < seq { //will bootstrap from a group node
			firstInGroup = false
			bootstrapNode = node
		}
	}
	
	/*
	 Bootstap DHT
	*/		
	if seq == 1 {//the first nodes is assumed to be the bootstrap node
		synClient.MustSignalEntry(ctx, nodeBootstrapCompleted)
	} else{
		//Bootstrap one by one
		<-synClient.MustBarrier(ctx, nodeBootstrapCompleted, int(seq-1)).C
		runenv.RecordMessage("Node %d from group %s will bootstrap from %s from group %s", seq, clusterId, bootstrapNode.Addr, bootstrapNode.ClusterId)
		if err := kademliaDHT.Host().Connect(ctx, *bootstrapNode.Addr); err != nil {
			runenv.RecordMessage("Error in bootstraping %s", err)
			return err
		}
		time.Sleep(time.Second * 3)
		synClient.MustSignalEntry(ctx, nodeBootstrapCompleted)	
	}
	synClient.MustSignalAndWait(ctx, dhtBootstrapCompleted,totalNodes)

	time.Sleep(time.Second * 2)

		
	/*
	 Listen for Lookup Events. They are used for tracking the number of hops
	*/
	ectx, dhtlkevents := dht.RegisterForLookupEvents(ctx)
	go func() {
		for e := range dhtlkevents {
			mapMutex.Lock()
			response := e.Response
			if  response != nil {
				/*
				runenv.RecordMessage("...Received DHT Lookup Response")
				runenv.RecordMessage("......Key: %x ",e.Key.Key)
				runenv.RecordMessage("......Query id: %s", e.ID)
				runenv.RecordMessage("......Source: %s", response.Source.Peer)
				runenv.RecordMessage("......Cause: %s", response.Cause.Peer)
				runenv.RecordMessage("......Learned:")
				*/
				for _,node := range response.Heard {
					//runenv.RecordMessage(".........: %s", node.Peer)
					//see if we already know this node
					_, exists := learnedPeersperKey[e.Key.Key][string(node.Peer)]
					//if we know it move to the next
					if exists {
						continue
					}
					hops := 1
					//If the cause is not us, add the number of hops until cause
					hopsToCause, exists := learnedPeersperKey[e.Key.Key][string(response.Cause.Peer)]
					if exists { // Otherwise, the cause is ourself
						hops = hopsToCause + 1
					}
					learnedPeersperKey[e.Key.Key][string(node.Peer)] = hops
				}
				if len(response.Queried) > 0 {
					//node := response.Queried[0]
					//runenv.RecordMessage("......Queried:")
					//runenv.RecordMessage(".........: %s", node.Peer)
					if len(response.Heard) == 0 { //this node gave us the response
						hopsToCause := learnedPeersperKey[e.Key.Key][string(response.Cause.Peer)]
						learnedPeersperKey[e.Key.Key]["provider"] = hopsToCause
					}
				}
			}
			mapMutex.Unlock()
		}	
	}()

		
	/*
	 Create records and announce that you can provide them
	*/
	//runenv.RecordMessage("Routing table size %d", kademliaDHT.RoutingTable().Size())
	packet := fmt.Sprintf("Hello from %s", addrInfo)
	cid    := cid.NewCidV0(u.Hash([]byte(packet)))
	//Announce in the DHT
	err = kademliaDHT.Provide(ctx, cid, true)
	if err != nil {
		runenv.RecordMessage("Error in providing record %s", err)
		return err
	}
	synClient.Publish(ectx, item_info_topic,  &ItemInfo{cid})
	itemInfoChannel := make(chan *ItemInfo)
	synClient.Subscribe(ectx, item_info_topic,  itemInfoChannel)
	//We consider one item per node
	item:= make([]*ItemInfo, totalItems)
	for i := 0; i < totalItems; i++ {
		item[i] = <- itemInfoChannel
    }

	/*
	 Find providers of records
	*/
	recordsFound   := 0
	recordsMissed  := 0
	hopsToProvider := 0
	for i:=0; i< itemsToFind; i++ {
		index := rand.Intn(totalItems)
		keyMH :=  item[index].ItemCid.Hash()
		mapMutex.Lock()
		learnedPeersperKey[string(keyMH)] = make(map[string]int)
		mapMutex.Unlock()
		provChan := kademliaDHT.FindProvidersAsync(ectx, item[index].ItemCid, 1)
		//peer ,ok :=<-provChan
		_,ok :=<-provChan
		if ok {
			//Let's wait some time because NodesLookUpEvent seems to arrive after
			time.Sleep(time.Second * 1)
			mapMutex.RLock()
			hops, exists := learnedPeersperKey[string(keyMH)]["provider"]
			mapMutex.RUnlock()
			if exists {//Otherwise it means that the provider was in local cache
				hopsToProvider += hops
			}
			//runenv.RecordMessage("Found provider %s for %x in %d hops", peer.ID, keyMH, hops)
			recordsFound++
			
		}else{
			recordsMissed++
			runenv.RecordMessage("Error, cannot find record")
		    break;
		}
		
	}
	
	/*
	 Record statistics
	*/
	runenv.R().RecordPoint("routing-table-size", float64(kademliaDHT.RoutingTable().Size()))
	runenv.R().RecordPoint("records-found", float64(recordsFound))
	runenv.R().RecordPoint("hops-to-provider", float64(hopsToProvider)/float64(recordsFound))

	/*
	 Finish experiment
	*/
	synClient.MustSignalAndWait(ctx, experimentCompleted,totalNodes)
	runenv.RecordMessage("Ending test case")
	return nil
}
