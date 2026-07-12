package scope

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nasij/nasij/internal/container"
	"github.com/nasij/nasij/internal/scope"
	"github.com/nasij/nasij/internal/storage"
	"github.com/nasij/nasij/internal/ui"
)

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

func NewScopeCmd(c *container.Container) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scope",
		Short: "Manage workspace scope definitions",
		Long: `Define the scope of a reconnaissance workspace: allowed domains,
subdomains, CIDR ranges, exclude rules, regex patterns, rate limits, and auth.

Scope entries are stored per workspace in the SQLite database.
If no include rules are defined, the workspace target domain is used as the default scope.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.SetHelpFunc(customHelpFunc(c))

	cmd.AddCommand(newSetCmd(c))
	cmd.AddCommand(newViewCmd(c))
	cmd.AddCommand(newCheckCmd(c))
	cmd.AddCommand(newClearCmd(c))
	cmd.AddCommand(newRobotsCmd(c))

	return cmd
}

func customHelpFunc(c *container.Container) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, args []string) {
		w := c.UI.Writer()

		fmt.Fprintln(w)
		fmt.Fprintln(w, "  "+ui.StyleHeader.Render("USAGE"))
		fmt.Fprintln(w, "    "+ui.StyleWhite.Render(cmd.UseLine()))
		fmt.Fprintln(w)

		if cmd.HasAvailableSubCommands() {
			fmt.Fprintln(w, "  "+ui.StyleHeader.Render("COMMANDS"))
			cmds := []struct{ name, desc string }{
				{"set", "Add a scope rule (domain, subdomain, cidr, exclude, regex)"},
				{"view", "Show the full scope definition"},
				{"check", "Check if a URL is within scope"},
				{"clear", "Remove scope rules"},
				{"robots", "Fetch and parse robots.txt for the target"},
			}
			for _, c := range cmds {
				n := ui.StyleAccent.Render(padRight(c.name, 12))
				d := ui.StyleMuted.Render(c.desc)
				fmt.Fprintln(w, "    "+n+d)
			}
			fmt.Fprintln(w)
		}

		if cmd.HasAvailableLocalFlags() {
			fmt.Fprintln(w, "  "+ui.StyleHeader.Render("FLAGS"))
			ui.PrintFlagUsages(w, cmd.LocalFlags(), "    ")
			fmt.Fprintln(w)
		}

		extra := ui.StyleMuted.Render(`Run "nasij scope --help" for more info.`)
		fmt.Fprintln(w, "  "+extra)
		fmt.Fprintln(w)
	}
}

// --- set command ---

var scopeTypes = []string{"domain", "subdomain", "cidr", "exclude", "regex"}

func newSetCmd(c *container.Container) *cobra.Command {
	var (
		workspaceID string
		entryType   string
		pattern     string
		rateLimit   float64
		burst       int
		authType    string
		authValue   string
	)

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Add a scope rule or configure scope settings",
		Long: `Add an inclusion or exclusion rule to the workspace scope.

Rule types:
  domain     - e.g. "example.com" (matches domain + all subdomains)
  subdomain  - e.g. "admin.example.com" (exact subdomain match)
  cidr       - e.g. "10.0.0.0/8"
  exclude    - e.g. "admin.example.com" or "/logout"
  regex      - e.g. ".*\.internal\.example\.com"

Use --rate-limit and --burst to configure per-workspace rate limiting.
Use --auth-type and --auth-value to set authentication headers.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSet(c, workspaceID, entryType, pattern, rateLimit, burst, authType, authValue)
		},
	}

	cmd.Flags().StringVarP(&workspaceID, "workspace", "w", "", "Workspace ID (required)")
	cmd.Flags().StringVarP(&entryType, "type", "t", "", "Entry type: "+strings.Join(scopeTypes, ", "))
	cmd.Flags().StringVarP(&pattern, "pattern", "p", "", "Scope pattern (domain, CIDR, regex, exclude)")
	cmd.Flags().Float64Var(&rateLimit, "rate-limit", 0, "Requests per second rate limit")
	cmd.Flags().IntVar(&burst, "burst", 0, "Rate limit burst size")
	cmd.Flags().StringVar(&authType, "auth-type", "", "Auth type: header, cookie, basic, bearer")
	cmd.Flags().StringVar(&authValue, "auth-value", "", "Auth token, header value, or credentials")
	_ = cmd.MarkFlagRequired("workspace")

	return cmd
}

