package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

type options struct {
	BaseURL        string
	UsersFile      string
	Rate           float64
	Duration       time.Duration
	Drain          time.Duration
	RequestTimeout time.Duration
	ConnectWorkers int
	ClientQueue    int
	ReportInterval time.Duration
	Text           string
	PrepareUsers   int
	PrepareGroups  int
	GroupMembers   int
	Password       string
	GroupName      string
}

type loadUser struct {
	UserID          uint   `json:"user_id"`
	AccessToken     string `json:"access_token"`
	RefreshToken    string `json:"refresh_token,omitempty"`
	ConversationIDs []uint `json:"conversation_ids"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "message-loadtest:", err)
		os.Exit(1)
	}
}

func run() error {
	opts, err := parseOptions()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()
	if opts.PrepareGroups > 0 {
		return prepareLoadUsers(ctx, opts)
	}

	users, err := loadUsers(opts.UsersFile)
	if err != nil {
		return err
	}

	stats := newStatistics(users)
	clients, err := connectClients(ctx, opts, users, stats)
	if err != nil {
		return err
	}
	defer closeClients(clients)

	stats.start()
	fmt.Printf(
		"ready: users=%d conversations=%d rate=%.2f/s duration=%s\n",
		len(clients),
		conversationCount(users),
		opts.Rate,
		opts.Duration,
	)

	interrupted := sendLoad(ctx, opts, clients, stats)
	stopClientSenders(clients)
	stats.stopSending()
	fmt.Printf("sending stopped, draining for %s\n", opts.Drain)
	wait(opts.Drain)

	syncCtx, cancel := context.WithTimeout(
		context.Background(),
		opts.RequestTimeout*time.Duration(len(clients)+1),
	)
	defer cancel()
	if err := syncClients(syncCtx, clients); err != nil {
		stats.recordSyncError()
		fmt.Fprintln(os.Stderr, "message sync:", err)
	}

	stats.finish()
	stats.printReport()
	if interrupted {
		return errors.New("load test interrupted")
	}
	if stats.failed() {
		return errors.New("load test correctness check failed")
	}
	return nil
}

func parseOptions() (options, error) {
	var opts options
	flag.StringVar(&opts.BaseURL, "base-url", "http://127.0.0.1:6666", "mmchat HTTP base URL")
	flag.StringVar(&opts.UsersFile, "users", "", "load users JSON input or preparation output")
	flag.Float64Var(&opts.Rate, "rate", 100, "global message rate per second")
	flag.DurationVar(&opts.Duration, "duration", 5*time.Minute, "sending duration")
	flag.DurationVar(&opts.Drain, "drain", 30*time.Second, "wait for final events after sending")
	flag.DurationVar(&opts.RequestTimeout, "timeout", 10*time.Second, "HTTP and WebSocket setup timeout")
	flag.IntVar(&opts.ConnectWorkers, "connect-workers", 32, "parallel connection workers")
	flag.IntVar(&opts.ClientQueue, "client-queue", 1024, "outbound queue size per client")
	flag.DurationVar(&opts.ReportInterval, "report-interval", 5*time.Second, "progress report interval")
	flag.StringVar(&opts.Text, "text", "load-test", "message text prefix")
	flag.IntVar(&opts.PrepareUsers, "prepare-users", 0, "create users for one group, then exit")
	flag.IntVar(&opts.PrepareGroups, "prepare-groups", 0, "create multiple groups, then exit")
	flag.IntVar(&opts.GroupMembers, "group-members", 0, "users in each prepared group")
	flag.StringVar(&opts.Password, "password", "LoadTest#2026", "password for prepared users")
	flag.StringVar(&opts.GroupName, "group-name", "message-load-test", "prepared group name")
	flag.Parse()

	opts.BaseURL = strings.TrimRight(strings.TrimSpace(opts.BaseURL), "/")
	switch {
	case opts.BaseURL == "":
		return opts, errors.New("base-url is required")
	case opts.UsersFile == "":
		return opts, errors.New("users is required")
	case opts.Rate <= 0:
		return opts, errors.New("rate must be positive")
	case opts.Duration <= 0 || opts.Drain < 0 || opts.RequestTimeout <= 0:
		return opts, errors.New("durations are invalid")
	case opts.ConnectWorkers <= 0 || opts.ClientQueue <= 0:
		return opts, errors.New("worker and queue sizes must be positive")
	case opts.ReportInterval <= 0:
		return opts, errors.New("report interval must be positive")
	case strings.TrimSpace(opts.Text) == "":
		return opts, errors.New("text must not be empty")
	case opts.PrepareUsers < 0 || opts.PrepareUsers == 1:
		return opts, errors.New("prepare-users must be 0 or at least 2")
	case opts.PrepareGroups < 0 || opts.GroupMembers < 0:
		return opts, errors.New("prepare-groups and group-members must not be negative")
	case opts.PrepareUsers > 0 && (opts.PrepareGroups > 0 || opts.GroupMembers > 0):
		return opts, errors.New("prepare-users cannot be combined with prepare-groups or group-members")
	case opts.PrepareGroups == 0 && opts.GroupMembers > 0:
		return opts, errors.New("prepare-groups is required with group-members")
	case opts.PrepareGroups > 0 && opts.GroupMembers < 2:
		return opts, errors.New("group-members must be at least 2")
	}
	if opts.PrepareUsers > 0 {
		opts.PrepareGroups, opts.GroupMembers = 1, opts.PrepareUsers
	}
	switch {
	case opts.PrepareGroups > 0 && strings.TrimSpace(opts.Password) == "":
		return opts, errors.New("password must not be empty")
	case opts.PrepareGroups > 0 && strings.TrimSpace(opts.GroupName) == "":
		return opts, errors.New("group-name must not be empty")
	}
	return opts, nil
}

func loadUsers(path string) ([]loadUser, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open users: %w", err)
	}
	defer file.Close()

	var users []loadUser
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&users); err != nil {
		return nil, fmt.Errorf("decode users: %w", err)
	}
	if len(users) == 0 {
		return nil, errors.New("users are empty")
	}

	seenUsers := make(map[uint]struct{}, len(users))
	for i := range users {
		user := &users[i]
		if user.UserID == 0 || strings.TrimSpace(user.AccessToken) == "" || len(user.ConversationIDs) == 0 {
			return nil, fmt.Errorf("user %d is invalid", i)
		}
		if _, exists := seenUsers[user.UserID]; exists {
			return nil, fmt.Errorf("user_id %d is duplicated", user.UserID)
		}
		seenUsers[user.UserID] = struct{}{}

		seenConversations := make(map[uint]struct{}, len(user.ConversationIDs))
		for _, conversationID := range user.ConversationIDs {
			if conversationID == 0 {
				return nil, fmt.Errorf("user %d has invalid conversation_id", user.UserID)
			}
			if _, exists := seenConversations[conversationID]; exists {
				return nil, fmt.Errorf(
					"user %d has duplicated conversation_id %d",
					user.UserID,
					conversationID,
				)
			}
			seenConversations[conversationID] = struct{}{}
		}
	}
	return users, nil
}

func connectClients(
	ctx context.Context,
	opts options,
	users []loadUser,
	stats *statistics,
) ([]*loadClient, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	clients := make([]*loadClient, len(users))
	jobs := make(chan int)
	workerCount := min(opts.ConnectWorkers, len(users))
	var wg sync.WaitGroup
	var firstErr error
	var errOnce sync.Once

	for range workerCount {
		wg.Go(func() {
			for index := range jobs {
				client, err := newLoadClient(
					ctx,
					opts.BaseURL,
					users[index],
					opts.RequestTimeout,
					opts.ClientQueue,
					stats,
				)
				if err != nil {
					errOnce.Do(func() {
						firstErr = fmt.Errorf("connect user %d: %w", users[index].UserID, err)
						cancel()
					})
					continue
				}
				clients[index] = client
			}
		})
	}

sendJobs:
	for index := range users {
		select {
		case jobs <- index:
		case <-ctx.Done():
			break sendJobs
		}
	}
	close(jobs)
	wg.Wait()
	if firstErr != nil {
		closeClients(clients)
		return nil, firstErr
	}
	return clients, nil
}

func sendLoad(
	ctx context.Context,
	opts options,
	clients []*loadClient,
	stats *statistics,
) bool {
	interval := time.Duration(float64(time.Second) / opts.Rate)
	if interval < time.Nanosecond {
		interval = time.Nanosecond
	}
	sendTicker := time.NewTicker(interval)
	reportTicker := time.NewTicker(opts.ReportInterval)
	deadline := time.NewTimer(opts.Duration)
	defer sendTicker.Stop()
	defer reportTicker.Stop()
	defer deadline.Stop()

	index := 0
	for {
		select {
		case <-ctx.Done():
			return true
		case <-deadline.C:
			return false
		case <-reportTicker.C:
			stats.printProgress()
		case <-sendTicker.C:
			client := clients[index%len(clients)]
			index++
			if client.enqueueMessage(opts.Text) {
				stats.recordOffered()
			} else {
				stats.recordLocalDrop()
			}
		}
	}
}

func syncClients(ctx context.Context, clients []*loadClient) error {
	var wg sync.WaitGroup
	var firstErr error
	var errOnce sync.Once
	for _, client := range clients {
		wg.Go(func() {
			if err := client.syncMessages(ctx); err != nil {
				errOnce.Do(func() { firstErr = err })
			}
		})
	}
	wg.Wait()
	return firstErr
}

func closeClients(clients []*loadClient) {
	for _, client := range clients {
		if client != nil {
			client.close()
		}
	}
}

func stopClientSenders(clients []*loadClient) {
	for _, client := range clients {
		client.stopSending()
	}
}

func conversationCount(users []loadUser) int {
	conversations := make(map[uint]struct{})
	for _, user := range users {
		for _, conversationID := range user.ConversationIDs {
			conversations[conversationID] = struct{}{}
		}
	}
	return len(conversations)
}

func wait(duration time.Duration) {
	if duration > 0 {
		time.Sleep(duration)
	}
}
