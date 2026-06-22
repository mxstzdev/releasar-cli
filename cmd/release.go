package cmd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/mxstzdev/releasar-cli/internal/config"
	"github.com/mxstzdev/releasar-cli/internal/git"
	"github.com/mxstzdev/releasar-cli/internal/log"
	"github.com/mxstzdev/releasar-cli/internal/notify"
	"github.com/mxstzdev/releasar-cli/internal/scm"
	"github.com/mxstzdev/releasar-cli/internal/tasks"
	"github.com/mxstzdev/releasar-cli/internal/tracker"
	"github.com/mxstzdev/releasar-cli/internal/ui"
	"github.com/mxstzdev/releasar-cli/internal/versioning"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

var (
	flagVersion       string
	flagDryRun        bool
	flagNoInteraction bool
	flagConfigFile    string
)

func init() {
	releaseCmd.Flags().StringVar(&flagVersion, "version", "", "release this exact version, skipping detection")
	releaseCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "walk through the release flow without writing anything")
	releaseCmd.Flags().BoolVarP(&flagNoInteraction, "no-interaction", "n", false, "skip interactive confirmations")
	releaseCmd.Flags().StringVarP(&flagConfigFile, "configuration", "c", "", "explicit path to config file, bypasses auto-detection")
	rootCmd.AddCommand(releaseCmd)
}

var releaseCmd = &cobra.Command{
	Use:       "release [major|minor|patch]",
	Short:     "Run the full project deployment workflow",
	GroupID:   "primary",
	ValidArgs: []string{"major", "minor", "patch"},
	Args:      cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagQuiet && flagVerbosity > 0 {
			return fmt.Errorf("--quiet and --verbose are mutually exclusive")
		}
		var bumpArg string
		if len(args) > 0 {
			bumpArg = args[0]
		}
		r := &releaseRunner{
			dryRun:         flagDryRun,
			noInteraction:  flagNoInteraction,
			verbose:        flagVerbosity >= 1,
			debug:          flagVerbosity >= 2,
			versionFlag:    flagVersion,
			bumpArg:        bumpArg,
			quiet:          flagQuiet,
			configFile:     flagConfigFile,
			workingDirFlag: flagWorkingDir,
		}
		return r.run()
	},
}

type releaseRunner struct {
	cfg           *config.Config
	git           *git.Client
	log           *log.Channel
	dryRun        bool
	noInteraction bool
	verbose       bool
	debug         bool
	quiet         bool

	versionFlag    string
	bumpArg        string
	configFile     string
	workingDirFlag string

	workingDir     string
	repoRoot       string
	originalBranch string
	mainHeadSHA    string
	devHeadSHA     string

	scmProvider      scm.Provider    // nil if no SCM provider is configured
	trackerProvider  tracker.Tracker // nil if no tracker is configured
	trackerVersionID string          // ID of the created or selected tracker version
	notifier         notify.Notifier // nil if no channels are configured
	scmReleaseURL    string          // URL returned by scmCreateRelease; forwarded to notifications

	singleBranch         bool
	releaseBranchCreated bool
	tagCreated           bool
	completed            bool

	scheme         versioning.Scheme
	currentVersion versioning.Version
	nextVersion    versioning.Version
	parsedCommits  []versioning.ParsedCommit
	releaseBranch  string
	tag            string
	changedFiles   []string
}

func (r *releaseRunner) vprintf(format string, args ...any) {
	if r.verbose {
		fmt.Printf("  "+format, args...)
	}
}

func (r *releaseRunner) print(title string, desc ...string) {
	if !r.quiet {
		ui.Print(title, desc...)
	}
}

func (r *releaseRunner) run() error {
	if err := r.initialize(); err != nil {
		return err
	}
	if err := r.detectVersion(); err != nil {
		return err
	}

	// Register rollback — fires on any error after this point, until r.completed is set.
	defer r.rollback()

	if r.versionFlag != "" && r.bumpArg != "" {
		return fmt.Errorf("cannot combine --version with a positional bump argument")
	}

	if r.versionFlag != "" {
		parsed, err := r.scheme.Parse(r.versionFlag)
		if err != nil {
			return fmt.Errorf("invalid version %q: %w", r.versionFlag, err)
		}
		r.nextVersion = parsed
		r.log.Info("version overridden via flag", map[string]any{"version": r.nextVersion.String()})
	} else if r.bumpArg != "" {
		bump := bumpKindFromArg(r.bumpArg)
		next, err := r.currentVersion.Increment(bump)
		if err != nil {
			return fmt.Errorf("computing next version: %w", err)
		}
		r.nextVersion = next
		r.log.Info("version set via bump argument", map[string]any{"bump": r.bumpArg, "version": r.nextVersion.String()})
		if !r.quiet {
			fmt.Printf("Current version : %s\nRequested bump  : %s\nNext version    : %s\n",
				r.currentVersion.String(), r.bumpArg, r.nextVersion.String())
		}
		if err := r.confirmVersion(); err != nil {
			return err
		}
	} else {
		if err := r.recommendVersion(); err != nil {
			return err
		}
		if err := r.confirmVersion(); err != nil {
			return err
		}
	}

	r.tag = r.cfg.Git.TagPrefix + r.nextVersion.String()

	if err := r.runSecretsScan(); err != nil {
		return err
	}

	if err := r.createReleaseBranch(); err != nil {
		return err
	}
	if !r.singleBranch {
		if err := r.mergeMainIntoRelease(); err != nil {
			return err
		}
	}
	if err := r.trackerCreateVersion(); err != nil {
		return err
	}
	if err := r.trackerAssignTickets(); err != nil {
		return err
	}
	if err := r.updateChangelog(); err != nil {
		return err
	}
	if err := r.applySubstitutions(); err != nil {
		return err
	}
	if err := r.runBuildTasks(); err != nil {
		return err
	}
	if err := r.runTests(); err != nil {
		return err
	}
	if err := r.reviewGate(); err != nil {
		return err
	}
	if err := r.commitRelease(); err != nil {
		return err
	}
	if err := r.tagRelease(); err != nil {
		return err
	}
	if err := r.mergeIntoMain(); err != nil {
		return err
	}
	if !r.singleBranch {
		if err := r.mergeIntoDev(); err != nil {
			return err
		}
	}
	if err := r.cleanupReleaseBranch(); err != nil {
		return err
	}
	if err := r.push(); err != nil {
		return err
	}
	if err := r.scmCreateRelease(); err != nil {
		return err
	}
	if err := r.trackerCloseVersion(); err != nil {
		return err
	}
	r.sendNotifications()
	r.printSummary()
	return nil
}

