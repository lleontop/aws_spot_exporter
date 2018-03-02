package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	_ "testing"
	"time"
)

var (
	binary = filepath.Join(os.Getenv("GOPATH"), "bin/aws_spot_exporter.exe")
)

const (
	address = "localhost:19190"
)

func TestHandlingOfDuplicatedMetrics(t *testing.T) {
	if _, err := os.Stat(binary); err != nil {
		t.Skipf("aws_spot_exporter binary not available: %s", err)
	}

	dir, err := ioutil.TempDir("", "aws_spot_exporter")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	exporter := exec.Command(binary, "--web.listen-address", address)
	test := func(_ int) error {
		return queryExporter(address)
	}

	if err := runCommandAndTests(exporter, address, test); err != nil {
		t.Error(err)
	}
}

func queryExporter(address string) error {
	resp, err := http.Get(fmt.Sprintf("http://%s/metrics", address))
	if err != nil {
		return err
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if err := resp.Body.Close(); err != nil {
		return err
	}
	if want, have := http.StatusOK, resp.StatusCode; want != have {
		return fmt.Errorf("want /metrics status code %d, have %d. Body:\n%s", want, have, b)
	}
	return nil
}

func runCommandAndTests(cmd *exec.Cmd, address string, fn func(pid int) error) error {
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %s", err)
	}
	time.Sleep(50 * time.Millisecond)
	for i := 0; i < 10; i++ {
		if err := queryExporter(address); err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
		if cmd.Process == nil || i == 9 {
			return fmt.Errorf("can't start command")
		}
	}

	errc := make(chan error)
	go func(pid int) {
		errc <- fn(pid)
	}(cmd.Process.Pid)

	err := <-errc
	if cmd.Process != nil {
		cmd.Process.Kill()
	}
	return err
}
