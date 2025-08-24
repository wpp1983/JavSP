//go:build benchmark

package benchmark

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"javsp-go/internal/avid"
	"javsp-go/internal/config"
	"javsp-go/internal/scanner"
	"javsp-go/test/testutils"
)

func BenchmarkFileScanning(b *testing.B) {
	// Create temporary directory with many files
	tmpDir := testutils.CreateTempDir(&testing.T{})
	
	// Create large number of test files
	var testFiles []string
	for i := 0; i < 1000; i++ {
		testFiles = append(testFiles, fmt.Sprintf("STARS-%04d.mp4", i+1))
		testFiles = append(testFiles, fmt.Sprintf("SSIS-%04d.mkv", i+1))
		testFiles = append(testFiles, fmt.Sprintf("IPX-%04d.avi", i+1))
	}
	
	testutils.CreateTestFiles(&testing.T{}, tmpDir, testFiles)
	
	cfg := config.GetDefaultConfig()
	cfg.Scanner.InputDirectory = tmpDir
	cfg.Scanner.FilenameExtensions = []string{".mp4", ".mkv", ".avi"}
	cfg.Scanner.MinimumSize = "1B"
	
	fileScanner := scanner.NewScanner(cfg)
	
	b.ResetTimer()
	b.Run("ScanDirectory", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			files, err := fileScanner.ScanDirectory(tmpDir)
			if err != nil {
				b.Fatalf("Failed to scan directory: %v", err)
			}
			if len(files) == 0 {
				b.Error("No files found")
			}
		}
	})
	
	b.Run("FilterFiles", func(b *testing.B) {
		files, err := fileScanner.ScanDirectory(tmpDir)
		if err != nil {
			b.Fatalf("Failed to scan directory: %v", err)
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			filtered := 0
			for _, file := range files {
				if fileScanner.PassesFilters(file) {
					filtered++
				}
			}
			if filtered == 0 {
				b.Error("No files passed filters")
			}
		}
	})
}

func BenchmarkBatchProcessing(b *testing.B) {
	recognizer := avid.NewRecognizer()
	generator := testutils.NewTestDataGenerator()
	
	// Create large batch of filenames
	batchSizes := []int{100, 500, 1000, 5000}
	
	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("Batch_%d", batchSize), func(b *testing.B) {
			filenames := make([]string, batchSize)
			for i := 0; i < batchSize; i++ {
				filenames[i] = fmt.Sprintf("STARS-%04d.mp4", i+1)
			}
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				recognized := 0
				for _, filename := range filenames {
					if avid := recognizer.Recognize(filename); avid != "" {
						recognized++
					}
				}
				if recognized == 0 {
					b.Error("No files recognized")
				}
			}
		})
	}
}

func BenchmarkParallelProcessing(b *testing.B) {
	recognizer := avid.NewRecognizer()
	
	// Create test data
	testFilenames := make([]string, 10000)
	for i := 0; i < 10000; i++ {
		switch i % 5 {
		case 0:
			testFilenames[i] = fmt.Sprintf("STARS-%04d.mp4", i+1)
		case 1:
			testFilenames[i] = fmt.Sprintf("SSIS-%04d.mkv", i+1)
		case 2:
			testFilenames[i] = fmt.Sprintf("IPX-%04d.avi", i+1)
		case 3:
			testFilenames[i] = fmt.Sprintf("FC2-PPV-%07d.mp4", 1000000+i)
		case 4:
			testFilenames[i] = fmt.Sprintf("300MIUM-%03d.mp4", i%999+1)
		}
	}
	
	b.Run("Sequential", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			recognized := 0
			for _, filename := range testFilenames {
				if avid := recognizer.Recognize(filename); avid != "" {
					recognized++
				}
			}
			if recognized == 0 {
				b.Error("No files recognized")
			}
		}
	})
	
	b.Run("Parallel", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				filename := testFilenames[i%len(testFilenames)]
				recognizer.Recognize(filename)
				i++
			}
		})
	})
}

