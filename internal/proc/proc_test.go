package proc

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSplitLines(t *testing.T) {
	var got []string
	SplitLines(strings.NewReader("one\r\ntwo\rthree\n\n\nfour\xff"), func(s string) { got = append(got, s) })
	want := []string{"one", "two", "three", "four�"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBuildEnv(t *testing.T) {
	env := buildEnv([]string{"HOME=/h", "Path=/usr/bin"}, "/tools", []string{"X=1"})
	want := []string{"HOME=/h", "Path=/tools" + string(os.PathListSeparator) + "/usr/bin", "X=1"}
	if !reflect.DeepEqual(env, want) {
		t.Fatalf("got %q, want %q", env, want)
	}
	if env := buildEnv([]string{"HOME=/h"}, "/tools", nil); env[len(env)-1] != "PATH=/tools" {
		t.Fatalf("PATH not added: %q", env)
	}
}

// The test binary doubles as the program Run starts.
func TestMain(m *testing.M) {
	switch os.Getenv("PROC_HELPER") {
	case "lines":
		fmt.Print("progress 1%\rprogress 2%\nready\n")
		fmt.Fprintln(os.Stderr, "WARNING: careful")
		os.Exit(3)
	case "sleep":
		fmt.Println("started")
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func helper(mode string) Spec {
	return Spec{Path: os.Args[0], Env: []string{"PROC_HELPER=" + mode}}
}

func TestRunCollectsLinesAndExitCode(t *testing.T) {
	var stdout, stderr []string
	code, err := Run(context.Background(), helper("lines"), func(l Line) {
		if l.Stderr {
			stderr = append(stderr, l.Text)
		} else {
			stdout = append(stdout, l.Text)
		}
	})
	if err != nil || code != 3 {
		t.Fatalf("code %d, err %v", code, err)
	}
	if !reflect.DeepEqual(stdout, []string{"progress 1%", "progress 2%", "ready"}) {
		t.Errorf("stdout %q", stdout)
	}
	if !reflect.DeepEqual(stderr, []string{"WARNING: careful"}) {
		t.Errorf("stderr %q", stderr)
	}
}

func TestRunCancelEndsTheProgram(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	start := time.Now()
	_, err := Run(ctx, helper("sleep"), func(l Line) {
		if l.Text == "started" {
			cancel()
		}
	})
	if err != context.Canceled {
		t.Fatalf("err %v, want context.Canceled", err)
	}
	if time.Since(start) > 20*time.Second {
		t.Fatal("cancel did not end the program")
	}
}
