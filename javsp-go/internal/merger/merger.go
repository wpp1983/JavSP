package merger

import (
	"fmt"
	"sort"

	"javsp-go/internal/datatype"
)

// MergeStrategy defines how to merge conflicting data
type MergeStrategy int

const (
	// StrategyPreferFirst prefers the first non-empty value
	StrategyPreferFirst MergeStrategy = iota
	// StrategyPreferLongest prefers the longest text value
	StrategyPreferLongest
	// StrategyPreferMostRecent prefers the most recently scraped data
	StrategyPreferMostRecent
	// StrategyPreferBestQuality prefers higher quality data based on source ranking
	StrategyPreferBestQuality
	// StrategyCombine combines all values where applicable
	StrategyCombine
)

// SourceRanking defines the quality ranking of different sources
type SourceRanking map[string]int

// DefaultSourceRanking provides default quality rankings for known sources
func DefaultSourceRanking() SourceRanking {
	return SourceRanking{
		"javbus2": 10,
		"avwiki":  8,
		"javdb":   9,
		"javlib":  7,
		"dmm":     6,
		"fc2":     5,
		"unknown": 1,
	}
}

// MergeConfig contains configuration for the merger
type MergeConfig struct {
	DefaultStrategy  MergeStrategy   `json:"default_strategy"`
	FieldStrategies  map[string]MergeStrategy `json:"field_strategies"`
	SourceRanking    SourceRanking   `json:"source_ranking"`
	RequiredFields   []string        `json:"required_fields"`
	PreferredSources []string        `json:"preferred_sources"`
	MaxActressCount  int             `json:"max_actress_count"`
	MaxGenreCount    int             `json:"max_genre_count"`
}

// NewDefaultMergeConfig creates a default merge configuration
func NewDefaultMergeConfig() *MergeConfig {
	return &MergeConfig{
		DefaultStrategy: StrategyPreferBestQuality,
		FieldStrategies: map[string]MergeStrategy{
			"title":       StrategyPreferLongest,
			"plot":        StrategyPreferLongest,
			"actress":     StrategyCombine,
			"genre":       StrategyCombine,
			"preview":     StrategyCombine,
			"cover":       StrategyPreferBestQuality,
			"source_url":  StrategyPreferFirst,
		},
		SourceRanking:    DefaultSourceRanking(),
		RequiredFields:   []string{"title", "dvdid"},
		PreferredSources: []string{"javbus2", "javdb", "avwiki"},
		MaxActressCount:  10,
		MaxGenreCount:    15,
	}
}

// Merger handles merging movie information from multiple sources
type Merger struct {
	config *MergeConfig
}

// NewMerger creates a new merger with the given configuration
func NewMerger(config *MergeConfig) *Merger {
	if config == nil {
		config = NewDefaultMergeConfig()
	}
	return &Merger{config: config}
}

// MergeResult contains the result of a merge operation
type MergeResult struct {
	MergedMovie    *datatype.MovieInfo `json:"merged_movie"`
	SourcesUsed    []string            `json:"sources_used"`
	ConflictsFound map[string][]string `json:"conflicts_found"`
	MergeStats     *MergeStats         `json:"merge_stats"`
}

// MergeStats contains statistics about the merge operation
type MergeStats struct {
	TotalSources      int                      `json:"total_sources"`
	SuccessfulSources int                      `json:"successful_sources"`
	FieldsMerged      map[string]int           `json:"fields_merged"`
	StrategiesUsed    map[string]MergeStrategy `json:"strategies_used"`
	QualityScore      float64                  `json:"quality_score"`
}

