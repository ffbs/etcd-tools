package main

import (
	"context"
	"fmt"
	"log"
	"reflect"

	"github.com/ffbs/etcd-tools/ffbs"

	"github.com/spf13/cobra"
)

func init() {
	var etcdConfig string

	cmd := &cobra.Command{
		Use:   "showoverrides",
		Short: "Shows all Pubkeys overriding a default value",
		Run: func(cmd *cobra.Command, args []string) {
			showoverrides(etcdConfig)
		},
	}

	cmd.PersistentFlags().StringVarP(&etcdConfig, "etcdconfig", "e", "/etc/etcd-client.json", "Path to the etcd client configuration file")
	cmd.MarkFlagFilename("etcdconfig", "json")

	rootCmd.AddCommand(cmd)
}

func showoverrides(etcdConfig string) {
	etcd, err := ffbs.CreateEtcdConnection(etcdConfig)
	if err != nil {
		log.Fatalln("Couldn't setup etcd connection:", err)
	}

	nodes, def, err := etcd.GetAllNodeInfo(context.Background())
	if err != nil {
		log.Fatalln("Couldn't get all nodes:", err)
	}
	defval := reflect.ValueOf(def).Elem()

	unchanged := make(map[string]uint64)
	for _, field := range reflect.VisibleFields(defval.Type()) {
		unchanged[field.Name] = 0
	}

	for pubkey, nodeinfo := range nodes {
		nodeinfovalue := reflect.ValueOf(nodeinfo).Elem()

		for _, field := range reflect.VisibleFields(defval.Type()) {
			d := defval.FieldByIndex(field.Index)
			v := nodeinfovalue.FieldByIndex(field.Index)

			if d.IsNil() {
				continue
			}
			if v.IsNil() {
				unchanged[field.Name]++
				continue
			}

			if v.Kind() == reflect.Pointer {
				v = v.Elem()
				d = d.Elem()
			}
			if fmt.Sprintf("%s", v.Interface()) != fmt.Sprintf("%s", d.Interface()) {
				fmt.Println("Overridden", field.Name, "for", pubkey, "with value", v.Interface())
			}
		}
	}

	fmt.Println("Nodes affected by the corresponding default values:", unchanged)
}
