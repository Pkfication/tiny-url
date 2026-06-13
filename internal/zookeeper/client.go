package zookeeper

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/go-zookeeper/zk"
)

const (
	counterPath = "/kgs-counter"
)

type Client struct {
	conn *zk.Conn
}

func NewClient(zkServers string) (*Client, error) {
	servers := strings.Split(zkServers, ",")

	conn, eventChan, err := zk.Connect(servers, time.Second*5)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ZooKeeper: %v", err)
	}

	// Wait for connection to establish
	connected := false
	timeout := time.After(10 * time.Second)
	for !connected {
		select {
		case event := <-eventChan:
			if event.State == zk.StateHasSession {
				connected = true
			}
		case <-timeout:
			return nil, fmt.Errorf("timed out waiting for ZooKeeper connection")
		}
	}

	client := &Client{conn: conn}
	if err := client.ensureCounterNode(); err != nil {
		return nil, err
	}

	return client, nil
}

func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *Client) ensureCounterNode() error {
	exists, _, err := c.conn.Exists(counterPath)
	if err != nil {
		return fmt.Errorf("failed to check node existence: %v", err)
	}

	if !exists {
		_, err = c.conn.Create(counterPath, []byte("0"), 0, zk.WorldACL(zk.PermAll))
		if err != nil && err != zk.ErrNodeExists {
			return fmt.Errorf("failed to create counter node: %v", err)
		}
	}
	return nil
}

// FetchRange fetches a block of keys from ZooKeeper atomically
func (c *Client) FetchRange(rangeSize uint64) (uint64, uint64, error) {
	for {
		data, stat, err := c.conn.Get(counterPath)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to get counter: %v", err)
		}

		currentCount, err := strconv.ParseUint(string(data), 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid counter data: %v", err)
		}

		newCount := currentCount + rangeSize
		newData := []byte(strconv.FormatUint(newCount, 10))

		_, err = c.conn.Set(counterPath, newData, stat.Version)
		if err != nil {
			if err == zk.ErrBadVersion {
				// Another node updated the counter, retry
				continue
			}
			return 0, 0, fmt.Errorf("failed to update counter: %v", err)
		}

		log.Printf("Fetched new range: [%d, %d)", currentCount, newCount)
		return currentCount, newCount, nil
	}
}