// Merge combines multiple MovieInfo objects into a single optimized result
func (m *Merger) Merge(movies []*datatype.MovieInfo) (*MergeResult, error) {
	if len(movies) == 0 {
		return nil, fmt.Errorf("no movies to merge")
	}

	// Filter out nil movies and validate
	validMovies := make([]*datatype.MovieInfo, 0, len(movies))
	for _, movie := range movies {
		if movie != nil && movie.DVDID != "" {
			validMovies = append(validMovies, movie)
		}
	}

	if len(validMovies) == 0 {
		return nil, fmt.Errorf("no valid movies to merge")
	}

	// If only one movie, still process it through merge logic to apply limits and cleanup
	// This ensures consistency in behavior

	// Sort movies by source quality
	sortedMovies := m.sortByQuality(validMovies)

	// Initialize result
	result := &MergeResult{
		MergedMovie:    datatype.NewMovieInfo(sortedMovies[0].DVDID),
		SourcesUsed:    make([]string, 0, len(sortedMovies)),
		ConflictsFound: make(map[string][]string),
		MergeStats: &MergeStats{
			TotalSources:      len(sortedMovies),
			SuccessfulSources: len(sortedMovies),
			FieldsMerged:      make(map[string]int),
			StrategiesUsed:    make(map[string]MergeStrategy),
			QualityScore:      0.0,
		},
	}

	// Track sources used
	for _, movie := range sortedMovies {
		result.SourcesUsed = append(result.SourcesUsed, movie.Source)
	}

	// Merge each field
	m.mergeTitle(sortedMovies, result)
	m.mergePlot(sortedMovies, result)
	m.mergeActresses(sortedMovies, result)
	m.mergeGenres(sortedMovies, result)
	m.mergeCover(sortedMovies, result)
	m.mergePreviews(sortedMovies, result)
	m.mergeBasicFields(sortedMovies, result)
	m.mergeDates(sortedMovies, result)

	// Calculate quality score
	result.MergeStats.QualityScore = m.calculateQualityScore(result.MergedMovie, result.MergeStats)

	// Validate the merged result
	if err := result.MergedMovie.Validate(); err != nil {
		return nil, fmt.Errorf("merged movie validation failed: %w", err)
	}

	return result, nil
}

// sortByQuality sorts movies by source quality ranking
func (m *Merger) sortByQuality(movies []*datatype.MovieInfo) []*datatype.MovieInfo {
	sorted := make([]*datatype.MovieInfo, len(movies))
	copy(sorted, movies)

	sort.Slice(sorted, func(i, j int) bool {
		rankI := m.config.SourceRanking[sorted[i].Source]
		rankJ := m.config.SourceRanking[sorted[j].Source]
		if rankI == 0 {
			rankI = m.config.SourceRanking["unknown"]
		}
		if rankJ == 0 {
			rankJ = m.config.SourceRanking["unknown"]
		}
		return rankI > rankJ
	})

	return sorted
}

// mergeTitle merges title information
func (m *Merger) mergeTitle(movies []*datatype.MovieInfo, result *MergeResult) {
	strategy := m.getFieldStrategy("title")
	result.MergeStats.StrategiesUsed["title"] = strategy

	titles := make([]string, 0, len(movies))
	for _, movie := range movies {
		if movie.Title != "" {
			titles = append(titles, movie.Title)
		}
	}

	if len(titles) == 0 {
		return
	}

	var selectedTitle string
	switch strategy {
	case StrategyPreferFirst:
		selectedTitle = titles[0]
	case StrategyPreferLongest:
		selectedTitle = m.selectLongest(titles)
	case StrategyPreferBestQuality:
		selectedTitle = titles[0] // Already sorted by quality
	}

	result.MergedMovie.Title = selectedTitle
	result.MergeStats.FieldsMerged["title"] = len(titles)

	// Track conflicts
	if len(m.getUniqueStrings(titles)) > 1 {
		result.ConflictsFound["title"] = titles
	}
}

// mergePlot merges plot information
func (m *Merger) mergePlot(movies []*datatype.MovieInfo, result *MergeResult) {
	strategy := m.getFieldStrategy("plot")
	result.MergeStats.StrategiesUsed["plot"] = strategy

	plots := make([]string, 0, len(movies))
	for _, movie := range movies {
		if movie.Plot != "" {
			plots = append(plots, movie.Plot)
		}
	}

	if len(plots) == 0 {
		return
	}

	var selectedPlot string
	switch strategy {
	case StrategyPreferFirst:
		selectedPlot = plots[0]
	case StrategyPreferLongest:
		selectedPlot = m.selectLongest(plots)
	case StrategyPreferBestQuality:
		selectedPlot = plots[0]
	}

	result.MergedMovie.Plot = selectedPlot
	result.MergeStats.FieldsMerged["plot"] = len(plots)

	if len(m.getUniqueStrings(plots)) > 1 {
		result.ConflictsFound["plot"] = plots
	}
}

