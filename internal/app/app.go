// Package app 是组合根：配置、日志、Fx 应用启动与退出。
// cmd/server/main.go 仅调用 app.Run()。
package app

import (
	"flag"
	"fmt"
	"log"

	"github.com/cng1985/nwatch/internal/platform/config"
	"github.com/cng1985/nwatch/internal/platform/version"
	"go.uber.org/fx"
)

// Run 解析启动定位参数、加载配置并运行应用。失败时输出原因并以非零码退出。
func Run() {
	paths, stop := parseFlags()
	if stop {
		return
	}
	paths, err := config.ResolvePaths(paths)
	if err != nil {
		log.Fatal(err)
	}
	cfg, err := config.Load(paths)
	if err != nil {
		log.Fatal(err)
	}
	application := fx.New(fx.NopLogger, fx.Supply(cfg), Module)
	application.Run()
	if err := application.Err(); err != nil {
		log.Fatal(err)
	}
}

// parseFlags 处理启动定位参数与 --version。属于项目应实现的契约：
// --base-dir/APP_BASE_DIR、--config-dir/APP_CONFIG_DIR、--data-dir/APP_DATA_DIR、--profile/APP_PROFILE。
func parseFlags() (config.Paths, bool) {
	var paths config.Paths
	var showVersion bool
	flag.StringVar(&paths.BaseDir, "base-dir", "", "基础目录，默认启动工作目录")
	flag.StringVar(&paths.ConfigDir, "config-dir", "", "配置目录，默认 configs，相对 base-dir")
	flag.StringVar(&paths.DataDir, "data-dir", "", "数据目录，默认 ./data，相对 base-dir")
	flag.StringVar(&paths.Profile, "profile", "", "运行环境，dev 或 pro；未指定时只加载基础配置")
	flag.BoolVar(&showVersion, "version", false, "输出版本后退出")
	flag.Parse()
	if showVersion {
		fmt.Printf("nwatch %s\n", version.Version)
		return config.Paths{}, true
	}
	return paths, false
}