func bumpKindFromArg(arg string) versioning.BumpKind {
	switch arg {
	case "major":
		return versioning.BumpMajor
	case "minor":
		return versioning.BumpMinor
	default:
		return versioning.BumpPatch
	}
}

func (r *releaseRunner) initialize() error {
	r.print("Initializing")

	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git is not installed or not in PATH")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	if r.workingDirFlag != "" {
		abs, err := filepath.Abs(r.workingDirFlag)
		if err != nil {
			return fmt.Errorf("resolving working directory: %w", err)
		}
		r.workingDir = abs
	} else {
		r.workingDir = cwd
	}

	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return fmt.Errorf("not inside a git repository")
	}
	r.repoRoot = strings.TrimSpace(string(out))

	if r.workingDir != r.repoRoot && r.workingDirFlag == "" {
		if !r.quiet {
			fmt.Printf("Working directory: %s\nRepo root:        %s\n", r.workingDir, r.repoRoot)
		}
		ok, err := ui.Confirm("Confirm working directory", fmt.Sprintf("%s is the package root for this release", r.workingDir))
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("release cancelled — adjust your working directory and retry")
		}
	}

	if err := config.LoadDotEnv(r.repoRoot, r.workingDir); err != nil {
		return fmt.Errorf("loading .env: %w", err)
	}

	var cfg *config.Config
	if r.configFile != "" {
		cfg, err = config.LoadFile(r.configFile, r.workingDir)
	} else {
		cfg, err = config.LoadLayered(r.repoRoot, r.workingDir)
	}
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	r.cfg = cfg

	logCfg := log.Config{
		Directory: cfg.Log.Directory,
		Filename:  cfg.Log.Filename,
	}
	if r.debug {
		lvl := zerolog.DebugLevel
		logCfg.Level = &lvl
		r.vprintf("Debug log: %s\n", filepath.Join(cfg.Log.Directory, cfg.Log.Filename))
	}
	log.Init(logCfg)
	r.log = log.Get("release")

	r.singleBranch = cfg.Git.DefaultBranch == cfg.Git.DevelopmentBranch

	gitCfg := git.Config{
		DefaultBranch:     cfg.Git.DefaultBranch,
		DevelopmentBranch: cfg.Git.DevelopmentBranch,
		TagPrefix:         cfg.Git.TagPrefix,
	}
	r.git, err = git.New(r.repoRoot, gitCfg, log.Get("git"))
	if err != nil {
		return fmt.Errorf("initialising git client: %w", err)
	}

	branchStrategy := "git-flow"
	if r.singleBranch {
		branchStrategy = "single-branch"
	}
	r.vprintf("Branch strategy: %s\n", branchStrategy)
	r.vprintf("Repository: %s\n", r.repoRoot)

	var owner, repo string
	if cfg.SCM.Provider != "" || cfg.Tracker.Provider != "" {
		remoteURL, err := r.git.RemoteURL(cfg.Git.Remote)
		if err != nil {
			return fmt.Errorf("resolving remote URL: %w", err)
		}
		var parseErr error
		owner, repo, parseErr = scm.ParseRemoteURL(remoteURL)
		if parseErr != nil {
			return fmt.Errorf("parsing remote URL: %w", parseErr)
		}
	}

	if cfg.SCM.Provider != "" {
		var err error
		r.scmProvider, err = scm.New(cfg.SCM.Provider, scm.Config{
			Host:  cfg.SCM.Host,
			Token: os.Getenv(cfg.SCM.TokenEnv),
			Owner: owner,
			Repo:  repo,
		}, log.Get("scm"))
		if err != nil {
			return fmt.Errorf("initialising SCM provider: %w", err)
		}
		r.log.Debug("SCM provider initialised", map[string]any{"provider": cfg.SCM.Provider})
	}

	if cfg.Tracker.Provider != "" {
		var err error
		r.trackerProvider, err = tracker.New(cfg.Tracker.Provider, tracker.Config{
			Host:       cfg.Tracker.Host,
			Token:      os.Getenv(cfg.Tracker.TokenEnv),
			Email:      os.Getenv(cfg.Tracker.EmailEnv),
			ProjectKey: cfg.Tracker.ProjectKey,
			Owner:      owner,
			Repo:       repo,
		}, log.Get("tracker"))
		if err != nil {
			return fmt.Errorf("initialising tracker: %w", err)
		}
		r.log.Debug("tracker initialised", map[string]any{"provider": cfg.Tracker.Provider})
	}

	n := cfg.Notify
	if n.Email != nil || n.Telegram != nil || n.Desktop != nil || n.Slack != nil || n.Webhook != nil {
		notifyCfg := notify.Config{Desktop: n.Desktop != nil}
		if n.Email != nil {
			notifyCfg.Email = &notify.EmailConfig{
				SMTPHost: n.Email.SMTPHost,
				SMTPPort: n.Email.SMTPPort,
				SMTPUser: os.Getenv(n.Email.SMTPUserEnv),
				SMTPPass: os.Getenv(n.Email.SMTPPassEnv),
				From:     n.Email.From,
				To:       n.Email.To,
				Subject:  n.Email.Subject,
			}
		}
		if n.Telegram != nil {
			notifyCfg.Telegram = &notify.TelegramConfig{
				Token:  os.Getenv(n.Telegram.TokenEnv),
				ChatID: n.Telegram.ChatID,
			}
		}
		if n.Slack != nil {
			notifyCfg.Slack = &notify.SlackConfig{
				WebhookURL: os.Getenv(n.Slack.WebhookEnv),
			}
		}
		if n.Webhook != nil {
			headers := make(map[string]string)
			for k, v := range n.Webhook.Headers {
				headers[k] = v
			}
			for k, envName := range n.Webhook.HeadersEnv {
				if v := os.Getenv(envName); v != "" {
					headers[k] = v
				}
			}
			notifyCfg.Webhook = &notify.WebhookConfig{
				URL:     n.Webhook.URL,
				Headers: headers,
			}
		}
		var err error
		r.notifier, err = notify.Build(notifyCfg, log.Get("notify"))
		if err != nil {
			return fmt.Errorf("initialising notifier: %w", err)
		}
		r.log.Debug("notifier initialised")
	}

	r.log.Debug("config loaded", map[string]any{
		"strategy":    branchStrategy,
		"scheme":      cfg.Versioning.Scheme,
		"scm":         cfg.SCM.Provider,
		"tracker":     cfg.Tracker.Provider,
		"working_dir": r.workingDir,
	})
	return nil
}

