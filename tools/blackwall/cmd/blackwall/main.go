// Command blackwall is a k9s-style TUI for the Blackwall security project.
//
// See docs/superpowers/specs/2026-05-17-blackwall-tui-design.md.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/app"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/config"
	bwlog "github.com/greengrunt/blackwall/tools/blackwall/internal/log"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/oneshot"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/tunnel"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/statusbar"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/views"
	agentsview "github.com/greengrunt/blackwall/tools/blackwall/internal/views/agents"
	alertsview "github.com/greengrunt/blackwall/tools/blackwall/internal/views/alerts"
	ctxview "github.com/greengrunt/blackwall/tools/blackwall/internal/views/contexts"
	helpview "github.com/greengrunt/blackwall/tools/blackwall/internal/views/help"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

// version is the program version string.  Override at build time with:
//
//	go build -ldflags "-X main.version=v1.2.3" ./cmd/blackwall
var version = "v0.0.1"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "blackwall:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		flagContext              string
		flagConfig               string
		flagRefresh              string
		flagOnce                 bool
		flagSelectContextOnStart bool
		flagNoSelectContext      bool
	)
	flag.StringVar(&flagContext, "context", "", "override current context")
	flag.StringVar(&flagConfig, "config", "", "config file path")
	flag.StringVar(&flagRefresh, "refresh", "", "refresh interval (e.g. 3s)")
	flag.BoolVar(&flagOnce, "once", false, "print agents table and exit (no TUI)")
	flag.BoolVar(&flagSelectContextOnStart, "select-context-on-start", false, "force enable startup picker")
	flag.BoolVar(&flagNoSelectContext, "no-select-context", false, "force disable startup picker")
	flag.Parse()

	cfgPath := flagConfig
	if cfgPath == "" {
		if v := os.Getenv("BW_CONFIG"); v != "" {
			cfgPath = v
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("home dir: %w", err)
			}
			cfgPath = filepath.Join(home, ".config", "blackwall", "config.yaml")
		}
	}

	created, err := config.EnsureExists(cfgPath)
	if err != nil {
		return err
	}
	if created {
		fmt.Fprintf(os.Stderr,
			"Created config at %s\nEdit it to point at your Wazuh manager, then run `blackwall` again.\n",
			cfgPath,
		)
		return nil
	}

	raw, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	cfg := config.ApplyDefaults(raw)
	cfg, err = config.ExpandEnv(cfg)
	if err != nil {
		return err
	}

	ov := config.Overrides{Context: flagContext, ContextWasSet: flagContext != ""}
	if flagRefresh != "" {
		d, err := time.ParseDuration(flagRefresh)
		if err != nil {
			return fmt.Errorf("--refresh: %w", err)
		}
		ov.Refresh = &d
	}
	if flagSelectContextOnStart && flagNoSelectContext {
		return errors.New("--select-context-on-start and --no-select-context are mutually exclusive")
	}
	switch {
	case flagSelectContextOnStart:
		t := true
		ov.SelectContextOnStart = &t
	case flagNoSelectContext:
		f := false
		ov.SelectContextOnStart = &f
	}

	cfg, err = config.Resolve(cfg, ov, config.ReadEnv())
	if err != nil {
		return err
	}
	if err := config.Validate(cfg); err != nil {
		return err
	}

	logger, closer, err := bwlog.Setup(bwlog.Options{File: cfg.Logging.File, Level: cfg.Logging.Level})
	if err != nil {
		return err
	}
	defer closer.Close()
	logger.Info("blackwall startup", "context", cfg.Current)

	currentCtx := cfg.Contexts[cfg.Current]
	if currentCtx.Wazuh == nil {
		return fmt.Errorf("context %q has no wazuh: subsystem", cfg.Current)
	}

	// Ctrl+C / SIGTERM / SIGHUP before the TUI owns the terminal (tunnel start,
	// --once) cancel sigCtx, so the deferred Close still kills ssh.
	sigCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stopSignals()
	live := newLiveTunnel()
	defer live.Close() // also runs after a panic in this goroutine

	// dialCtx is currentCtx, rewritten to 127.0.0.1:<local port> when the
	// context goes through ssh.
	if currentCtx.SSH != nil && !flagOnce {
		fmt.Fprintf(os.Stderr, "blackwall: opening ssh tunnel to %s ...\n", currentCtx.SSH.Host)
	}
	live.begin()
	dialCtx, tn, err := tunnel.ForContext(sigCtx, currentCtx)
	live.swap(tn)
	live.end()
	if sigCtx.Err() != nil { // Ctrl+C / SIGTERM while the tunnel was opening
		return fmt.Errorf("interrupted: %w", context.Cause(sigCtx))
	}
	if err != nil {
		if flagOnce || tn == nil {
			return err
		}
		// The TUI starts anyway: the status bar shows the error and
		// ctrl+r retries (or pick another context).
		logger.Warn("ssh tunnel", "context", cfg.Current, "err", err)
	}
	client, err := wazuh.New(*dialCtx.Wazuh)
	if err != nil {
		return err
	}

	if flagOnce {
		return oneshot.Print(sigCtx, os.Stdout, client)
	}

	ctxView := ctxview.New(cfg)
	agentsList := agentsview.NewView(client, cfg.UI.RefreshInterval)
	idx, _, err := alertsview.SearcherFor(dialCtx.Indexer)
	if err != nil {
		return err
	}
	alertsSettings := func(name string, sub *config.IndexerSubsystem) alertsview.Settings {
		st := alertsview.Settings{Context: name, MinLevel: config.DefaultMinLevel,
			ExcludeGroups: config.DefaultExcludeGroups(), Refresh: cfg.UI.RefreshInterval}
		if sub != nil {
			st.MinLevel = *sub.Alerts.MinLevel
			st.ExcludeGroups = sub.Alerts.ExcludeGroups
		}
		return st
	}
	alertsView := alertsview.New(idx, alertsSettings(cfg.Current, currentCtx.Indexer))
	if idx != nil {
		agentsList.SetIndexer(idx)
	}
	registered := []views.View{ctxView, agentsList, alertsView}
	liveIndexer := indexerAddr(currentCtx)

	initial := ":agents"
	if cfg.SelectContextOnStart {
		initial = ":contexts"
	}

	// liveCtx / liveWazuh track the active context for the status bar
	// and are updated on every context switch.
	liveCtx := cfg.Current
	liveWazuh := wazuhAddr(currentCtx)

	// switchContext prepares a switch off the UI loop (tunnel, clients,
	// config file) and returns the state changes as Apply, which the app runs
	// on the UI loop. The app allows one switch at a time.
	switchContext := func(name string) tea.Cmd {
		return func() (msg tea.Msg) {
			ctx := cfg.Contexts[name]
			if ctx.Wazuh == nil {
				return app.ContextSwitchedMsg{Name: name,
					Err: fmt.Errorf("context %q has no wazuh subsystem", name)}
			}
			if !live.begin() {
				return app.ContextSwitchedMsg{Name: name, Err: errShuttingDown}
			}
			defer live.end()
			var newTn *tunnel.Tunnel
			fail := func(err error) tea.Msg {
				live.discard(newTn)
				return app.ContextSwitchedMsg{Name: name, Err: err}
			}
			defer func() {
				if r := recover(); r != nil {
					logger.Error("context switch panic", "context", name, "panic", r)
					msg = fail(fmt.Errorf("internal error: %v", r))
				}
			}()
			// The new tunnel comes up before the old one goes, so a failed
			// switch leaves the current context working.
			dialCtx, tn, err := tunnel.ForContext(live.ctx, ctx)
			newTn = tn
			live.stage(newTn)
			if err != nil {
				return fail(err)
			}
			newClient, err := wazuh.New(*dialCtx.Wazuh)
			if err != nil {
				return fail(err)
			}
			newIdx, _, err := alertsview.SearcherFor(dialCtx.Indexer)
			if err != nil {
				return fail(err)
			}
			if err := config.SetCurrent(cfgPath, name); err != nil {
				return fail(err)
			}
			return app.ContextSwitchedMsg{Name: name, Apply: func() {
				live.swap(newTn)
				alertsView.Replace(newIdx, alertsSettings(name, ctx.Indexer))
				agentsList.SetIndexer(newIdx)
				agentsList.ReplaceClient(newClient)
				ctxView.UpdateCurrent(name)
				cfg.Current = name
				liveCtx = name
				liveWazuh = wazuhAddr(ctx)
				liveIndexer = indexerAddr(ctx)
			}}
		}
	}

	root := app.New(app.Deps{
		Views:         registered,
		InitialView:   initial,
		Version:       version,
		HelpView:      helpview.New(),
		SwitchContext: switchContext,
		OnRefresh:     live.Retry,
		Status: func() statusbar.State {
			tl, bad := live.label()
			return statusbar.State{
				Context:   liveCtx,
				Wazuh:     liveWazuh,
				Tunnel:    tl,
				TunnelBad: bad,
				Indexer:   liveIndexer,
				Refresh:   cfg.UI.RefreshInterval,
			}
		},
	})

	prog := tea.NewProgram(root, tea.WithAltScreen())
	// sigCtx stays registered for the whole TUI (bubbletea's own SIGINT/
	// SIGTERM handler is installed only inside Run, and it ignores SIGHUP), so
	// there is no window where a signal kills the process without running the
	// deferred tunnel Close. A signal stops the program; stopSignals at return
	// only releases this goroutine.
	go func() {
		<-sigCtx.Done()
		prog.Kill()
	}()
	if _, err := prog.Run(); err != nil {
		if errors.Is(err, tea.ErrProgramKilled) && sigCtx.Err() != nil {
			return fmt.Errorf("terminated by signal")
		}
		return err
	}
	return nil
}