// mergeActresses merges actress information
func (m *Merger) mergeActresses(movies []*datatype.MovieInfo, result *MergeResult) {
	strategy := m.getFieldStrategy("actress")
	result.MergeStats.StrategiesUsed["actress"] = strategy

	allActresses := make([]string, 0)
	for _, movie := range movies {
		allActresses = append(allActresses, movie.Actress...)
	}

	if len(allActresses) == 0 {
		return
	}

	var finalActresses []string
	switch strategy {
	case StrategyPreferFirst:
		if len(movies[0].Actress) > 0 {
			finalActresses = movies[0].Actress
		}
	case StrategyCombine:
		finalActresses = m.combineAndDeduplicate(allActresses)
	case StrategyPreferBestQuality:
		// Use actresses from the highest quality source that has any
		for _, movie := range movies {
			if len(movie.Actress) > 0 {
				finalActresses = movie.Actress
				break
			}
		}
	}

	// Apply max count limit
	if m.config.MaxActressCount > 0 && len(finalActresses) > m.config.MaxActressCount {
		finalActresses = finalActresses[:m.config.MaxActressCount]
	}

	result.MergedMovie.Actress = finalActresses
	result.MergeStats.FieldsMerged["actress"] = len(allActresses)
}

// mergeGenres merges genre information
func (m *Merger) mergeGenres(movies []*datatype.MovieInfo, result *MergeResult) {
	strategy := m.getFieldStrategy("genre")
	result.MergeStats.StrategiesUsed["genre"] = strategy

	allGenres := make([]string, 0)
	for _, movie := range movies {
		allGenres = append(allGenres, movie.Genre...)
	}

	if len(allGenres) == 0 {
		return
	}

	var finalGenres []string
	switch strategy {
	case StrategyPreferFirst:
		if len(movies[0].Genre) > 0 {
			finalGenres = movies[0].Genre
		}
	case StrategyCombine:
		finalGenres = m.combineAndDeduplicate(allGenres)
	case StrategyPreferBestQuality:
		for _, movie := range movies {
			if len(movie.Genre) > 0 {
				finalGenres = movie.Genre
				break
			}
		}
	}

	// Apply max count limit
	if m.config.MaxGenreCount > 0 && len(finalGenres) > m.config.MaxGenreCount {
		finalGenres = finalGenres[:m.config.MaxGenreCount]
	}

	result.MergedMovie.Genre = finalGenres
	result.MergeStats.FieldsMerged["genre"] = len(allGenres)
}

// mergeCover merges cover image information
func (m *Merger) mergeCover(movies []*datatype.MovieInfo, result *MergeResult) {
	strategy := m.getFieldStrategy("cover")
	result.MergeStats.StrategiesUsed["cover"] = strategy

	covers := make([]string, 0, len(movies))
	for _, movie := range movies {
		if movie.Cover != "" {
			covers = append(covers, movie.Cover)
		}
	}

	if len(covers) == 0 {
		return
	}

	switch strategy {
	case StrategyPreferFirst:
		result.MergedMovie.Cover = covers[0]
	case StrategyPreferBestQuality:
		result.MergedMovie.Cover = covers[0] // Already sorted by quality
	}

	result.MergeStats.FieldsMerged["cover"] = len(covers)

	if len(m.getUniqueStrings(covers)) > 1 {
		result.ConflictsFound["cover"] = covers
	}
}

// mergePreviews merges preview images
func (m *Merger) mergePreviews(movies []*datatype.MovieInfo, result *MergeResult) {
	strategy := m.getFieldStrategy("preview")
	result.MergeStats.StrategiesUsed["preview"] = strategy

	allPreviews := make([]string, 0)
	for _, movie := range movies {
		allPreviews = append(allPreviews, movie.Preview...)
	}

	if len(allPreviews) == 0 {
		return
	}

	switch strategy {
	case StrategyCombine:
		result.MergedMovie.Preview = m.combineAndDeduplicate(allPreviews)
	case StrategyPreferBestQuality:
		for _, movie := range movies {
			if len(movie.Preview) > 0 {
				result.MergedMovie.Preview = movie.Preview
				break
			}
		}
	}

	result.MergeStats.FieldsMerged["preview"] = len(allPreviews)
}

