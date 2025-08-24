package nfo

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"javsp-go/internal/datatype"
)

// MediaServerType represents different media server types
type MediaServerType int

const (
	// Emby media server
	MediaServerEmby MediaServerType = iota
	// Jellyfin media server
	MediaServerJellyfin
	// Kodi media center
	MediaServerKodi
	// Plex media server
	MediaServerPlex
)

func (m MediaServerType) String() string {
	switch m {
	case MediaServerEmby:
		return "emby"
	case MediaServerJellyfin:
		return "jellyfin"
	case MediaServerKodi:
		return "kodi"
	case MediaServerPlex:
		return "plex"
	default:
		return "unknown"
	}
}

// NFOConfig contains configuration for NFO generation
type NFOConfig struct {
	MediaServerType    MediaServerType   `json:"media_server_type"`
	Template           string            `json:"template"`
	CustomTemplate     string            `json:"custom_template"`
	IncludeActressInfo bool              `json:"include_actress_info"`
	IncludeGenres      bool              `json:"include_genres"`
	IncludePreview     bool              `json:"include_preview"`
	CustomFields       map[string]string `json:"custom_fields"`
	DateFormat         string            `json:"date_format"`
	RatingScale        int               `json:"rating_scale"` // 1-10 or 1-5
}

// DefaultNFOConfig returns default NFO configuration
func DefaultNFOConfig() *NFOConfig {
	return &NFOConfig{
		MediaServerType:    MediaServerJellyfin,
		Template:           "default",
		IncludeActressInfo: true,
		IncludeGenres:      true,
		IncludePreview:     true,
		CustomFields:       make(map[string]string),
		DateFormat:         "2006-01-02",
		RatingScale:        10,
	}
}

// MovieNFO represents the XML structure for movie NFO files
type MovieNFO struct {
	XMLName          xml.Name `xml:"movie"`
	Title            string   `xml:"title,omitempty"`
	OriginalTitle    string   `xml:"originaltitle,omitempty"`
	SortTitle        string   `xml:"sorttitle,omitempty"`
	Set              string   `xml:"set,omitempty"`
	Plot             string   `xml:"plot,omitempty"`
	Outline          string   `xml:"outline,omitempty"`
	Tagline          string   `xml:"tagline,omitempty"`
	Runtime          int      `xml:"runtime,omitempty"`
	Thumb            []Thumb  `xml:"thumb,omitempty"`
	Fanart           *Fanart  `xml:"fanart,omitempty"`
	MPAA             string   `xml:"mpaa,omitempty"`
	Premiered        string   `xml:"premiered,omitempty"`
	ReleaseDate      string   `xml:"releasedate,omitempty"`
	Year             int      `xml:"year,omitempty"`
	Director         string   `xml:"director,omitempty"`
	Studio           string   `xml:"studio,omitempty"`
	Publisher        string   `xml:"publisher,omitempty"`
	Genre            []string `xml:"genre,omitempty"`
	Tag              []string `xml:"tag,omitempty"`
	Country          string   `xml:"country,omitempty"`
	Credits          string   `xml:"credits,omitempty"`
	Actor            []Actor  `xml:"actor,omitempty"`
	Rating           float64  `xml:"rating,omitempty"`
	Votes            int      `xml:"votes,omitempty"`
	CriticRating     float64  `xml:"criticrating,omitempty"`
	UserRating       float64  `xml:"userrating,omitempty"`
	Top250           int      `xml:"top250,omitempty"`
	ID               string   `xml:"id,omitempty"`
	UniqueID         []ID     `xml:"uniqueid,omitempty"`
	Trailer          string   `xml:"trailer,omitempty"`
	Watched          bool     `xml:"watched,omitempty"`
	PlayCount        int      `xml:"playcount,omitempty"`
	LastPlayed       string   `xml:"lastplayed,omitempty"`
	FileInfo         *FileInfo `xml:"fileinfo,omitempty"`
	CustomFields     map[string]interface{} `xml:"-"` // For custom template rendering
}

