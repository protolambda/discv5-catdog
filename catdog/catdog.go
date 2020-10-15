package catdog

import (
	"crypto/ecdsa"
	"fmt"
	"github.com/ethereum/go-ethereum/log"
	gdv5 "github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/ethereum/go-ethereum/p2p/netutil"
	dv50 "github.com/protolambda/discv5-catdog/discv50/discover"
	dv51 "github.com/protolambda/discv5-catdog/discv51/discover"
	"time"
)

type CatDogConfig struct {
	PrivateKeyV50 *ecdsa.PrivateKey
	PrivateKeyV51 *ecdsa.PrivateKey

	// These settings are optional:
	NetRestrict  *netutil.Netlist   // network whitelist
	BootnodesV50 []*enode.Node      // list of bootstrap nodes
	BootnodesV51 []*enode.Node      // list of bootstrap nodes
	LogV50       log.Logger         // if set, log messages go here
	LogV51       log.Logger         // if set, log messages go here
	ValidSchemes enr.IdentityScheme // allowed identity schemes
}

type CatDog struct {
	init   chan struct{}
	UDPv50 *dv50.UDPv5
	UDPv51 *dv51.UDPv5
}

func (cd *CatDog) Revalidate(n *enode.Node) (uint64, error) {
	// wait with any revalidation until we have both v5.0 and v5.1 available
	<-cd.init

	// try hit disc v5.1 first
	seq, err := cd.UDPv51.PingSeq(n)
	if err == nil {
		return seq, nil
	}
	// try hit disc v5.0 otherwise
	return cd.UDPv50.PingSeq(n)
}

func (cd *CatDog) OnSeenV50(n *enode.Node, at time.Time, liveness uint) {
	<-cd.init
	// TODO: just a guess: instead of adding like any node, put it in front of the bucket.
	// Migrating nodes need an extra hand, right?
	cd.UDPv51.AddRecentNode(n, at, liveness) // add to the other table
}

func (cd *CatDog) OnSeenV51(n *enode.Node, at time.Time, liveness uint) {
	<-cd.init
	// TODO: just a guess: instead of adding like any node, put it in front of the bucket.
	// Migrating nodes need an extra hand, right?
	cd.UDPv50.AddRecentNode(n, at, liveness) // add to the other table
}

func NewCatDog(connv50 gdv5.UDPConn, connv51 gdv5.UDPConn,
	ln50 *enode.LocalNode, ln51 *enode.LocalNode, cfg *CatDogConfig) (*CatDog, error) {
	cd := &CatDog{
		init: make(chan struct{}),
	}

	v50Cfg := dv50.Config{
		PrivateKey:   cfg.PrivateKeyV50,
		Revalidator:  cd.Revalidate,
		OnSeen:       cd.OnSeenV50,
		NetRestrict:  cfg.NetRestrict,
		Bootnodes:    cfg.BootnodesV50,
		Log:          cfg.LogV50,
		ValidSchemes: cfg.ValidSchemes,
	}
	udp50, err := dv50.ListenV5(connv50, ln50, v50Cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to start discv 5.0: %v", err)
	}
	cd.UDPv50 = udp50

	v51Cfg := dv51.Config{
		PrivateKey:   cfg.PrivateKeyV51,
		Revalidator:  cd.Revalidate,
		OnSeen:       cd.OnSeenV51,
		NetRestrict:  cfg.NetRestrict,
		Bootnodes:    cfg.BootnodesV51,
		Log:          cfg.LogV51,
		ValidSchemes: cfg.ValidSchemes,
	}
	udp51, err := dv51.ListenV5(connv51, ln51, v51Cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to start discv 5.1: %v", err)
	}
	cd.UDPv51 = udp51

	// both discv5s are ready now
	close(cd.init)

	return cd, nil
}
