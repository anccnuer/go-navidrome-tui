// Package keymap implements go-musicfox style keybindings: a set of immutable
// built-in navigation keys plus a fully overridable action table.
package keymap

// Action identifies a command in the UI.
type Action string

const (
	// Built-in, not overridable.
	ActionRerender   Action = "rerender"
	ActionMoveLeft   Action = "moveLeft"
	ActionMoveRight  Action = "moveRight"
	ActionMoveUp     Action = "moveUp"
	ActionMoveDown   Action = "moveDown"
	ActionMoveTop    Action = "moveToTop"
	ActionMoveBottom Action = "moveToBottom"
	ActionEnter      Action = "enter"
	ActionBack       Action = "goBack"
	ActionSearch     Action = "search"
	ActionQuit       Action = "quit"

	// Views and help.
	ActionHelp           Action = "help"
	ActionPageUp         Action = "pageUp"
	ActionPageDown       Action = "pageDown"
	ActionPlaylistView   Action = "curPlaylist"
	ActionLyricsView     Action = "lyricsView"
	ActionSettingsView   Action = "settingsView"
	ActionGlobalSearch   Action = "globalSearch"
	ActionNowPlayingView Action = "nowPlayingView"

	// Playback.
	ActionPlayToggle    Action = "playortoggle"
	ActionPrev          Action = "previous"
	ActionNext          Action = "next"
	ActionSeekBack1     Action = "backwardOneSec"
	ActionSeekBack5     Action = "backwardFiveSec"
	ActionSeekForward5  Action = "forwardFiveSec"
	ActionSeekForward10 Action = "forwardTenSec"
	ActionVolumeDown    Action = "downVolume"
	ActionVolumeUp      Action = "upVolume"
	ActionSwitchMode    Action = "switchPlayMode"

	// Queue editing.
	ActionAppendNext      Action = "appendSongsToNext"
	ActionAppendEnd       Action = "appendSongsAfterCurPlaylist"
	ActionDeleteFromQueue Action = "delSongFromCurPlaylist"
	ActionClearQueue      Action = "clearCurPlaylist"

	// Annotation of the playing / selected item.
	ActionLikePlaying         Action = "likePlayingSong"
	ActionDislikePlaying      Action = "dislikePlayingSong"
	ActionTrashPlaying        Action = "trashPlayingSong"
	ActionLikeSelected        Action = "likeSelectedSong"
	ActionDislikeSelected     Action = "dislikeSelectedSong"
	ActionTrashSelected       Action = "trashSelectedSong"
	ActionOpenAlbumPlaying    Action = "openAlbumOfPlayingSong"
	ActionOpenArtistPlaying   Action = "openArtistOfPlayingSong"
	ActionOpenBrowserPlaying  Action = "openPlayingSongInWeb"
	ActionSimilarPlaying      Action = "simiSongsOfPlayingSong"
	ActionOpenAlbumSelected   Action = "openAlbumOfSelectedSong"
	ActionOpenArtistSelected  Action = "openArtistOfSelectedSong"
	ActionOpenBrowserSelected Action = "openSelectedItemInWeb"
	ActionSimilarSelected     Action = "simiSongsOfSelectedSong"
	ActionAddToPlaylist       Action = "addSelectedSongToUserPlaylist"
	ActionRemoveFromPlaylist  Action = "removePlayingSongFromUserPlaylist"
	ActionCollectPlaylist     Action = "collectSelectedPlaylist"
	ActionDiscollectPlaylist  Action = "discollectSelectedPlaylist"
	ActionActionSelected      Action = "actionOfSelected"
	ActionActionPlaying       Action = "actionOfPlayingSong"

	// System.
	ActionSwitchTheme Action = "switchTheme"
	ActionClearCache  Action = "clearSongCache"
	ActionRelogin     Action = "logout"
	ActionToggleSort  Action = "toggleSortOrder"
)

