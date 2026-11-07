// Command shellvane is the Shellvane desktop application.
package main

import (
	"context"
	"embed"
	"github.com/csturiale/shellvane/core/plugins"
	"github.com/csturiale/shellvane/core/plugins/builtin"
	"github.com/csturiale/shellvane/core/shellenv"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/csturiale/shellvane/app"
	"github.com/csturiale/shellvane/core/audit"
	"github.com/csturiale/shellvane/core/automation"
	"github.com/csturiale/shellvane/core/biometric"
	"github.com/csturiale/shellvane/core/environments"
	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/hosts"
	"github.com/csturiale/shellvane/core/i18n"
	"github.com/csturiale/shellvane/core/keys"
	"github.com/csturiale/shellvane/core/knownhosts"
	"github.com/csturiale/shellvane/core/runbooks"
	"github.com/csturiale/shellvane/core/snippets"
	"github.com/csturiale/shellvane/core/store"
	"github.com/csturiale/shellvane/core/sync"
	"github.com/csturiale/shellvane/core/themes"
	"github.com/csturiale/shellvane/core/tunnel"
	"github.com/csturiale/shellvane/core/vault"
	"github.com/csturiale/shellvane/core/workspaces"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/tray.png
var trayIcon []byte

var serviceOptions = application.ServiceOptions{MarshalError: errs.Marshal}

func init() {
	// Registered events get typed TypeScript bindings.
	application.RegisterEvent[app.SessionState](app.EventSessionState)
	application.RegisterEvent[app.Prompt](app.EventSessionPrompt)
	application.RegisterEvent[string](app.EventSessionReturn)
	application.RegisterEvent[app.TriggerEvent](app.EventSessionTrigger)
	application.RegisterEvent[app.FileEdit](app.EventFileEdit)
	application.RegisterEvent[app.RunbookCell](app.EventRunbookCell)
	application.RegisterEvent[app.RunbookDone](app.EventRunbookDone)
	application.RegisterEvent[app.VaultStatus](app.EventVaultState)
	application.RegisterEvent[app.TunnelStatus](app.EventTunnelState)
	application.RegisterEvent[app.TransferStatus](app.EventTransfer)
	application.RegisterEvent[app.DroppedFiles](app.EventFilesDropped)
	application.RegisterEvent[app.OpenRequest](app.EventOpen)
	application.RegisterEvent[sync.Report](app.EventSyncDone)
}

func main() {
	// Apps opened from the Finder/Dock miss the login shell's PATH, where
	// kubectl, gcloud, aws and their credential plugins live.
	shellenv.Start(context.Background())
	dbPath, err := store.DefaultPath()
	if err != nil {
		log.Fatalf("locate data directory: %v", err)
	}
	db, err := store.Open(context.Background(), dbPath)
	if err != nil {
		log.Fatalf("open database %s: %v", dbPath, err)
	}
	defer db.Close()

	if os.Getenv(app.RestartEnv) != "" {
		os.Unsetenv(app.RestartEnv)
		app.WaitForPreviousInstance(filepath.Dir(dbPath), 15*time.Second)
	}

	appService := app.NewAppService(i18n.FromEnv())
	repo := hosts.New(db)
	known := knownhosts.New(filepath.Join(filepath.Dir(dbPath), "known_hosts"), knownhosts.DefaultUserFiles()...)
	// The emitter is bound once the application exists.
	var emit app.Emitter = func(string, any) {}
	emitter := func(name string, data any) { emit(name, data) }
	vaultStore := vault.New(db, vault.DefaultKDF)
	vaultService := app.NewVaultService(vaultStore, emitter)
	pluginService := app.NewPluginService(plugins.NewRegistry(builtin.All()...), plugins.NewConfigs(db), repo, vaultStore, filepath.Join(filepath.Dir(dbPath), "plugins"))
	keyRepo := keys.NewRepo(db, vaultStore)
	sessions := app.NewSessionService(repo, known, vaultService, keyRepo, emitter)
	app.EnablePanels(pluginService, sessions)
	snippetService, themeService := app.NewSnippetService(snippets.New(db)), app.NewThemeService(themes.NewRepo(db))
	rules, auditLog := automation.NewRepo(db), audit.New(db)
	app.EnableAutomation(sessions, rules, auditLog)
	app.EnableLocal(sessions, filepath.Dir(dbPath))
	filesService := app.NewFilesService(sessions, emitter)
	tunnelService := app.NewTunnelService(tunnel.NewRepo(db), sessions, emitter)
	control := app.NewControl(repo, tunnelService, emitter)
	dataDir := filepath.Dir(dbPath)

	// Features of the official builds plug in here (none in the community
	// edition); they may route host credentials to their own brokers.
	brokers := map[string]app.Broker{}
	extensionServices := app.ExtensionServices(app.ExtensionDeps{
		DB: db, Vault: vaultStore, Sessions: sessions, DataDir: dataDir, Emit: emitter, Options: serviceOptions,
		Hosts: repo, Environments: environments.New(db), Tunnels: tunnel.NewRepo(db), Snippets: snippets.New(db), Workspaces: workspaces.New(db), Rules: rules,
		AddBroker: func(prefix string, b app.Broker) { brokers[prefix] = b },
	})
	app.EnableBrokers(sessions, app.RouteBrokers(app.PluginBroker(pluginService), brokers))

	wailsApp := application.New(application.Options{
		Name:        "Shellvane",
		Description: "SSH, SFTP and tunnel manager",
		// Errors cross the bridge as {code, params, detail} and are translated
		// by the frontend. Wails applies the app-level MarshalError only
		// outside service methods, so each service gets it too.
		Services: append([]application.Service{
			application.NewServiceWithOptions(appService, serviceOptions),
			application.NewServiceWithOptions(app.NewHostService(repo), serviceOptions),
			application.NewServiceWithOptions(sessions, serviceOptions),
			application.NewServiceWithOptions(vaultService, serviceOptions),
			application.NewServiceWithOptions(snippetService, serviceOptions),
			application.NewServiceWithOptions(tunnelService, serviceOptions),
			application.NewServiceWithOptions(app.NewControlService(control), serviceOptions),
			application.NewServiceWithOptions(filesService, serviceOptions),
			application.NewServiceWithOptions(app.NewKeyService(keyRepo, sessions), serviceOptions),
			application.NewServiceWithOptions(app.NewAutomationService(rules, auditLog), serviceOptions),
			application.NewServiceWithOptions(app.NewRunbookService(runbooks.NewRepo(db), repo, sessions, auditLog, emitter), serviceOptions),
			application.NewServiceWithOptions(app.NewWorkspaceService(workspaces.New(db)), serviceOptions),
			application.NewServiceWithOptions(app.NewEnvironmentService(environments.New(db), repo, tunnel.NewRepo(db), workspaces.New(db)), serviceOptions),
			application.NewServiceWithOptions(app.NewUpdateService(), serviceOptions),
			application.NewServiceWithOptions(app.NewSyncService(db, vaultStore, dataDir, emitter), serviceOptions),
			application.NewServiceWithOptions(app.NewLaunchService(sessions, tunnelService), serviceOptions),
			application.NewServiceWithOptions(app.NewMonitorService(sessions), serviceOptions),
			application.NewServiceWithOptions(themeService, serviceOptions),
			application.NewServiceWithOptions(app.NewGalleryService(snippetService, themeService), serviceOptions),
			application.NewServiceWithOptions(app.NewUsageService(), serviceOptions),
			application.NewServiceWithOptions(app.NewDiagnosticsService(sessions, repo, filepath.Dir(dbPath)), serviceOptions),
			application.NewServiceWithOptions(app.NewImportService(db, repo, tunnel.NewRepo(db), filepath.Join(filepath.Dir(dbPath), "backups")), serviceOptions),
			application.NewServiceWithOptions(app.NewDownloadsService(os.Getenv("SHELLVANE_DOWNLOADS")), serviceOptions),
			application.NewServiceWithOptions(pluginService, serviceOptions),
		}, extensionServices...),
		MarshalError: errs.Marshal,
		// A second launch (a link clicked while Shellvane runs) hands its
		// arguments to this instance.
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "io.github.csturiale.shellvane",
			OnSecondInstanceLaunch: func(d application.SecondInstanceData) {
				control.Show()
				control.OpenAll(context.Background(), d.Args)
			},
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
			// Pages of other sites (the sign-in window) cannot call the app.
			Middleware: app.RejectForeignOrigins,
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	emit = func(name string, data any) { wailsApp.Event.Emit(name, data) }
	wailsApp.HandleStream("terminal", app.TerminalStream(sessions))

	window := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Shellvane",
		Width:            1280,
		Height:           800,
		MinWidth:         800,
		MinHeight:        500,
		BackgroundColour: application.NewRGB(24, 24, 27),
		URL:              "/",
		// Files dropped on elements marked data-file-drop-target are uploaded.
		EnableFileDrop: true,
	})
	window.OnWindowEvent(events.Common.WindowFilesDropped, func(e *application.WindowEvent) {
		target := ""
		if d := e.Context().DropTargetDetails(); d != nil {
			target = d.ElementID
		}
		emit(app.EventFilesDropped, app.DroppedFiles{Files: e.Context().DroppedFiles(), Target: target})
	})

	// Tabs moved out of the main window open in windows of their own.
	app.EnableDetach(sessions, func(u string) {
		log.Printf("opening a window for moved tabs: %s", u)
		wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
			Title:            "Shellvane",
			Width:            1000,
			Height:           650,
			MinWidth:         500,
			MinHeight:        300,
			BackgroundColour: application.NewRGB(24, 24, 27),
			URL:              u,
		})
	})
	control.Show = func() {
		window.Show()
		window.Focus()
	}
	// Touch ID / Windows Hello confirm the keychain unlock, with a translated reason.
	app.EnableBiometric(vaultService, biometric.System(), func() string {
		return app.TranslatorOf(appService).T("vault.biometricReason", nil)
	})
	tray := app.NewTray(repo, tunnelService, vaultService, control, app.TranslatorOf(appService))
	appService.OnLanguageChange = tray.SetTranslator
	tray.Start(wailsApp, trayIcon)
	// Links opened through the system (ssh:// and sftp:// handlers).
	wailsApp.Event.OnApplicationEvent(events.Common.ApplicationLaunchedWithUrl, func(e *application.ApplicationEvent) {
		control.OpenAll(context.Background(), []string{e.Context().URL()})
	})
	control.OpenAll(context.Background(), os.Args[1:])
	// The CLI talks to this instance through the control channel.
	if err := control.Listen(dataDir); err != nil {
		log.Printf("control channel: %v", err)
	}
	defer control.Close()

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}
