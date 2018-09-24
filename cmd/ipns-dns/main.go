package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	blocks "github.com/ipfs/go-block-format"
	datastore "github.com/ipfs/go-datastore"
	ipns "github.com/ipfs/go-ipns"
	floodsub "github.com/libp2p/go-floodsub"
	libp2p "github.com/libp2p/go-libp2p"
	p2phost "github.com/libp2p/go-libp2p-host"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	dhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	record "github.com/libp2p/go-libp2p-record"
	routing "github.com/libp2p/go-libp2p-routing"
	maddr "github.com/multiformats/go-multiaddr"
	// dns "github.com/miekg/dns"
)

const Topic = "/ipns/.well-known/all"

var Bootstrap = []string{
	// "/ip4/104.236.179.241/tcp/4001/ipfs/QmSoLPppuBtQSGwKDZT2M73ULpjvfd3aZ6ha4oFGL1KrGM",
	// "/ip4/104.236.76.40/tcp/4001/ipfs/QmSoLV4Bbm51jM9C4gDYZQ9Cy3U6aXMJDAbzgu2fzaDs64",
	// "/ip4/128.199.219.111/tcp/4001/ipfs/QmSoLSafTMBsPKadTEgaXctDQVcqN88CNLHXMkTNwMKPnu",
	"/ip4/178.62.158.247/tcp/4001/ipfs/QmSoLer265NRgSp2LA3dPaeykiS1J6DifTC88f5uVQKNAd",
}

type Resolver struct {
	Host    p2phost.Host
	PubSub  *floodsub.PubSub
	Routing routing.IpfsRouting

	IpnsSub *floodsub.Subscription
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	host, err := libp2p.New(ctx)
	if err != nil {
		panic(err)
	}

	r, err := NewResolver(ctx, host)
	if err != nil {
		panic(err)
	}
	if err = r.Bootstrap(ctx); err != nil {
		panic(err)
	}

	fmt.Printf("bootstrapped: ok\n")

	go r.ReceiveUpdates(ctx)
	go r.ServeDNS(ctx)
	go r.ServeHTTP(ctx)

	go func() {
		for range time.Tick(10 * time.Second) {
			fmt.Printf("total peers: %d\n", len(host.Network().Conns()))
			fmt.Printf("topic peers: %d\n", len(r.PubSub.ListPeers(Topic)))
		}
	}()

	select {}
}

func NewResolver(ctx context.Context, host p2phost.Host) (*Resolver, error) {
	r := &Resolver{Host: host}

	pubsub, err := floodsub.NewGossipSub(ctx, host)
	if err != nil {
		return nil, err
	}

	ds := datastore.NewMapDatastore()
	validator := record.NamespacedValidator{
		"pk":   record.PublicKeyValidator{},
		"ipns": ipns.Validator{KeyBook: host.Peerstore()},
	}
	dht, err := dht.New(ctx, host, dhtopts.Datastore(ds), dhtopts.Validator(validator))
	if err != nil {
		return nil, err
	}

	r.PubSub = pubsub
	r.Routing = dht

	return r, nil
}

func (r *Resolver) Bootstrap(ctx context.Context) error {
	if err := r.BootstrapNetwork(ctx, Bootstrap); err != nil {
		return err
	}
	if err := r.Subscribe(Topic); err != nil {
		return err
	}

	go r.BootstrapPubsub(ctx, Topic)
	return nil
}

func (r *Resolver) BootstrapNetwork(ctx context.Context, addrs []string) error {
	for _, ma := range r.Host.Network().ListenAddresses() {
		fmt.Printf("listening: %s/p2p/%s\n", ma.String(), r.Host.ID().Pretty())
	}

	for _, a := range addrs {
		pinfo, err := peerstore.InfoFromP2pAddr(maddr.StringCast(a))
		if err != nil {
			return err
		}
		if err = r.Host.Connect(ctx, *pinfo); err != nil {
			return err
		}
		fmt.Printf("connected: /p2p/%s\n", pinfo.ID.Pretty())
	}

	return nil
}

// see also: go-libp2p-pubsub-router/pubsub.go
//
// TODO: announce our membership (dht put)
// TODO: keep looking for providers
func (r *Resolver) BootstrapPubsub(ctx context.Context, topic string) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cid := blocks.NewBlock([]byte("floodsub:" + topic)).Cid()

	provs := r.Routing.FindProvidersAsync(ctx, cid, 10)
	wg := &sync.WaitGroup{}
	for p := range provs {
		wg.Add(1)
		go func(pi peerstore.PeerInfo) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(ctx, time.Second*10)
			defer cancel()
			err := r.Host.Connect(ctx, pi)
			if err != nil {
				return
			}
		}(p)
	}

	wg.Wait()
}

func (r *Resolver) Subscribe(topic string) error {
	if err := r.PubSub.RegisterTopicValidator(topic, r.validateMessage); err != nil {
		return err
	}

	sub, err := r.PubSub.Subscribe(topic)
	if err != nil {
		return err
	}

	r.IpnsSub = sub
	return nil
}

func (r *Resolver) validateMessage(ctx context.Context, msg *floodsub.Message) bool {
	return true
}

func (r *Resolver) ReceiveUpdates(ctx context.Context) {
	for {
		msg, err := r.IpnsSub.Next(ctx)
		if err != nil {
			continue
		}
		fmt.Printf("received: %s (seqno %d)\n", msg.Data, msg.Seqno)
	}
}

func (r *Resolver) ServeDNS(ctx context.Context)  {}
func (r *Resolver) ServeHTTP(ctx context.Context) {}
