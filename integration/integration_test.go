package integration

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestRunUDP(t *testing.T) {
	configFile1 := filepath.Join(os.TempDir(), uuid.New().String()+".json")
	defer os.RemoveAll(configFile1)
	configFile2 := filepath.Join(os.TempDir(), uuid.New().String()+".json")
	defer os.RemoveAll(configFile2)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	require.NoError(t, run(ctx, t, "--config-file", configFile1, "init"))
	require.NoError(t, run(ctx, t, "--config-file", configFile2, "init"))

	var config1 struct {
		KeyPair struct {
			Public, Private string
		}
	}
	bs, err := os.ReadFile(configFile1)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(bs, &config1))

	var config2 struct {
		KeyPair struct {
			Public, Private string
		}
	}
	bs, err = os.ReadFile(configFile2)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(bs, &config2))

	routeArgs := []string{
		"add-route",
		"--local-port", "10001",
		"--remote-port", "10002",
		"--local-peer", config1.KeyPair.Public,
		"--remote-peer", config2.KeyPair.Public,
		"--type", "UDP",
	}
	require.NoError(t, run(ctx, t, append([]string{"--config-file", configFile1}, routeArgs...)...))
	require.NoError(t, run(ctx, t, append([]string{"--config-file", configFile2}, routeArgs...)...))

	client, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 10001,
	})
	require.NoError(t, err)

	server, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 10002,
	})
	require.NoError(t, err)

	eg, egctx := errgroup.WithContext(ctx)
	eg.Go(func() error { return run(egctx, t, "--config-file", configFile1, "run") })
	eg.Go(func() error { return run(egctx, t, "--config-file", configFile2, "run") })
	eg.Go(func() error {
		b := make([]byte, 40)
		for {
			n, _, _, _, err := server.ReadMsgUDP(b, nil)
			if err != nil {
				return err
			}
			if bytes.Equal(b[:n], []byte("TEST")) {
				return context.Canceled
			}
		}
	})
	eg.Go(func() error {
		ctx, clearTimeout := context.WithTimeout(egctx, time.Second*10)
		defer clearTimeout()

		ticker := time.NewTicker(time.Millisecond * 50)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
			}
			_, _, _ = client.WriteMsgUDP([]byte("TEST"), nil, nil)
		}
	})
	eg.Go(func() error {
		<-egctx.Done()
		_ = server.Close()
		_ = client.Close()
		return nil
	})
	if err := eg.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		require.NoError(t, err)
	}
}

func run(ctx context.Context, t *testing.T, args ...string) error {
	cmd := exec.CommandContext(ctx, "go", append([]string{"run", "../cmd/rtctunnel"}, args...)...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	go func() {
		s := bufio.NewScanner(stdout)
		for s.Scan() {
			t.Log(s.Text())
		}
	}()
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	go func() {
		s := bufio.NewScanner(stderr)
		for s.Scan() {
			t.Log(s.Text())
		}
	}()
	return cmd.Run()
}
