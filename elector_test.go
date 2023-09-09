package elector

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/stretchr/testify/assert"
)

var (
	testConfig = Config{
		Endpoints: []string{"http://127.0.0.1:2379"},
	}
	testElectionPath = "/gocron/elector/"
)

func TestGocronWithElector(t *testing.T) {
	el, err := NewElector(context.Background(), testConfig, WithTTL(1))
	assert.Equal(t, nil, err)
	go el.Start(testElectionPath + "gocron_one")

	defer el.Stop()

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
	elections := []*Elector{}
	schedulers := []*gocron.Scheduler{}
	workers := 3

	resultChan := make(chan int, 100)
	fn := func(leaderIdx int) {
		resultChan <- leaderIdx
	}

	for i := 0; i < workers; i++ {
		el, err := NewElector(context.Background(), testConfig, WithTTL(1))
		assert.Equal(t, nil, err)

		go el.Start(testElectionPath + "gocron_multi")

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
		elections[i].Stop()
		schedulers[i].Stop()
	}
}

func TestElectorSingleAcquire(t *testing.T) {
	el, err := NewElector(context.Background(), testConfig, WithTTL(10))
	assert.Equal(t, nil, err)
	sig := make(chan struct{}, 1)
	go func() {
		err := el.Start(testElectionPath + "single")
		assert.Equal(t, nil, err)
		close(sig)
	}()

	time.Sleep(2 * time.Second)
	assert.Equal(t, nil, el.IsLeader(nil))
	assert.Equal(t, el.GetLeaderID(), el.GetID())
	el.Stop()

	select {
	case <-sig:
	case <-time.After(2 * time.Second):
		t.Error("elector exit timeout")
	}

	// after elector.stop, current instance is not leader
	assert.Equal(t, ErrNonLeader, el.IsLeader(nil))
}

func TestElectorMultipleAcquire(t *testing.T) {
	var elections = []*Elector{}
	var workers = 3

	// start all electors
	for i := 0; i < workers; i++ {
		el, err := NewElector(context.Background(), testConfig, WithTTL(10))
		assert.Equal(t, nil, err)
		elections = append(elections, el)

		go func() {
			err := el.Start(testElectionPath + "multi")
			assert.Equal(t, nil, err)
		}()
	}

	time.Sleep(5 * time.Second)

	var leaderCounter int
	for _, el := range elections {
		if el.IsLeader(nil) == nil {
			leaderCounter++
		}
	}

	// only one leader, other instance is non-leader.
	assert.Equal(t, leaderCounter, 1)

	// stop all electors
	for _, el := range elections {
		el.Stop()
	}
}

func TestElectorAcquireRace(t *testing.T) {
	var elections = []*Elector{}
	var workers = 3

	// start all electors
	for i := 0; i < workers; i++ {
		el, err := NewElector(context.Background(), testConfig, WithTTL(1))
		assert.Equal(t, nil, err)
		el.id = fmt.Sprintf("idx-%v", i)

		elections = append(elections, el)

		go func() {
			err := el.Start(testElectionPath + "race")
			assert.Equal(t, nil, err)
		}()

		time.Sleep(100 * time.Millisecond)
	}

	getCounter := func() int {
		var counter int
		for _, el := range elections {
			if el.isLeader {
				counter++
			}
		}
		return counter
	}

	assert.Equal(t, 1, getCounter())

	for idx, el := range elections {
		last := len(elections) - 1

		el.Stop()

		time.Sleep(3 * time.Second)
		if idx == last {
			assert.Equal(t, 0, getCounter())
			break
		}
		assert.Equal(t, 1, getCounter())
	}
}

func TestElectorStop(t *testing.T) {
	el, err := NewElector(context.Background(), testConfig)
	assert.Equal(t, nil, err)
	el.Stop()
	err = el.Start(testElectionPath)
	assert.Equal(t, err, ErrClosed)
}

func TestGetSid(t *testing.T) {
	count := 100000
	set := make(map[string]struct{}, count)

	for i := 0; i < count; i++ {
		set[getID()] = struct{}{}
	}

	assert.Equal(t, count, len(set))
}
