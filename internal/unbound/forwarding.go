package unbound

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	forwardCheckWorkers       = 4
	forwardCheckTargetTimeout = 4 * time.Second
	forwardCheckTotalTimeout  = 20 * time.Second
	maxForwardCheckDetail     = 512
)

type ForwardTargetCheck struct {
	Zone      string `json:"zone"`
	Address   string `json:"address"`
	Reachable bool   `json:"reachable"`
	Detail    string `json:"detail"`
}

func (m *Manager) CheckForwardTargets(ctx context.Context, zones []ForwardZone) ([]ForwardTargetCheck, error) {
	settings := DefaultSettings()
	settings.ForwardZones = zones
	if err := settings.Validate(); err != nil {
		return nil, err
	}

	type target struct {
		index   int
		zone    string
		address string
	}
	targets := make([]target, 0)
	for _, zone := range zones {
		for _, address := range zone.Servers {
			targets = append(targets, target{index: len(targets), zone: zone.Name, address: address})
		}
	}
	results := make([]ForwardTargetCheck, len(targets))
	if len(targets) == 0 {
		return results, nil
	}

	ctx, cancel := context.WithTimeout(ctx, forwardCheckTotalTimeout)
	defer cancel()
	jobs := make(chan target)
	var workers sync.WaitGroup
	workerCount := min(forwardCheckWorkers, len(targets))
	workers.Add(workerCount)
	for range workerCount {
		go func() {
			defer workers.Done()
			for item := range jobs {
				results[item.index] = m.checkForwardTarget(ctx, item.zone, item.address)
			}
		}()
	}
	for _, item := range targets {
		select {
		case jobs <- item:
		case <-ctx.Done():
			close(jobs)
			workers.Wait()
			return results, fmt.Errorf("forward target checks timed out: %w", ctx.Err())
		}
	}
	close(jobs)
	workers.Wait()
	return results, nil
}

func (m *Manager) checkForwardTarget(ctx context.Context, zone, address string) ForwardTargetCheck {
	targetContext, cancel := context.WithTimeout(ctx, forwardCheckTargetTimeout)
	defer cancel()
	output, err := m.run(
		targetContext,
		"docker", "exec", m.containerName,
		"dig", "+time=3", "+tries=1", "+noall", "+comments", "@"+address, zone, "SOA",
	)
	detail := strings.TrimSpace(string(output))
	if len(detail) > maxForwardCheckDetail {
		detail = detail[:maxForwardCheckDetail] + "…"
	}
	if err != nil {
		if detail == "" {
			detail = err.Error()
		} else {
			detail = fmt.Sprintf("%v: %s", err, detail)
		}
		return ForwardTargetCheck{Zone: zone, Address: address, Detail: detail}
	}
	if detail == "" {
		detail = "DNS server answered the reachability probe"
	}
	return ForwardTargetCheck{Zone: zone, Address: address, Reachable: true, Detail: detail}
}
