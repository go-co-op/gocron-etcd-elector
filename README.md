# gocron-etcd-elector

## install

```
go get github.com/go-co-op/gocron-etcd-elector
```

## usage
### gocron v2
For a full production environment example with etcd clusters, you can see [gocron-etcd-elector-example](https://github.com/seriouspoop/gocron-etcd-elector-example)

Here is an example usage that would be deployed in multiple instances.

```go
package main

import (
	"context"
	"fmt"
	"time"

	elector "github.com/go-co-op/gocron-etcd-elector"
	"github.com/go-co-op/gocron/v2"
)

func main() {
	// Printing some value to verify if container is running in docker logs
	fmt.Println("go-app")

	// Configuring elector with etdc cluster
	cfg := elector.Config{
		Endpoints:   []string{"http://etcd-1:2379", "http://etcd-2:2379", "http://etcd-3:2379"},
		DialTimeout: 3 * time.Second,
	}

	// Build new elector
	el, err := elector.NewElector(context.Background(), cfg, elector.WithTTL(10))
	if err != nil {
		panic(err)
	}

	// el.Start() is a blocking method
	// so running with goroutine
	go func() {
		err := el.Start("/rahul-gandhi/pappu") // specify a directory for storing key value for election
		if err == elector.ErrClosed {
			return
		}
	}()

	// New scheduler with elector
	sh, err := gocron.NewScheduler(gocron.WithDistributedElector(el))
	if err != nil {
		panic(err)
	}

	// The scheduler elected as the leader is only allowed to run, other instances don't execute
	_, err = sh.NewJob(gocron.DurationJob(1*time.Second), gocron.NewTask(func() {
		// This if statement doesn't work as intended as only the leader is running
		// So true always
		if el.IsLeader(context.Background()) == nil {
			fmt.Println("Current instance is leader")
			fmt.Println("executing job")
		} else {
			// To see this log printed remove gocron.WithDistributedElector(el) option from the scheduler
			fmt.Printf("Not leader, current leader is %s\n", el.GetLeaderID())
		}
	}))
	if err != nil {
		panic(err)
	}

	sh.Start()
	fmt.Println("exit")
}
```