func runSet(c *container.Container, wsID, entryType, pattern string, rateLimit float64, burst int, authType, authValue string) error {
	ui.PrintBanner()
	term := c.UI

	ctx := context.Background()
	sm, _, db, err := getWorkspace(c, wsID)
	if err != nil {
		term.Error(err.Error())
		return err
	}
	defer db.Close()

	if entryType != "" && pattern != "" {
		if err := sm.AddRule(ctx, wsID, entryType, pattern); err != nil {
			term.Error("Failed to add rule: " + err.Error())
			return err
		}
		term.Success(fmt.Sprintf("Added %s rule: %s", entryType, pattern))
	}

	if rateLimit > 0 {
		if burst == 0 {
			burst = 5
		}
		if err := sm.SetRateLimit(ctx, wsID, rateLimit, burst); err != nil {
			term.Error("Failed to set rate limit: " + err.Error())
			return err
		}
		term.Success(fmt.Sprintf("Rate limit set to %.1f req/s (burst %d)", rateLimit, burst))
	}

	if authType != "" {
		if err := sm.SetAuth(ctx, wsID, authType, authValue); err != nil {
			term.Error("Failed to set auth: " + err.Error())
			return err
		}
		term.Success(fmt.Sprintf("Auth set: %s", authType))
	}

	if entryType == "" && rateLimit == 0 && authType == "" {
		term.Warning("No scope changes specified. Use --type/--pattern, --rate-limit, or --auth-type.")
	}

	term.Blank()
	return nil
}

// --- view command ---

func newViewCmd(c *container.Container) *cobra.Command {
	var workspaceID string

	cmd := &cobra.Command{
		Use:   "view",
		Short: "Show the full scope definition for a workspace",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runView(c, workspaceID)
		},
	}

	cmd.Flags().StringVarP(&workspaceID, "workspace", "w", "", "Workspace ID (required)")
	_ = cmd.MarkFlagRequired("workspace")

	return cmd
}

func runView(c *container.Container, wsID string) error {
	ui.PrintBanner()
	term := c.UI

	ctx := context.Background()
	sm, s, db, err := getWorkspace(c, wsID)
	if err != nil {
		term.Error(err.Error())
		return err
	}
	defer db.Close()

	term.Header("Scope Definition")
	term.Divider()
	term.Blank()

	if s.TargetHost != "" {
		term.KeyValue("Target", s.TargetHost)
	}
	term.KeyValue("Rate Limit", fmt.Sprintf("%.1f req/s (burst %d)", s.RateLimit.RequestsPerSec, s.RateLimit.Burst))
	if s.Auth.Type != "" {
		term.KeyValue("Auth Type", s.Auth.Type)
	}
	if s.RespectRobots {
		term.KeyValue("Robots.txt", "respected")
	} else {
		term.KeyValue("Robots.txt", "ignored")
	}
	term.Blank()

	entries, err := sm.ListEntries(ctx, wsID)
	if err != nil {
		term.Error("Failed to list entries: " + err.Error())
		return err
	}

	if len(entries) == 0 {
		term.Info("No scope rules defined. Using target domain as default scope.")
		term.Blank()
		return nil
	}

	byType := map[string][]scope.ScopeEntry{}
	for _, e := range entries {
		byType[e.EntryType] = append(byType[e.EntryType], e)
	}

	for _, t := range scopeTypes {
		entries := byType[t]
		if len(entries) == 0 {
			continue
		}
		term.Subheader(fmt.Sprintf("  %s (%d)", capitalize(t), len(entries)))
		for _, e := range entries {
			term.Table([][2]string{
				{e.ID[:8], ui.StyleAccent.Render(e.Pattern)},
			})
		}
		term.Blank()
	}

	return nil
}

// --- check command ---

func newCheckCmd(c *container.Container) *cobra.Command {
	var workspaceID string

	cmd := &cobra.Command{
		Use:   "check <url>",
		Short: "Check if a URL is within scope",
		Long: `Evaluate whether a URL is within the workspace scope.
Returns in-scope or out-of-scope with the matching rule.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCheck(c, workspaceID, args[0])
		},
	}

	cmd.Flags().StringVarP(&workspaceID, "workspace", "w", "", "Workspace ID (required)")
	_ = cmd.MarkFlagRequired("workspace")

	return cmd
}

func runCheck(c *container.Container, wsID, rawURL string) error {
	ui.PrintBanner()
	term := c.UI

	_, s, db, err := getWorkspace(c, wsID)
	if err != nil {
		term.Error(err.Error())
		return err
	}
	defer db.Close()

	term.Header("Scope Check")
	term.Divider()
	term.Blank()

	term.KeyValue("URL", rawURL)
	term.Blank()

	if s.IsInScope(rawURL) {
		term.Success(fmt.Sprintf("In scope: %s", rawURL))
	} else {
		term.Warning(fmt.Sprintf("Out of scope: %s", rawURL))
	}
	term.Blank()

	return nil
}

// --- clear command ---

func newClearCmd(c *container.Container) *cobra.Command {
	var (
		workspaceID string
		entryType   string
		force       bool
	)

	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Remove scope rules from a workspace",
		Long: `Remove all scope rules or rules of a specific type.
Use --type to filter: domain, subdomain, cidr, exclude, regex.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runClear(c, workspaceID, entryType, force)
		},
	}

	cmd.Flags().StringVarP(&workspaceID, "workspace", "w", "", "Workspace ID (required)")
	cmd.Flags().StringVarP(&entryType, "type", "t", "", "Rule type to clear (omit for all)")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")
	_ = cmd.MarkFlagRequired("workspace")

	return cmd
}