// Thumb represents thumbnail information
type Thumb struct {
	URL      string `xml:",chardata"`
	Aspect   string `xml:"aspect,attr,omitempty"`
	Preview  string `xml:"preview,attr,omitempty"`
	Season   string `xml:"season,attr,omitempty"`
	Type     string `xml:"type,attr,omitempty"`
}

// Fanart represents fanart information
type Fanart struct {
	URL   string `xml:"thumb"`
}

// Actor represents actor/actress information
type Actor struct {
	Name      string `xml:"name"`
	Role      string `xml:"role,omitempty"`
	Order     int    `xml:"order,omitempty"`
	Thumb     string `xml:"thumb,omitempty"`
	Profile   string `xml:"profile,omitempty"`
}

// ID represents unique identifier information
type ID struct {
	Type    string `xml:"type,attr"`
	Default bool   `xml:"default,attr,omitempty"`
	Value   string `xml:",chardata"`
}

// FileInfo represents file information
type FileInfo struct {
	StreamDetails *StreamDetails `xml:"streamdetails,omitempty"`
}

// StreamDetails represents stream details
type StreamDetails struct {
	Video []VideoStream `xml:"video,omitempty"`
	Audio []AudioStream `xml:"audio,omitempty"`
}

// VideoStream represents video stream information
type VideoStream struct {
	Codec             string  `xml:"codec,omitempty"`
	Aspect            float64 `xml:"aspect,omitempty"`
	Width             int     `xml:"width,omitempty"`
	Height            int     `xml:"height,omitempty"`
	DurationInSeconds int     `xml:"durationinseconds,omitempty"`
}

// AudioStream represents audio stream information
type AudioStream struct {
	Codec    string `xml:"codec,omitempty"`
	Language string `xml:"language,omitempty"`
	Channels int    `xml:"channels,omitempty"`
}

// NFOGenerator handles NFO file generation
type NFOGenerator struct {
	config    *NFOConfig
	templates map[string]*template.Template
}

// NewNFOGenerator creates a new NFO generator
func NewNFOGenerator(config *NFOConfig) *NFOGenerator {
	if config == nil {
		config = DefaultNFOConfig()
	}

	generator := &NFOGenerator{
		config:    config,
		templates: make(map[string]*template.Template),
	}

	// Initialize default templates
	generator.initDefaultTemplates()

	return generator
}

// Generate generates NFO content from movie information
func (g *NFOGenerator) Generate(movie *datatype.MovieInfo) ([]byte, error) {
	// Convert movie info to NFO structure
	nfo, err := g.convertMovieToNFO(movie)
	if err != nil {
		return nil, fmt.Errorf("failed to convert movie to NFO: %w", err)
	}

	// Use custom template if specified
	if g.config.CustomTemplate != "" {
		return g.generateWithTemplate(nfo, g.config.CustomTemplate)
	}

	// Use template based on media server type
	templateName := g.getDefaultTemplateName()
	return g.generateWithTemplate(nfo, templateName)
}

// GenerateToFile generates NFO and writes to file
func (g *NFOGenerator) GenerateToFile(movie *datatype.MovieInfo, filename string) error {
	content, err := g.Generate(movie)
	if err != nil {
		return err
	}

	// Write to file
	return writeFileWithBackup(filename, content)
}

