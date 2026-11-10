// Package tests 承载跨模块与兼容性测试：通过完整装配的业务模块走 HTTP 验证行为。
package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/cng1985/nwatch/internal/modules/admin/access"
	adminbackup "github.com/cng1985/nwatch/internal/modules/admin/backup"
	adminlogview "github.com/cng1985/nwatch/internal/modules/admin/logview"
	adminsettings "github.com/cng1985/nwatch/internal/modules/admin/settings"
	"github.com/cng1985/nwatch/internal/modules/jobs/cronjob"
	"github.com/cng1985/nwatch/internal/modules/monitoring/alert"
	"github.com/cng1985/nwatch/internal/modules/monitoring/checker"
	"github.com/cng1985/nwatch/internal/modules/monitoring/dashboard"
	"github.com/cng1985/nwatch/internal/modules/monitoring/group"
	"github.com/cng1985/nwatch/internal/modules/monitoring/host"
	"github.com/cng1985/nwatch/internal/modules/monitoring/metric"
	"github.com/cng1985/nwatch/internal/modules/monitoring/monitor"
	"github.com/cng1985/nwatch/internal/modules/monitoring/scheduler"
	"github.com/cng1985/nwatch/internal/modules/monitoring/script"
	"github.com/cng1985/nwatch/internal/modules/monitoring/state"
	"github.com/cng1985/nwatch/internal/modules/notify/mailer"
	"github.com/cng1985/nwatch/internal/modules/notify/notifier"
	"github.com/cng1985/nwatch/internal/modules/ops/docker"
	"github.com/cng1985/nwatch/internal/modules/ops/kube"
	"github.com/cng1985/nwatch/internal/modules/ops/service"
	"github.com/cng1985/nwatch/internal/modules/ops/shell"
	"github.com/cng1985/nwatch/internal/platform/auth"
	"github.com/cng1985/nwatch/internal/platform/config"
	"github.com/cng1985/nwatch/internal/platform/logging"
	"github.com/cng1985/nwatch/internal/platform/routing"
	"github.com/cng1985/nwatch/internal/platform/server"
	"github.com/cng1985/nwatch/internal/platform/settings"
	"github.com/cng1985/nwatch/tests/harness"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// testServer 手工装配全部业务模块，验证组合根之外的同一套构造函数。
type testServer struct {
	engine    *gin.Engine
	cfg       *config.Config
	db        *gorm.DB
	store     *settings.Store
	tokens    *auth.Service
	users     *auth.Store
	logs      *logging.Store
	collector *host.Collector
	alerts    *alert.Manager
	mail      *mailer.Service
	pool      *scheduler.Pool
	cron      *cronjob.Service
	shellHub  *shell.Hub
	services  *service.Manager
	docker    *docker.Manager
	kube      *kube.Manager
}

// testOptions 允许测试注入运维适配器的命令执行替身与终端开关。
type testOptions struct {
	serviceRunner   service.Runner
	serviceDisabled bool
	dockerRunner    docker.Runner
	dockerDisabled  bool
	kubeRunner      kube.Runner
	kubeDisabled    bool
	shellDisabled   bool
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	return newTestServerWithOptions(t, testOptions{})
}

