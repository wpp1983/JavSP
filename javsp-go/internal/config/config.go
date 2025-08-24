package config

import (
	"time"
)

// Config represents the main configuration structure
type Config struct {
	Scanner    ScannerConfig    `mapstructure:"scanner" yaml:"scanner" json:"scanner"`
	Network    NetworkConfig    `mapstructure:"network" yaml:"network" json:"network"`
	Crawler    CrawlerConfig    `mapstructure:"crawler" yaml:"crawler" json:"crawler"`
	Summarizer SummarizerConfig `mapstructure:"summarizer" yaml:"summarizer" json:"summarizer"`
	Other      OtherConfig      `mapstructure:"other" yaml:"other" json:"other"`
}

// ScannerConfig represents the scanner section
type ScannerConfig struct {
	IgnoredIDPattern          []string `mapstructure:"ignored_id_pattern" yaml:"ignored_id_pattern" json:"ignored_id_pattern"`
	InputDirectory            string   `mapstructure:"input_directory" yaml:"input_directory" json:"input_directory"`
	FilenameExtensions        []string `mapstructure:"filename_extensions" yaml:"filename_extensions" json:"filename_extensions"`
	IgnoredFolderNamePattern  []string `mapstructure:"ignored_folder_name_pattern" yaml:"ignored_folder_name_pattern" json:"ignored_folder_name_pattern"`
	MinimumSize               string   `mapstructure:"minimum_size" yaml:"minimum_size" json:"minimum_size"`
	SkipNfoDir                bool     `mapstructure:"skip_nfo_dir" yaml:"skip_nfo_dir" json:"skip_nfo_dir"`
	Manual                    bool     `mapstructure:"manual" yaml:"manual" json:"manual"`
}

// NetworkConfig represents the network section
type NetworkConfig struct {
	ProxyServer *string          `mapstructure:"proxy_server" yaml:"proxy_server" json:"proxy_server"`
	Retry       int              `mapstructure:"retry" yaml:"retry" json:"retry"`
	Timeout     time.Duration    `mapstructure:"timeout" yaml:"timeout" json:"timeout"`
}

// CrawlerConfig represents the crawler section
type CrawlerConfig struct {
	Selection              CrawlerSelection `mapstructure:"selection" yaml:"selection" json:"selection"`
	RequiredKeys           []string         `mapstructure:"required_keys" yaml:"required_keys" json:"required_keys"`
	Hardworking            bool             `mapstructure:"hardworking" yaml:"hardworking" json:"hardworking"`
	RespectSiteAvid        bool             `mapstructure:"respect_site_avid" yaml:"respect_site_avid" json:"respect_site_avid"`
	FC2FanLocalPath        *string          `mapstructure:"fc2fan_local_path" yaml:"fc2fan_local_path" json:"fc2fan_local_path"`
	SleepAfterScraping     time.Duration    `mapstructure:"sleep_after_scraping" yaml:"sleep_after_scraping" json:"sleep_after_scraping"`
	UseJavdbCover          string           `mapstructure:"use_javdb_cover" yaml:"use_javdb_cover" json:"use_javdb_cover"`
	NormalizeActressName   bool             `mapstructure:"normalize_actress_name" yaml:"normalize_actress_name" json:"normalize_actress_name"`
}

// CrawlerSelection represents the crawler selection
type CrawlerSelection struct {
	Normal []string `mapstructure:"normal" yaml:"normal" json:"normal"`
	FC2    []string `mapstructure:"fc2" yaml:"fc2" json:"fc2"`
	CID    []string `mapstructure:"cid" yaml:"cid" json:"cid"`
	Getchu []string `mapstructure:"getchu" yaml:"getchu" json:"getchu"`
	Gyutto []string `mapstructure:"gyutto" yaml:"gyutto" json:"gyutto"`
}