// convertMovieToNFO converts MovieInfo to MovieNFO structure
func (g *NFOGenerator) convertMovieToNFO(movie *datatype.MovieInfo) (*MovieNFO, error) {
	nfo := &MovieNFO{
		CustomFields: make(map[string]interface{}),
	}

	// Basic information
	nfo.Title = movie.Title
	nfo.OriginalTitle = movie.Title
	nfo.Plot = movie.Plot
	nfo.Outline = truncateString(movie.Plot, 200)
	nfo.ID = movie.DVDID
	nfo.Studio = movie.Producer
	nfo.Publisher = movie.Publisher
	nfo.Director = movie.Director
	nfo.Set = movie.Series

	// Release date handling
	if movie.ReleaseDate != "" {
		nfo.Premiered = formatDate(movie.ReleaseDate, g.config.DateFormat)
		nfo.ReleaseDate = nfo.Premiered
		if year := extractYear(movie.ReleaseDate); year > 0 {
			nfo.Year = year
		}
	}

	// Runtime handling
	if movie.Runtime != "" {
		if minutes := parseRuntime(movie.Runtime); minutes > 0 {
			nfo.Runtime = minutes
		}
	}

	// Rating (generate based on various factors)
	nfo.Rating = g.calculateRating(movie)
	nfo.UserRating = nfo.Rating

	// Genres
	if g.config.IncludeGenres && len(movie.Genre) > 0 {
		nfo.Genre = movie.Genre
		nfo.Tag = movie.Genre // Also set as tags
	}

	// Actresses as actors
	if g.config.IncludeActressInfo && len(movie.Actress) > 0 {
		for i, actress := range movie.Actress {
			actor := Actor{
				Name:  actress,
				Role:  "Actress",
				Order: i + 1,
			}
			nfo.Actor = append(nfo.Actor, actor)
		}
	}

	// Cover image as thumb
	if movie.Cover != "" {
		nfo.Thumb = append(nfo.Thumb, Thumb{
			URL:    movie.Cover,
			Aspect: "poster",
			Type:   "poster",
		})
	}

	// Preview images
	if g.config.IncludePreview && len(movie.Preview) > 0 {
		for _, preview := range movie.Preview {
			nfo.Thumb = append(nfo.Thumb, Thumb{
				URL:    preview,
				Aspect: "landscape",
				Type:   "fanart",
			})
		}

		// Set first preview as fanart
		if len(movie.Preview) > 0 {
			nfo.Fanart = &Fanart{URL: movie.Preview[0]}
		}
	}

	// Unique IDs
	if movie.DVDID != "" {
		nfo.UniqueID = append(nfo.UniqueID, ID{
			Type:    "dvdid",
			Default: true,
			Value:   movie.DVDID,
		})
	}

	if movie.CID != "" {
		nfo.UniqueID = append(nfo.UniqueID, ID{
			Type:  "cid",
			Value: movie.CID,
		})
	}

	// Country (assume Japan for AV content)
	nfo.Country = "Japan"

	// MPAA rating (Adult content)
	nfo.MPAA = "XXX"

	// Custom fields from config
	for key, value := range g.config.CustomFields {
		nfo.CustomFields[key] = value
	}

	// Add movie-specific custom fields
	nfo.CustomFields["source"] = movie.Source
	nfo.CustomFields["source_url"] = movie.SourceURL
	if movie.Uncensored {
		nfo.CustomFields["censored"] = "No"
	} else {
		nfo.CustomFields["censored"] = "Yes"
	}

	return nfo, nil
}

// generateWithTemplate generates NFO using specified template
func (g *NFOGenerator) generateWithTemplate(nfo *MovieNFO, templateName string) ([]byte, error) {
	tmpl, exists := g.templates[templateName]
	if !exists {
		return nil, fmt.Errorf("template not found: %s", templateName)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nfo); err != nil {
		return nil, fmt.Errorf("template execution failed: %w", err)
	}

	// Format XML output
	return formatXML(buf.Bytes())
}

// getDefaultTemplateName returns template name based on media server type
func (g *NFOGenerator) getDefaultTemplateName() string {
	switch g.config.MediaServerType {
	case MediaServerEmby:
		return "emby"
	case MediaServerJellyfin:
		return "jellyfin"
	case MediaServerKodi:
		return "kodi"
	case MediaServerPlex:
		return "plex"
	default:
		return "default"
	}
}

