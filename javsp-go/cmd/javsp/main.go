package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"javsp-go/internal/config"
	"javsp-go/internal/crawler"
	"javsp-go/internal/datatype"
	"javsp-go/internal/downloader"
	"javsp-go/internal/merger"
	"javsp-go/internal/nfo"
	"javsp-go/internal/organizer"
	"javsp-go/internal/scanner"
	"javsp-go/pkg/ui"
)

var (
	version = "v1.0.0"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	// Set the processing function
	config.RunJavSPProcessing = processJavSP
	config.Execute()
}

// processJavSP implements the main processing logic
func processJavSP(cfg *config.Config) error {
	ctx := context.Background()
	
	// Step 1: Scanning
	fmt.Println("=== Step 1: Scanning for video files ===")
	fileScanner, err := scanner.NewScanner(cfg)
	if err != nil {
		return fmt.Errorf("failed to create scanner: %w", err)
	}
	
	scanResult, err := fileScanner.Scan()
	if err != nil {
		return fmt.Errorf("scanning failed: %w", err)
	}
	
	// Display scan results
	fmt.Printf("Total files found: %d\n", scanResult.TotalFiles)
	fmt.Printf("Valid movies identified: %d\n", scanResult.ValidMovies)
	fmt.Printf("Files skipped: %d\n", scanResult.SkippedFiles)
	
	if len(scanResult.Errors) > 0 {
		fmt.Printf("Scan errors: %d\n", len(scanResult.Errors))
		for _, scanErr := range scanResult.Errors {
			fmt.Printf("  - %s: %s\n", scanErr.Path, scanErr.Message)
		}
	}
	
	if len(scanResult.Movies) == 0 {
		fmt.Println("No movies found to process.")
		return nil
	}
	
	fmt.Printf("Found %d movies to process.\n\n", len(scanResult.Movies))
	
	// Step 2: Initialize components
	fmt.Println("=== Step 2: Initializing processing components ===")
	
	// Initialize crawler engine
	crawlerEngine, err := crawler.NewEngine(cfg)
	if err != nil {
		return fmt.Errorf("failed to create crawler engine: %w", err)
	}
	defer crawlerEngine.Close()
	
	// Initialize merger
	mergeConfig := merger.NewDefaultMergeConfig()
	merger := merger.NewMerger(mergeConfig)
	
	// Initialize NFO generator
	nfoConfig := nfo.DefaultNFOConfig()
	nfoGenerator := nfo.NewNFOGenerator(nfoConfig)
	
	// Initialize file organizer
	organizerConfig := organizer.DefaultOrganizerConfig()
	// Set output directory from config
	if cfg.Summarizer.Path.OutputFolderPattern != "" {
		organizerConfig.OutputDir = cfg.Summarizer.Path.OutputFolderPattern
		organizerConfig.Pattern = cfg.Summarizer.Path.BasenamePattern
	}
	organizerConfig.CreateDirectories = true
	organizerConfig.OverwriteExisting = false
	organizerConfig.BackupOriginal = true
	organizerConfig.DryRun = cfg.Other.DryRun
	fileOrganizer := organizer.NewFileOrganizer(organizerConfig)
	
	// Initialize image downloader
	downloadConfig := downloader.DefaultDownloadConfig()
	downloadConfig.MaxConcurrency = 2 // Limit concurrent downloads
	downloadConfig.SkipExisting = true
	imageDownloader, err := downloader.NewImageDownloader(downloadConfig)
	if err != nil {
		return fmt.Errorf("failed to create image downloader: %w", err)
	}
	defer imageDownloader.Close()
	
	fmt.Println("All components initialized successfully.")
	
	// Step 3: Process each movie
	fmt.Println("=== Step 3: Processing movies ===")
	processedCount := 0
	successCount := 0
	
	// Initialize color system
	ui.SetColorEnabled(true) // Enable by default, can be controlled by config later
	
	// Create progress display
	progressDisplay := ui.NewMovieProgressDisplay(len(scanResult.Movies))
	displayLines := 0
	
	for i, movie := range scanResult.Movies {
		processedCount++
		
		// Start movie processing
		progressDisplay.StartMovie(i+1, movie.FileName)
		
		// Get movie ID for crawling
		movieID := movie.GetID()
		if movieID == "" {
			progressDisplay.UpdateStep(ui.Error("❌ No valid movie ID found"))
			progressDisplay.FinishMovie(false)
			
			// Display current state
			if displayLines > 0 {
				progressDisplay.MoveCursorUp(displayLines)
			}
			output := progressDisplay.Render()
			fmt.Print(output)
			displayLines = strings.Count(output, "\n")
			
			time.Sleep(1 * time.Second)
			continue
		}
		
		// Step 3a: Crawl metadata
		progressDisplay.UpdateStep(fmt.Sprintf("📡 Crawling metadata for ID: %s", ui.Highlight(movieID)))
		
		// Set up progress callback for enhanced crawler display
		crawlerEngine.SetProgressCallback(func(crawlerName, message string, prog float64, elapsed, remaining time.Duration) {
			if !cfg.Other.Progress.Enabled {
				return
			}
			
			var status string
			var attempt, maxAttempts int = 1, 1
			
			// Parse crawler message for status and attempt info
			if strings.Contains(message, "Connecting") {
				status = "connecting"
			} else if strings.Contains(message, "Retrying") {
				status = "retrying"
				// Extract attempt information
				if strings.Contains(message, "attempt") {
					fmt.Sscanf(message, "Retrying %*s (attempt %d/%d)", &attempt, &maxAttempts)
				}
			} else if strings.Contains(message, "Success") {
				status = "success"
			} else if strings.Contains(message, "Failed") {
				status = "failed"
			} else {
				status = "connecting"
			}
			
			// Calculate progress based on timeout
			if remaining > 0 {
				timeout := elapsed + remaining
				prog = float64(elapsed) / float64(timeout)
			}
			
			progressDisplay.UpdateCrawler(crawlerName, status, message, prog, elapsed, attempt, maxAttempts)
			
			// Update display
			if displayLines > 0 {
				progressDisplay.MoveCursorUp(displayLines)
			}
			output := progressDisplay.Render()
			fmt.Print(output)
			displayLines = strings.Count(output, "\n")
		})
		
		crawlResults, err := crawlerEngine.CrawlMovie(ctx, movieID)
		
		if err != nil {
			progressDisplay.UpdateStep(ui.Error("❌ Crawling failed: " + err.Error()))
			progressDisplay.FinishMovie(false)
			
			// Display current state
			if displayLines > 0 {
				progressDisplay.MoveCursorUp(displayLines)
			}
			output := progressDisplay.Render()
			fmt.Print(output)
			displayLines = strings.Count(output, "\n")
			
			time.Sleep(1 * time.Second)
			continue
		}
		
		// Process crawl results
		var validMovieInfos []*datatype.MovieInfo
		var successSources, failedSources []string
		
		for _, result := range crawlResults {
			if result.Error == nil && result.MovieInfo != nil {
				validMovieInfos = append(validMovieInfos, result.MovieInfo)
				successSources = append(successSources, fmt.Sprintf("%s (%.1fs)", result.Source, result.Duration.Seconds()))
				// Update crawler status to success
				progressDisplay.UpdateCrawler(result.Source, "success", "Success", 1.0, result.Duration, 1, 1)
			} else {
				failureReason := "unknown error"
				if result.Error != nil {
					failureReason = result.Error.Error()
					if len(failureReason) > 50 {
						failureReason = failureReason[:47] + "..."
					}
				}
				failedSources = append(failedSources, fmt.Sprintf("%s (%s)", result.Source, failureReason))
				// Update crawler status to failed
				progressDisplay.UpdateCrawler(result.Source, "failed", failureReason, 1.0, result.Duration, 1, 1)
			}
		}
		
		if len(validMovieInfos) == 0 {
			progressDisplay.UpdateStep(ui.Error("❌ No valid metadata found from any source"))
			progressDisplay.FinishMovie(false)
			
			// Display current state
			if displayLines > 0 {
				progressDisplay.MoveCursorUp(displayLines)
			}
			output := progressDisplay.Render()
			fmt.Print(output)
			displayLines = strings.Count(output, "\n")
			
			time.Sleep(1 * time.Second)
			continue
		}
		
		// Step 3b: Merge metadata
		progressDisplay.UpdateStep(fmt.Sprintf("🔗 Merging data from %d sources", len(validMovieInfos)))
		mergeResult, err := merger.Merge(validMovieInfos)
		if err != nil {
			progressDisplay.UpdateStep(ui.Error("❌ Merging failed: " + err.Error()))
			progressDisplay.FinishMovie(false)
			
			// Display current state
			if displayLines > 0 {
				progressDisplay.MoveCursorUp(displayLines)
			}
			output := progressDisplay.Render()
			fmt.Print(output)
			displayLines = strings.Count(output, "\n")
			
			time.Sleep(1 * time.Second)
			continue
		}
		
		// Assign merged info to movie
		movie.Info = mergeResult.MergedMovie
		
		// Step 3c: Generate NFO file
		nfoPath := getMovieNFOPath(movie, cfg)
		progressDisplay.UpdateStep(fmt.Sprintf("📝 Generating NFO file"))
		if err := nfoGenerator.GenerateToFile(movie.Info, nfoPath); err != nil {
			progressDisplay.UpdateStep(ui.Warning("⚠ NFO generation failed: " + err.Error()))
		}
		
		// Step 3d: Download images
		progressDisplay.UpdateStep("📥 Downloading images")
		if err := downloadMovieImages(ctx, movie, imageDownloader, cfg); err != nil {
			progressDisplay.UpdateStep(ui.Warning("⚠ Image download failed: " + err.Error()))
		}
		
		// Step 3e: Organize file
		if cfg.Summarizer.MoveFiles {
			progressDisplay.UpdateStep("📁 Organizing file")
			operation, err := fileOrganizer.OrganizeMovie(ctx, movie)
			if err != nil {
				progressDisplay.UpdateStep(ui.Warning("⚠ File organization failed: " + err.Error()))
			} else if operation.Status == organizer.StatusCompleted {
				progressDisplay.UpdateStep(ui.Success("✓ File moved to: " + operation.Destination))
			} else {
				progressDisplay.UpdateStep(ui.Warning("⚠ File organization status: " + string(operation.Status)))
			}
		} else {
			progressDisplay.UpdateStep(ui.DimText("ℹ File organization skipped (disabled in config)"))
		}
		
		// Mark as successful
		successCount++
		progressDisplay.UpdateStep(ui.Success("✅ Movie processed successfully"))
		progressDisplay.FinishMovie(true)
		
		// Final display update
		if displayLines > 0 {
			progressDisplay.MoveCursorUp(displayLines)
		}
		output := progressDisplay.Render()
		fmt.Print(output)
		displayLines = strings.Count(output, "\n")
		
		// Small delay between movies
		time.Sleep(500 * time.Millisecond)
	}
	
	// Clear display area for final summary
	fmt.Print("\n\n")
	
	// Collect statistics
	var crawlerStats, downloadStats, organizerStats map[string]interface{}
	
	if engineStats := crawlerEngine.GetStats(); engineStats != nil {
		crawlerStats = map[string]interface{}{
			"total_requests":    engineStats.TotalRequests,
			"successful_crawls": engineStats.SuccessfulCrawls,
			"failed_crawls":     engineStats.FailedCrawls,
		}
	}
	
	if imgStats := imageDownloader.GetStats(); imgStats != nil {
		downloadStats = map[string]interface{}{
			"total_downloads":      imgStats.TotalDownloads,
			"successful_downloads": imgStats.SuccessfulDownloads,
			"failed_downloads":     imgStats.FailedDownloads,
			"skipped_downloads":    imgStats.SkippedDownloads,
			"bytes_downloaded":     int64(imgStats.BytesDownloaded),
		}
	}
	
	if orgStats := fileOrganizer.GetStats(); orgStats != nil {
		organizerStats = map[string]interface{}{
			"total_operations":     orgStats.TotalOperations,
			"completed_operations": orgStats.CompletedOperations,
			"failed_operations":    orgStats.FailedOperations,
		}
	}
	
	// Display beautiful final summary
	finalSummary := progressDisplay.FinalSummary(crawlerStats, downloadStats, organizerStats)
	fmt.Println(finalSummary)
	return nil
}