func newTestServerWithOptions(t *testing.T, opts testOptions) *testServer {
	t.Helper()
	cfg := harness.NewConfig(t)
	cfg.Database.Path = filepath.Join(cfg.App.DataDir, "nwatch.db")
	db := harness.OpenDB(t, cfg)

	store, err := settings.New(db, cfg)
	if err != nil {
		t.Fatal(err)
	}
	users := auth.NewStore(db)
	if err := users.Ensure(cfg, store); err != nil {
		t.Fatal(err)
	}
	tokens, err := auth.New(cfg, users)
	if err != nil {
		t.Fatal(err)
	}
	logs := logging.NewStore()
	logs.Bind(db)
	collector := host.NewCollector()
	registry := notifier.NewRegistry(
		notifier.NewDingTalk(store), notifier.NewWeCom(store), notifier.NewWebhook(),
	)
	alerts := alert.NewManager(db, registry, nil)
	alerts.Start(1)
	t.Cleanup(func() { alerts.Stop(context.Background()) })
	mail := mailer.New(db, store, nil)
	mail.Start()
	t.Cleanup(func() { mail.Stop(context.Background()) })
	checkers := checker.NewRegistry(
		checker.NewHTTPChecker(), checker.NewTLSChecker(), checker.NewTCPChecker(),
		checker.NewCPUChecker(collector), checker.NewMemoryChecker(collector), checker.NewDiskChecker(), checker.NewScriptChecker(), checker.NewProcessChecker(collector),
	)
	processor := scheduler.NewProcessor(db, checkers, state.NewEngine(), alerts, metric.NewAggregator(), store, mail)
	pool := scheduler.NewPool(2, 8, processor, nil)
	pool.Start()
	t.Cleanup(func() { pool.Stop(context.Background()) })
	sched := scheduler.NewScheduler(time.Hour, db, pool, nil)
	sched.Start()
	t.Cleanup(sched.Stop)
	cron := cronjob.New(db, store, time.Hour, nil)
	t.Cleanup(cron.Stop)

	shellHub := shell.NewHub(shell.Options{Enabled: !opts.shellDisabled, IdleTimeout: time.Minute, MaxLifetime: time.Hour, MaxSessions: 4})
	t.Cleanup(shellHub.Close)
	serviceEnabled := !opts.serviceDisabled
	services := service.New(serviceEnabled)
	if opts.serviceRunner != nil {
		services = service.NewWithRunner(serviceEnabled, opts.serviceRunner)
	}
	dockerEnabled := !opts.dockerDisabled
	dockerMgr := docker.New(dockerEnabled)
	if opts.dockerRunner != nil {
		dockerMgr = docker.NewWithRunner(dockerEnabled, opts.dockerRunner)
	}
	kubeMgr := kube.New(kube.Options{Enabled: !opts.kubeDisabled, Context: "lab"})
	if opts.kubeRunner != nil {
		kubeMgr = kube.NewWithRunner(kube.Options{Enabled: !opts.kubeDisabled, Context: "lab"}, opts.kubeRunner)
	}

	registrars := []routing.Registrar{
		access.NewController(access.NewResource(cfg, tokens, store)),
		monitor.NewController(monitor.NewResource(monitor.NewRepository(db), store, pool, processor)),
		group.NewController(group.NewResource(group.NewRepository(db))),
		dashboard.NewController(dashboard.NewResource(db)),
		host.NewController(host.NewResource(collector, db)),
		script.NewController(script.NewResource(script.NewRepository(db))),
		notifier.NewController(notifier.NewResource(notifier.NewRepository(db), registry)),
		cronjob.NewController(cronjob.NewResource(cronjob.NewRepository(db), cron, store)),
		shell.NewController(shell.NewResource(shell.NewRepository(db), shellHub), shellHub),
		service.NewController(service.NewResource(services)),
		docker.NewController(docker.NewResource(dockerMgr)),
		kube.NewController(kube.NewResource(kubeMgr)),
		adminsettings.NewController(adminsettings.NewResource(cfg, store, mail, cron)),
		adminbackup.NewController(adminbackup.NewResource(cfg, db, store, cron)),
		adminlogview.NewController(adminlogview.NewResource(db)),
	}

	params := server.Params{}
	params.Cfg = cfg
	params.DB = db
	params.Tokens = tokens
	params.Registrars = registrars
	srv := server.NewServer(params, nil)
	t.Cleanup(shellHub.Close)

	return &testServer{
		engine:    srv.Engine(),
		cfg:       cfg,
		db:        db,
		store:     store,
		tokens:    tokens,
		users:     users,
		logs:      logs,
		collector: collector,
		alerts:    alerts,
		mail:      mail,
		pool:      pool,
		cron:      cron,
		shellHub:  shellHub,
		services:  services,
		docker:    dockerMgr,
		kube:      kubeMgr,
	}
}

func (s *testServer) serve(rec *httptest.ResponseRecorder, req *http.Request) {
	s.engine.ServeHTTP(rec, req)
}
