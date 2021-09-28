module github.com/mmlab-aueb/mldht-test-plans/tests

go 1.16

require (
	github.com/ipfs/go-cid v0.0.7
	github.com/ipfs/go-datastore v0.4.5
	github.com/ipfs/go-ipfs-util v0.0.2
	github.com/ipfs/go-log v1.0.5
	github.com/libp2p/go-libp2p v0.14.4
	github.com/libp2p/go-libp2p-core v0.8.6
	github.com/libp2p/go-libp2p-kad-dht v0.12.1
	github.com/multiformats/go-multiaddr v0.3.3
	github.com/multiformats/go-multiaddr-net v0.2.0
	github.com/testground/sdk-go v0.2.7
)

//replace github.com/libp2p/go-libp2p-kad-dht => github.com/mmlab-aueb/go-libp2p-kad-dht v1.0.0
//replace github.com/libp2p/go-libp2p-kad-dht => /plan/go-libp2p-kad-dht
replace github.com/libp2p/go-libp2p-kad-dht => /home/mmlab/mldht-test-plans/tests/go-libp2p-kad-dht
