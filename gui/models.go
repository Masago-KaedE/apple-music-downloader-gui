package main

type Settings struct {
	ProjectRoot     string `json:"projectRoot"`
	Distribution    string `json:"distribution"`
	Storefront      string `json:"storefront"`
	AlacSaveFolder  string `json:"alacSaveFolder"`
	AtmosSaveFolder string `json:"atmosSaveFolder"`
	AacSaveFolder   string `json:"aacSaveFolder"`
	EmbedCover      bool   `json:"embedCover"`
	CoverSize       string `json:"coverSize"`
	CoverFormat     string `json:"coverFormat"`
	EmbedLyrics     bool   `json:"embedLyrics"`
	SaveLyricsFile  bool   `json:"saveLyricsFile"`
	LyricsFormat    string `json:"lyricsFormat"`
	AacType         string `json:"aacType"`
	AlacMax         int    `json:"alacMax"`
	AtmosMax        int    `json:"atmosMax"`
}

type ProjectStatus struct {
	Valid       bool   `json:"valid"`
	Root        string `json:"root"`
	Executable  bool   `json:"executable"`
	Config      bool   `json:"config"`
	Wrapper     bool   `json:"wrapper"`
	Description string `json:"description"`
}

type PortStatus struct {
	Port      int  `json:"port"`
	Listening bool `json:"listening"`
}

type WrapperStatus struct {
	Ready        bool         `json:"ready"`
	OwnedByGUI   bool         `json:"ownedByGUI"`
	Running      bool         `json:"running"`
	Needs2FA     bool         `json:"needs2FA"`
	Distribution string       `json:"distribution"`
	Ports        []PortStatus `json:"ports"`
	Message      string       `json:"message"`
}

type DownloadRequest struct {
	URLs    []string `json:"urls"`
	Quality string   `json:"quality"`
}

type TrackResult struct {
	Path   string `json:"path"`
	Artist string `json:"artist"`
	Album  string `json:"album"`
	Song   string `json:"song"`
	Status string `json:"status"`
}

type DownloadState struct {
	Running   bool          `json:"running"`
	Canceled  bool          `json:"canceled"`
	Phase     string        `json:"phase"`
	Queue     int           `json:"queue"`
	QueueSize int           `json:"queueSize"`
	Track     int           `json:"track"`
	Tracks    int           `json:"tracks"`
	Completed int           `json:"completed"`
	Warnings  int           `json:"warnings"`
	Errors    int           `json:"errors"`
	Results   []TrackResult `json:"results"`
	Message   string        `json:"message"`
}

type AppSnapshot struct {
	Settings Settings      `json:"settings"`
	Project  ProjectStatus `json:"project"`
	Wrapper  WrapperStatus `json:"wrapper"`
	Download DownloadState `json:"download"`
	Distros  []string      `json:"distros"`
}
