package elector

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	testConfig = Config{
		Endpoints: []string{"http://127.0.0.1:2379"},
	}
	testElectionPath = "/gocron/elector/"
)

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
	assert.Equal(t, nil, el.IsLeader(context.Background()))
	assert.Equal(t, el.GetLeaderID(), el.GetID())
	_ = el.Stop()

	select {
	case <-sig:
	case <-time.After(2 * time.Second):
		t.Error("elector exit timeout")
	}

	// after elector.stop, current instance is not leader
	assert.Equal(t, ErrNonLeader, el.IsLeader(context.Background()))
}

func TestElectorMultipleAcquire(t *testing.T) {
	elections := []*Elector{}
	workers := 3

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
		if el.IsLeader(context.Background()) == nil {
			leaderCounter++
		}
	}

	// only one leader, other instance is non-leader.
	assert.Equal(t, leaderCounter, 1)

	// stop all electors
	for _, el := range elections {
		_ = el.Stop()
	}
}

func TestElectorAcquireRace(t *testing.T) {
	elections := []*Elector{}
	workers := 3

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

	time.Sleep(2 * time.Second)
	assert.Equal(t, 1, getCounter())

	for idx, el := range elections {
		last := len(elections) - 1

		_ = el.Stop()

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
	_ = el.Stop()
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
