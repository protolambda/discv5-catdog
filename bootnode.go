package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/protolambda/ask"
	"github.com/protolambda/discv5-catdog/catdog"
	"github.com/protolambda/rumor/control/actor/flags"
	"github.com/protolambda/rumor/p2p/addrutil"
	"github.com/protolambda/zrnt/eth2/beacon"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type BootnodeCmd struct {
	Privv50       flags.P2pPrivKeyFlag `ask:"--priv-v50" help:"Private key for v5.0, in raw hex encoded format"`
	Privv51       flags.P2pPrivKeyFlag `ask:"--priv-v51" help:"Private key for v5.1, in raw hex encoded format"`
	ENRIP         net.IP               `ask:"--enr-ip" help:"IP to put in ENR"`
	ENRUDPv50     uint16               `ask:"--enr-udp-v50" help:"UDP port to put in v5.0 ENR"`
	ENRUDPv51     uint16               `ask:"--enr-udp-v51" help:"UDP port to put in v5.1 ENR"`
	ListenIP      net.IP               `ask:"--listen-ip" help:"Listen IP."`
	ListenUDPv50  uint16               `ask:"--listen-udp-v50" help:"Listen UDP port for v5.0. Will try ENR port otherwise."`
	ListenUDPv51  uint16               `ask:"--listen-udp-v51" help:"Listen UDP port for v5.1. Will try ENR port otherwise."`
	APIAddr       string               `ask:"--api-addr" help:"Address to bind HTTP API server to. API is disabled if empty."`
	NodeDBPathv50 string               `ask:"--node-db-v50" help:"Path to dv5 node DB for v5.0. Memory DB if empty."`
	NodeDBPathv51 string               `ask:"--node-db-v51" help:"Path to dv5 node DB for v5.1. Memory DB if empty."`
	Attnets       beacon.AttnetBits    `ask:"--attnets" help:"Attnet bitfield, as bytes."`
	Bootnodesv50  []string             `ask:"--bootnodes-v50" help:"Optionally befriend other bootnodes"`
	Bootnodesv51  []string             `ask:"--bootnodes-v51" help:"Optionally befriend other bootnodes"`
	ForkVersion   beacon.Version       `ask:"--fork-version" help:"Eth2 fork version"`
	Color         bool                 `ask:"--color" help:"Log with colors"`
	Level         string               `ask:"--level" help:"Log level"`
}

func (b *BootnodeCmd) Help() string {
	return "Run CATDOG bootnode. A friendly discovery bridging monster between v5.0 and v5.1."
}

func (b *BootnodeCmd) Default() {
	b.ListenIP = net.IPv4zero
	b.Color = true
	b.Level = "debug"
	b.APIAddr = "0.0.0.0:8000"
}