// initDefaultTemplates initializes built-in templates
func (g *NFOGenerator) initDefaultTemplates() {
	// Default template (compatible with most servers)
	defaultTemplate := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<movie>
	{{- if .Title}}<title>{{.Title | xmlEscape}}</title>{{end}}
	{{- if .OriginalTitle}}<originaltitle>{{.OriginalTitle | xmlEscape}}</originaltitle>{{end}}
	{{- if .Set}}<set>{{.Set | xmlEscape}}</set>{{end}}
	{{- if .Plot}}<plot>{{.Plot | xmlEscape}}</plot>{{end}}
	{{- if .Outline}}<outline>{{.Outline | xmlEscape}}</outline>{{end}}
	{{- if .Runtime}}<runtime>{{.Runtime}}</runtime>{{end}}
	{{- if .Premiered}}<premiered>{{.Premiered}}</premiered>{{end}}
	{{- if .ReleaseDate}}<releasedate>{{.ReleaseDate}}</releasedate>{{end}}
	{{- if .Year}}<year>{{.Year}}</year>{{end}}
	{{- if .Director}}<director>{{.Director | xmlEscape}}</director>{{end}}
	{{- if .Studio}}<studio>{{.Studio | xmlEscape}}</studio>{{end}}
	{{- if .Publisher}}<publisher>{{.Publisher | xmlEscape}}</publisher>{{end}}
	{{- range .Genre}}<genre>{{. | xmlEscape}}</genre>{{end}}
	{{- range .Tag}}<tag>{{. | xmlEscape}}</tag>{{end}}
	{{- if .Country}}<country>{{.Country}}</country>{{end}}
	{{- if .MPAA}}<mpaa>{{.MPAA}}</mpaa>{{end}}
	{{- if .Rating}}<rating>{{.Rating}}</rating>{{end}}
	{{- if .UserRating}}<userrating>{{.UserRating}}</userrating>{{end}}
	{{- if .ID}}<id>{{.ID}}</id>{{end}}
	{{- range .UniqueID}}<uniqueid type="{{.Type}}"{{if .Default}} default="true"{{end}}>{{.Value}}</uniqueid>{{end}}
	{{- range .Actor}}
	<actor>
		<name>{{.Name | xmlEscape}}</name>
		{{- if .Role}}<role>{{.Role | xmlEscape}}</role>{{end}}
		{{- if .Order}}<order>{{.Order}}</order>{{end}}
		{{- if .Thumb}}<thumb>{{.Thumb}}</thumb>{{end}}
	</actor>
	{{- end}}
	{{- range .Thumb}}<thumb{{if .Aspect}} aspect="{{.Aspect}}"{{end}}{{if .Type}} type="{{.Type}}"{{end}}>{{.URL}}</thumb>{{end}}
	{{- if .Fanart}}<fanart><thumb>{{.Fanart.URL}}</thumb></fanart>{{end}}
</movie>`

	// Register templates with helper functions
	funcMap := template.FuncMap{
		"xmlEscape": html.EscapeString,
		"truncate": func(s string, length int) string {
			return truncateString(s, length)
		},
	}

	// Parse and register templates
	templates := map[string]string{
		"default":   defaultTemplate,
		"emby":      defaultTemplate,
		"jellyfin":  defaultTemplate,
		"kodi":      defaultTemplate,
		"plex":      defaultTemplate,
	}

	for name, tmplStr := range templates {
		tmpl, err := template.New(name).Funcs(funcMap).Parse(tmplStr)
		if err == nil {
			g.templates[name] = tmpl
		}
	}
}

// AddCustomTemplate adds a custom template
func (g *NFOGenerator) AddCustomTemplate(name, templateStr string) error {
	funcMap := template.FuncMap{
		"xmlEscape": html.EscapeString,
		"truncate": func(s string, length int) string {
			return truncateString(s, length)
		},
	}

	tmpl, err := template.New(name).Funcs(funcMap).Parse(templateStr)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	g.templates[name] = tmpl
	return nil
}

// ValidateTemplate validates a template string
func (g *NFOGenerator) ValidateTemplate(templateStr string) error {
	funcMap := template.FuncMap{
		"xmlEscape": html.EscapeString,
		"truncate": func(s string, length int) string {
			return truncateString(s, length)
		},
	}

	_, err := template.New("validation").Funcs(funcMap).Parse(templateStr)
	return err
}

// Helper functions

func formatDate(dateStr, format string) string {
	// Try to parse common date formats
	formats := []string{
		"2006-01-02",
		"2006/01/02",
		"2006.01.02",
		"20060102",
		"01/02/2006",
		"02/01/2006",
	}

	for _, inputFormat := range formats {
		if t, err := time.Parse(inputFormat, dateStr); err == nil {
			return t.Format(format)
		}
	}

	// If parsing fails, return original string
	return dateStr
}

func extractYear(dateStr string) int {
	// Try to extract year from date string
	if t, err := time.Parse("2006-01-02", dateStr); err == nil {
		return t.Year()
	}

	// Try other formats
	formats := []string{"2006/01/02", "2006.01.02", "20060102"}
	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t.Year()
		}
	}

	return 0
}

func parseRuntime(runtimeStr string) int {
	// Extract minutes from runtime string
	runtimeStr = strings.TrimSpace(runtimeStr)
	
	// Remove common suffixes
	runtimeStr = strings.TrimSuffix(runtimeStr, "分鐘")
	runtimeStr = strings.TrimSuffix(runtimeStr, "minutes")
	runtimeStr = strings.TrimSuffix(runtimeStr, "min")
	runtimeStr = strings.TrimSuffix(runtimeStr, "m")
	
	if minutes, err := strconv.Atoi(strings.TrimSpace(runtimeStr)); err == nil {
		return minutes
	}
	
	return 0
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	
	// Try to truncate at word boundary
	if maxLen > 10 {
		truncated := s[:maxLen-3]
		if lastSpace := strings.LastIndex(truncated, " "); lastSpace > maxLen/2 {
			return truncated[:lastSpace] + "..."
		}
	}
	
	return s[:maxLen-3] + "..."
}

func (g *NFOGenerator) calculateRating(movie *datatype.MovieInfo) float64 {
	// Simple rating calculation based on available information
	rating := 5.0 // Base rating
	
	// Boost rating based on completeness of information
	if movie.Title != "" {
		rating += 1.0
	}
	if len(movie.Actress) > 0 {
		rating += 1.0
	}
	if movie.Plot != "" {
		rating += 0.5
	}
	if movie.Cover != "" {
		rating += 0.5
	}
	if len(movie.Genre) > 0 {
		rating += 0.5
	}
	if movie.Director != "" {
		rating += 0.5
	}
	
	// Scale to configured rating scale
	if g.config.RatingScale == 5 {
		rating = rating / 2.0
	}
	
	// Ensure within bounds
	maxRating := float64(g.config.RatingScale)
	if rating > maxRating {
		rating = maxRating
	}
	if rating < 1.0 {
		rating = 1.0
	}
	
	return rating
}

func formatXML(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	
	// Add XML declaration if not present
	xmlStr := string(data)
	if !strings.HasPrefix(xmlStr, "<?xml") {
		buf.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
		buf.WriteString("\n")
	}
	
	buf.Write(data)
	return buf.Bytes(), nil
}

func writeFileWithBackup(filename string, content []byte) error {
	// Create directory if needed
	if err := ensureDir(filepath.Dir(filename)); err != nil {
		return err
	}

	// Create backup if file exists
	if fileExists(filename) {
		backupName := filename + ".backup"
		if err := copyFile(filename, backupName); err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
	}

	// Write new content
	return writeFile(filename, content)
}

// ensureDir creates directory if it doesn't exist
func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// fileExists checks if file exists
func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// writeFile writes content to file
func writeFile(filename string, content []byte) error {
	return os.WriteFile(filename, content, 0644)
}