func BenchmarkConfigOperations(b *testing.B) {
	tmpDir := testutils.CreateTempDir(&testing.T{})
	
	b.Run("DefaultConfig", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cfg := config.GetDefaultConfig()
			if cfg == nil {
				b.Error("Config should not be nil")
			}
		}
	})
	
	b.Run("ConfigValidation", func(b *testing.B) {
		cfg := config.GetDefaultConfig()
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := config.ValidateConfig(cfg); err != nil {
				b.Fatalf("Config validation failed: %v", err)
			}
		}
	})
	
	b.Run("ConfigSerialization", func(b *testing.B) {
		cfg := config.GetDefaultConfig()
		cfg.Scanner.InputDirectory = tmpDir
		
		for i := 0; i < b.N; i++ {
			configPath := filepath.Join(tmpDir, fmt.Sprintf("config_%d.yml", i))
			
			// Save config
			if err := config.SaveConfig(cfg, configPath); err != nil {
				b.Fatalf("Failed to save config: %v", err)
			}
			
			// Clean up
			os.Remove(configPath)
		}
	})
	
	b.Run("ConfigDeserialization", func(b *testing.B) {
		cfg := config.GetDefaultConfig()
		cfg.Scanner.InputDirectory = tmpDir
		
		configPath := filepath.Join(tmpDir, "benchmark_config.yml")
		if err := config.SaveConfig(cfg, configPath); err != nil {
			b.Fatalf("Failed to save initial config: %v", err)
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			config.ResetConfig()
			_, err := config.LoadFromFile(configPath)
			if err != nil {
				b.Fatalf("Failed to load config: %v", err)
			}
		}
	})
}

func BenchmarkDataStructures(b *testing.B) {
	generator := testutils.NewTestDataGenerator()
	
	b.Run("MovieCreation", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			movieID := fmt.Sprintf("TEST-%06d", i)
			movie := generator.GenerateMovie(fmt.Sprintf("/test/%s.mp4", movieID), movieID)
			if movie == nil {
				b.Error("Movie should not be nil")
			}
		}
	})
	
	b.Run("MovieInfoCreation", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			movieID := fmt.Sprintf("TEST-%06d", i)
			info := generator.GenerateMovieInfo(movieID)
			if info == nil {
				b.Error("MovieInfo should not be nil")
			}
		}
	})
	
	b.Run("BatchMovieCreation", func(b *testing.B) {
		batchSizes := []int{10, 100, 1000}
		
		for _, batchSize := range batchSizes {
			b.Run(fmt.Sprintf("Batch_%d", batchSize), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					movies := generator.CreateTestMovies(batchSize)
					if len(movies) != batchSize {
						b.Errorf("Expected %d movies, got %d", batchSize, len(movies))
					}
				}
			})
		}
	})
}

func BenchmarkStringProcessing(b *testing.B) {
	testStrings := []string{
		"[JavBus] STARS-123 桃乃木かな タイトル名前 1080p.mp4",
		"www.javbus.com_SSIS-001_深田えいみ_長いタイトル名前_HD.mkv",
		"【高清】IPX-177 天海つばさ スペシャル版 [中文字幕].avi",
		"FC2-PPV-1234567 素人美女 個人撮影 4K.mp4",
		"300MIUM-001 ナンパ企画 美女 ハメ撮り.mp4",
		"複雜的中文文件名無法識別.mp4",
		"very_long_filename_without_any_recognizable_pattern.mp4",
	}
	
	recognizer := avid.NewRecognizer()
	
	b.Run("FilenameRecognition", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				filename := testStrings[i%len(testStrings)]
				recognizer.Recognize(filename)
				i++
			}
		})
	})
	
	b.Run("StringCleaning", func(b *testing.B) {
		testTexts := []string{
			"  Text with  multiple   spaces  ",
			"Text\nwith\nnewlines",
			"・Prefixed text with special chars・",
			"VERY LONG TEXT WITH MANY WORDS AND CHARACTERS THAT NEEDS CLEANING",
		}
		
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				text := testTexts[i%len(testTexts)]
				web.CleanText(text)
				i++
			}
		})
	})
}

func BenchmarkMemoryIntensive(b *testing.B) {
	b.Run("LargeFileList", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Simulate processing large number of files
			files := make([]string, 10000)
			for j := 0; j < 10000; j++ {
				files[j] = fmt.Sprintf("/test/STARS-%04d.mp4", j+1)
			}
			
			// Process files
			recognizer := avid.NewRecognizer()
			recognized := 0
			for _, file := range files {
				filename := filepath.Base(file)
				if avid := recognizer.Recognize(filename); avid != "" {
					recognized++
				}
			}
			
			if recognized == 0 {
				b.Error("No files recognized")
			}
			
			// Force cleanup
			files = nil
		}
	})
	
	b.Run("LargeMovieDatabase", func(b *testing.B) {
		generator := testutils.NewTestDataGenerator()
		
		for i := 0; i < b.N; i++ {
			// Create large dataset
			movies := generator.CreateTestMovies(1000)
			
			// Process all movies
			processedCount := 0
			for _, movie := range movies {
				if movie.DVDID != "" {
					processedCount++
				}
				if movie.Info != nil && movie.Info.Title != "" {
					processedCount++
				}
			}
			
			if processedCount == 0 {
				b.Error("No movies processed")
			}
			
			// Force cleanup
			movies = nil
		}
	})
}

