# 快捷键对照表

设计参照 [go-musicfox](https://github.com/go-musicfox/go-musicfox)。**内置键不可自定义**，其余操作都可在 `config.toml` 的 `[keybindings.app]` 中覆盖（值可为字符串或字符串数组；空字符串表示取消该绑定）。设置 `useDefaultKeyBindings = false` 可从零开始配置，但内置键始终保留。

在输入框（搜索、列表过滤、帮助过滤、登录向导）激活时，按键用于输入：`Esc` 退出输入（帮助页会同时返回上一视图），输入框内退出应用请用 `Ctrl+C`。`c` / `y` / `F2` / `z` 再按一次可返回原视图。登录向导的四个输入框与「连接并保存」按钮均可用鼠标点击；主界面可点击菜单/队列行、双击进入或播放、滚轮浏览、点击底部进度条跳转；终端高度小于 14 行时播放条为单行，不支持点击跳转（可用 `[ui] enableMouse = false` 关闭鼠标）。

## 内置导航（不可自定义）

| 默认键 | 操作标识 | 说明 |
| --- | --- | --- |
| `r` `R` | `rerender` | 重新渲染界面 |
| `h` `H` `Left` | `moveLeft` | 左移 / 返回上级（双列时为左列） |
| `l` `L` `Right` | `moveRight` | 右移 / 进入（双列时为右列） |
| `k` `K` `Up` | `moveUp` | 上移 |
| `j` `J` `Down` | `moveDown` | 下移 |
| `g` | `moveToTop` | 跳到顶部 |
| `G` | `moveToBottom` | 跳到底部 |
| `n` `N` `Enter` | `enter` | 进入子级 / 播放选中歌曲 |
| `b` `B` `Esc` | `goBack` | 返回上一级 |
| `/` `／` `、` | `search` | 过滤当前列表 / 在搜索页重新聚焦输入框 |
| `q` `Q` | `quit` | 退出 |

## 播放控制

| 默认键 | 操作标识 | 说明 |
| --- | --- | --- |
| `Space` | `playortoggle` | 播放 / 暂停 |
| `[` `【` | `previous` | 上一首 |
| `]` `】` | `next` | 下一首 |
| `x` | `backwardOneSec` | 快退 1 秒 |
| `X` | `backwardFiveSec` | 快退 5 秒 |
| `v` | `forwardFiveSec` | 快进 5 秒 |
| `V` | `forwardTenSec` | 快进 10 秒 |
| `-` `−` `ー` | `downVolume` | 音量 -5 |
| `=` `＝` | `upVolume` | 音量 +5 |
| `p` | `switchPlayMode` | 循环切换：顺序 → 列表循环 → 单曲循环 → 随机 |

## 播放队列

| 默认键 | 操作标识 | 说明 |
| --- | --- | --- |
| `c` `C` | `curPlaylist` | 打开当前播放队列（`c` 队列视图中再按可返回） |
| `e` | `appendSongsToNext` | 选中歌曲添加为下一曲播放 |
| `E` | `appendSongsAfterCurPlaylist` | 选中歌曲追加到队列末尾 |
| `\` | `delSongFromCurPlaylist` | 从队列删除选中歌曲 |
| `Ctrl+X` | `clearCurPlaylist` | 清空播放队列 |

## 视图与界面

| 默认键 | 操作标识 | 说明 |
| --- | --- | --- |
| `?` `？` | `help` | 帮助 / 快捷键 |
| `Ctrl+U` `PgUp` | `pageUp` | 上一页 |
| `Ctrl+D` `PgDown` | `pageDown` | 下一页 |
| `y` `Y` | `lyricsView` | 同步歌词视图 |
| `Ctrl+F` | `globalSearch` | 全库搜索 |
| `z` `Z` | `nowPlayingView` | 沉浸式正在播放（再按返回） |
| `F2` | `settingsView` | 设置 / 主题预览 |
| `Ctrl+T` | `switchTheme` | 切换主题 |
| `` ` `` | `toggleSortOrder` | 反转当前列表排序 |

> `Ctrl+U` 在登录向导的输入框内表示「清空当前字段」，不会翻页。

## 收藏与关联

| 默认键 | 操作标识 | 说明 |
| --- | --- | --- |
| `,` `，` | `likePlayingSong` | 喜欢播放中歌曲 |
| `.` `。` | `dislikePlayingSong` | 取消喜欢播放中歌曲 |
| `t` | `trashPlayingSong` | 播放中歌曲标记为不喜欢（评分 1） |
| `<` `〈` `《` | `likeSelectedSong` | 喜欢选中项 |
| `>` `〉` `》` | `dislikeSelectedSong` | 取消喜欢选中项 |
| `T` | `trashSelectedSong` | 选中歌曲标记为不喜欢 |
| `Tab` | `addSelectedSongToUserPlaylist` | 选中歌曲加入歌单（进入「选择目标歌单」） |
| `Shift+Tab` | `removePlayingSongFromUserPlaylist` | 在歌单曲目列表中从歌单移除选中歌曲 |
| `;` `；` | `collectSelectedPlaylist` | 歌单收藏（当前给出提示：Subsonic API 无对应端点，请在网页端操作） |
| `'` | `discollectSelectedPlaylist` | 取消歌单收藏（同上） |
| `o` | `openPlayingSongInWeb` | 浏览器打开 Navidrome |
| `f` | `simiSongsOfPlayingSong` | 播放中歌曲的相似歌曲 |
| `O` | `openSelectedItemInWeb` | 浏览器打开选中项 |
| `F` | `simiSongsOfSelectedSong` | 选中歌曲的相似歌曲 |
| `m` | `actionOfSelected` | 显示选中项可用操作提示 |
| `M` | `actionOfPlayingSong` | 显示播放中歌曲可用操作提示 |

> 相似歌曲依赖 Navidrome 的 Last.fm 外部集成（`getSimilarSongs2`）；未启用集成时结果可能为空。

## 系统

| 默认键 | 操作标识 | 说明 |
| --- | --- | --- |
| `u` `U` | `clearSongCache` | 清除 API 与封面缓存并重新加载 |
| `w` `W` | `logout` | 打开登录向导，切换服务器 / 重新登录 |

## 自定义规则

- 字符键区分大小写：`a` 与 `A` 是不同操作；组合键（`ctrl+u`、`shift+tab`）大小写不敏感。
- 支持中文全角别名：`／`→`/`、`？`→`?`、`【`→`[`、`】`→`]`、`，`→`,`、`。`→`.`、`<`/`>` 的全角变体、`＝`→`=`、`−`/`ー`→`-`、`；`→`;`。
- 重复绑定会被检测：内置键优先，其余保留先注册的操作，冲突会写入日志并在 `F2` 设置页显示数量。
- 示例：

```toml
[keybindings]
useDefaultKeyBindings = true

[keybindings.app]
next = ["alt+n", "n"]          # n 是内置 enter，会被忽略
help = ["?", "ctrl+h"]
switchTheme = "t"              # 注意与 trashPlayingSong 冲突
intelligence = ""              # 未知操作会被忽略并给出警告
```

`?` 帮助页展示的是**最终生效**的绑定表（配置文件里的覆盖会直接反映出来），且默认处于过滤输入模式：输入即筛选动作与键位，`Esc` 返回上一视图。`F2` 设置页展示主题、服务器、缓存与快捷键冲突数的摘要。
