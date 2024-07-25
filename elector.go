package elector

import (
	"context"
	"crypto/rand"
	"fmt"
	mrand "math/rand"
	"os"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/go-co-op/gocron"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

var (
	ErrNonLeader = errors.New("the elector is not leader")
	ErrClosed    = errors.New("the elector is already closed")
	ErrPingEtcd  = errors.New("ping etcd server timeout")
)

var (
	// alias options
	WithTTL     = concurrency.WithTTL
	WithContext = concurrency.WithContext
	WithLease   = concurrency.WithLease
)

type (
	// alias clientv3.config
	Config = clientv3.Config
)

func nullLogger(msg ...interface{}) {}

var _ gocron.Elector = (*Elector)(nil)

type Elector struct {
	ctx    context.Context
	cancel context.CancelFunc

	config  clientv3.Config
	options []concurrency.SessionOption
	client  *clientv3.Client
	id      string

	mu       sync.RWMutex
	closed   bool
	isLeader bool
	leaderID string

	logger func(msg ...interface{})
}

func NewElector(ctx context.Context, cfg clientv3.Config, options ...concurrency.SessionOption) (*Elector, error) {
	return newElector(ctx, nil, cfg, options...)
}

func NewElectorWithClient(ctx context.Context, cli *clientv3.Client, options ...concurrency.SessionOption) (*Elector, error) {
	return newElector(ctx, cli, Config{}, options...)
}

func newElector(ctx context.Context, cli *clientv3.Client, cfg clientv3.Config, options ...concurrency.SessionOption) (*Elector, error) {
	var err error
	if cli == nil {
		cli, err = clientv3.New(cfg) // async dial etcd
		if err != nil {
			return nil, err
		}
	}

	cctx, cancel := context.WithCancel(ctx)
	el := &Elector{
		ctx:     cctx,
		cancel:  cancel,
		config:  cfg,
		options: options,
		id:      getID(),
		client:  cli,
		logger:  nullLogger,
	}

	err = el.pingEtcd("/")
	if err != nil {
		return nil, err
	}
	return el, nil
}

func (e *Elector) SetLogger(fn func(msg ...interface{})) {
	e.logger = fn
}

func (e *Elector) GetID() string {
	return e.id
}

func (e *Elector) GetLeaderID() string {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.leaderID
}

func (e *Elector) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return nil
	}

	e.cancel()
	e.closed = true
	e.client.Close()
	return nil
}

func (e *Elector) IsLeader(_ context.Context) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.isLeader {
		return nil
	}

	return ErrNonLeader
}

func (e *Elector) setLeader(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.isLeader = true
	e.leaderID = id
}

func (e *Elector) unsetLeader(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.isLeader = false
	e.leaderID = id
}

func (e *Elector) pingEtcd(electionPath string) error {
	timeoutCtx, cancel := context.WithTimeout(e.ctx, 6*time.Second)
	defer cancel()

	_, _ = e.client.KV.Get(timeoutCtx, electionPath)
	if timeoutCtx.Err() == context.DeadlineExceeded {
		return ErrPingEtcd
	}
	return nil
}

// Start Start the election.
// This method will keep trying the election. When the election is successful, set isleader to true.
// If it fails, the election directory will be monitored until the election is successful.
// The parameter electionPath is used to specify the etcd directory for the operation.
func (e *Elector) Start(electionPath string) error {
	if e.closed {
		return ErrClosed
	}

	session, err := concurrency.NewSession(e.client, e.options...)
	if err != nil {
		return err
	}
	defer session.Close()

	electionHandler := concurrency.NewElection(session, electionPath)
	go func() {
		for e.ctx.Err() == nil {
			// If the election cannot be obtained, it will be blocked until the election can be obtained.
			if err := electionHandler.Campaign(e.ctx, e.id); err != nil {
				e.logger(fmt.Errorf("election failed to campaign, err: %w", err))
			}

			time.Sleep(100 * time.Millisecond)
		}
	}()

	defer func() {
		// unset leader
		e.unsetLeader("")
	}()

	ch := electionHandler.Observe(e.ctx)
	for e.ctx.Err() == nil {
		select {
		case resp := <-ch:
			if len(resp.Kvs) == 0 {
				continue
			}

			for i := 0; i < len(resp.Kvs); i++ {
				val := string(resp.Kvs[i].Value)
				if val != e.id {
					e.unsetLeader(val)
					e.logger("switch to non-leader, the current leader is ", val)
					continue
				}

				if e.IsLeader(e.ctx) != nil { // is non-leader
					e.setLeader(val)
					e.logger("switch to leader, the current instance is leader")
				}
			}

		case <-e.ctx.Done():
			return nil
		}
	}

	return nil
}

func getID() string {
	hostname, _ := os.Hostname()
	bs := make([]byte, 10)
	_, err := rand.Read(bs)
	if err != nil {
		return fmt.Sprintf("%s-%d", hostname, mrand.Int63())
	}
	return fmt.Sprintf("%s-%x", hostname, bs)
}
