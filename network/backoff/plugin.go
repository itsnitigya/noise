package backoff

import (
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/perlin-network/noise/network"
)

type Plugin struct {
	*network.Plugin

	// map[string]*Backoff
	backoffs sync.Map
}

var (
	PluginID        = (*Plugin)(nil)
	initialDelay    = 5 * time.Second
	limitIterations = 100
)

func (p *Plugin) PeerDisconnect(client *network.PeerClient) {
	addr := client.Address

	go func() {
		p.startBackoff(addr, client)
	}()
}

func (p *Plugin) startBackoff(addr string, client *network.PeerClient) {
	// this callback is called before the disconnect, so wait until disconnected
	time.Sleep(initialDelay)

	if _, exists := p.backoffs.Load(addr); exists {
		// don't activate if it already active
		glog.Infof("backoff skipped for addr %s, already active\n", addr)
		return
	}
	// reset the backoff counter
	p.backoffs.Store(addr, DefaultBackoff())
	startTime := time.Now()
	for i := 0; i < limitIterations; i++ {
		s, active := p.backoffs.Load(addr)
		if !active {
			break
		}
		b := s.(*Backoff)
		if b.TimeoutExceeded() {
			glog.Infof("backoff ended for addr %s, timed out after %s\n", addr, time.Now().Sub(startTime))
			break
		}
		d := b.NextDuration()
		glog.Infof("backoff reconnecting to %s in %s iteration %d", addr, d, i+1)
		time.Sleep(d)
		if p.checkConnected(client, addr) {
			// check that the connection is still empty before dialing
			break
		}
		if _, err := client.Network.Client(client.Address); err != nil {
			continue
		}
		if !p.checkConnected(client, addr) {
			// check if successfully connected
			continue
		}
		// success
		break
	}
	// clean up this back off
	p.backoffs.Delete(addr)
}

func (p *Plugin) checkConnected(client *network.PeerClient, addr string) bool {
	_, connected := client.Network.Connections.Load(addr)
	return connected
}