func (c *BootnodeCmd) Run(ctx context.Context, args ...string) error {
	bootNodesv50 := make([]*enode.Node, 0, len(c.Bootnodesv50))
	for i := 0; i < len(c.Bootnodesv50); i++ {
		dv5Addr, err := addrutil.ParseEnrOrEnode(c.Bootnodesv50[i])
		if err != nil {
			return fmt.Errorf("bootnode (V5.0) %d is bad: %v", i, err)
		}
		bootNodesv50 = append(bootNodesv50, dv5Addr)
	}

	bootNodesv51 := make([]*enode.Node, 0, len(c.Bootnodesv51))
	for i := 0; i < len(c.Bootnodesv51); i++ {
		dv5Addr, err := addrutil.ParseEnrOrEnode(c.Bootnodesv51[i])
		if err != nil {
			return fmt.Errorf("bootnode (V5.1) %d is bad: %v", i, err)
		}
		bootNodesv51 = append(bootNodesv51, dv5Addr)
	}

	if c.Privv50.Priv == nil {
		return fmt.Errorf("need p2p priv key for v5.0")
	}

	if c.Privv51.Priv == nil {
		return fmt.Errorf("need p2p priv key for v5.1")
	}

	ecdsaPrivKey50 := (*ecdsa.PrivateKey)(c.Privv50.Priv)
	ecdsaPrivKey51 := (*ecdsa.PrivateKey)(c.Privv51.Priv)

	if c.ListenUDPv50 == 0 {
		c.ListenUDPv50 = c.ENRUDPv50
	}

	if c.ListenUDPv51 == 0 {
		c.ListenUDPv51 = c.ENRUDPv51
	}

	udpAddrV50 := &net.UDPAddr{
		IP:   c.ListenIP,
		Port: int(c.ListenUDPv50),
	}

	udpAddrV51 := &net.UDPAddr{
		IP:   c.ListenIP,
		Port: int(c.ListenUDPv51),
	}

	localNodeDBv50, err := enode.OpenDB(c.NodeDBPathv50)
	if err != nil {
		return err
	}

	localNodeDBv51, err := enode.OpenDB(c.NodeDBPathv51)
	if err != nil {
		return err
	}

	localNodev50 := enode.NewLocalNode(localNodeDBv50, ecdsaPrivKey50)
	localNodev51 := enode.NewLocalNode(localNodeDBv51, ecdsaPrivKey51)
	if c.ENRIP != nil {
		localNodev50.SetStaticIP(c.ENRIP)
		localNodev51.SetStaticIP(c.ENRIP)
	}
	if c.ENRUDPv50 != 0 {
		localNodev50.SetFallbackUDP(int(c.ENRUDPv50))
	}
	if c.ENRUDPv51 != 0 {
		localNodev51.SetFallbackUDP(int(c.ENRUDPv51))
	}
	localNodev50.Set(addrutil.NewAttnetsENREntry(&c.Attnets))
	localNodev51.Set(addrutil.NewAttnetsENREntry(&c.Attnets))

	entry := addrutil.NewEth2DataEntry(&beacon.Eth2Data{
		ForkDigest:      beacon.ComputeForkDigest(c.ForkVersion, beacon.Root{}),
		NextForkVersion: c.ForkVersion,
		NextForkEpoch:   ^beacon.Epoch(0),
	})
	localNodev50.Set(entry)
	localNodev51.Set(entry)

	fmt.Println("v5.0: ", localNodev50.Node().String())
	fmt.Println("v5.1: ", localNodev51.Node().String())

	connv50, err := net.ListenUDP("udp", udpAddrV50)
	if err != nil {
		return err
	}
	connv51, err := net.ListenUDP("udp", udpAddrV51)
	if err != nil {
		return err
	}
	lvl, err := log.LvlFromString(c.Level)
	if err != nil {
		return err
	}
	gethLogger := log.New()
	outHandler := log.StreamHandler(os.Stdout, log.TerminalFormat(c.Color))
	gethLogger.SetHandler(log.LvlFilterHandler(lvl, outHandler))

	// Optional HTTP server, to read the ENR from
	var srv *http.Server
	if c.APIAddr != "" {
		router := http.NewServeMux()
		srv = &http.Server{
			Addr:    c.APIAddr,
			Handler: router,
		}
		router.HandleFunc("/enr", func(w http.ResponseWriter, req *http.Request) {
			gethLogger.Info("received ENR API request", "remote", req.RemoteAddr)
			w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
			w.WriteHeader(200)
			enr := localNodev50.Node().String()
			_, _ = io.WriteString(w, "v5.0: \n")
			if _, err := io.WriteString(w, enr); err != nil {
				gethLogger.Error("failed to respond to request from", "remote", req.RemoteAddr, "err", err)
			}
			_, _ = io.WriteString(w, "\n\nv5.1: \n")
			enr = localNodev51.Node().String()
			if _, err := io.WriteString(w, enr); err != nil {
				gethLogger.Error("failed to respond to request from", "remote", req.RemoteAddr, "err", err)
			}
		})

		go func() {
			gethLogger.Info("starting API server, ENR reachable on: http://" + srv.Addr + "/enr")
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				gethLogger.Error("API server listen failure", "err", err)
			}
		}()
	}

	cfg := catdog.CatDogConfig{
		PrivateKeyV50: ecdsaPrivKey50,
		PrivateKeyV51: ecdsaPrivKey51,
		NetRestrict:   nil,
		BootnodesV50:  bootNodesv50,
		BootnodesV51:  bootNodesv51,
		Log:           gethLogger,
		ValidSchemes:  enode.ValidSchemes,
	}
	cd, err := catdog.NewCatDog(connv50, connv51, localNodev50, localNodev51, &cfg)
	if err != nil {
		return err
	}
	defer cd.UDPv50.Close()
	defer cd.UDPv51.Close()
	<-ctx.Done()

	// Close API server
	if srv != nil {
		ctx, _ := context.WithTimeout(context.Background(), time.Second*5)
		if err := srv.Shutdown(ctx); err != nil {
			log.Error("Server shutdown failed", "err", err)
		}
	}
	return nil
}

func main() {
	loadedCmd, err := ask.Load(&BootnodeCmd{})
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sig
		cancel()
	}()

	if cmd, isHelp, err := loadedCmd.Execute(ctx, os.Args[1:]...); err != nil {
		_, _ = os.Stderr.WriteString(err.Error())
	} else if isHelp {
		_, _ = os.Stderr.WriteString(cmd.Usage())
	}
}
