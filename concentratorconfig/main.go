/*
concentratorconfig configures the Wireguard interface based on the etcd KV configuration.
It checks every minute for updates and applies these in Wireguard.
If an error occurs, it will print it and won't update any node.

Pass the simulate argument to only show the wireguard interface changes that would be applied.
When it is started this way, it exits after printing the changes.

The program expects by default a Wireguard interface named "wg-nodes"
and an etcd configuration file at "/etc/etcd-client.json".
These can be changed using command line flags.
*/
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"os"
	"sort"
	"time"

	"github.com/ffbs/etcd-tools/ffbs"

	"github.com/spf13/cobra"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func sortIPNet(s []net.IPNet) {
	sort.Slice(s, func(i, j int) bool {
		res := bytes.Compare(s[i].IP[:], s[j].IP[:])
		if res != 0 {
			return res > 0
		}
		res = bytes.Compare(s[i].Mask[:], s[j].Mask[:])
		return res > 0
	})
}

func calculateWGPeerUpdates(etcd *ffbs.EtcdHandler, dev *wgtypes.Device) ([]wgtypes.PeerConfig, error) {
	nodes, defNode, err := etcd.GetAllNodeInfo(context.Background())
	if err != nil {
		return nil, err
	}

	updates := make([]wgtypes.PeerConfig, 0, 10)

	// remove and update existing nodes
	for _, peer := range dev.Peers {
		pubkey := base64.URLEncoding.EncodeToString(peer.PublicKey[:])
		node, ok := nodes[pubkey]
		delete(nodes, pubkey)
		if !ok {
			// remove key as node vanished
			updates = append(updates, wgtypes.PeerConfig{
				PublicKey: peer.PublicKey,
				Remove:    true,
			})
			continue
		}

		nets := node.IPNets()
		sortIPNet(nets)
		sortIPNet(peer.AllowedIPs)

		equalNet := true
		if len(nets) != len(peer.AllowedIPs) {
			equalNet = false
		} else {
			for i, cur := range nets {
				if !bytes.Equal(cur.IP[:], peer.AllowedIPs[i].IP[:]) {
					equalNet = false
					break
				}
				if !bytes.Equal(cur.Mask[:], peer.AllowedIPs[i].Mask[:]) {
					equalNet = false
					break
				}
			}
		}

		keepalive := node.WGKeepaliveTime()
		if keepalive == nil {
			keepalive = defNode.WGKeepaliveTime()
			if keepalive == nil {
				disable := 0 * time.Second
				keepalive = &disable
			}
		}
		keepaliveChanged := *keepalive != peer.PersistentKeepaliveInterval

		if !equalNet || keepaliveChanged {
			updates = append(updates, wgtypes.PeerConfig{
				PublicKey:                   peer.PublicKey,
				PersistentKeepaliveInterval: keepalive,
				ReplaceAllowedIPs:           true,
				AllowedIPs:                  nets,
			})
		}
	}

	// add new nodes
	for pubkey, node := range nodes {
		decpkey, err := base64.URLEncoding.DecodeString(pubkey)
		if err != nil {
			return nil, fmt.Errorf("couldn't base64 decode pubkey '%s'", pubkey)
		}
		pkey, err := wgtypes.NewKey(decpkey)
		if err != nil {
			return nil, err
		}

		keepalive := node.WGKeepaliveTime()
		if keepalive == nil {
			keepalive = defNode.WGKeepaliveTime()
			if keepalive == nil {
				disable := 0 * time.Second
				keepalive = &disable
			}
		}

		updates = append(updates, wgtypes.PeerConfig{
			PublicKey:                   pkey,
			PersistentKeepaliveInterval: keepalive,
			ReplaceAllowedIPs:           true,
			AllowedIPs:                  node.IPNets(),
		})
	}

	return updates, nil
}

type CLIConfig struct {
	EtcdConfig     string
	UpdateInterval time.Duration
	WGDeviceName   string
	Simulate       bool
}

func run(config *CLIConfig) {
	etcd, err := ffbs.CreateEtcdConnection(config.EtcdConfig)
	if err != nil {
		log.Fatalln("Couldn't setup etcd connection:", err)
	}

	wg, err := wgctrl.New()
	if err != nil {
		log.Fatalln("Couldn't open connection to configure wireguard:", err)
	}

	for {
		// misusing a loop to break at any moment and still run the sleep call
		for {
			dev, err := wg.Device(config.WGDeviceName)
			if err != nil {
				log.Println("Error getting Wireguard device", config.WGDeviceName, "and got error:", err)
				break
			}

			updates, err := calculateWGPeerUpdates(etcd, dev)
			if err != nil {
				log.Println("Error trying to determine the node updates:", err)
				break
			}
			if config.Simulate {
				fmt.Printf("Peer updates: %v\n", updates)
				return
			}
			if len(updates) == 0 {
				break
			}

			if err := wg.ConfigureDevice(config.WGDeviceName, wgtypes.Config{Peers: updates}); err != nil {
				log.Println("Error trying to apply the node updates:", err)
				break
			}
			log.Println("Updated", len(updates), "peers")
			break
		}
		time.Sleep(config.UpdateInterval)
	}
}

func main() {
	var config CLIConfig

	rootCmd := &cobra.Command{
		Use:   "concentratorconfig",
		Short: "Configure the Wireguard interface based on the etcd KV configuration",
		Run: func(cmd *cobra.Command, args []string) {
			run(&config)
		},
	}

	rootCmd.PersistentFlags().StringVarP(&config.EtcdConfig, "etcdconfig", "e", "/etc/etcd-client.json", "Path to the etcd client configuration file")
	rootCmd.MarkFlagFilename("etcdconfig", "json")
	rootCmd.PersistentFlags().DurationVarP(&config.UpdateInterval, "interval", "i", 60*time.Second, "Interval to update the wireguard configuration from the etcd store")
	rootCmd.PersistentFlags().StringVarP(&config.WGDeviceName, "devicename", "d", "wg-nodes", "Wireguard device name to update the configuration")

	simulateCmd := &cobra.Command{
		Use:   "simulate",
		Short: "Simulate updating of the wireguard devices, but don't apply them",
		Run: func(cmd *cobra.Command, args []string) {
			config.Simulate = true
			run(&config)
		},
	}

	rootCmd.AddCommand(simulateCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