const (
	GroupNavigation = "导航（内置，不可修改）"
	GroupPlayback   = "播放控制"
	GroupQueue      = "播放队列"
	GroupAnnotation = "收藏与关联"
	GroupViews      = "视图与界面"
	GroupSystem     = "系统"
)

// Binding is one action with its effective keys.
type Binding struct {
	Action  Action
	Keys    []string
	Label   string
	Group   string
	Builtin bool
}

// DefaultBindings mirrors go-musicfox's defaults, minus Netease-only features.
var DefaultBindings = []Binding{
	{Action: ActionRerender, Keys: []string{"r", "R"}, Label: "重新渲染界面", Group: GroupNavigation, Builtin: true},
	{Action: ActionMoveLeft, Keys: []string{"h", "H", "left"}, Label: "向左 / 返回上级", Group: GroupNavigation, Builtin: true},
	{Action: ActionMoveRight, Keys: []string{"l", "L", "right"}, Label: "向右 / 进入", Group: GroupNavigation, Builtin: true},
	{Action: ActionMoveUp, Keys: []string{"k", "K", "up"}, Label: "上移", Group: GroupNavigation, Builtin: true},
	{Action: ActionMoveDown, Keys: []string{"j", "J", "down"}, Label: "下移", Group: GroupNavigation, Builtin: true},
	{Action: ActionMoveTop, Keys: []string{"g"}, Label: "跳到顶部", Group: GroupNavigation, Builtin: true},
	{Action: ActionMoveBottom, Keys: []string{"G"}, Label: "跳到底部", Group: GroupNavigation, Builtin: true},
	{Action: ActionEnter, Keys: []string{"n", "N", "enter"}, Label: "进入 / 播放", Group: GroupNavigation, Builtin: true},
	{Action: ActionBack, Keys: []string{"b", "B", "esc"}, Label: "返回上一级", Group: GroupNavigation, Builtin: true},
	{Action: ActionSearch, Keys: []string{"/", "／", "、"}, Label: "过滤当前列表", Group: GroupNavigation, Builtin: true},
	{Action: ActionQuit, Keys: []string{"q", "Q"}, Label: "退出", Group: GroupNavigation, Builtin: true},

	{Action: ActionHelp, Keys: []string{"?", "？"}, Label: "帮助 / 快捷键", Group: GroupViews},
	{Action: ActionPageUp, Keys: []string{"ctrl+u", "pgup"}, Label: "上一页", Group: GroupViews},
	{Action: ActionPageDown, Keys: []string{"ctrl+d", "pgdown"}, Label: "下一页", Group: GroupViews},
	{Action: ActionPlaylistView, Keys: []string{"c", "C"}, Label: "当前播放队列", Group: GroupViews},
	{Action: ActionLyricsView, Keys: []string{"y", "Y"}, Label: "同步歌词", Group: GroupViews},
	{Action: ActionGlobalSearch, Keys: []string{"ctrl+f"}, Label: "全库搜索", Group: GroupViews},
	{Action: ActionSettingsView, Keys: []string{"f2"}, Label: "设置 / 主题", Group: GroupViews},
	{Action: ActionNowPlayingView, Keys: []string{"z", "Z"}, Label: "沉浸式正在播放", Group: GroupViews},
	{Action: ActionSwitchTheme, Keys: []string{"ctrl+t"}, Label: "切换主题", Group: GroupViews},
	{Action: ActionToggleSort, Keys: []string{"`"}, Label: "切换排序", Group: GroupViews},

	{Action: ActionPlayToggle, Keys: []string{"space"}, Label: "播放 / 暂停", Group: GroupPlayback},
	{Action: ActionPrev, Keys: []string{"[", "【"}, Label: "上一首", Group: GroupPlayback},
	{Action: ActionNext, Keys: []string{"]", "】"}, Label: "下一首", Group: GroupPlayback},
	{Action: ActionSeekBack1, Keys: []string{"x"}, Label: "快退 1 秒", Group: GroupPlayback},
	{Action: ActionSeekBack5, Keys: []string{"X"}, Label: "快退 5 秒", Group: GroupPlayback},
	{Action: ActionSeekForward5, Keys: []string{"v"}, Label: "快进 5 秒", Group: GroupPlayback},
	{Action: ActionSeekForward10, Keys: []string{"V"}, Label: "快进 10 秒", Group: GroupPlayback},
	{Action: ActionVolumeDown, Keys: []string{"-", "−", "ー"}, Label: "减小音量", Group: GroupPlayback},
	{Action: ActionVolumeUp, Keys: []string{"=", "＝"}, Label: "加大音量", Group: GroupPlayback},
	{Action: ActionSwitchMode, Keys: []string{"p"}, Label: "切换播放模式", Group: GroupPlayback},

	{Action: ActionAppendNext, Keys: []string{"e"}, Label: "添加为下一曲播放", Group: GroupQueue},
	{Action: ActionAppendEnd, Keys: []string{"E"}, Label: "添加到队列末尾", Group: GroupQueue},
	{Action: ActionDeleteFromQueue, Keys: []string{"\\"}, Label: "从队列删除选中歌曲", Group: GroupQueue},
	{Action: ActionClearQueue, Keys: []string{"ctrl+x"}, Label: "清空播放队列", Group: GroupQueue},

	{Action: ActionLikePlaying, Keys: []string{","}, Label: "喜欢播放中歌曲", Group: GroupAnnotation},
	{Action: ActionDislikePlaying, Keys: []string{"."}, Label: "取消喜欢播放中歌曲", Group: GroupAnnotation},
	{Action: ActionTrashPlaying, Keys: []string{"t"}, Label: "标记播放中歌曲为不喜欢", Group: GroupAnnotation},
	{Action: ActionLikeSelected, Keys: []string{"<", "〈", "《"}, Label: "喜欢选中歌曲", Group: GroupAnnotation},
	{Action: ActionDislikeSelected, Keys: []string{">", "〉", "》"}, Label: "取消喜欢选中歌曲", Group: GroupAnnotation},
	{Action: ActionTrashSelected, Keys: []string{"T"}, Label: "标记选中歌曲为不喜欢", Group: GroupAnnotation},
	{Action: ActionAddToPlaylist, Keys: []string{"tab"}, Label: "选中歌曲加入歌单", Group: GroupAnnotation},
	{Action: ActionRemoveFromPlaylist, Keys: []string{"shift+tab"}, Label: "从歌单移除歌曲", Group: GroupAnnotation},
	{Action: ActionCollectPlaylist, Keys: []string{";", "；"}, Label: "收藏选中歌单", Group: GroupAnnotation},
	{Action: ActionDiscollectPlaylist, Keys: []string{"'"}, Label: "取消收藏选中歌单", Group: GroupAnnotation},
	{Action: ActionOpenBrowserPlaying, Keys: []string{"o"}, Label: "浏览器打开播放中歌曲", Group: GroupAnnotation},
	{Action: ActionSimilarPlaying, Keys: []string{"f"}, Label: "播放中歌曲的相似歌曲", Group: GroupAnnotation},
	{Action: ActionOpenBrowserSelected, Keys: []string{"O"}, Label: "浏览器打开选中项", Group: GroupAnnotation},
	{Action: ActionSimilarSelected, Keys: []string{"F"}, Label: "选中歌曲的相似歌曲", Group: GroupAnnotation},
	{Action: ActionActionSelected, Keys: []string{"m"}, Label: "选中项操作菜单", Group: GroupAnnotation},
	{Action: ActionActionPlaying, Keys: []string{"M"}, Label: "播放中歌曲操作菜单", Group: GroupAnnotation},

	{Action: ActionClearCache, Keys: []string{"u", "U"}, Label: "清除缓存", Group: GroupSystem},
	{Action: ActionRelogin, Keys: []string{"w", "W"}, Label: "切换服务器 / 重新登录", Group: GroupSystem},
}

// Resolver is the final, immutable action lookup table.
type Resolver struct {
	byKey      map[string]Action
	actionKeys map[Action][]string
	bindings   []Binding
	conflicts  []string
}
