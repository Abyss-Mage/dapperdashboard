// dapper: the installer and launcher for a fresh checkout.
//
//	dapper start    run Dapper. If there is no completed .env yet, the setup wizard opens FIRST; when it finishes Dapper starts.
//	dapper setup    only the browser wizard (database, site, email, first owner); writes .env
//	dapper migrate  apply database migrations after an update (git pull)
//	dapper status   say whether this checkout has been set up
//
// Run it from anywhere inside the repository: scripts/start.ps1 (Windows) or scripts/start.sh (macOS/Linux) do the same.
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
	"dapper.local/platform/internal/postgres"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	home := fs.String("home", "", "repository root (default: found by walking up from the current folder)")
	addr := fs.String("listen", "127.0.0.1:8099", "setup wizard address")
	noOpen := fs.Bool("no-browser", false, "do not open the setup page in a browser automatically")
	_ = fs.Parse(args)
	root, err := repoRoot(*home)
	if err != nil {
		log.Fatal(err)
	}
	switch cmd {
	case "setup":
		if !runSetup(root, *addr, !*noOpen) {
			os.Exit(1)
		}
	case "start":
		runStart(root, *addr, !*noOpen)
	case "migrate":
		runMigrate(root)
	case "status":
		if install.Installed(root) {
			fmt.Println("Installed. Configuration:", install.ConfigPath(root))
		} else {
			fmt.Println("Not installed: there is no completed .env. Run: dapper start (setup opens first)")
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: dapper start | setup | migrate | status   [--home <repository root>] [--listen 127.0.0.1:8099] [--no-browser]")
}

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

func openBrowser(url string) {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		c = exec.Command("open", url)
	default:
		c = exec.Command("xdg-open", url)
	}
	_ = c.Start()
}

// listenFree binds the wizard's address, trying the next few ports when the first is busy.
func listenFree(addr string) (net.Listener, string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, "", err
	}
	var p int
	fmt.Sscanf(port, "%d", &p)
	for i := 0; i < 10; i++ {
		a := net.JoinHostPort(host, fmt.Sprint(p+i))
		if ln, err := net.Listen("tcp", a); err == nil {
			return ln, a, nil
		}
	}
	return nil, "", errors.New("no free port near " + addr)
}

// runSetup serves the wizard until it finishes; it returns true when a completed .env has been written.
func runSetup(root, addr string, open bool) bool {
	if install.Installed(root) {
		fmt.Println("This checkout is already set up (" + install.ConfigPath(root) + ").")
		fmt.Println("To run setup again, stop Dapper and delete the .env file. Your database is not touched by that.")
		return true
	}
	ln, bound, err := listenFree(addr)
	if err != nil {
		log.Fatal("setup could not listen: " + err.Error())
	}
	srv, err := install.NewServer(root, bound)
	if err != nil {
		log.Fatal(err)
	}
	host, port, _ := net.SplitHostPort(bound)
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	page := fmt.Sprintf("http://%s:%s", host, port)
	fmt.Println()
	fmt.Println("  No .env was found, so Dapper setup is opening first.")
	fmt.Println("  Open:        " + page)
	fmt.Printf("  Setup code:  %s\n", srv.Token)
	fmt.Println("  (enter the code on the first page; setup stops by itself when it finishes)")
	fmt.Println()
	hs := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 4 * time.Minute, IdleTimeout: 60 * time.Second}
	go func() {
		if err := hs.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	if open {
		openBrowser(page)
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)
	select {
	case <-srv.Done:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = hs.Shutdown(ctx)
		fmt.Println("  Setup finished; .env was written.")
		return true
	case <-stop:
		fmt.Println("  Setup stopped before finishing; nothing was installed unless it said so above.")
		return false
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

// build compiles the two Go services into .dapper/bin the first time.
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

// ensureWeb installs the web dependencies and builds the web app the first time (this takes a few minutes).
func ensureWeb(root string) error {
	web := filepath.Join(root, "dapper-platform", "apps", "web")
	if _, err := os.Stat(filepath.Join(web, ".next", "BUILD_ID")); err == nil {
		return nil
	}
	npm, err := exec.LookPath("npm")
	if err != nil {
		return errors.New("npm was not found on PATH; install Node.js (https://nodejs.org) and run again")
	}
	fmt.Println("  Building the web application for the first time (a few minutes) …")
	for _, args := range [][]string{{"ci"}, {"run", "build"}} {
		c := exec.Command(npm, args...)
		c.Dir, c.Stdout, c.Stderr = web, os.Stdout, os.Stderr
		if err := c.Run(); err != nil {
			return fmt.Errorf("npm %v failed: %w", args, err)
		}
	}
	return nil
}

func runMigrate(root string) {
	cfg, err := install.Load(root)
	if err != nil {
		log.Fatal("Not set up yet. Run: dapper start (setup opens first)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	store, err := postgres.Open(ctx, cfg.Database.URL)
	if err != nil {
		log.Fatal("Could not reach the database from .env: " + err.Error())
	}
	defer store.Close()
	if err = store.Migrate(ctx); err != nil {
		log.Fatal("Migration failed; nothing was half-applied by this command. Restore from your backup if the database is in doubt.")
	}
	fmt.Println("Database is up to date.")
}

func runStart(root, setupAddr string, open bool) {
	// Setup comes first: until a completed .env exists nothing else runs.
	if !install.Installed(root) {
		if !runSetup(root, setupAddr, open) {
			os.Exit(1)
		}
	}
	cfg, err := install.Load(root)
	if err != nil {
		log.Fatal(err)
	}
	if err = ensureWeb(root); err != nil {
		log.Fatal(err)
	}
	if err = build(root); err != nil {
		log.Fatal(err)
	}
	// An update may bring database changes; say so instead of starting a service that would refuse to run.
	checkCtx, cancelCheck := context.WithTimeout(context.Background(), 20*time.Second)
	if store, e := postgres.Open(checkCtx, cfg.Database.URL); e != nil {
		cancelCheck()
		log.Fatal("Could not reach the database named in .env. Is PostgreSQL running? (" + e.Error() + ")")
	} else {
		if e = store.ValidateSchema(checkCtx); e != nil {
			store.Close()
			cancelCheck()
			log.Fatal("The database needs updating for this version. Run: dapper migrate   (back up the database first)")
		}
		store.Close()
	}
	cancelCheck()

	web := filepath.Join(root, "dapper-platform", "apps", "web")
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
					if open {
						openBrowser(cfg.Site.URL + "/dashboard")
					}
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
