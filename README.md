# gocron-etcd-elector

## install

```
go get github.com/go-co-op/gocron-etcd-elector
```

## usage

Here is an example usage that would be deployed in multiple instances.

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-co-op/gocron"
	elector "github.com/go-co-op/gocron-etcd-elector"
)

func main() {
	cfg := elector.Config{
		Endpoints:   []string{"http://127.0.0.1:2379"},
		DialTimeout: 3 * time.Second,
	}

	el, err := elector.NewElector(context.Background(), cfg, elector.WithTTL(10))
	if err != nil {
		panic(err)
	}

	go func() {
		for {
			err := el.Start("/gocron/elector")
			if err == elector.ErrClosed {
				return
			}

			time.Sleep(1e9)
		}
	}()

	s := gocron.NewScheduler(time.UTC)
	s.WithDistributedElector(el)

	_, _ = s.Every("1s").Do(func() {
		if el.IsLeader(context.Background()) == nil {
			fmt.Println("the current instance is leader")
		} else {
			fmt.Println("the current leader is", el.GetLeaderID())
		}

		fmt.Println("call 1s")
	})

	s.StartAsync()

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	<-c

	fmt.Println("exit")
}
```
