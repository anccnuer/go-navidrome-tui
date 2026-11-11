# go-navidrome-tui

**简体中文** | [English](./README.en.md)

[![Go](https://img.shields.io/github/go-mod/go-version/anccnuer/go-navidrome-tui?logo=go)](go.mod)
[![Platform](https://img.shields.io/badge/platform-Windows%20%7C%20Linux%20%7C%20macOS-4c566a)](#安装与运行)
[![mpv](https://img.shields.io/badge/audio-mpv-orange)](https://mpv.io/)
[![License: MIT](https://img.shields.io/badge/license-MIT-green)](#许可)
[![Stars](https://img.shields.io/github/stars/anccnuer/go-navidrome-tui?style=social)](https://github.com/anccnuer/go-navidrome-tui)

在终端里听 Navidrome 上的歌。界面参考了 [go-musicfox](https://github.com/go-musicfox/go-musicfox)，声音交给 [mpv](https://mpv.io/) 播放——解码和格式支持都由它负责，这个程序只管界面和队列，所以很轻。

跑起来需要三样东西：一个能连上的 Navidrome 服务、装好的 mpv、一个支持 24 位色的终端（Windows Terminal、Kitty、iTerm2、Alacritty 都行；Windows CMD 和 macOS 自带终端不行）。

## 长什么样

```
 » 主页 / 全部歌曲                navidrome-tui · [3/120]                 ● 已连接   home

                             ← 全部歌曲 · 128 首

                             =>   1. Hysteria           Muse
                                2. Absolution         Muse
                                3. Time Is Running Out  Muse
                                4. Plug In Baby       Muse
                                ...（仅渲染视口内可见行）

                             ↑↓/jk 导航  Enter 播放  b/Esc 返回  / 过滤

                             Hysteria
                             Muse · Absolution

 [顺序播放] 80% ♫ ♪ ♫ ♪ ♥ Hysteria  Muse
 ####################........................ 00:42/03:45
```

最上面是状态栏（当前位置、连接状态、服务器名），中间是菜单，底部两行是播放条。按 `z` 可以切到全屏播放页：左边封面和歌名，右边滚动歌词。

## 能做什么

- 浏览只有两个入口：**全部歌曲** 和 **歌单**。艺术家、专辑、流派这些没做，用不上。
- `Ctrl+F` 搜全库，`/` 在当前列表里过滤。
- 播放、暂停、上下曲、快进快退、音量、四种播放模式（顺序 / 列表循环 / 单曲循环 / 随机）、队列增删和下一曲播放。
- 同步歌词、半块字符画的专辑封面、喜欢 / 加入歌单、浏览器打开、主题切换、帮助页、快捷键自定义。
- `z` 打开全屏播放页，页内 `h/l` 快退快进、`j/k` 调音量。
- Last.fm scrobble 和 Navidrome 播放计数上报，默认关闭，要自己填凭据。
- 鼠标能点输入框、列表行，双击播放，点进度条跳转，滚轮翻页。

窗口窄了会自动收成单列；没装 mpv、断网、没歌词都会给提示或占位，不会白屏。

## 安装与运行

### 先装 mpv

- Windows：从 [mpv.io](https://mpv.io/installation/) 下载，或 `scoop install mpv` / `winget install mpv`。不想改 PATH 就在配置里写 `player.mpvPath` 的完整路径。
- Linux / macOS：`apt install mpv` / `brew install mpv`。

终端用等宽字体，中文最好也用等宽 CJK 字体，不然排版会歪。

### 编译

需要 Go 1.26 或更新。

```bash
git clone https://github.com/anccnuer/go-navidrome-tui.git
cd go-navidrome-tui
make build     # Windows 得到 bin/ndtui.exe，其它平台 bin/ndtui
make run       # 直接跑
make test      # 跑测试
```

不想用 make 也行（PowerShell）：

```powershell
go build -o bin/ndtui.exe ./cmd/ndtui
.\bin\ndtui.exe
```

Linux / macOS：

```bash
go build -o bin/ndtui ./cmd/ndtui
./bin/ndtui
```

| 参数 | 作用 |
| --- | --- |
| `--config <path>` | 指定配置文件（同 `NAVIDROME_TUI_CONFIG`） |
| `--debug` | 开 debug 日志，只写文件，不刷屏 |
| `--version` | 打印版本 |
| `--sixel-check` | 探测终端（DA1 / 字符格像素）并输出两张 Sixel 对照图，然后退出 |
| `--sixel-dump <file>` | 把自检图的 Sixel 载荷写入文件，便于与 `chafa -f sixel` 对比 |

## 想先试试

官方有个公开的 [demo 实例](https://demo.navidrome.org)，账号密码都是 `demo`。拿它试浏览、搜索和播放就好，别在里面收藏、改歌单。

| 项 | 值 |
| --- | --- |
| 地址 | `https://demo.navidrome.org`（写 `demo.navidrome.org` 也行，会补 https） |
| 账号 / 密码 | `demo` / `demo` |

不想填向导可以用环境变量（PowerShell）：

```powershell
$env:NAVIDROME_TUI_URL='https://demo.navidrome.org'
$env:NAVIDROME_TUI_USER='demo'
$env:NAVIDROME_TUI_PASSWORD='demo'
.\bin\ndtui.exe
```

这个实例经常限流，连不上等一会儿再试；向导超时会自动重试一次。仓库里还有个默认跳过的只读联调测试：

```bash
NAVIDROME_TUI_DEMO_TEST=1 go test ./internal/subsonic -run TestPublicDemoReadOnly -v
```

## 配置

第一次启动会弹向导，填名称、地址、用户名、密码，回车会先试连一下再保存。输入框可以用鼠标点，地址直接粘贴即可（首尾空格、全角字符会清理，不写协议会自动补）。向导里 `Ctrl+C` 退出；主界面按 `w` 重新登录时 `Esc` 取消。

配置文件位置：

| 平台 | 路径 |
| --- | --- |
| Windows | `%AppData%\navidrome-tui\config.toml` |
| Linux | `$XDG_CONFIG_HOME/navidrome-tui/config.toml` 或 `~/.config/navidrome-tui/config.toml` |
| macOS | `~/Library/Application Support/navidrome-tui/config.toml` |

从旧版本（`subsonic-tui`）升级不用重新登录：新目录没有配置时会继续读旧目录的，旧的 `SUBSONIC_TUI_*` 环境变量也还能用，新名优先。

完整配置项看 [`config.example.toml`](./config.example.toml)。几个常用点：

- `[[servers]]` 可以写多份，用 `active` 选当前那个。
- 密码存成 `enc:<hex>`，只是不让人一眼看到，**不是加密**。不想落盘就别写 `password`，用环境变量：

```bash
NAVIDROME_TUI_URL=https://music.example.com \
NAVIDROME_TUI_USER=alice \
NAVIDROME_TUI_PASSWORD=secret \
./bin/ndtui
```

其它环境变量：`NAVIDROME_TUI_CONFIG`、`NAVIDROME_TUI_THEME`、`NAVIDROME_TUI_MPV`、`NAVIDROME_TUI_LOG_LEVEL`、`NAVIDROME_TUI_SERVER_NAME`。

- 除内置导航键外，快捷键都能在 `[keybindings.app]` 里改，见 [`docs/shortcuts.md`](./docs/shortcuts.md)。`Ctrl+T` 切主题，内置 `musicfox`（默认，网易红）、`aurora`、`dracula`、`nord`、`gruvbox`、`mono`，也能用 `theme.customPath` 指向自己的 TOML 色板。
- `[lastfm]` 默认关闭，要填 `apiKey` / `apiSecret` / `sessionKey`。播放过半或满 4 分钟才上报，`scrobbleAt = 0` 表示只在播完时上报。

## 快捷键

内置导航键固定，其它都能在 `[keybindings.app]` 里改。输入框激活时按键是文字，搜索页 / 帮助页的 `Esc` 会返回上一视图，过滤框的 `Esc` 只关过滤；`q` 或 `Ctrl+C` 随时能退出。`c` / `y` / `F2` / `z` 再按一次返回，全屏播放页 `Esc` / `b` / `z` 都能退出来。

| 键 | 操作 | 键 | 操作 |
| --- | --- | --- | --- |
| `j` `k` / `↑` `↓` | 上下移动（双列时同列：1→3→5、2→4→6） | `Space` | 播放 / 暂停 |
| `h` `l` / `←` `→` | 左右移动（双列时同行：1↔2；单列时返回 / 进入） | `[` `]` | 上一首 / 下一首 |
| `g` `G` | 顶部 / 底部 | `-` `=` | 音量减 / 加 |
| `Enter` `n` | 进入 / 播放 | `x` `X` `v` `V` | 快退 1/5 秒、快进 5/10 秒 |
| `b` `Esc` | 返回 | `p` | 切换播放模式 |
| `/` | 过滤当前列表 | `Ctrl+F` | 全库搜索 |
| `c` | 播放队列 | `y` | 同步歌词 |
| `?` | 帮助 | `F2` | 设置 |
| `z` | 全屏正在播放（再按返回） |  | 进度条点击可跳转 |
| `e` `E` | 下一曲播放 / 追加末尾 | `\` | 从队列删除 |
| `,` `.` | 喜欢 / 取消喜欢（播放中） | `<` `>` | 喜欢 / 取消喜欢（选中） |
| `Tab` | 加入歌单 | `O` | 浏览器打开选中项 |
| `f` | 播放中歌曲的相似歌曲 | `Ctrl+T` | 切换主题 |
| `u` | 清缓存 | `w` | 切换服务器 |
| `q` | 退出 | `r` | 重新渲染 |

鼠标默认开着；如果终端把鼠标事件解析成乱字符，在 `[ui]` 里设 `enableMouse = false`，键盘一样能完成所有操作（终端高度小于 14 行时进度条不能点击跳转）。

## 为什么不太卡

- 网络请求、封面解码、mpv 命令都放后台跑，不会卡住界面。
- 列表只画屏幕上看得见的行，滚动到底才拉下一页。
- 歌曲、歌单、搜索、歌词、封面都有缓存，样式和渐变也是预先算好的。
- mpv 用事件通知，界面每 500ms 才刷一次，不会每帧重画整屏。

日志默认只写 `warn` 级别到文件，URL 里的 token、salt、密码都会脱敏。

## 封面：半块字符还是 Sixel

默认用半块字符画封面：每个字符格是 `▀`，前景色是上半个像素、背景色是下半个像素。不依赖任何图形协议，SSH、tmux、16 色终端都能显示。

如果你的终端支持 Sixel（Windows Terminal 1.22+、mlterm、foot、wezterm 等），先在**真实终端**里按下面的顺序验一遍：

```bash
chafa -f sixel 某张图.png     # 第 1 步：自证终端。注意默认的 chafa 图片 用的是半块字符
ndtui --sixel-check           # 第 2 步：自证程序
```

`--sixel-check` 会做三件事：

1. **真探测**：向终端发 DA1（`CSI c`，属性 4 = Sixel）与字符格像素查询（`CSI 16 t`），带超时读应答，打印原始应答与实测字符格尺寸。
2. **两张对照图**：图 1 是手写的 12×6 品红色块（公认正确的常量），图 2 是程序编码器生成的 240×120 渐变图。两张都出＝终端与编码器都正常；只出品红块＝编码器输出被拒（用 `ndtui --sixel-dump sixel.txt` 导出载荷和 chafa 对比）；两张都不出＝当前窗口不渲染 Sixel。
3. **版本号**：第一行就写着版本，避免拿旧 exe 的结果排查新问题。

确认能出图后，在配置里开启（`sixelCellWidth/Height` 按自检报告的实测字符格像素填）：

```toml
[ui]
coverProtocol = "sixel"
sixelCellWidth = 8      # 一个字符格多少像素，按字体调
sixelCellHeight = 16
sixelColors = 256       # 调色板颜色数，2-256
```

调色板不是固定的 4×4×4 色立方，而是按封面自身颜色做中位切分选出最多 256 色，再做 Floyd-Steinberg 抖动；Sixel 载荷按 6 像素带逐色 RLE 压缩，末尾的透明像素直接省略。所以 200×200 的封面包通常只有 30 KB 左右，肤色和渐变不会被压成几块色斑。

Sixel 不能塞进每帧重绘的文本里（Bubble Tea 会截断、跳过或擦掉它），所以程序走带外通道：在文本帧写完后按屏幕坐标单独发送图形，播放页给封面留一块固定的空白区。图片带缓存，只在切歌、缩放、进出播放页，或歌词滚动让封面所在的行被文本重绘时重发；进度条、状态栏这类与封面无关的 500ms 刷新不会重复传输。短控制序列（切换备用屏幕、隐藏光标等）不会提前消费这张图。

几个实话实说的限制：

- **默认是 `text`**，`coverProtocol = "sixel"` 是显式强制开启。终端环境变量只作日志提示，不再偷偷改回 `text`：`TERM=dumb`、没设 `WT_SESSION`、SSH、tmux 里也按配置发 DCS。开启前务必先跑 `--sixel-check`；只有 stdout 被重定向到普通文件，或图片编码失败时才会回退到半块字符。
- `WT_SESSION`、`TERM`、`COLORTERM` 这些环境变量**只作参考**：nushell 等 shell 常常不转发 `WT_SESSION`，IDE 内置终端（xterm.js）根本不支持 Sixel，`TERM` 在原生 Windows 进程里通常为空。别用它们下结论，用 `--sixel-check` 的 DA1 探测和两张对照图。
- 编码器发送 `ESC P 0;1;0q`，与 Chafa / libsixel 一样把 `P2` 设为 1（零位透明）。裸 `ESC P q` 默认 `P2=0`，同一像素带里的多个颜色层会互相用背景色擦除，在 Windows Terminal 上表现为几行空白而不是图片。
- Windows Terminal 1.22+ 画完 Sixel 会把光标移到图像下方（xterm/foot/WezTerm 留在起始行）。程序在 DCS 前后加了 `ESC 7` / `ESC 8` 保存恢复光标，否则 Bubble Tea 的相对光标移动会失步、把图片覆盖掉。
- Bubble Tea 按整行重绘：歌词滚动会连带重写封面所在的那些行，图片必须重发。为了不让"擦掉→重画"被看见，文本帧和 DCS 载荷放进同一个写操作，并用 DECSET 2026（同步输出）包成一个原子帧；Windows Terminal 1.23.20211.0 起支持该模式，不支持的终端会忽略它、行为与以前一致。
- Sixel 没有"删除单张图"的指令。离开播放页时靠清屏重绘擦掉；歌词滚动时被文本重绘擦到的部分会自动重发补回，个别终端仍显示不全时按 `r` 可强制重绘，或把 `coverProtocol` 改回 `text`。
- 图片比例不对就调 `sixelCellWidth` / `sixelCellHeight`。

## 常见问题

| 现象 | 怎么办 |
| --- | --- |
| 底部写着「未找到 mpv」 | 装 mpv 并加进 PATH，或配 `player.mpvPath`。浏览还能用，只是不能播。 |
| 退出后还有声音 | 新版会随程序一起结束 mpv（Windows 用 Job Object，Linux 用 Pdeathsig，macOS 靠 SIGHUP）。如果还有残留，多半是旧版本留下的，手动结束一次即可。 |
| 连不上 / 401 | 检查地址有没有 `http(s)://`、端口、账号密码；自签证书设 `insecure = true`。 |
| 连接超时 | 向导会重试一次；demo 实例常有负载限制，等会儿再试或换自己的 Navidrome。 |
| 鼠标点击没反应 | 确认 `[ui] enableMouse` 不是 `false`（旧配置里可能是），并用支持鼠标报告的终端。 |
| 歌词一直是空的 | Navidrome 需要嵌入歌词或同名 `.lrc`；程序优先用 OpenSubsonic `getLyricsBySongId`，没有再回退 `getLyrics`。 |
| 封面是一堆色块 | 本来就是半块字符画的。开 `ui.trueColor = true` 看真彩，关掉会降到 256 色；想要真正的图片可以试试 Sixel（见上一节）。 |
| 开了 Sixel 仍是色块 | 先 `chafa -f sixel 图.png` 自证终端（默认 chafa 是半块字符，有图也不算数），再 `ndtui --sixel-check` 看两张对照图。`sixelCellWidth/Height` 按自检报告的实测字符格填。 |
| 自检只看到品红块、看不到渐变图 | 终端本身支持 Sixel，是本程序的载荷被拒。用 `ndtui --sixel-dump sixel.txt` 导出载荷，与 `chafa -f sixel` 的输出对比后反馈。 |
| 自检两张图都没有 | 当前窗口不渲染 Sixel：conhost、VS Code / CodeBuddy 等 IDE 内置终端（xterm.js）、SSH/tmux 会话都属于这类；换 Windows Terminal 1.22+ 的标签页再试。 |
| 排版歪了 | 换等宽字体（中文也要等宽）；宽屏双列可在配置里设 `ui.doubleColumn = false`。 |
| 按键没反应 | 先看是不是在输入框里，`Esc` 退出；`F2` 设置页能看到快捷键冲突数。 |
| 帮助页里 `q` 不退出 | 帮助页默认在过滤输入状态，按 `Esc` 返回，或 `Ctrl+C` 退出。 |
| 所有键突然都没反应 | 先试 `q` / `Ctrl+C`。能退出说明只是当前页面的按键语义（全屏播放页 `h/l/j/k` 是快进快退和音量，会有提示）；都不行就是终端没把按键送进来，试试 `[ui] enableMouse = false` 或换个终端。 |

## 开发

```bash
make test     # go test ./...
make lint     # go vet ./...
make tidy     # go mod tidy
make windows linux darwin   # 交叉编译，Windows 上需要 Git Bash / MSYS / WSL
```

发版不是每次 push 都跑。给版本打 `v*` tag 才会触发 `.github/workflows/release.yml`：先跑测试，再交叉编译 Windows / Linux / macOS 共 5 个平台，打包并附 `SHA256SUMS` 发到 GitHub Release。

```bash
git tag v1.0.0
git push origin v1.0.0
```

也可以在 Actions 页面手动运行 Release 工作流，填入一个已存在的 tag 补发。

目录结构：

```
cmd/ndtui/               入口
internal/app/            依赖装配与优雅退出
internal/config/         TOML 配置、路径、原子保存
internal/subsonic/       (Open)Subsonic REST 客户端与 DTO
internal/model/          领域模型与枚举
internal/cache/          泛型 TTL + LRU
internal/player/         mpv JSON IPC 与播放队列状态机
internal/lyrics/         LRC / 结构化歌词解析
internal/cover/          图片解码与半块字符渲染
internal/keymap/         快捷键解析、冲突检测、帮助页数据
internal/theme/          主题与 Lip Gloss 样式工厂
internal/scrobbler/      Navidrome / Last.fm 播放记录
internal/ui/             根 Model、消息、路由、向导、各视图
internal/ui/components/  纯渲染组件
```

## 许可

MIT