func (r *releaseRunner) detectVersion() error {
	r.print("Detecting release version")

	branch, err := r.git.CurrentBranch()
	if err != nil {
		return fmt.Errorf("detecting current branch: %w", err)
	}
	r.originalBranch = branch

	r.mainHeadSHA, err = r.git.RevParse(r.cfg.Git.DefaultBranch)
	if err != nil {
		return fmt.Errorf("snapshotting %s HEAD: %w", r.cfg.Git.DefaultBranch, err)
	}
	if !r.singleBranch {
		r.devHeadSHA, err = r.git.RevParse(r.cfg.Git.DevelopmentBranch)
		if err != nil {
			return fmt.Errorf("snapshotting %s HEAD: %w", r.cfg.Git.DevelopmentBranch, err)
		}
	}
	r.log.Debug("HEAD refs snapshotted", map[string]any{
		"main": r.mainHeadSHA,
		"dev":  r.devHeadSHA,
	})

	switch r.cfg.Versioning.Scheme {
	case "calver":
		r.scheme = versioning.CalVer{}
	default:
		r.scheme = versioning.SemVer{}
	}

	latestTag, err := r.git.LatestTag()
	if err != nil {
		return fmt.Errorf("detecting latest tag: %w", err)
	}

	if latestTag == "" {
		initial, err := ui.Input(
			"No previous tag reference found",
			"Enter the initial version",
			"0.1.0",
		)
		if err != nil {
			return err
		}
		r.currentVersion, err = r.scheme.Parse(initial)
		if err != nil {
			return fmt.Errorf("parsing initial version %q: %w", initial, err)
		}
		r.log.Info("initial version set by user", map[string]any{"version": r.currentVersion.String()})
	} else {
		r.currentVersion, err = r.scheme.Parse(latestTag)
		if err != nil {
			return fmt.Errorf("parsing latest tag %q: %w", latestTag, err)
		}
	}

	commits, err := r.git.Log(latestTag)
	if err != nil {
		return fmt.Errorf("reading commit log: %w", err)
	}
	r.parsedCommits = versioning.ParseCommits(commits)

	if latestTag == "" {
		r.vprintf("Current version: %s (initial)\n", r.currentVersion.String())
	} else {
		r.vprintf("Current version: %s (tag %s)\n", r.currentVersion.String(), latestTag)
	}
	r.vprintf("Commits analyzed: %d\n", len(commits))
	r.log.Info("version detected", map[string]any{
		"current": r.currentVersion.String(),
		"tag":     latestTag,
		"commits": len(commits),
	})
	return nil
}

func (r *releaseRunner) recommendVersion() error {
	r.print("Analyzing commits")

	bump := versioning.Recommend(r.parsedCommits)

	next, err := r.currentVersion.Increment(bump)
	if err != nil {
		return fmt.Errorf("computing next version: %w", err)
	}
	r.nextVersion = next

	var feat, fix, breaking, other int
	for _, c := range r.parsedCommits {
		switch {
		case c.Breaking:
			breaking++
		case c.Type == "feat":
			feat++
		case c.Type == "fix":
			fix++
		default:
			other++
		}
	}
	if r.verbose {
		r.vprintf("feat: %d  fix: %d  breaking: %d  other: %d\n", feat, fix, breaking, other)
	}

	if bump == versioning.BumpNone {
		r.log.Warn("no releasable commits found")
		if !r.quiet {
			fmt.Println(ui.Warning("No releasable commits found — no version bump is required."))
		}
		if !r.noInteraction {
			ok, err := ui.Confirm("Continue anyway?", "A release will be created at the same version.")
			if err != nil {
				return err
			}
			if !ok {
				r.log.Warn("release cancelled by user at no-version-bump gate")
				return fmt.Errorf("release cancelled")
			}
		}
		r.log.Info("proceeding with no version bump")
	}

	bumpLabel := map[versioning.BumpKind]string{
		versioning.BumpNone:  "none",
		versioning.BumpPatch: "patch",
		versioning.BumpMinor: "minor",
		versioning.BumpMajor: "major",
	}[bump]

	r.log.Info("version recommended", map[string]any{
		"bump":     bumpLabel,
		"next":     r.nextVersion.String(),
		"feat":     feat,
		"fix":      fix,
		"breaking": breaking,
	})

	if !r.quiet {
		fmt.Printf("Current version : %s\nRecommended bump: %s\nNext version    : %s\n",
			r.currentVersion.String(), bumpLabel, r.nextVersion.String())
	}
	return nil
}

func (r *releaseRunner) confirmVersion() error {
	override, err := ui.Input(
		"Confirm release version",
		"Press Enter to accept or type a custom version",
		r.nextVersion.String(),
	)
	if err != nil {
		return err
	}
	override = strings.TrimSpace(override)
	if override != "" && override != r.nextVersion.String() {
		parsed, err := r.scheme.Parse(override)
		if err != nil {
			return fmt.Errorf("invalid version %q: %w", override, err)
		}
		r.nextVersion = parsed
		r.log.Info("version overridden by user", map[string]any{"version": r.nextVersion.String()})
	}
	return nil
}