// mergeBasicFields merges basic string fields
func (m *Merger) mergeBasicFields(movies []*datatype.MovieInfo, result *MergeResult) {
	fields := []struct {
		name   string
		getter func(*datatype.MovieInfo) string
		setter func(*datatype.MovieInfo, string)
	}{
		{"director", func(m *datatype.MovieInfo) string { return m.Director }, 
		 func(m *datatype.MovieInfo, v string) { m.Director = v }},
		{"producer", func(m *datatype.MovieInfo) string { return m.Producer },
		 func(m *datatype.MovieInfo, v string) { m.Producer = v }},
		{"publisher", func(m *datatype.MovieInfo) string { return m.Publisher },
		 func(m *datatype.MovieInfo, v string) { m.Publisher = v }},
		{"series", func(m *datatype.MovieInfo) string { return m.Series },
		 func(m *datatype.MovieInfo, v string) { m.Series = v }},
		{"runtime", func(m *datatype.MovieInfo) string { return m.Runtime },
		 func(m *datatype.MovieInfo, v string) { m.Runtime = v }},
	}

	for _, field := range fields {
		strategy := m.getFieldStrategy(field.name)
		result.MergeStats.StrategiesUsed[field.name] = strategy

		values := make([]string, 0, len(movies))
		for _, movie := range movies {
			if value := field.getter(movie); value != "" {
				values = append(values, value)
			}
		}

		if len(values) == 0 {
			continue
		}

		var selectedValue string
		switch strategy {
		case StrategyPreferFirst:
			selectedValue = values[0]
		case StrategyPreferLongest:
			selectedValue = m.selectLongest(values)
		case StrategyPreferBestQuality:
			selectedValue = values[0]
		}

		field.setter(result.MergedMovie, selectedValue)
		result.MergeStats.FieldsMerged[field.name] = len(values)

		if len(m.getUniqueStrings(values)) > 1 {
			result.ConflictsFound[field.name] = values
		}
	}
}

// mergeDates merges date fields
func (m *Merger) mergeDates(movies []*datatype.MovieInfo, result *MergeResult) {
	// Merge release date
	releaseDates := make([]string, 0, len(movies))
	for _, movie := range movies {
		if movie.ReleaseDate != "" {
			releaseDates = append(releaseDates, movie.ReleaseDate)
		}
	}

	if len(releaseDates) > 0 {
		result.MergedMovie.ReleaseDate = releaseDates[0] // Prefer first valid date
		result.MergeStats.FieldsMerged["release_date"] = len(releaseDates)
		
		if len(m.getUniqueStrings(releaseDates)) > 1 {
			result.ConflictsFound["release_date"] = releaseDates
		}
	}

	// Set other metadata
	result.MergedMovie.Source = "merged"
	result.MergedMovie.SourceURL = ""
	result.MergedMovie.Uncensored = movies[0].Uncensored // Use first movie's censorship status
}

// Helper methods

func (m *Merger) getFieldStrategy(fieldName string) MergeStrategy {
	if strategy, exists := m.config.FieldStrategies[fieldName]; exists {
		return strategy
	}
	return m.config.DefaultStrategy
}

func (m *Merger) selectLongest(values []string) string {
	if len(values) == 0 {
		return ""
	}
	
	longest := values[0]
	for _, value := range values[1:] {
		if len(value) > len(longest) {
			longest = value
		}
	}
	return longest
}

func (m *Merger) combineAndDeduplicate(values []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(values))
	
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	
	return result
}

func (m *Merger) getUniqueStrings(values []string) []string {
	return m.combineAndDeduplicate(values)
}

func (m *Merger) calculateQualityScore(movie *datatype.MovieInfo, stats *MergeStats) float64 {
	score := 0.0
	maxScore := 0.0

	// Check each important field
	fieldWeights := map[string]float64{
		"title":        1.0,
		"actress":      0.8,
		"cover":        0.6,
		"plot":         0.4,
		"genre":        0.3,
		"release_date": 0.3,
		"director":     0.2,
		"producer":     0.2,
	}

	for field, weight := range fieldWeights {
		maxScore += weight
		
		switch field {
		case "title":
			if movie.Title != "" {
				score += weight
			}
		case "actress":
			if len(movie.Actress) > 0 {
				score += weight
			}
		case "cover":
			if movie.Cover != "" {
				score += weight
			}
		case "plot":
			if movie.Plot != "" {
				score += weight
			}
		case "genre":
			if len(movie.Genre) > 0 {
				score += weight
			}
		case "release_date":
			if movie.ReleaseDate != "" {
				score += weight
			}
		case "director":
			if movie.Director != "" {
				score += weight
			}
		case "producer":
			if movie.Producer != "" {
				score += weight
			}
		}
	}

	if maxScore > 0 {
		return score / maxScore
	}
	return 0.0
}