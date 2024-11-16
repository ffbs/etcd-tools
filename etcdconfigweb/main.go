/*
etcdconfigweb provides an http interface to query and register nodes from the etcd KV store.

By default it will listen on port 8080 on any interface.
It expects an etcd configuration file (by default at "/etc/etcd-client.json")
and a signify private key to sign the requests (by default at "/etc/ffbs/node-config.sec").
You can change these settings using command line options.

As it doesn't need any root capabilities, it should be considered to run this executable as a normal user.

The HTTP server supports two endpoints:
  - /config to retrieve node configurations or create new nodes
  - /etcd_status to retrieve the current node count in etcd and the amount of successful and failed requests to the /config endpoint
*/
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/ffbs/etcd-tools/ffbs"

	"github.com/spf13/cobra"
)

type CLIConfig struct {
	ListenAddr string
	Key        string
	EtcdConfig string
}

func run(config *CLIConfig) {
	etcd, err := ffbs.CreateEtcdConnection(config.EtcdConfig)
	if err != nil {
		log.Fatalln("Couldn't setup etcd connection: ", err)
	}

	signer, err := NewSignifySignerFromPrivateKeyFile(config.Key)
	if err != nil {
		log.Fatalln("Couldn't parse signify private key:", err)
	}

	metrics := NewMetrics(etcd)

	http.Handle("/config", &ConfigHandler{tracker: metrics, signer: signer, etcdHandler: etcd})
	http.Handle("/etcd_status", metrics)

	log.Println("Starting server on", config.ListenAddr)
	log.Fatal("Error running webserver:", http.ListenAndServe(config.ListenAddr, nil))
}

func main() {
	var config CLIConfig

	rootCmd := &cobra.Command{
		Use:   "etcdconfigweb",
		Short: "Provide an HTTP interface to query and register nodes from the etcd KV store",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			run(&config)
		},
	}

	rootCmd.PersistentFlags().StringVarP(&config.ListenAddr, "listen", "l", ":8080", "HTTP listening address to bind to")
	rootCmd.PersistentFlags().StringVarP(&config.Key, "key", "k", "/etc/ffbs/node-config.sec", "Path to signify private key file to sign responses")
	rootCmd.MarkFlagFilename("key", "sec")
	rootCmd.PersistentFlags().StringVarP(&config.EtcdConfig, "etcdconfig", "e", "/etc/etcd-client.json", "Path to the etcd client configuration file")
	rootCmd.MarkFlagFilename("etcdconfig", "json")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
