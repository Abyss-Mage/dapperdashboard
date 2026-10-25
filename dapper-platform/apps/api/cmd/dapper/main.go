// dapper: the installer and launcher for a fresh checkout.
//
//	dapper setup   open the browser wizard (database, site, email, first owner); writes .dapper/config.json
//	dapper start   run the application from that configuration (API, web app and HTTPS gateway)
//	dapper status  say whether this checkout has been set up
//
// Run it from anywhere inside the repository: go run ./cmd/dapper setup (from dapper-platform/apps/api), or use scripts/setup.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"dapper.local/platform/internal/install"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	home := fs.String("home", "", "repository root (default: found by walking up from the current folder)")
	addr := fs.String("listen", "127.0.0.1:8099", "setup wizard address (setup only)")
	_ = fs.Parse(args)
	root, err := repoRoot(*home)
	if err != nil {
		log.Fatal(err)
	}
	switch cmd {
	case "setup":
		runSetup(root, *addr)
	case "start":
		runStart(root)
	case "status":
		if install.Installed(root) {
			fmt.Println("Installed. Configuration:", install.ConfigPath(root))
		} else {
			fmt.Println("Not installed. Run: dapper setup")
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() { fmt.Fprintln(os.Stderr, "usage: dapper setup | start | status   [--home <repository root>]") }

func repoRoot(explicit string) (string, error) {
	marker := filepath.Join("dapper-platform", "apps", "api", "go.mod")
	if explicit != "" {
		if _, err := os.Stat(filepath.Join(explicit, marker)); err != nil {
			return "", errors.New("--home is not a Dapper repository root")
		}
		return filepath.Abs(explicit)
	}
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("run this from inside the Dapper repository (or pass --home)")
		}
		dir = parent
	}
}

func runSetup(root, addr string) {
	if install.Installed(root) {
		fmt.Println("This checkout is already set up (" + install.ConfigPath(root) + ").")
		fmt.Println("To run setup again, stop Dapper and delete the .dapper folder. Your database is not touched by that.")
		return
	}
	srv, err := install.NewServer(root, addr)
	if err != nil {
		log.Fatal(err)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal("setup could not listen on " + addr + ": " + err.Error())
	}
	host, port, _ := net.SplitHostPort(addr)
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	fmt.Println()
	fmt.Println("  Dapper setup is running.")
	fmt.Printf("  Open:        http://%s:%s\n", host, port)
	fmt.Printf("  Setup code:  %s\n", srv.Token)
	fmt.Println("  (enter the code on the first page; it stops when setup finishes)")
	fmt.Println()
	hs := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 4 * time.Minute, IdleTimeout: 60 * time.Second}
	go func() {
		if err := hs.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case <-srv.Done:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = hs.Shutdown(ctx)
		fmt.Println("  Setup finished. Start Dapper with: dapper start")
	case <-stop:
		fmt.Println("  Setup stopped before finishing; nothing was installed unless it said so above.")
	}
}

type service struct {
	name string
	cmd  *exec.Cmd
}

func exe(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// build compiles the two Go services into .dapper/bin the first time (and whenever the sources are newer is left to the operator).
func build(root string) error {
	if err := os.MkdirAll(install.BinDir(root), 0o700); err != nil {
		return err
	}
	for _, target := range []string{"workspace", "gateway"} {
		out := filepath.Join(install.BinDir(root), exe(target))
		if _, err := os.Stat(out); err == nil {
			continue
		}
		fmt.Println("  Building", target, "…")
		c := exec.Command("go", "build", "-o", out, "./cmd/"+target)
		c.Dir = filepath.Join(root, "dapper-platform", "apps", "api")
		c.Stdout, c.Stderr = os.Stdout, os.Stderr
		if err := c.Run(); err != nil {
			return fmt.Errorf("building %s failed: %w", target, err)
		}
	}
	return nil
}

func runStart(root string) {
	if !install.Installed(root) {
		log.Fatal("Not set up yet. Run: dapper setup")
	}
	cfg, err := install.Load(root)
	if err != nil {
		log.Fatal(err)
	}
	web := filepath.Join(root, "dapper-platform", "apps", "web")
	if _, err = os.Stat(filepath.Join(web, ".next", "BUILD_ID")); err != nil {
		log.Fatal("The web application is not built. Run:  cd dapper-platform/apps/web  &&  npm ci  &&  npm run build")
	}
	if err = build(root); err != nil {
		log.Fatal(err)
	}
	logDir := filepath.Join(install.Dir(root), "logs")
	_ = os.MkdirAll(logDir, 0o700)
	env := append(os.Environ(), cfg.Env(root)...)
	node, err := exec.LookPath("node")
	if err != nil {
		log.Fatal("node was not found on PATH")
	}
	specs := []struct {
		name, dir, bin string
		args           []string
	}{
		{"api", filepath.Join(root, "dapper-platform", "apps", "api"), filepath.Join(install.BinDir(root), exe("workspace")), nil},
		{"web", web, node, []string{"node_modules/next/dist/bin/next", "start", "--hostname", "127.0.0.1", "--port", "3006"}},
		{"gateway", filepath.Join(root, "dapper-platform", "apps", "api"), filepath.Join(install.BinDir(root), exe("gateway")), nil},
	}
	var services []service
	var logs []*os.File
	for _, sp := range specs {
		f, err := os.OpenFile(filepath.Join(logDir, sp.name+".log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			log.Fatal(err)
		}
		logs = append(logs, f)
		c := exec.Command(sp.bin, sp.args...)
		c.Dir, c.Env, c.Stdout, c.Stderr = sp.dir, env, f, f
		if err = c.Start(); err != nil {
			for _, s := range services {
				_ = s.cmd.Process.Kill()
			}
			log.Fatalf("could not start %s: %v", sp.name, err)
		}
		services = append(services, service{sp.name, c})
	}
	exited := make(chan string, len(services))
	var wg sync.WaitGroup
	for _, s := range services {
		wg.Add(1)
		go func(s service) { defer wg.Done(); _ = s.cmd.Wait(); exited <- s.name }(s)
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		client := &http.Client{Timeout: 2 * time.Second}
		for i := 0; i < 60; i++ {
			if resp, err := client.Get("http://127.0.0.1:8121/api/v1/health"); err == nil {
				resp.Body.Close()
				if resp.StatusCode == 200 {
					fmt.Println("  Dapper is running at", cfg.Site.URL, "  (dashboard:", cfg.Site.URL+"/dashboard, staff workspace:", cfg.Site.URL+"/workspace)")
					fmt.Println("  Logs:", logDir, " Press Ctrl+C to stop.")
					return
				}
			}
			time.Sleep(time.Second)
		}
		fmt.Println("  Dapper did not report healthy within a minute; check the logs in", logDir)
	}()
	select {
	case name := <-exited:
		fmt.Println("  The", name, "service stopped; stopping the rest. See", logDir)
	case <-stop:
		fmt.Println("  Stopping…")
	}
	for _, s := range services {
		if s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
		}
	}
	wg.Wait()
	for _, f := range logs {
		f.Close()
	}
}