func (r *releaseRunner) createReleaseBranch() error {
	r.print("Creating release branch")

	if tmpl := r.cfg.Git.ReleaseBranchTemplate; tmpl != "" {
		r.releaseBranch = strings.NewReplacer(
			"{{version}}", r.nextVersion.String(),
			"{{tag}}", r.tag,
		).Replace(tmpl)
	} else {
		r.releaseBranch = deriveReleaseBranch(r.cfg.Git.DevelopmentBranch, r.nextVersion.String())
	}
	if !r.quiet {
		fmt.Printf("Release branch: %s\n", r.releaseBranch)
	}
	r.vprintf("Base branch: %s\n", r.cfg.Git.DevelopmentBranch)

	if r.dryRun {
		if !r.quiet {
			fmt.Printf("[dry-run] would create branch %s from %s\n", r.releaseBranch, r.cfg.Git.DevelopmentBranch)
		}
		return nil
	}

	if err := r.git.CreateBranch(r.releaseBranch, r.cfg.Git.DevelopmentBranch); err != nil {
		return fmt.Errorf("creating release branch: %w", err)
	}
	r.releaseBranchCreated = true
	r.log.Info("release branch created", map[string]any{"branch": r.releaseBranch})
	return nil
}

// deriveReleaseBranch computes the release branch name from the dev branch.
// "dev" → "release/X.Y.Z", "project/component/dev" → "project/component/release/X.Y.Z"
func deriveReleaseBranch(devBranch, version string) string {
	segments := strings.Split(devBranch, "/")
	last := segments[len(segments)-1]
	releaseSegment := "release/" + version
	switch last {
	case "dev", "develop", "development":
		segments[len(segments)-1] = releaseSegment
		return strings.Join(segments, "/")
	default:
		return releaseSegment
	}
}

func (r *releaseRunner) mergeMainIntoRelease() error {
	r.print("Merging main into release")

	if r.dryRun {
		if !r.quiet {
			fmt.Printf("[dry-run] would merge %s into %s\n", r.cfg.Git.DefaultBranch, r.releaseBranch)
		}
		return nil
	}

	err := r.git.Merge(r.cfg.Git.DefaultBranch)
	if err == nil {
		r.log.Debug("main merged into release", map[string]any{"source": r.cfg.Git.DefaultBranch})
		return nil
	}

	conflicted, cerr := r.git.ConflictedFiles()
	if cerr != nil || len(conflicted) == 0 {
		return fmt.Errorf("merging %s into release branch: %w", r.cfg.Git.DefaultBranch, err)
	}

	return r.resolveConflicts(conflicted)
}

func (r *releaseRunner) resolveConflicts(conflicted []string) error {
	if !r.quiet {
		fmt.Println(ui.Warning(fmt.Sprintf("%d conflicted file(s):\n  %s", len(conflicted), strings.Join(conflicted, "\n  "))))
	}

	ok, err := ui.Confirm(
		"Auto-resolve conflicts?",
		"Files inside the working directory will use ours; files outside will use theirs.",
	)
	if err != nil {
		return err
	}
	if !ok {
		r.log.Warn("merge conflict resolution declined by user")
		return fmt.Errorf("merge conflict resolution declined — resolve manually and retry")
	}

	for _, file := range conflicted {
		abs := filepath.Join(r.repoRoot, file)
		rel, err := filepath.Rel(r.workingDir, abs)
		strategy := "--theirs"
		if err == nil && !strings.HasPrefix(rel, "..") {
			strategy = "--ours"
		}
		if err := r.git.Checkout(strategy, file); err != nil {
			return fmt.Errorf("resolving conflict in %s: %w", file, err)
		}
	}
	if err := r.git.Add("."); err != nil {
		return fmt.Errorf("staging resolved conflicts: %w", err)
	}
	r.log.Info("merge conflicts auto-resolved", map[string]any{"count": len(conflicted)})
	return nil
}

func (r *releaseRunner) trackerCreateVersion() error {
	if r.trackerProvider == nil {
		return nil
	}
	r.print("Tracker version")

	versions, err := r.trackerProvider.ListVersions()
	if err != nil {
		return fmt.Errorf("listing tracker versions: %w", err)
	}

	if len(versions) > 0 {
		options := make([]huh.Option[string], 0, len(versions)+1)
		for _, v := range versions {
			options = append(options, huh.NewOption(v.Name, v.ID))
		}
		const createNew = "__create_new__"
		options = append(options, huh.NewOption(fmt.Sprintf("Create new version %q", r.nextVersion.String()), createNew))

		selected, err := ui.SelectString("Select tracker version",
			"Choose an existing open version or create a new one", options)
		if err != nil {
			return err
		}
		if selected != createNew {
			r.trackerVersionID = selected
			r.log.Info("tracker version ready", map[string]any{"id": r.trackerVersionID, "version": r.nextVersion.String()})
			return nil
		}
	}

	if !r.noInteraction {
		ok, err := ui.Confirm(fmt.Sprintf("Create version %q in issue tracker?", r.nextVersion.String()), "")
		if err != nil {
			return err
		}
		if !ok {
			r.log.Info("tracker version creation skipped by user")
			return nil
		}
	}

	if r.dryRun {
		if !r.quiet {
			fmt.Printf("[dry-run] would create tracker version %q\n", r.nextVersion.String())
		}
		return nil
	}

	desc := fmt.Sprintf("Generated by releasar v%s", appVersion)
	id, err := r.trackerProvider.CreateVersion(r.nextVersion.String(), desc)
	if err != nil {
		return fmt.Errorf("creating tracker version: %w", err)
	}
	r.trackerVersionID = id
	r.log.Info("tracker version ready", map[string]any{"id": r.trackerVersionID, "version": r.nextVersion.String()})
	return nil
}

