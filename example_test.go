package elector

import (
	"context"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func ExampleWithContext() {
	cfg := Config{
		Endpoints:   []string{"http://127.0.0.1:2379"},
		DialTimeout: 3 * time.Second,
	}

	myContext := context.Background()

	_, err := NewElector(context.Background(), cfg, WithContext(myContext))
	if err != nil {
		panic(err)
	}
}

func ExampleWithLease() {
	cfg := Config{
		Endpoints:   []string{"http://127.0.0.1:2379"},
		DialTimeout: 3 * time.Second,
	}

	_, err := NewElector(context.Background(), cfg, WithLease(clientv3.LeaseID(1234)))
	if err != nil {
		panic(err)
	}
}

func ExampleNewElectorWithClient() {
	cfg := Config{
		Endpoints:   []string{"http://127.0.0.1:2379"},
		DialTimeout: 3 * time.Second,
	}

	client, err := clientv3.New(cfg)
	if err != nil {
		panic(err)
	}

	_, err = NewElectorWithClient(context.Background(), client, WithLease(clientv3.LeaseID(1234)))
	if err != nil {
		panic(err)
	}
}
