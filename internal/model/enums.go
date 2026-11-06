package model

// PlayMode controls how the queue advances when a track ends.
type PlayMode int

const (
	ModeSequential PlayMode = iota
	ModeLoopAll
	ModeLoopOne
	ModeShuffle
)

// String returns the Chinese display label of the mode.
func (m PlayMode) String() string {
	switch m {
	case ModeLoopAll:
		return "列表循环"
	case ModeLoopOne:
		return "单曲循环"
	case ModeShuffle:
		return "随机播放"
	default:
		return "顺序播放"
	}
}

// Icon returns a compact glyph for the mode.
func (m PlayMode) Icon() string {
	switch m {
	case ModeLoopAll:
		return "🔁"
	case ModeLoopOne:
		return "🔂"
	case ModeShuffle:
		return "🔀"
	default:
		return "➡"
	}
}

// Next cycles to the following mode.
func (m PlayMode) Next() PlayMode { return (m + 1) % 4 }

// PlayerStatus is the coarse playback state.
type PlayerStatus int

const (
	StatusStopped PlayerStatus = iota
	StatusLoading
	StatusPlaying
	StatusPaused
	StatusError
)

// Icon returns the glyph used in the now-playing bar.
func (s PlayerStatus) Icon() string {
	switch s {
	case StatusPlaying:
		return "▶"
	case StatusPaused:
		return "⏸"
	case StatusLoading:
		return "◌"
	case StatusError:
		return "⚠"
	default:
		return "■"
	}
}

// ViewKind identifies a top-level screen.
type ViewKind int

const (
	ViewWizard ViewKind = iota
	ViewLibrary
	ViewSearch
	ViewQueue
	ViewLyrics
	ViewHelp
	ViewSettings
	ViewNowPlaying
)

// NodeKind identifies what a navigation node contains.
type NodeKind int

const (
	NodeRoot NodeKind = iota
	NodeArtists
	NodeArtist
	NodeAlbums
	NodeAlbum
	NodePlaylists
	NodePlaylist
	NodeGenres
	NodeGenreAlbums
	NodeStarred
	NodeRandom
	NodeSearch
	NodeAllSongs
)

// Title returns the default title for a node kind.
func (n NodeKind) Title() string {
	switch n {
	case NodeArtists:
		return "艺术家"
	case NodeArtist:
		return "艺术家专辑"
	case NodeAlbums:
		return "专辑"
	case NodeAlbum:
		return "专辑曲目"
	case NodePlaylists:
		return "歌单"
	case NodePlaylist:
		return "歌单曲目"
	case NodeGenres:
		return "流派"
	case NodeGenreAlbums:
		return "流派专辑"
	case NodeStarred:
		return "我的收藏"
	case NodeRandom:
		return "随机漫游"
	case NodeSearch:
		return "搜索结果"
	case NodeAllSongs:
		return "全部歌曲"
	default:
		return "音乐库"
	}
}