func (r *releaseRunner) trackerAssignTickets() error {
	if r.trackerProvider == nil || r.trackerVersionID == "" {
		return nil
	}

	var refs []string
	for _, c := range r.parsedCommits {
		refs = append(refs, c.Refs...)
	}
	if len(refs) == 0 {
		return nil
	}

	resolved, err := r.trackerProvider.ResolveRefs(refs)
	if err != nil {
		return fmt.Errorf("resolving ticket refs: %w", err)
	}
	if len(resolved) == 0 {
		return nil
	}

	r.print(fmt.Sprintf("Found %d ticket(s) referenced in commits", len(resolved)))
	if !r.quiet {
		for _, ref := range resolved {
			fmt.Printf("  %-14s  %-12s  %s\n", ref.Ref, ref.State, ref.Title)
		}
	}

	if !r.noInteraction {
		ok, err := ui.Confirm(
			fmt.Sprintf("Assign %d ticket(s) to version %q?", len(resolved), r.nextVersion.String()), "")
		if err != nil {
			return err
		}
		if !ok {
			r.log.Info("ticket assignment skipped by user")
			return nil
		}
	}

	if r.dryRun {
		if !r.quiet {
			fmt.Printf("[dry-run] would assign %d ticket(s) to tracker version %q\n", len(resolved), r.nextVersion.String())
		}
		return nil
	}

	assignRefs := make([]string, len(resolved))
	for i, ref := range resolved {
		assignRefs[i] = ref.Ref
	}
	if err := r.trackerProvider.AssignTickets(assignRefs, r.trackerVersionID); err != nil {
		return fmt.Errorf("assigning tickets to tracker version: %w", err)
	}
	r.log.Info("tickets assigned", map[string]any{"count": len(assignRefs), "version": r.nextVersion.String()})
	return nil
}

func (r *releaseRunner) updateChangelog() error {
	r.print("Updating changelog")

	if !r.cfg.Changelog.Enabled {
		r.vprintf("Disabled — skipping\n")
		return nil
	}

	changelogPath := filepath.Join(r.workingDir, r.cfg.Changelog.Path)
	r.vprintf("Path: %s\n", changelogPath)
	entry := versioning.ChangelogFromCommits(r.nextVersion.String(), time.Now(), r.parsedCommits)

	if r.dryRun {
		if !r.quiet {
			fmt.Printf("[dry-run] would prepend changelog entry for %s to %s\n", r.nextVersion.String(), changelogPath)
		}
		return nil
	}

	if err := versioning.PrependChangelog(changelogPath, entry); err != nil {
		return fmt.Errorf("updating changelog: %w", err)
	}
	r.changedFiles = append(r.changedFiles, changelogPath)
	r.log.Info("changelog updated", map[string]any{"path": changelogPath, "version": r.nextVersion.String()})
	return nil
}

func (r *releaseRunner) applySubstitutions() error {
	r.print("Applying version substitutions")
	r.vprintf("Working directory: %s\n", r.workingDir)

	if r.dryRun {
		if !r.quiet {
			fmt.Printf("[dry-run] would substitute placeholders in %s\n", r.workingDir)
		}
		return nil
	}

	before := len(r.changedFiles)
	if err := r.substitutePlaceholders(); err != nil {
		return err
	}
	if err := r.updateManifestVersion(); err != nil {
		return err
	}
	r.log.Info("substitutions applied", map[string]any{"files": len(r.changedFiles) - before})
	return nil
}

func (r *releaseRunner) substitutePlaceholders() error {
	return filepath.WalkDir(r.workingDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		rel, _ := filepath.Rel(r.workingDir, path)

		for _, pattern := range r.cfg.Versioning.PlaceholderExclude {
			matched, _ := filepath.Match(pattern, rel)
			if matched {
				return nil
			}
		}

		if isBinary(path) {
			return nil
		}

		if err := versioning.ReplaceInFile(path, r.currentVersion); err != nil {
			return fmt.Errorf("substituting placeholders in %s: %w", rel, err)
		}
		r.changedFiles = append(r.changedFiles, path)
		return nil
	})
}

func isBinary(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 8192)
	n, err := f.Read(buf)
	if err != nil {
		return false
	}
	for _, b := range buf[:n] {
		if b == 0 {
			return true
		}
	}
	return false
}

func (r *releaseRunner) updateManifestVersion() error {
	for _, name := range []string{"package.json", "composer.json"} {
		path := filepath.Join(r.workingDir, name)
		if _, err := os.Stat(path); err != nil {
			continue
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", name, err)
		}

		var manifest map[string]json.RawMessage
		if err := json.Unmarshal(data, &manifest); err != nil {
			continue
		}

		if _, ok := manifest["version"]; !ok {
			continue
		}

		versionJSON, err := json.Marshal(r.nextVersion.String())
		if err != nil {
			return fmt.Errorf("marshalling version: %w", err)
		}
		manifest["version"] = versionJSON

		indent := detectIndent(data)
		updated, err := json.MarshalIndent(manifest, "", indent)
		if err != nil {
			return fmt.Errorf("marshalling %s: %w", name, err)
		}
		updated = append(updated, '\n')

		info, _ := os.Stat(path)
		mode := fs.FileMode(0o644)
		if info != nil {
			mode = info.Mode()
		}

		if err := os.WriteFile(path, updated, mode); err != nil {
			return fmt.Errorf("writing %s: %w", name, err)
		}
		r.changedFiles = append(r.changedFiles, path)
	}
	return nil
}

func detectIndent(data []byte) string {
	lines := strings.SplitN(string(data), "\n", 5)
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed == "" || strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "}") {
			continue
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		if indent != "" {
			return indent
		}
	}
	return "  "
}

func (r *releaseRunner) runSecretsScan() error {
	if !r.cfg.Tasks.SecretScanning {
		return nil
	}
	r.print("Scanning for secrets")
	if err := tasks.RunSecretsScan(r.workingDir, r.verbose); err != nil {
		return err
	}
	r.log.Info("secrets scan passed")
	return nil
}

func (r *releaseRunner) runBuildTasks() error {
	return r.runTasks("build", r.cfg.Tasks.Build)
}

func (r *releaseRunner) runTests() error {
	return r.runTasks("test", r.cfg.Tasks.Test)
}