func BenchmarkConcurrencyPatterns(b *testing.B) {
	generator := testutils.NewTestDataGenerator()
	testMovies := generator.CreateTestMovies(1000)
	
	b.Run("WorkerPool", func(b *testing.B) {
		workerCount := 10
		jobChan := make(chan string, len(testMovies))
		resultChan := make(chan string, len(testMovies))
		
		// Start workers
		for w := 0; w < workerCount; w++ {
			go func() {
				recognizer := avid.NewRecognizer()
				for filename := range jobChan {
					if avid := recognizer.Recognize(filename); avid != "" {
						resultChan <- avid
					} else {
						resultChan <- ""
					}
				}
			}()
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Send jobs
			for _, movie := range testMovies {
				filename := filepath.Base(movie.FilePath)
				jobChan <- filename
			}
			
			// Collect results
			for j := 0; j < len(testMovies); j++ {
				<-resultChan
			}
		}
	})
	
	b.Run("FanOutFanIn", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Fan out to multiple goroutines
			numGoroutines := 10
			chunkSize := len(testMovies) / numGoroutines
			resultChans := make([]chan string, numGoroutines)
			
			for g := 0; g < numGoroutines; g++ {
				resultChans[g] = make(chan string, chunkSize+1)
				go func(goroutineIndex int, results chan string) {
					defer close(results)
					recognizer := avid.NewRecognizer()
					
					start := goroutineIndex * chunkSize
					end := start + chunkSize
					if end > len(testMovies) {
						end = len(testMovies)
					}
					
					for j := start; j < end; j++ {
						filename := filepath.Base(testMovies[j].FilePath)
						if avid := recognizer.Recognize(filename); avid != "" {
							results <- avid
						}
					}
				}(g, resultChans[g])
			}
			
			// Fan in - collect all results
			totalResults := 0
			for _, ch := range resultChans {
				for result := range ch {
					if result != "" {
						totalResults++
					}
				}
			}
			
			if totalResults == 0 {
				b.Error("No results collected")
			}
		}
	})
}

func BenchmarkCachePatterns(b *testing.B) {
	// Simple cache simulation
	cache := make(map[string]string)
	recognizer := avid.NewRecognizer()
	
	testFilenames := []string{
		"STARS-123.mp4", "SSIS-001.mkv", "IPX-177.avi", "FC2-PPV-1234567.mp4",
		"300MIUM-001.mp4", "GANA-2156.mp4", "259LUXU-1234.mp4", "HEYDOUGA-4017-123.mp4",
	}
	
	b.Run("WithoutCache", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				filename := testFilenames[i%len(testFilenames)]
				recognizer.Recognize(filename)
				i++
			}
		})
	})
	
	b.Run("WithCache", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				filename := testFilenames[i%len(testFilenames)]
				
				// Check cache first
				if avid, exists := cache[filename]; exists {
					_ = avid
				} else {
					avid := recognizer.Recognize(filename)
					cache[filename] = avid
				}
				i++
			}
		})
	})
}

func BenchmarkErrorHandling(b *testing.B) {
	b.Run("ErrorCreation", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			err := fmt.Errorf("test error %d", i)
			if err == nil {
				b.Error("Error should not be nil")
			}
		}
	})
	
	b.Run("ErrorWrapping", func(b *testing.B) {
		baseErr := fmt.Errorf("base error")
		
		for i := 0; i < b.N; i++ {
			err := fmt.Errorf("wrapped error %d: %w", i, baseErr)
			if err == nil {
				b.Error("Error should not be nil")
			}
		}
	})
}

func BenchmarkContextUsage(b *testing.B) {
	b.Run("ContextCreation", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			if ctx == nil {
				b.Error("Context should not be nil")
			}
			cancel()
		}
	})
	
	b.Run("ContextPropagation", func(b *testing.B) {
		baseCtx := context.Background()
		
		for i := 0; i < b.N; i++ {
			ctx := context.WithValue(baseCtx, "key", i)
			value := ctx.Value("key")
			if value != i {
				b.Errorf("Expected %d, got %v", i, value)
			}
		}
	})
}