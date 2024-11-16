package main

import (
	"testing"

	"github.com/ffbs/etcd-tools/ffbs"

	"github.com/stretchr/testify/assert"
)

func TestGenerateAddressesAndRanges(t *testing.T) {
	// Using https://freifunk-bs.de/map/#!/de/map/f4068dcf9fe1 as an example
	id := uint64(30)
	node := ffbs.NodeInfo{ID: &id}
	generateNodeAddressesAndRanges(&node)
	assert.Equal(t, "10.0.120.1", *node.Address4)
	assert.Equal(t, "10.0.120.0/22", *node.Range4)
	assert.Equal(t, "2001:bf7:381:1e::1", *node.Address6)
	assert.Equal(t, "2001:bf7:381:1e::/64", *node.Range6)
}

func TestGenerateHighestAddressesAndRanges(t *testing.T) {
	// IPv4 base range is /8. Range of every subnet is /22 => 22-8 = 14 bits of freedom
	id := uint64(16383)
	node := ffbs.NodeInfo{ID: &id}
	generateNodeAddressesAndRanges(&node)
	assert.Equal(t, "10.255.252.1", *node.Address4)
	assert.Equal(t, "10.255.252.0/22", *node.Range4)
	assert.Equal(t, "2001:bf7:381:3fff::1", *node.Address6)
	assert.Equal(t, "2001:bf7:381:3fff::/64", *node.Range6)
}