// getMovieNFOPath generates the NFO file path for a movie
func getMovieNFOPath(movie *datatype.Movie, cfg *config.Config) string {
	// Use the same directory as the movie file
	movieDir := filepath.Dir(movie.FilePath)
	
	// Generate NFO filename based on config
	nfoBasename := cfg.Summarizer.NFO.BasenamePattern
	if nfoBasename == "" {
		nfoBasename = "movie"
	}
	
	nfoFilename := nfoBasename + ".nfo"
	return filepath.Join(movieDir, nfoFilename)
}

// downloadMovieImages downloads cover and preview images for a movie
func downloadMovieImages(ctx context.Context, movie *datatype.Movie, imageDownloader *downloader.ImageDownloader, cfg *config.Config) error {
	if movie.Info == nil {
		return fmt.Errorf("no movie info available")
	}
	
	downloads := make(map[string]string)
	movieDir := filepath.Dir(movie.FilePath)
	
	// Download cover image
	if movie.Info.Cover != "" {
		coverBasename := cfg.Summarizer.Cover.BasenamePattern
		if coverBasename == "" {
			coverBasename = "poster"
		}
		coverExt := downloader.GetFileExtensionFromURL(movie.Info.Cover)
		coverPath := filepath.Join(movieDir, coverBasename + coverExt)
		downloads[movie.Info.Cover] = coverPath
	}
	
	// Download fanart
	if movie.Info.Fanart != "" {
		fanartBasename := cfg.Summarizer.Fanart.BasenamePattern
		if fanartBasename == "" {
			fanartBasename = "fanart"
		}
		fanartExt := downloader.GetFileExtensionFromURL(movie.Info.Fanart)
		fanartPath := filepath.Join(movieDir, fanartBasename + fanartExt)
		downloads[movie.Info.Fanart] = fanartPath
	}
	
	// Download preview images if enabled
	if cfg.Summarizer.ExtraArts.Enabled && len(movie.Info.Preview) > 0 {
		// Create extrafanart directory
		extraFanartDir := filepath.Join(movieDir, "extrafanart")
		
		for i, previewURL := range movie.Info.Preview {
			if i >= 10 { // Limit to 10 preview images
				break
			}
			
			previewExt := downloader.GetFileExtensionFromURL(previewURL)
			previewFilename := fmt.Sprintf("fanart%d%s", i+1, previewExt)
			previewPath := filepath.Join(extraFanartDir, previewFilename)
			downloads[previewURL] = previewPath
		}
	}
	
	if len(downloads) == 0 {
		return nil // No images to download
	}
	
	// Perform batch download
	results, err := imageDownloader.DownloadBatch(ctx, downloads)
	if err != nil {
		return fmt.Errorf("batch download failed: %w", err)
	}
	
	// Check results
	successCount := 0
	for _, result := range results {
		if result.Error == nil && !result.Skipped {
			successCount++
		}
	}
	
	fmt.Printf("    Downloaded %d/%d images\n", successCount, len(downloads))
	return nil
}