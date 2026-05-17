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
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/app"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/config"
	bwlog "github.com/greengrunt/blackwall/tools/blackwall/internal/log"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/oneshot"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/statusbar"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/views"
	agentsview "github.com/greengrunt/blackwall/tools/blackwall/internal/views/agents"
	ctxview "github.com/greengrunt/blackwall/tools/blackwall/internal/views/contexts"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

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
	client, err := wazuh.New(*currentCtx.Wazuh)
	if err != nil {
		return err
	}

	if flagOnce {
		return oneshot.Print(context.Background(), os.Stdout, client)
	}

	ctxView := ctxview.New(cfg)
	agentsList := agentsview.NewView(client, cfg.UI.RefreshInterval)
	registered := []views.View{ctxView, agentsList}

	initial := ":agents"
	if cfg.SelectContextOnStart {
		initial = ":contexts"
	}

	// liveCtx / liveWazuh track the active context for the status bar
	// and are updated on every context switch.
	liveCtx := cfg.Current
	liveWazuh := fmt.Sprintf("%s:%d", currentCtx.Wazuh.Server, currentCtx.Wazuh.Port)

	switchContext := func(name string) tea.Cmd {
		return func() tea.Msg {
			ctx := cfg.Contexts[name]
			if ctx.Wazuh == nil {
				return app.ContextSwitchedMsg{Name: name,
					Err: fmt.Errorf("context %q has no wazuh subsystem", name)}
			}
			newClient, err := wazuh.New(*ctx.Wazuh)
			if err != nil {
				return app.ContextSwitchedMsg{Name: name, Err: err}
			}
			if err := config.SetCurrent(cfgPath, name); err != nil {
				return app.ContextSwitchedMsg{Name: name, Err: err}
			}
			cfg.Current = name
			liveCtx = name
			liveWazuh = fmt.Sprintf("%s:%d", ctx.Wazuh.Server, ctx.Wazuh.Port)
			agentsList.ReplaceClient(newClient)
			ctxView.UpdateCurrent(name)
			return app.ContextSwitchedMsg{Name: name}
		}
	}

	root := app.New(app.Deps{
		Views:         registered,
		InitialView:   initial,
		SwitchContext: switchContext,
		Status: func() statusbar.State {
			return statusbar.State{
				Context: liveCtx,
				Wazuh:   liveWazuh,
				Refresh: cfg.UI.RefreshInterval,
			}
		},
	})

	prog := tea.NewProgram(root, tea.WithAltScreen())
	if _, err := prog.Run(); err != nil {
		return err
	}
	return nil
}