func runClear(c *container.Container, wsID, entryType string, force bool) error {
	term := c.UI

	if !force {
		label := "all scope rules"
		if entryType != "" {
			label = entryType + " rules"
		}
		fmt.Fprintf(term.Writer(), "  "+ui.StyleWarning.Render("This will remove %s for workspace %s. Continue? [y/N]: "), label, wsID[:8])
		var response string
		_, _ = fmt.Scanln(&response)
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			term.Info("Cancelled.")
			term.Blank()
			return nil
		}
	}

	sm, _, db, err := getWorkspace(c, wsID)
	if err != nil {
		term.Error(err.Error())
		return err
	}
	defer db.Close()

	if err := sm.ClearRules(context.Background(), wsID, entryType); err != nil {
		term.Error("Failed to clear rules: " + err.Error())
		return err
	}

	label := "All scope rules"
	if entryType != "" {
		label = entryType + " rules"
	}
	term.Success(fmt.Sprintf("%s cleared for workspace %s.", label, wsID[:8]))
	term.Blank()
	return nil
}

// --- robots command ---

func newRobotsCmd(c *container.Container) *cobra.Command {
	var (
		workspaceID  string
		fetch        bool
		saveDisallow bool
	)

	cmd := &cobra.Command{
		Use:   "robots",
		Short: "Parse robots.txt and optionally add disallowed paths as excludes",
		Long: `Load robots.txt for the workspace target, display parsed rules,
and optionally add disallowed paths as exclude rules.

With --save-disallow, each Disallow path is added as an exclude rule
so the scanner will skip those paths.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRobots(c, workspaceID, fetch, saveDisallow)
		},
	}

	cmd.Flags().StringVarP(&workspaceID, "workspace", "w", "", "Workspace ID (required)")
	cmd.Flags().BoolVar(&fetch, "fetch", true, "Fetch robots.txt from the target")
	cmd.Flags().BoolVar(&saveDisallow, "save-disallow", false, "Add Disallow paths as exclude rules")
	_ = cmd.MarkFlagRequired("workspace")

	return cmd
}

func runRobots(c *container.Container, wsID string, fetch bool, saveDisallow bool) error {
	ui.PrintBanner()
	term := c.UI

	ws, err := c.Workspace.Open(wsID)
	if err != nil {
		term.Error("Workspace not found: " + err.Error())
		return err
	}

	ctx := context.Background()
	sm, _, db, err := getWorkspace(c, wsID)
	if err != nil {
		term.Error(err.Error())
		return err
	}
	defer db.Close()

	term.Header("Robots.txt")
	term.Divider()
	term.Blank()

	var rt *scope.RobotsTxt

	if fetch && ws.Target != "" {
		term.Info("Fetching " + scope.ExtractHost(ws.Target) + "/robots.txt ...")
		term.Blank()

		client := &http.Client{Timeout: 10 * time.Second}
		rt, _, err = scope.FetchRobotsTxt(ctx, ws.Target, client)
		if err != nil {
			term.Warning("Could not fetch robots.txt: " + err.Error())
			term.Blank()
			term.Info("You can manually add exclude rules with `nasij scope set --type exclude --pattern /path`")
			term.Blank()
			return nil
		}

		if saveDisallow {
			for _, path := range rt.DisallowedPaths {
				if err := sm.AddRule(ctx, wsID, "exclude", path); err != nil {
					term.Warning("Failed to add exclude rule: " + err.Error())
				}
			}
		}
	}

	if rt == nil {
		term.Warning("No robots.txt data available.")
		term.Blank()
		return nil
	}

	term.KeyValue("Disallowed paths", fmt.Sprintf("%d", len(rt.DisallowedPaths)))
	term.KeyValue("Allowed paths", fmt.Sprintf("%d", len(rt.AllowedPaths)))
	term.KeyValue("Crawl delay", rt.CrawlDelay.String())
	term.KeyValue("Sitemaps", fmt.Sprintf("%d", len(rt.Sitemaps)))

	if len(rt.DisallowedPaths) > 0 {
		term.Blank()
		term.Subheader("  Disallowed")
		for _, p := range rt.DisallowedPaths {
			term.Table([][2]string{{"", ui.StyleMuted.Render(p)}})
		}
	}

	if saveDisallow && len(rt.DisallowedPaths) > 0 {
		term.Blank()
		term.Success(fmt.Sprintf("Added %d disallowed paths as exclude rules.", len(rt.DisallowedPaths)))
	}

	term.Blank()
	return nil
}

// --- helpers ---

func getWorkspace(c *container.Container, id string) (*scope.Manager, *scope.Scope, *storage.DB, error) {
	ws, err := c.Workspace.Open(id)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("workspace: %w", err)
	}

	ctx := context.Background()
	db, err := storage.Open(ctx, ws.DBPath())
	if err != nil {
		return nil, nil, nil, fmt.Errorf("database: %w", err)
	}

	sm := scope.NewManager(db.DB())
	s, err := sm.Load(ctx, ws.ID, scope.ExtractHost(ws.Target))
	if err != nil {
		db.Close()
		return nil, nil, nil, fmt.Errorf("scope load: %w", err)
	}

	return sm, s, db, nil
}

func capitalize(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