// SummarizerConfig represents the summarizer section
type SummarizerConfig struct {
	MoveFiles bool                 `mapstructure:"move_files" yaml:"move_files" json:"move_files"`
	Path      PathConfig           `mapstructure:"path" yaml:"path" json:"path"`
	Title     TitleConfig          `mapstructure:"title" yaml:"title" json:"title"`
	Default   DefaultConfig        `mapstructure:"default" yaml:"default" json:"default"`
	NFO       NFOConfig            `mapstructure:"nfo" yaml:"nfo" json:"nfo"`
	Censor    []string             `mapstructure:"censor_options_representation" yaml:"censor_options_representation" json:"censor_options_representation"`
	Cover     CoverConfig          `mapstructure:"cover" yaml:"cover" json:"cover"`
	Fanart    FanartConfig         `mapstructure:"fanart" yaml:"fanart" json:"fanart"`
	ExtraArts ExtraFanartsConfig   `mapstructure:"extra_fanarts" yaml:"extra_fanarts" json:"extra_fanarts"`
}

// PathConfig represents the path configuration
type PathConfig struct {
	OutputFolderPattern string `mapstructure:"output_folder_pattern" yaml:"output_folder_pattern" json:"output_folder_pattern"`
	BasenamePattern     string `mapstructure:"basename_pattern" yaml:"basename_pattern" json:"basename_pattern"`
	LengthMaximum       int    `mapstructure:"length_maximum" yaml:"length_maximum" json:"length_maximum"`
	LengthByByte        bool     `mapstructure:"length_by_byte" yaml:"length_by_byte" json:"length_by_byte"`
	MaxActressCount     int    `mapstructure:"max_actress_count" yaml:"max_actress_count" json:"max_actress_count"`
	HardLink            bool     `mapstructure:"hard_link" yaml:"hard_link" json:"hard_link"`
	CleanupEmpty        bool     `mapstructure:"cleanup_empty_folders" yaml:"cleanup_empty_folders" json:"cleanup_empty_folders"`
}

// TitleConfig represents the title configuration
type TitleConfig struct {
	RemoveTrailingActorName bool     `mapstructure:"remove_trailing_actor_name" yaml:"remove_trailing_actor_name" json:"remove_trailing_actor_name"`
}

// DefaultConfig represents the default values configuration
type DefaultConfig struct {
	Title     string `mapstructure:"title" yaml:"title" json:"title"`
	Actress   string `mapstructure:"actress" yaml:"actress" json:"actress"`
	Series    string `mapstructure:"series" yaml:"series" json:"series"`
	Director  string `mapstructure:"director" yaml:"director" json:"director"`
	Producer  string `mapstructure:"producer" yaml:"producer" json:"producer"`
	Publisher string `mapstructure:"publisher" yaml:"publisher" json:"publisher"`
}

// NFOConfig represents the NFO configuration
type NFOConfig struct {
	BasenamePattern      string   `mapstructure:"basename_pattern" yaml:"basename_pattern" json:"basename_pattern"`
	TitlePattern         string   `mapstructure:"title_pattern" yaml:"title_pattern" json:"title_pattern"`
	CustomGenresFields   []string `mapstructure:"custom_genres_fields" yaml:"custom_genres_fields" json:"custom_genres_fields"`
	CustomTagsFields     []string `mapstructure:"custom_tags_fields" yaml:"custom_tags_fields" json:"custom_tags_fields"`
}

// CoverConfig represents the cover configuration
type CoverConfig struct {
	BasenamePattern string      `mapstructure:"basename_pattern" yaml:"basename_pattern" json:"basename_pattern"`
	HighRes         bool        `mapstructure:"highres" yaml:"highres" json:"highres"`
	AddLabel        bool        `mapstructure:"add_label" yaml:"add_label" json:"add_label"`
	Crop            CropConfig  `mapstructure:"crop" yaml:"crop" json:"crop"`
}

// CropConfig represents the crop configuration
type CropConfig struct {
	OnIDPattern []string    `mapstructure:"on_id_pattern" yaml:"on_id_pattern" json:"on_id_pattern"`
	Engine      *EngineConfig `mapstructure:"engine" yaml:"engine" json:"engine"`
}

// EngineConfig represents the engine configuration
type EngineConfig struct {
	Name string `mapstructure:"name" yaml:"name" json:"name"`
}

// FanartConfig represents the fanart configuration
type FanartConfig struct {
	BasenamePattern string `mapstructure:"basename_pattern" yaml:"basename_pattern" json:"basename_pattern"`
}