func (r *releaseRunner) runTasks(label string, tasks []config.TaskRef) error {
	if len(tasks) == 0 {
		return nil
	}
	r.print("Running " + label + " tasks")
	for _, ref := range tasks {
		pm, script := config.ParseTaskRef(ref)
		if pm == "" {
			pm = r.cfg.DetectedProjectType.PackageManager()
		}
		if pm == "" {
			supported := make([]string, len(config.SupportedProjectTypes))
			for i, k := range config.SupportedProjectTypes {
				supported[i] = string(k)
			}
			return fmt.Errorf("cannot run %s task %q: no project type detected\nhint: set \"projectType\" in releasar.json (supported values: %s)", label, script, strings.Join(supported, ", "))
		}

		if !r.quiet {
			fmt.Printf("Running %s task: %s run %s\n", label, pm, script)
		}
		if r.dryRun {
			continue
		}

		cmd := exec.Command(pm, "run", script)
		cmd.Dir = r.workingDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s task %q failed: %w", label, script, err)
		}
	}
	return nil
}

func (r *releaseRunner) reviewGate() error {
	r.print("Review gate")

	if !r.quiet && len(r.changedFiles) > 0 {
		fmt.Println("\nModified files:")
		for _, f := range r.changedFiles {
			rel, _ := filepath.Rel(r.repoRoot, f)
			fmt.Printf("  %s\n", rel)
		}
	}

	if r.noInteraction || r.dryRun {
		return nil
	}

	ok, err := ui.Confirm(
		"Review complete?",
		"Make any manual edits now, then confirm to proceed to commit.",
	)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("release cancelled at review gate")
	}
	r.log.Info("review gate passed by user")

	if len(r.changedFiles) > 0 {
		if err := r.git.Add(r.changedFiles...); err != nil {
			return fmt.Errorf("re-staging files after review: %w", err)
		}
	}
	return nil
}

func (r *releaseRunner) commitRelease() error {
	r.print("Committing release")

	var subject string
	if r.singleBranch {
		subject = strings.ReplaceAll(r.cfg.Git.ReleaseCommitTemplate, "{{tag}}", r.tag)
	} else {
		subject = "chore(release): " + r.nextVersion.String()
	}
	message := subject + "\n\nCo-authored-by: releasar v" + appVersion + " <noreply@releasar.dev>"

	r.vprintf("Commit: %s\n", subject)

	if r.dryRun {
		if !r.quiet {
			fmt.Printf("[dry-run] would commit on %s: %q\n", r.releaseBranch, subject)
		}
		return nil
	}

	if err := r.git.Add(r.changedFiles...); err != nil {
		return fmt.Errorf("staging release files: %w", err)
	}
	if err := r.git.Commit(message); err != nil {
		return fmt.Errorf("committing release changes: %w", err)
	}
	r.log.Info("release committed", map[string]any{"subject": subject})
	return nil
}

func (r *releaseRunner) tagRelease() error {
	r.print("Tagging release")
	r.vprintf("Tag: %s\n", r.tag)

	if r.dryRun {
		if !r.quiet {
			fmt.Printf("[dry-run] would create tag %s on release branch HEAD\n", r.tag)
		}
		return nil
	}

	if err := r.git.Tag(r.nextVersion.String(), "Release "+r.tag); err != nil {
		return fmt.Errorf("creating tag %s: %w", r.tag, err)
	}
	r.tagCreated = true
	r.log.Info("release tagged", map[string]any{"tag": r.tag})
	return nil
}

func (r *releaseRunner) mergeIntoMain() error {
	r.print("Merging into main")
	r.vprintf("Target branch: %s\n", r.cfg.Git.DefaultBranch)

	if r.dryRun {
		if !r.quiet {
			fmt.Printf("[dry-run] would merge %s into %s\n", r.releaseBranch, r.cfg.Git.DefaultBranch)
		}
		return nil
	}

	if err := r.git.Switch(r.cfg.Git.DefaultBranch); err != nil {
		return fmt.Errorf("switching to %s: %w", r.cfg.Git.DefaultBranch, err)
	}

	if r.singleBranch {
		if err := r.git.Merge(r.releaseBranch, "--no-ff", "--no-edit"); err != nil {
			return err
		}
		r.log.Info("merged into main", map[string]any{"target": r.cfg.Git.DefaultBranch})
		return nil
	}

	if err := r.git.Merge(r.releaseBranch, "--squash"); err != nil {
		return fmt.Errorf("squash-merging release into %s: %w", r.cfg.Git.DefaultBranch, err)
	}

	if err := r.verifySquashCommitFiles(); err != nil {
		return err
	}

	subject := strings.ReplaceAll(r.cfg.Git.ReleaseCommitTemplate, "{{tag}}", r.tag)
	message := subject + "\n\n" + r.coAuthorTrailers()
	if err := r.git.Commit(message); err != nil {
		return fmt.Errorf("committing squash merge into %s: %w", r.cfg.Git.DefaultBranch, err)
	}
	r.log.Info("merged into main", map[string]any{"target": r.cfg.Git.DefaultBranch})
	return nil
}

func (r *releaseRunner) verifySquashCommitFiles() error {
	out, err := exec.Command("git", "-C", r.repoRoot, "diff", "--cached", "--name-only").Output()
	if err != nil {
		return nil // best-effort
	}
	var outsideFiles []string
	for _, f := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if f == "" {
			continue
		}
		abs := filepath.Join(r.repoRoot, f)
		rel, err := filepath.Rel(r.workingDir, abs)
		if err != nil || strings.HasPrefix(rel, "..") {
			outsideFiles = append(outsideFiles, f)
		}
	}
	if len(outsideFiles) == 0 {
		return nil
	}
	if !r.quiet {
		fmt.Println(ui.Warning(fmt.Sprintf(
			"Squash commit includes %d file(s) outside the working directory:\n  %s",
			len(outsideFiles), strings.Join(outsideFiles, "\n  "),
		)))
	}
	ok, err := ui.Confirm("Continue with these files in the release commit?", "")
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("release cancelled: unexpected files in squash commit")
	}
	return nil
}

