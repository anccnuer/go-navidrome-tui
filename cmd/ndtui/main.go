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
	sixelCheck := flag.Bool("sixel-check", false, "探测终端能力并输出两张 Sixel 对照图，然后退出")
	sixelDump := flag.String("sixel-dump", "", "把自检图的 Sixel 载荷写入指定文件（便于与 chafa -f sixel 对比），然后退出")
	flag.Parse()

	if *showVersion {
		fmt.Printf("navidrome-tui %s\n", version)
		return
	}
	if *sixelDump != "" {
		if err := dumpSixel(*sixelDump); err != nil {
			fmt.Fprintf(os.Stderr, "写出 Sixel 载荷失败：%v\n", err)
			os.Exit(1)
		}
		return
	}
	if *sixelCheck {
		// The query/response handshake must happen before anything else is
		// printed: the replies arrive on stdin and are only readable in raw
		// mode, which this standalone command owns for its whole lifetime.
		probe := sixel.QueryTerminal(os.Stdout, os.Stdin, sixel.DefaultProbeTimeout)
		if err := sixel.SelfCheck(os.Stdout, version, &probe); err != nil {
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

// dumpSixel writes the self-check payload to a file. Comparing it byte for
// byte with `chafa -f sixel` output is the quickest way to tell an encoder bug
// from a terminal that refuses our DCS framing.
func dumpSixel(path string) error {
	payload, err := sixel.Encode(sixel.GradientImage(240, 120), 240, 120)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		return err
	}
	fmt.Printf("已写出 %d 字节的 Sixel 载荷：%s\n", len(payload), path)
	fmt.Println("对比方法（同一个终端窗口里执行）：")
	fmt.Println(`  chafa -f sixel -s 30x15 <某张 png> > chafa.sixel`)
	fmt.Println("  再看 ndtui --sixel-check 里的两张对照图，判断问题出在终端还是编码器。")
	return nil
}
