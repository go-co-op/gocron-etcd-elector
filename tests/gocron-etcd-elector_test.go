package tests

import (
	"context"
	"testing"
	"time"

	"github.com/go-co-op/gocron"
	elector "github.com/go-co-op/gocron-etcd-elector"
	"github.com/stretchr/testify/assert"
)

var (
	testConfig = elector.Config{
		Endpoints: []string{"http://127.0.0.1:2379"},
	}
	testElectionPath = "/gocron/elector/"
)

func TestGocronDialTimeout(t *testing.T) {
	start := time.Now()
	_, err := elector.NewElector(context.Background(), elector.Config{
		Endpoints: []string{"http://127.0.0.1:2000"}, // invalid etcd
	})
	assert.Equal(t, elector.ErrPingEtcd, err)

	// 5< 6 < 7
	assert.Greater(t, int(time.Since(start).Seconds()), 5)
	assert.Less(t, int(time.Since(start).Seconds()), 8)
}

func TestGocronWithElector(t *testing.T) {
	el, err := elector.NewElector(context.Background(), testConfig, elector.WithTTL(1))
	assert.Equal(t, nil, err)
	go func() {
		err := el.Start(testElectionPath + "gocron_one")
		if err != nil {
			t.Error(err)
		}
	}()

	defer func() {
		_ = el.Stop()
	}()

	done := make(chan struct{}, 1)
	counter := 0
	fn := func() {
		counter++
		if counter == 10 {
			close(done)
		}
	}

	sched := gocron.NewScheduler(time.UTC)
	sched.WithDistributedElector(el)
	sched.StartAsync()
	_, err = sched.Every("50ms").Do(fn)
	assert.Equal(t, nil, err)

	defer sched.Stop()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Error("done timeout")
	}
}

func TestGocronWithMultipleElectors(t *testing.T) {
	var elections []*elector.Elector
	var schedulers []*gocron.Scheduler
	workers := 3

	resultChan := make(chan int, 100)
	fn := func(leaderIdx int) {
		resultChan <- leaderIdx
	}

	for i := 0; i < workers; i++ {
		el, err := elector.NewElector(context.Background(), testConfig, elector.WithTTL(1))
		assert.Equal(t, nil, err)

		go func() {
			_ = el.Start(testElectionPath + "gocron_multi")
		}()

		s := gocron.NewScheduler(time.UTC)
		s.WithDistributedElector(el)
		_, err = s.Every("50ms").Do(fn, i)
		assert.Equal(t, nil, err)
		s.StartAsync()

		elections = append(elections, el)
		schedulers = append(schedulers, s)
	}

	getLeader := func() int {
		select {
		case leader := <-resultChan:
			return leader
		case <-time.After(3 * time.Second):
			t.Error("wait result timeout")
			return -1
		}
	}

	// all index of the leader are the same.
	leader := getLeader()
	for i := 0; i < 10; i++ {
		cur := getLeader()
		assert.Equal(t, leader, cur)
	}

	for i := 0; i < workers; i++ {
		_ = elections[i].Stop()
		schedulers[i].Stop()
	}
}
