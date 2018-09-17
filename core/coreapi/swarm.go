package coreapi

import (
	"context"
	"fmt"
	"sort"
	"time"

	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"

	peer "gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"
	inet "gx/ipfs/QmZNJyx9GGCX4GeuHnLB8fxaxMLs4MjTjHokxfQcCd6Nve/go-libp2p-net"
	net "gx/ipfs/QmZNJyx9GGCX4GeuHnLB8fxaxMLs4MjTjHokxfQcCd6Nve/go-libp2p-net"
	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	pstore "gx/ipfs/Qmda4cPRvSRyox3SqgJN6DfSZGU5TtHufPTp9uXjFj71X6/go-libp2p-peerstore"
	swarm "gx/ipfs/QmeDpqUwwdye8ABKVMPXKuWwPVURFdqTqssbTUB39E2Nwd/go-libp2p-swarm"
	iaddr "gx/ipfs/QmePSRaGafvmURQwQkHPDBJsaGwKXC1WpBBHVCQxdr8FPn/go-ipfs-addr"
)

type SwarmAPI struct {
	*CoreAPI
}

type connInfo struct {
	api  *CoreAPI
	conn net.Conn
	dir  net.Direction

	addr  ma.Multiaddr
	peer  peer.ID
	muxer string
}

func (api *SwarmAPI) Connect(ctx context.Context, pi pstore.PeerInfo) error {
	if api.node.PeerHost == nil {
		return coreiface.ErrOffline
	}

	swrm, ok := api.node.PeerHost.Network().(*swarm.Swarm)
	if !ok {
		return fmt.Errorf("peerhost network was not swarm")
	}

	swrm.Backoff().Clear(pi.ID)

	return api.node.PeerHost.Connect(ctx, pi)
}

func (api *SwarmAPI) Disconnect(ctx context.Context, addr ma.Multiaddr) error {
	if api.node.PeerHost == nil {
		return coreiface.ErrOffline
	}

	ia, err := iaddr.ParseMultiaddr(ma.Multiaddr(addr))
	if err != nil {
		return err
	}

	taddr := ia.Transport()
	id := ia.ID()
	net := api.node.PeerHost.Network()

	if taddr == nil {
		if net.Connectedness(id) != inet.Connected {
			return coreiface.ErrNotConnected
		} else if err := net.ClosePeer(id); err != nil {
			return err
		}
	} else {
		for _, conn := range net.ConnsToPeer(id) {
			if !conn.RemoteMultiaddr().Equal(taddr) {
				continue
			}

			return conn.Close()
		}

		return coreiface.ErrConnNotFound
	}

	return nil
}

func (api *SwarmAPI) KnownAddrs(context.Context) (map[peer.ID][]ma.Multiaddr, error) {
	if api.node.PeerHost == nil {
		return nil, coreiface.ErrOffline
	}

	addrs := make(map[peer.ID][]ma.Multiaddr)
	ps := api.node.PeerHost.Network().Peerstore()
	for _, p := range ps.Peers() {
		for _, a := range ps.Addrs(p) {
			addrs[p] = append(addrs[p], a)
		}
		sort.Slice(addrs[p], func(i, j int) bool {
			return addrs[p][i].String() < addrs[p][j].String()
		})
	}

	return addrs, nil
}

func (api *SwarmAPI) LocalAddrs(context.Context) ([]ma.Multiaddr, error) {
	if api.node.PeerHost == nil {
		return nil, coreiface.ErrOffline
	}

	return api.node.PeerHost.Addrs(), nil
}

func (api *SwarmAPI) ListenAddrs(context.Context) ([]ma.Multiaddr, error) {
	if api.node.PeerHost == nil {
		return nil, coreiface.ErrOffline
	}

	return api.node.PeerHost.Network().InterfaceListenAddresses()
}

func (api *SwarmAPI) Peers(context.Context) ([]coreiface.ConnectionInfo, error) {
	if api.node.PeerHost == nil {
		return nil, coreiface.ErrOffline
	}

	conns := api.node.PeerHost.Network().Conns()

	var out []coreiface.ConnectionInfo
	for _, c := range conns {
		pid := c.RemotePeer()
		addr := c.RemoteMultiaddr()

		ci := &connInfo{
			api:  api.CoreAPI,
			conn: c,
			dir:  c.Stat().Direction,

			addr: addr,
			peer: pid,
		}

		/*
			// FIXME(steb):
			swcon, ok := c.(*swarm.Conn)
			if ok {
				ci.muxer = fmt.Sprintf("%T", swcon.StreamConn().Conn())
			}
		*/

		out = append(out, ci)
	}

	return out, nil
}

func (ci *connInfo) ID() peer.ID {
	return ci.peer
}

func (ci *connInfo) Address() ma.Multiaddr {
	return ci.addr
}

func (ci *connInfo) Direction() net.Direction {
	return ci.dir
}

func (ci *connInfo) Latency(context.Context) (time.Duration, error) {
	return ci.api.node.Peerstore.LatencyEWMA(peer.ID(ci.ID())), nil
}

func (ci *connInfo) Streams(context.Context) ([]protocol.ID, error) {
	streams := ci.conn.GetStreams()

	out := make([]protocol.ID, len(streams))
	for i, s := range streams {
		out[i] = s.Protocol()
	}

	return out, nil
}