func (r *releaseRunner) mergeIntoDev() error {
	r.print("Merging into dev")
	r.vprintf("Target branch: %s\n", r.cfg.Git.DevelopmentBranch)

	if r.dryRun {
		if !r.quiet {
			fmt.Printf("[dry-run] would merge %s into %s\n", r.releaseBranch, r.cfg.Git.DevelopmentBranch)
		}
		return nil
	}

	if err := r.git.Switch(r.cfg.Git.DevelopmentBranch); err != nil {
		return fmt.Errorf("switching to %s: %w", r.cfg.Git.DevelopmentBranch, err)
	}
	if err := r.git.Merge(r.releaseBranch, "--no-ff", "--no-edit"); err != nil {
		return fmt.Errorf("merging release branch into %s: %w", r.cfg.Git.DevelopmentBranch, err)
	}
	r.log.Info("merged into dev", map[string]any{"target": r.cfg.Git.DevelopmentBranch})
	return nil
}

func (r *releaseRunner) cleanupReleaseBranch() error {
	r.print("Cleaning up release branch")
	r.vprintf("Branch: %s\n", r.releaseBranch)

	if r.dryRun {
		if !r.quiet {
			fmt.Printf("[dry-run] would delete branch %s\n", r.releaseBranch)
		}
		return nil
	}

	// Move off the release branch (to default) before deleting it.
	if err := r.git.Switch(r.cfg.Git.DefaultBranch); err != nil {
		return fmt.Errorf("switching to %s before branch deletion: %w", r.cfg.Git.DefaultBranch, err)
	}
	if err := r.git.DeleteBranch(r.releaseBranch); err != nil {
		if !r.quiet {
			fmt.Println(ui.Warning(fmt.Sprintf("Could not delete release branch %s: %v", r.releaseBranch, err)))
		}
		r.log.Warn("could not delete release branch", map[string]any{"branch": r.releaseBranch, "error": err})
	} else {
		r.log.Debug("release branch deleted", map[string]any{"branch": r.releaseBranch})
	}
	return nil
}

func (r *releaseRunner) push() error {
	r.print("Pushing to remote")
	r.vprintf("Remote: %s\n", r.cfg.Git.Remote)

	// Local release is done — disable rollback from this point.
	r.completed = true

	ok, err := ui.Confirm(
		"Push branches to remote?",
		fmt.Sprintf("Will push to remote %q. Declining is fine if your organisation requires a manual push.", r.cfg.Git.Remote),
	)
	if err != nil {
		return err
	}
	if ok {
		if err := r.pushBranches(); err != nil {
			return err
		}
	} else {
		r.log.Info("branch push skipped by user")
	}

	okTag, err := ui.Confirm(
		"Push tag to remote?",
		fmt.Sprintf("Will push tag %s to remote %q.", r.tag, r.cfg.Git.Remote),
	)
	if err != nil {
		return err
	}
	if okTag {
		if err := r.pushTag(); err != nil {
			return err
		}
	} else {
		r.log.Info("tag push skipped by user")
	}

	return nil
}

func (r *releaseRunner) pushBranches() error {
	remote := r.cfg.Git.Remote
	if r.dryRun {
		if !r.quiet {
			fmt.Printf("[dry-run] would push %s to %s\n", r.cfg.Git.DefaultBranch, remote)
			if !r.singleBranch {
				fmt.Printf("[dry-run] would push %s to %s\n", r.cfg.Git.DevelopmentBranch, remote)
			}
		}
		return nil
	}

	if err := r.git.Push(remote, r.cfg.Git.DefaultBranch); err != nil {
		return fmt.Errorf("pushing %s: %w", r.cfg.Git.DefaultBranch, err)
	}
	if !r.singleBranch {
		if err := r.git.Push(remote, r.cfg.Git.DevelopmentBranch); err != nil {
			return fmt.Errorf("pushing %s: %w", r.cfg.Git.DevelopmentBranch, err)
		}
	}
	r.log.Info("branches pushed", map[string]any{"remote": remote})

	if err := r.pushToExtraRemotes(r.cfg.Git.DefaultBranch); err != nil {
		return err
	}
	return nil
}

func (r *releaseRunner) pushTag() error {
	remote := r.cfg.Git.Remote
	if r.dryRun {
		if !r.quiet {
			fmt.Printf("[dry-run] would push tag %s to %s\n", r.tag, remote)
		}
		return nil
	}

	if err := r.git.Push(remote, r.tag); err != nil {
		return fmt.Errorf("pushing tag %s: %w", r.tag, err)
	}
	r.log.Info("tag pushed", map[string]any{"remote": remote, "tag": r.tag})

	if err := r.pushToExtraRemotes(r.tag); err != nil {
		return err
	}
	return nil
}

func (r *releaseRunner) pushToExtraRemotes(ref string) error {
	remotes, err := r.git.Remotes()
	if err != nil {
		return nil // best-effort
	}

	var extras []string
	for _, rem := range remotes {
		if rem != r.cfg.Git.Remote {
			extras = append(extras, rem)
		}
	}
	if len(extras) == 0 {
		return nil
	}

	opts := make([]huh.Option[string], len(extras))
	for i, rem := range extras {
		opts[i] = huh.NewOption(rem, rem)
	}

	selected, err := ui.MultiSelectString(
		fmt.Sprintf("Push %s to additional remotes?", ref),
		"Select any extra remotes (none is fine)",
		opts,
	)
	if err != nil || len(selected) == 0 {
		return nil
	}

	for _, rem := range selected {
		if r.dryRun {
			if !r.quiet {
				fmt.Printf("[dry-run] would push %s to %s\n", ref, rem)
			}
			continue
		}
		if err := r.git.Push(rem, ref); err != nil {
			r.log.Warn("could not push to extra remote", map[string]any{"ref": ref, "remote": rem, "error": err})
			if !r.quiet {
				fmt.Println(ui.Warning(fmt.Sprintf("Could not push %s to %s: %v", ref, rem, err)))
			}
		}
	}
	return nil
}