// ExtraFanartsConfig represents the extra fanarts configuration
type ExtraFanartsConfig struct {
	Enabled       bool          `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	ScrapInterval time.Duration   `mapstructure:"scrap_interval" yaml:"scrap_interval" json:"scrap_interval"`
}

// OtherConfig represents the other configuration options
type OtherConfig struct {
	Interactive  bool     `mapstructure:"interactive" yaml:"interactive" json:"interactive"`
	CheckUpdate  bool     `mapstructure:"check_update" yaml:"check_update" json:"check_update"`
	AutoUpdate   bool     `mapstructure:"auto_update" yaml:"auto_update" json:"auto_update"`
	LogLevel     string `mapstructure:"log_level" yaml:"log_level" json:"log_level"`
	DryRun       bool     `mapstructure:"dry_run" yaml:"dry_run" json:"dry_run"`
}

// GetDefaultConfig returns the default configuration
func GetDefaultConfig() *Config {
	return &Config{
		Scanner: ScannerConfig{
			IgnoredIDPattern: []string{
				"(144|240|360|480|720|1080)[Pp]",
				"[24][Kk]",
				"\\w+2048\\.com",
				"Carib(beancom)?",
				"[^a-z\\d](f?hd|lt)[^a-z\\d]",
			},
			InputDirectory: "",
			FilenameExtensions: []string{
				".3gp", ".avi", ".f4v", ".flv", ".iso", ".m2ts", ".m4v",
				".mkv", ".mov", ".mp4", ".mpeg", ".rm", ".rmvb", ".ts",
				".vob", ".webm", ".wmv", ".strm", ".mpg",
			},
			IgnoredFolderNamePattern: []string{
				"^\\.",
				"^#recycle$",
				"^#整理完成$",
				"^#不要扫描$",
			},
			MinimumSize: "232MiB",
			SkipNfoDir:  false,
			Manual:      false,
		},
		Network: NetworkConfig{
			ProxyServer: nil,
			Retry:       3,
			Timeout:     15 * time.Second,
		},
		Crawler: CrawlerConfig{
			Selection: CrawlerSelection{
				Normal: []string{"javbus2", "avwiki"},
				FC2:    []string{"javbus2", "avwiki"},
				CID:    []string{"javbus2", "avwiki"},
				Getchu: []string{"javbus2", "avwiki"},
				Gyutto: []string{"javbus2", "avwiki"},
			},
			RequiredKeys:         []string{"cover", "title"},
			Hardworking:          true,
			RespectSiteAvid:      true,
			FC2FanLocalPath:      nil,
			SleepAfterScraping:   1 * time.Second,
			UseJavdbCover:        "fallback",
			NormalizeActressName: true,
		},
		Summarizer: SummarizerConfig{
			MoveFiles: true,
			Path: PathConfig{
				OutputFolderPattern: "#整理完成/{actress}/[{num}] {title}",
				BasenamePattern:     "{num}",
				LengthMaximum:       250,
				LengthByByte:        true,
				MaxActressCount:     10,
				HardLink:            false,
				CleanupEmpty:        true,
			},
			Title: TitleConfig{
				RemoveTrailingActorName: true,
			},
			Default: DefaultConfig{
				Title:     "#未知标题",
				Actress:   "#未知女优",
				Series:    "#未知系列",
				Director:  "#未知导演",
				Producer:  "#未知制作商",
				Publisher: "#未知发行商",
			},
			NFO: NFOConfig{
				BasenamePattern:      "movie",
				TitlePattern:         "{num} {title}",
				CustomGenresFields:   []string{"{genre}", "{censor}"},
				CustomTagsFields:     []string{"{genre}", "{censor}"},
			},
			Censor: []string{"无码", "有码", "打码情况未知"},
			Cover: CoverConfig{
				BasenamePattern: "poster",
				HighRes:         true,
				AddLabel:        false,
				Crop: CropConfig{
					OnIDPattern: []string{
						"^\\d{6}[-_]\\d{3}$",
						"^ARA",
						"^SIRO",
						"^GANA",
						"^MIUM",
						"^HHL",
					},
					Engine: nil,
				},
			},
			Fanart: FanartConfig{
				BasenamePattern: "fanart",
			},
			ExtraArts: ExtraFanartsConfig{
				Enabled:       false,
				ScrapInterval: 1500 * time.Millisecond,
			},
		},
		Other: OtherConfig{
			Interactive: true,
			CheckUpdate: false,
			AutoUpdate:  false,
			LogLevel:    "INFO",
		},
	}
}