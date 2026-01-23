package main

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/pquerna/otp/totp"
	"github.com/spf13/cobra"
	"go.goblog.app/app/pkgs/utils"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "GoBlog",
		Short: "Main application, without any command, the app gets started.",
		Run: func(cmd *cobra.Command, args []string) {
			app := initializeApp(cmd)
			if err := app.initPlugins(); err != nil {
				app.logErrAndQuit("Failed to init plugins", "err", err)
				return
			}
			app.preStartHooks()
			initializeComponents(app)
			app.startHourlyHooks()
			app.startPprofServer()
			if err := app.startServer(); err != nil {
				app.logErrAndQuit("Failed to start server(s)", "err", err)
			}
			app.shutdown.Wait()
		},
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if cpuprofile, _ := cmd.Flags().GetString("cpuprofile"); cpuprofile != "" {
				r, w := io.Pipe()
				go func() {
					_ = w.CloseWithError(pprof.StartCPUProfile(w))
				}()
				go func() {
					_ = r.CloseWithError(utils.SaveToFile(r, cpuprofile))
				}()
			}
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			pprof.StopCPUProfile()
			if memprofile, _ := cmd.Flags().GetString("memprofile"); memprofile != "" {
				runtime.GC()
				r, w := io.Pipe()
				go func() {
					_ = w.CloseWithError(pprof.WriteHeapProfile(w))
				}()
				_ = r.CloseWithError(utils.SaveToFile(r, memprofile))
			}
		},
	}

	// Add flags
	rootCmd.PersistentFlags().String("cpuprofile", "", "write CPU profile to file")
	rootCmd.PersistentFlags().String("memprofile", "", "write memory profile to file")
	rootCmd.PersistentFlags().String("config", "", "use a specific config file")

	// Healthcheck command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "healthcheck",
		Short: "Perform health check",
		Long: `Perform a health check on the GoBlog server.

This command checks if the server is running and healthy by making an HTTP
request to the health endpoint. It returns exit code 0 if healthy, or 1 if
unhealthy.

Useful for container health checks (Docker, Kubernetes) and monitoring systems.

Example:
  ./GoBlog healthcheck
  echo $?  # 0 = healthy, 1 = unhealthy`,
		Run: func(cmd *cobra.Command, args []string) {
			app := initializeApp(cmd)
			health := app.healthcheckExitCode()
			app.shutdown.ShutdownAndWait()
			os.Exit(health)
		},
	})

	// Link check tool
	rootCmd.AddCommand(&cobra.Command{
		Use:   "check",
		Short: "Check all external links",
		Long: `Check all external links in published posts for broken links.

This command scans all published posts and verifies that external links are
still accessible. It reports any broken links (404s, connection errors, etc.)
to help you maintain link quality on your blog.

Example:
  ./GoBlog check`,
		Run: func(cmd *cobra.Command, args []string) {
			app := initializeApp(cmd)
			if err := app.initTemplateStrings(); err != nil {
				app.logErrAndQuit("Failed to start check", "err", err)
			}
			if err := app.checkAllExternalLinks(); err != nil {
				app.logErrAndQuit("Failed to check links", "err", err)
			}
			app.shutdown.ShutdownAndWait()
		},
	})

	// Markdown export command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "export [directory]",
		Short: "Export markdown files",
		Long: `Export all posts as Markdown files with front matter.

This command exports all posts from the database to individual Markdown files,
preserving the front matter metadata (title, date, tags, etc.). This is useful
for backups, migration to other platforms, or version control.

If no directory is specified, files are exported to the current directory.

Example:
  ./GoBlog export ./backup
  ./GoBlog export  # exports to current directory`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			app := initializeApp(cmd)
			var dir string
			if len(args) > 0 {
				dir = args[0]
			}
			if err := app.exportMarkdownFiles(dir); err != nil {
				app.logErrAndQuit("Failed to export markdown files", "err", err)
			}
			app.shutdown.ShutdownAndWait()
		},
	})

	// ActivityPub refetch followers
	activityPubCmd := &cobra.Command{
		Use:   "activitypub",
		Short: "ActivityPub related tasks",
		Long: `ActivityPub related tasks for managing your Fediverse presence.

These commands help you manage your ActivityPub/Fediverse account, including
follower management and account migration.`,
	}
	activityPubCmd.AddCommand(&cobra.Command{
		Use:   "refetch-followers blog",
		Short: "Refetch ActivityPub followers",
		Long: `Refetch and update ActivityPub follower information from remote servers.

This command contacts each follower's home server to refresh their profile
information (username, inbox URL, etc.). This is useful if follower data
has become stale or if there were federation issues.

Example:
  ./GoBlog activitypub refetch-followers default`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			app := initializeApp(cmd)
			if !app.apEnabled() {
				app.logErrAndQuit("ActivityPub not enabled")
				return
			}
			if err := app.initActivityPubBase(); err != nil {
				app.logErrAndQuit("Failed to init ActivityPub base", "err", err)
				return
			}
			blog := args[0]
			if err := app.apRefetchFollowers(blog); err != nil {
				app.logErrAndQuit("Failed to refetch ActivityPub followers", "blog", blog, "err", err)
			}
			app.shutdown.ShutdownAndWait()
		},
	})
	activityPubCmd.AddCommand(&cobra.Command{
		Use:   "move-followers blog target",
		Short: "Move all followers to a new Fediverse account by sending Move activities",
		Long: `Move all followers from the GoBlog ActivityPub account to a new Fediverse account.

This command sends a Move activity to all followers, instructing them to follow
the new account instead. Before running this command:

1. Set up the new account on the target Fediverse server
2. Add the GoBlog account URL to the new account's "alsoKnownAs" aliases
3. Run this command to initiate the move

Example:
  ./GoBlog activitypub move-followers default https://mastodon.social/users/newaccount`,
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			app := initializeApp(cmd)
			if !app.apEnabled() {
				app.logErrAndQuit("ActivityPub not enabled")
				return
			}
			if err := app.initActivityPubBase(); err != nil {
				app.logErrAndQuit("Failed to init ActivityPub base", "err", err)
				return
			}
			app.initAPSendQueue()
			blog := args[0]
			target := args[1]
			if err := app.apMoveFollowers(blog, target); err != nil {
				app.logErrAndQuit("Failed to move ActivityPub followers", "blog", blog, "target", target, "err", err)
			}
			app.shutdown.ShutdownAndWait()
		},
	})
	activityPubCmd.AddCommand(&cobra.Command{
		Use:   "clear-moved blog",
		Short: "Clear the movedTo setting for a blog after an account migration",
		Long: `Clear the movedTo setting for a blog's ActivityPub account.

After using move-followers to migrate followers to a new account, the blog's
ActivityPub profile will show "movedTo" pointing to the new account. Use this
command to clear that setting if you want to undo the migration or if you
accidentally set the wrong target.

Note: Clearing movedTo does not undo the Move activity that was already sent.
Followers who have already moved to follow the new account will not be
automatically moved back.

Example:
  ./GoBlog activitypub clear-moved default`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			app := initializeApp(cmd)
			if !app.apEnabled() {
				app.logErrAndQuit("ActivityPub not enabled")
				return
			}
			blog := args[0]
			if _, ok := app.cfg.Blogs[blog]; !ok {
				app.logErrAndQuit("Blog not found", "blog", blog)
				return
			}
			if err := app.deleteApMovedTo(blog); err != nil {
				app.logErrAndQuit("Failed to clear movedTo setting", "blog", blog, "err", err)
			}
			fmt.Printf("Cleared movedTo setting for blog %s\n", blog)
			app.shutdown.ShutdownAndWait()
		},
	})
	rootCmd.AddCommand(activityPubCmd)

	// Setup command for setting up user credentials
	setupCmd := &cobra.Command{
		Use:   "setup",
		Short: "Set up user credentials (username, password, and optionally TOTP)",
		Long: `Set up user credentials for GoBlog authentication.

This command allows you to configure the login credentials for your GoBlog
instance, including username, password, and optional TOTP two-factor
authentication. The password is securely hashed using bcrypt before storage.

This is useful for initial setup or when you need to reset credentials
without accessing the web interface.

Examples:
  ./GoBlog setup --username admin --password "secure-password"
  ./GoBlog setup --username admin --password "secure-password" --totp

Options:
  --username  Login username (required)
  --password  Login password, stored as bcrypt hash (required)
  --totp      Enable TOTP two-factor authentication`,
		Run: func(cmd *cobra.Command, args []string) {
			app := initializeApp(cmd)

			username, _ := cmd.Flags().GetString("username")
			password, _ := cmd.Flags().GetString("password")
			setupTOTP, _ := cmd.Flags().GetBool("totp")

			if username == "" || password == "" {
				fmt.Println("Error: --username and --password are required")
				app.shutdown.ShutdownAndWait()
				os.Exit(1)
			}

			// Update username
			if err := app.saveSettingValue(userNickSetting, username); err != nil {
				app.logErrAndQuit("Failed to save username", "err", err)
				return
			}
			app.cfg.User.Nick = username
			fmt.Println("Username set to:", username)

			// Set password
			if err := app.setPassword(password); err != nil {
				app.logErrAndQuit("Failed to set password", "err", err)
				return
			}
			fmt.Println("Password has been set (stored as secure hash)")

			// Setup TOTP if requested
			if setupTOTP {
				key, err := totp.Generate(totp.GenerateOpts{
					Issuer:      app.cfg.Server.PublicAddress,
					AccountName: username,
				})
				if err != nil {
					app.logErrAndQuit("Failed to generate TOTP secret", "err", err)
					return
				}
				if err := app.setTOTPSecret(key.Secret()); err != nil {
					app.logErrAndQuit("Failed to save TOTP secret", "err", err)
					return
				}
				fmt.Println("TOTP has been enabled")
				fmt.Println("TOTP Secret:", key.Secret())
				fmt.Println("Use this secret with your authenticator app (e.g., Google Authenticator, Authy)")
			}

			fmt.Println("\nSetup complete!")
			app.shutdown.ShutdownAndWait()
		},
	}
	setupCmd.Flags().String("username", "", "Login username (required)")
	setupCmd.Flags().String("password", "", "Login password (required)")
	setupCmd.Flags().Bool("totp", false, "Enable TOTP two-factor authentication")
	rootCmd.AddCommand(setupCmd)

	// Execute the root command
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func initializeApp(cmd *cobra.Command) *goBlog {
	app := &goBlog{
		httpClient: newHttpClient(),
	}
	configfile, _ := cmd.Flags().GetString("config")
	if err := app.loadConfigFile(configfile); err != nil {
		app.logErrAndQuit("Failed to load config file", "err", err)
		return nil
	}
	if err := app.initConfig(false); err != nil {
		app.logErrAndQuit("Failed to init config", "err", err)
		return nil
	}
	return app
}

func initializeComponents(app *goBlog) {
	app.info("Initialize components...")

	for _, f := range []func() error{
		app.initTemplateAssets, app.initTemplateStrings, app.initRegexRedirects,
		app.initHTTPLog, app.initActivityPub, app.initWebAuthn,
	} {
		if err := f(); err != nil {
			app.logErrAndQuit("Failed to initialize", "err", err)
			return
		}
	}
	for _, f := range []func(){
		app.initWebmention, app.initTelegram, app.initAtproto, app.initBlogStats,
		app.initTTS, app.initSessions, app.startPostsScheduler, app.initPostsDeleter,
		app.initIndexNow,
	} {
		f()
	}

	app.info("Initialized components")
}

func (a *goBlog) logErrAndQuit(msg string, args ...any) {
	a.error(msg, args...)
	a.shutdown.ShutdownAndWait()
	os.Exit(1)
}
