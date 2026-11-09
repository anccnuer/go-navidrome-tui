# subsonic-tui

一款用 Go 编写的终端 TUI 音乐播放器，面向 [Navidrome](https://www.navidrome.org/)（Subsonic / OpenSubsonic API）。界面按 [go-musicfox](https://github.com/go-musicfox/go-musicfox) 的设计改造：水平居中的无边框菜单、`=> 12.` 行骨架、选中项仅变色高亮、状态栏面包屑、`#` 渐变进度条与菜单下方的键位提示行；文字只用前景色、不铺底色色块，默认使用其网易红主题，并保留主题热切换。

> 音频播放由外部 **mpv** 进程通过 JSON IPC 驱动。mpv 负责解码与流式播放，本程序只负责界面与队列，因此格式支持广、CPU 占用低。

## 界面预览（结构示意）

```
 » 主页 / 全部歌曲                subsonic-tui · [3/120] · 已连接        musicfox

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

## 主要功能

- **多服务器配置**：首次启动交互式登录向导（含连通性校验），保存到 TOML；`w` 可随时切换/重新登录。
- **浏览**：只有「全部歌曲」与「歌单」两个入口。全部歌曲按字母序分页懒加载，歌单直接进入曲目列表；不提供艺术家、专辑、流派分类。
- **搜索**：`Ctrl+F` 搜索全部歌曲，`/` 过滤当前列表，支持中文全角键别名。
- **播放与队列**：播放/暂停、上下曲、快进快退、音量、随机/单曲循环/列表循环/顺序、下一曲播放、队列编辑与清空。
- **完整体验**：同步歌词滚动高亮、Unicode 半块字符封面、喜欢/评分、歌单加入与移除、浏览器打开、主题切换、帮助页、快捷键自定义、Last.fm scrobble 与 Navidrome 播放计数上报。
- **go-musicfox 风格界面**：无边框居中菜单（`=> 12. 标题 · 副标题`）、选中项仅变色（无前后竖杠/底色）、状态栏 `» 面包屑 / 子级` + 居中 `[3/120]` + 右侧主题、菜单下方居中的键位提示、帮助页 70% 宽的居中弹窗。主页菜单只显示「全部歌曲」「歌单」两个名称。
- **底部双行播放条**：`[模式] 音量 ♫ ♪ ♫ ♪ ♥ 歌名 歌手` + `#` 填充、`.` 空槽的逐格渐变进度条，时间固定 `00:42/03:45` 格式，支持鼠标点击跳转。
- **沉浸式播放页**：`z` 打开全屏正在播放视图（大进度条、封面、元数据与当前歌词），再按一次返回；页内 `h/l` 快退快进、`j/k` 调音量。
- **响应式布局**：宽屏双列菜单、窄屏单列；菜单块按窗口宽度取合适宽度并水平居中（标题、根菜单条目、快捷键提示行均居中），歌词预览在高度足够时显示 3 或 5 行。
- **降级提示**：无 mpv、无网络、无服务器、无歌词均有明确提示或占位；封面加载失败时显示占位图并记录 debug 日志，真彩不支持时自动降级为 256/16 色。

## 安装与运行

### 前置依赖

1. **mpv**（必须）：从 <https://mpv.io/installation/> 安装并加入 `PATH`。
   - Windows：下载官方构建或使用 `scoop install mpv` / `winget install mpv`；若不想改 PATH，可在配置里设置 `player.mpvPath` 为 `mpv.exe` 完整路径。
   - Linux/macOS：`apt install mpv` / `brew install mpv`。
2. **终端**：推荐 Windows Terminal、Kitty、iTerm2、Alacritty 等支持 24 位色的终端；使用等宽字体（中文建议等宽 CJK 字体，图标可选 Nerd Font）。不支持 Windows CMD / macOS 原生终端。
3. **Navidrome**：可访问的服务器地址、用户名与密码。

### 编译

需要 Go 1.26+（`go.mod` 的 `go` 指令为 1.26.0，由 Charm 组件的最低版本要求决定）。

```bash
git clone <your-repo-url> subsonic-tui
cd subsonic-tui
make build          # Windows 输出 bin/subsonic-tui.exe，其它平台输出 bin/subsonic-tui
make run            # 或直接 go run ./cmd/subsonic-tui
make test           # 全量单元测试
```

也可直接编译（PowerShell）：

```powershell
go build -o bin/subsonic-tui.exe ./cmd/subsonic-tui
.\bin\subsonic-tui.exe
```

Linux/macOS：

```bash
go build -o bin/subsonic-tui ./cmd/subsonic-tui
./bin/subsonic-tui
```

命令行参数：

| 参数 | 说明 |
| --- | --- |
| `--config <path>` | 指定配置文件路径（等价于环境变量 `SUBSONIC_TUI_CONFIG`） |
| `--debug` | 启用 debug 日志（写入日志文件，不会污染 TUI） |
| `--version` | 显示版本 |

## 快速体验（公共 demo 实例）

官方公共演示实例可用于验证浏览、搜索与播放（公开账号，请勿在其中做收藏、歌单等写操作）。该实例偶发限流，连接向导会自动重试一次并在超时时给出明确提示：

| 项 | 值 |
| --- | --- |
| 地址 | `https://demo.navidrome.org`（也可只填 `demo.navidrome.org`，会自动补 `https://`） |
| 用户名 / 密码 | `demo` / `demo` |

启动后在向导中填写即可，也可用环境变量跳过向导（PowerShell）：

```powershell
$env:SUBSONIC_TUI_URL='https://demo.navidrome.org'
$env:SUBSONIC_TUI_USER='demo'
$env:SUBSONIC_TUI_PASSWORD='demo'
.\bin\subsonic-tui.exe
```

项目自带一个默认跳过的只读集成测试，用同一实例核对认证、端点与 DTO 解析：

```bash
SUBSONIC_TUI_DEMO_TEST=1 go test ./internal/subsonic -run TestPublicDemoReadOnly -v
```

## 配置

首次启动会显示连接向导，依次填写**名称 / 地址 / 用户名 / 密码**，`Enter`（或点击「连接并保存」按钮）会校验连接并写入配置文件。向导内按 `Ctrl+C` 退出，从主界面按 `w` 重新登录时可用 `Esc` 取消返回。

向导支持鼠标点击任意输入框切换焦点；地址可直接粘贴，首尾空格、全角字符、终端转义残留会被自动清理，省略协议时按主机自动补全（本地地址用 `http://`，其它用 `https://`）。地址格式有误时错误信息会回显实际解析到的内容，便于定位输入污染。配置目录：

| 平台 | 路径 |
| --- | --- |
| Windows | `%AppData%\subsonic-tui\config.toml` |
| Linux | `$XDG_CONFIG_HOME/subsonic-tui/config.toml` 或 `~/.config/subsonic-tui/config.toml` |
| macOS | `~/Library/Application Support/subsonic-tui/config.toml` |

完整配置项见 [`config.example.toml`](./config.example.toml)。要点：

- `[[servers]]` 可写多份，`active` 选择当前使用的名称。
- 密码以 `enc:<hex>` 形式保存，**这只是避免明文展示，不是加密**。更安全的方式是不写 `password`，改用环境变量：

```bash
SUBSONIC_TUI_URL=https://music.example.com \
SUBSONIC_TUI_USER=alice \
SUBSONIC_TUI_PASSWORD=secret \
./bin/subsonic-tui
```

其它环境变量：`SUBSONIC_TUI_CONFIG`、`SUBSONIC_TUI_THEME`、`SUBSONIC_TUI_MPV`、`SUBSONIC_TUI_LOG_LEVEL`、`SUBSONIC_TUI_SERVER_NAME`。

- `[keybindings]` 支持覆盖任意**非内置**操作：值可为单个键或键数组，空字符串表示取消绑定；`useDefaultKeyBindings = false` 可完全自定义（内置导航键始终保留）。示例：

```toml
[keybindings.app]
next = ["alt+n", "n"]   # n 是内置 enter，会被忽略并给出警告
help = ["?", "ctrl+h"]
```

- 内置主题：`musicfox`（默认，网易红 `#EA403F` / `#1A1A1A`）、`aurora`、`dracula`、`nord`、`gruvbox`、`mono`，运行时 `Ctrl+T` 循环切换；`theme.customPath` 可指向自定义 TOML 色板（字段见 `internal/theme/theme.go`）。
- `[lastfm]` 默认关闭。启用需要 `apiKey`、`apiSecret`、`sessionKey`（通过 Last.fm 的 `auth.getMobileSession` / `auth.getSession` 流程自行获取后填入）。曲目长于 30 秒且播放达到 `scrobbleAt`（默认 50%）或满 4 分钟时上报；`scrobbleAt = 0` 表示仅在曲目播放结束时上报。同时会调用 Navidrome 的 `scrobble` 端点更新播放计数。

## 快捷键

内置导航键不可自定义，其余均可在 `[keybindings.app]` 覆盖。完整对照表见 [`docs/shortcuts.md`](./docs/shortcuts.md)。在搜索 / 过滤 / 帮助输入框激活时按键用于输入：`Esc` 退出输入（帮助页会同时返回上一视图），输入框内退出应用请用 `Ctrl+C`。`c` / `y` / `F2` / `z` 再按一次可返回原视图。

鼠标默认开启：向导中点击输入框或「连接并保存」按钮；主界面点击菜单/队列行选中、双击进入或播放、滚轮上下浏览、点击底部进度条跳转到对应位置（终端高度小于 14 行时播放条压缩为单行，此时不支持点击跳转）。若终端把鼠标事件解析成异常字符，可在 `[ui]` 中设置 `enableMouse = false`（键盘仍可完成全部操作）。常用键：

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
| `z` | 沉浸式正在播放（再按返回） |  | 进度条点击可跳转 |
| `e` `E` | 下一曲播放 / 追加末尾 | `\` | 从队列删除 |
| `,` `.` | 喜欢 / 取消喜欢（播放中） | `<` `>` | 喜欢 / 取消喜欢（选中） |
| `Tab` | 加入歌单 | `O` | 浏览器打开选中项 |
| `f` | 播放中歌曲的相似歌曲 | `Ctrl+T` | 切换主题 |
| `u` | 清缓存 | `w` | 切换服务器 |
| `q` | 退出 | `r` | 重新渲染 |

## 性能设计

- 所有网络请求、封面解码、mpv 命令均封装为 `tea.Cmd` 异步执行，耗时操作不会阻塞 UI；配置写回是轻量本地文件写入，收敛在 UI 线程完成，避免后台并发修改共享配置。
- 列表只渲染视口内可见行（O(可见行数)），Bubble Tea 差分渲染只输出变化的单元格。
- 泛型 TTL + LRU 缓存（全部歌曲/歌单/搜索/歌词/封面），样式对象与渐变文本预构建并复用，封面字符串按「ID + 尺寸 + 色深」缓存。
- HTTP 连接复用、并发信号量限流、请求 context 超时、分页懒加载；mpv 使用事件驱动 + 500ms 低频 tick，避免每帧全屏重绘。
- 日志只写文件（默认 `warn`），URL 中的 token/salt/密码一律脱敏。

## 常见问题

| 现象 | 处理 |
| --- | --- |
| 底部提示「未找到 mpv」 | 安装 mpv 并加入 PATH，或配置 `player.mpvPath`。界面与浏览仍可用，仅播放被禁用。 |
| 退出后 mpv 仍在后台播放 | 新版本会随程序结束 mpv：Windows 使用 kill-on-close 的 Job Object（即使崩溃/强杀/关闭终端也生效），Linux 使用 `Pdeathsig`，macOS 通过 SIGHUP 与退出清理。若仍残留，通常是旧版本遗留的进程，手动结束一次即可。 |
| 连接失败 / 401 | 检查地址是否含 `http(s)://`、端口、用户名密码；自签证书可设 `insecure = true`。 |
| 连接超时 | 向导会自动重试一次；公共 demo 实例常有负载限制，可稍后再试或改用本地 Navidrome。 |
| 输入 `https://…` 仍提示「地址需以 http:// 或 https:// 开头」 | 旧版本未消费鼠标事件，终端的鼠标转义序列会混进输入框，导致校验看到的是被污染的字符串。现已清洗粘贴/控制/全角字符、自动补全协议，并在出错时回显实际解析值；重新编译即可。 |
| 鼠标点击输入框没反应 | 确认 `[ui] enableMouse` 未被设为 `false`（从旧版本升级时配置文件里可能仍写着 `false`，改为 `true` 或删除该项即可），并使用支持鼠标报告的终端（Windows Terminal / Kitty / iTerm2）。 |
| 歌词一直为空 | Navidrome 需要嵌入歌词或同名 `.lrc` 文件；本程序优先 OpenSubsonic `getLyricsBySongId`，回退 `getLyrics`。 |
| 封面显示为色块或颜色怪异 | 这是设计：纯文本半块字符渲染。请启用终端 24 位色（`ui.trueColor = true`），或关闭以强制 256 色降级。 |
| 排版错乱 | 使用等宽字体（含中文等宽），宽屏双列可在配置里设 `ui.doubleColumn = false`。 |
| 快捷键无效 | 输入框/过滤框激活时按键用于输入，`Esc` 退出输入模式；查看 `F2` 设置页中的冲突数。 |
| 帮助页里 `q` 不退出 | 帮助页默认处于过滤输入模式，按 `Esc` 返回上一视图，或用 `Ctrl+C` 退出应用。 |

## 开发

```bash
make test       # go test ./...
make lint       # go vet ./...
make tidy       # go mod tidy
make windows linux darwin   # 交叉编译（使用 shell 环境变量前缀，Windows 上需 Git Bash / MSYS / WSL）
```

目录结构：

```
cmd/subsonic-tui/     入口
internal/app/         依赖装配与优雅退出
internal/config/      TOML 配置、路径、原子保存
internal/subsonic/    (Open)Subsonic REST 客户端与 DTO
internal/model/       领域模型与枚举
internal/cache/       泛型 TTL+LRU
internal/player/      mpv JSON IPC 与播放队列状态机
internal/lyrics/      LRC / 结构化歌词解析
internal/cover/       图片解码与半块字符渲染
internal/keymap/      快捷键解析、冲突检测、帮助页数据
internal/theme/       主题与 Lip Gloss 样式工厂
internal/scrobbler/   Navidrome / Last.fm 播放记录
internal/ui/          根 Model、消息、路由、向导、各视图
internal/ui/components/  纯渲染组件
```

## 许可

MIT