func (r *releaseRunner) trackerCloseVersion() error {
	if r.trackerProvider == nil || r.trackerVersionID == "" {
		return nil
	}

	if !r.noInteraction {
		ok, err := ui.Confirm(
			fmt.Sprintf("Close version %q in issue tracker?", r.nextVersion.String()),
			"This will mark the milestone/version as released.")
		if err != nil {
			return err
		}
		if !ok {
			r.log.Info("tracker version close skipped by user")
			return nil
		}
	}

	if r.dryRun {
		if !r.quiet {
			fmt.Printf("[dry-run] would close tracker version %q\n", r.nextVersion.String())
		}
		return nil
	}

	if err := r.trackerProvider.CloseVersion(r.trackerVersionID); err != nil {
		return fmt.Errorf("closing tracker version: %w", err)
	}
	r.log.Info("tracker version closed", map[string]any{"id": r.trackerVersionID, "version": r.nextVersion.String()})
	return nil
}

func (r *releaseRunner) scmCreateRelease() error {
	if r.scmProvider == nil {
		return nil
	}

	entry := versioning.ChangelogFromCommits(r.nextVersion.String(), time.Now(), r.parsedCommits)
	body := entry.Render()

	if r.dryRun {
		if !r.quiet {
			fmt.Printf("[dry-run] would create SCM release for %s\n", r.tag)
		}
		return nil
	}

	if !r.noInteraction {
		ok, err := ui.Confirm(
			"Create SCM release?",
			fmt.Sprintf("Will publish release %s on %s.", r.tag, r.cfg.SCM.Provider),
		)
		if err != nil {
			return err
		}
		if !ok {
			r.log.Info("SCM release creation skipped by user")
			return nil
		}
	}

	releaseURL, err := r.scmProvider.CreateRelease(r.tag, r.tag, body)
	if err != nil {
		return fmt.Errorf("creating SCM release: %w", err)
	}
	r.scmReleaseURL = releaseURL
	r.log.Info("SCM release created", map[string]any{"provider": r.cfg.SCM.Provider, "url": releaseURL})
	if !r.quiet {
		fmt.Printf("Release published: %s\n", releaseURL)
	}
	return nil
}

func (r *releaseRunner) sendNotifications() {
	if r.notifier == nil {
		return
	}
	if r.dryRun {
		if !r.quiet {
			fmt.Println("[dry-run] would send release notifications")
		}
		return
	}

	r.log.Debug("dispatching notifications")
	entry := versioning.ChangelogFromCommits(r.nextVersion.String(), time.Now(), r.parsedCommits)
	if err := r.notifier.Notify(notify.Event{
		Type:    notify.EventReleaseComplete,
		Tag:     r.tag,
		Version: r.nextVersion.String(),
		URL:     r.scmReleaseURL,
		Body:    entry.Render(),
	}); err != nil {
		r.log.Warn("notification error", map[string]any{"error": err})
		if !r.quiet {
			fmt.Printf("Warning: notification error: %s\n", err)
		}
	}
}

func (r *releaseRunner) printSummary() {
	if r.quiet {
		return
	}
	mode := "Git Flow"
	if r.singleBranch {
		mode = "single-branch"
	}
	fmt.Println(ui.Success(fmt.Sprintf(
		"Released %s  [%s]\nTag: %s | Branch: %s",
		r.nextVersion.String(), mode, r.tag, r.cfg.Git.DefaultBranch,
	)))
}

// coAuthorTrailers builds a block of Co-authored-by: trailer lines from the
// release commits, deduplicated by email, followed by the releasar trailer.
func (r *releaseRunner) coAuthorTrailers() string {
	seen := make(map[string]bool)
	var lines []string
	for _, c := range r.parsedCommits {
		if c.AuthorEmail == "" || seen[c.AuthorEmail] {
			continue
		}
		seen[c.AuthorEmail] = true
		lines = append(lines, fmt.Sprintf("Co-authored-by: %s <%s>", c.Author, c.AuthorEmail))
	}
	lines = append(lines, "Co-authored-by: releasar v"+appVersion+" <noreply@releasar.dev>")
	return strings.Join(lines, "\n")
}

func (r *releaseRunner) rollback() {
	if r.completed {
		return
	}
	r.log.Warn("rollback triggered", map[string]any{"main_sha": r.mainHeadSHA, "tag": r.tag})
	fmt.Println(ui.Warning("Rolling back release — restoring branches to pre-release state..."))

	if r.mainHeadSHA != "" {
		_ = r.git.Switch(r.cfg.Git.DefaultBranch)
		if err := r.git.Reset(r.mainHeadSHA, "--hard"); err == nil {
			fmt.Printf("  ✓ %s reset to %s\n", r.cfg.Git.DefaultBranch, r.mainHeadSHA[:8])
		}
	}
	if !r.singleBranch && r.devHeadSHA != "" {
		_ = r.git.Switch(r.cfg.Git.DevelopmentBranch)
		if err := r.git.Reset(r.devHeadSHA, "--hard"); err == nil {
			fmt.Printf("  ✓ %s reset to %s\n", r.cfg.Git.DevelopmentBranch, r.devHeadSHA[:8])
		}
	}
	if r.releaseBranchCreated {
		_ = r.git.Switch(r.cfg.Git.DefaultBranch)
		if err := r.git.DeleteBranch(r.releaseBranch); err == nil {
			fmt.Printf("  ✓ branch %s deleted\n", r.releaseBranch)
		}
	}
	if r.tagCreated {
		if err := r.git.DeleteTag(r.nextVersion.String()); err == nil {
			fmt.Printf("  ✓ local tag %s deleted\n", r.tag)
		} else {
			fmt.Println(ui.Caution(fmt.Sprintf(
				"Could not delete local tag %s — delete it manually before retrying:\n  git tag -d %s\n  git push <remote> :refs/tags/%s",
				r.tag, r.tag, r.tag,
			)))
		}
	}
	if r.originalBranch != "" {
		_ = r.git.Switch(r.originalBranch)
	}
	fmt.Println(ui.Note("All branches restored. No changes were pushed."))
}
