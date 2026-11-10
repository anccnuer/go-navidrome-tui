// Command ndtui is a terminal music player for Navidrome.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/anccnuer/go-navidrome-tui/internal/app"
	"github.com/anccnuer/go-navidrome-tui/internal/cover/sixel"
)

var version = "dev"

func main() {
	configPath := flag.String("config", "", "指定 config.toml 路径（也可设置 NAVIDROME_TUI_CONFIG）")
	debug := flag.Bool("debug", false, "启用 debug 日志（写入日志文件，不输出到终端）")
	showVersion := flag.Bool("version", false, "显示版本并退出")
	sixelCheck := flag.Bool("sixel-check", false, "输出一张 Sixel 测试图并打印终端能力信息，然后退出")
	flag.Parse()

	if *showVersion {
		fmt.Printf("navidrome-tui %s\n", version)
		return
	}
	if *sixelCheck {
		if err := sixel.SelfCheck(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "Sixel 自检失败：%v\n", err)
			os.Exit(1)
		}
		return
	}

	result, err := app.Build(app.Options{ConfigPath: *configPath, Debug: *debug})
	if err != nil {
		fmt.Fprintf(os.Stderr, "启动失败：%v\n", err)
		os.Exit(1)
	}
	defer result.Cleanup()

	signals := make(chan os.Signal, 1)
	// SIGHUP covers closing the terminal window on Unix; Windows has no
	// equivalent, so the mpv job object handles that case.
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-signals
		result.Program.Quit()
	}()

	if _, err := result.Program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "运行出错：%v\n", err)
		// os.Exit skips deferred functions, so clean up explicitly to make
		// sure mpv is terminated.
		result.Cleanup()
		os.Exit(1)
	}
}
