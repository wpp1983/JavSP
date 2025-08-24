package main

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"javsp-go/internal/config"
	"javsp-go/internal/crawler"
	"javsp-go/internal/datatype"
	"javsp-go/internal/downloader"
	"javsp-go/internal/merger"
	"javsp-go/internal/nfo"
	"javsp-go/internal/organizer"
	"javsp-go/internal/scanner"
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
	
	for i, movie := range scanResult.Movies {
		fmt.Printf("[%d/%d] Processing: %s\n", i+1, len(scanResult.Movies), movie.FileName)
		processedCount++
		
		// Get movie ID for crawling
		movieID := movie.GetID()
		if movieID == "" {
			fmt.Printf("  ❌ No valid movie ID found, skipping\n\n")
			continue
		}
		
		// Step 3a: Crawl metadata
		fmt.Printf("  📡 Crawling metadata for ID: %s\n", movieID)
		crawlResults, err := crawlerEngine.CrawlMovie(ctx, movieID)
		if err != nil {
			fmt.Printf("  ❌ Crawling failed: %s\n\n", err)
			continue
		}
		
		// Filter successful crawl results
		var validMovieInfos []*datatype.MovieInfo
		for _, result := range crawlResults {
			if result.Error == nil && result.MovieInfo != nil {
				validMovieInfos = append(validMovieInfos, result.MovieInfo)
				fmt.Printf("  ✓ Got data from: %s\n", result.Source)
			} else {
				fmt.Printf("  ⚠ Failed from %s: %v\n", result.Source, result.Error)
			}
		}
		
		if len(validMovieInfos) == 0 {
			fmt.Printf("  ❌ No valid metadata found from any source\n\n")
			continue
		}
		
		// Step 3b: Merge metadata
		fmt.Printf("  🔗 Merging data from %d sources\n", len(validMovieInfos))
		mergeResult, err := merger.Merge(validMovieInfos)
		if err != nil {
			fmt.Printf("  ❌ Merging failed: %s\n\n", err)
			continue
		}
		
		// Assign merged info to movie
		movie.Info = mergeResult.MergedMovie
		fmt.Printf("  ✓ Merged metadata (Quality: %.1f%%)\n", mergeResult.MergeStats.QualityScore*100)
		
		// Step 3c: Generate NFO file
		nfoPath := getMovieNFOPath(movie, cfg)
		fmt.Printf("  📝 Generating NFO file: %s\n", nfoPath)
		if err := nfoGenerator.GenerateToFile(movie.Info, nfoPath); err != nil {
			fmt.Printf("  ⚠ NFO generation failed: %s\n", err)
		} else {
			fmt.Printf("  ✓ NFO file created\n")
		}
		
		// Step 3d: Download images
		if err := downloadMovieImages(ctx, movie, imageDownloader, cfg); err != nil {
			fmt.Printf("  ⚠ Image download failed: %s\n", err)
		} else {
			fmt.Printf("  ✓ Images downloaded\n")
		}
		
		// Step 3e: Organize file
		if cfg.Summarizer.MoveFiles {
			fmt.Printf("  📁 Organizing file\n")
			operation, err := fileOrganizer.OrganizeMovie(ctx, movie)
			if err != nil {
				fmt.Printf("  ⚠ File organization failed: %s\n", err)
			} else if operation.Status == organizer.StatusCompleted {
				fmt.Printf("  ✓ File moved to: %s\n", operation.Destination)
			} else {
				fmt.Printf("  ⚠ File organization status: %s\n", operation.Status)
			}
		} else {
			fmt.Printf("  ℹ File organization skipped (disabled in config)\n")
		}
		
		successCount++
		fmt.Printf("  ✅ Movie processed successfully\n\n")
		
		// Small delay between movies
		time.Sleep(100 * time.Millisecond)
	}
	
	// Final summary
	fmt.Println("=== Processing Complete ===")
	fmt.Printf("Movies processed: %d/%d\n", processedCount, len(scanResult.Movies))
	fmt.Printf("Successful: %d\n", successCount)
	fmt.Printf("Failed: %d\n", processedCount-successCount)
	
	// Show statistics
	fmt.Println("\n=== Statistics ===")
	if engineStats := crawlerEngine.GetStats(); engineStats != nil {
		fmt.Printf("Crawler requests: %d (success: %d, failed: %d)\n", 
			engineStats.TotalRequests, engineStats.SuccessfulCrawls, engineStats.FailedCrawls)
	}
	
	if downloadStats := imageDownloader.GetStats(); downloadStats != nil {
		fmt.Printf("Image downloads: %d (success: %d, failed: %d, skipped: %d)\n", 
			downloadStats.TotalDownloads, downloadStats.SuccessfulDownloads, 
			downloadStats.FailedDownloads, downloadStats.SkippedDownloads)
		fmt.Printf("Data downloaded: %.2f MB\n", float64(downloadStats.BytesDownloaded)/(1024*1024))
	}
	
	if organizerStats := fileOrganizer.GetStats(); organizerStats != nil {
		fmt.Printf("File operations: %d (completed: %d, failed: %d)\n", 
			organizerStats.TotalOperations, organizerStats.CompletedOperations, organizerStats.FailedOperations)
	}
	
	fmt.Printf("\n🎉 Processing completed successfully!\n")
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