package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"golang.org/x/time/rate"
	"google.golang.org/genai"
	"gopkg.in/yaml.v3"
)

type Rule struct {
	Pattern string `yaml:"pattern"`
	Target  string `yaml:"target"`
}

type GptConfig struct {
	Enabled      bool   `yaml:"enabled"`
	ApiKey       string `yaml:"api_key"`
	Model        string `yaml:"model"`
	Instructions string `yaml:"instructions"`
}

type Config struct {
	Ignore IgnoreConfig `yaml:"ignore"`
	Rules  []Rule       `yaml:"rules"`
	Gpt    GptConfig    `yaml:"gpt"`
}

type Job struct {
	filename string
	resultCh chan string
}

type IgnoreConfig struct {
	OSDefaults bool     `yaml:"os_defaults"`
	Files      []string `yaml:"files"`
	Extensions []string `yaml:"extensions"`
	Folders    []string `yaml:"folders"`
}

func isIgnored(path string, cfg IgnoreConfig) bool {
	base := filepath.Base(path)

	// ignore prefixed "._"
	if strings.HasPrefix(base, "._") {
		return true
	}

	// OS defaults
	if cfg.OSDefaults {
		defaults := []string{".DS_Store", "Thumbs.db", "desktop.ini"}
		for _, ign := range defaults {
			if base == ign {
				return true
			}
		}
	}

	// explicit filenames
	for _, ign := range cfg.Files {
		if base == ign {
			return true
		}
	}

	// extensions
	ext := strings.ToLower(filepath.Ext(base))
	for _, ignExt := range cfg.Extensions {
		if strings.ToLower(ignExt) == ext {
			return true
		}
	}

	// folders
	for _, folder := range cfg.Folders {
		if strings.Contains(path, folder) {
			return true
		}
	}

	return false
}

func getGenAIClient(apiKey string) *genai.Client {
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Fatalf("Failed to create GenAI client: %v", err)
	}
	return client
}

var (
	jobQueue = make(chan Job, 100)
	limiter  = rate.NewLimiter(rate.Every(3*time.Second), 1)
)

func getFolderStructure(root string) string {
	var b strings.Builder
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			rel, _ := filepath.Rel(root, path)
			if rel == "." {
				return nil
			}
			b.WriteString(rel + "\n")
		}
		return nil
	})
	return b.String()
}

func getFileMetadata(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	ext := filepath.Ext(path)
	size := info.Size()

	return fmt.Sprintf("Extension: %s, Size: %d bytes", ext, size)
}

func suggestFolderWithGenAI(ctx context.Context, client *genai.Client, modelName, instructions string) {
	go func() {
		for job := range jobQueue {
			// rate limit
			if err := limiter.Wait(ctx); err != nil {
				log.Println("Rate limiter error:", err)
				job.resultCh <- "Unsorted"
				continue
			}
			folders := getFolderStructure("entropy")
			metadata := getFileMetadata(job.filename)

			prompt := fmt.Sprintf(`%s 
			Filename: %s
			Metadata: %s
			Existing folder structure: %s`, instructions, filepath.Base(job.filename), metadata, folders)
			log.Println(prompt)

			resp, err := client.Models.GenerateContent(ctx, modelName, genai.Text(prompt), nil)
			if err != nil {
				log.Println("GenAI error:", err)
				job.resultCh <- "Unsorted"
				continue
			}

			text := strings.TrimSpace(resp.Text())
			if text == "" {
				job.resultCh <- "Unsorted"
			} else {
				job.resultCh <- text
			}
		}
	}()
}

func loadConfig(path string) Config {
	// TODO: have a config file , rules and knowledge / custom instructions
	// TODO: add config to preserve folder structure ( no new folders are genrated )
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("couldn't open file %s: %v", path, err)
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Fatalf("Invalid YAML: %v", err)
	}

	return config
}

func matchRules(filename string, rules []Rule) string {
	for _, rule := range rules {
		re := regexp.MustCompile(rule.Pattern)
		if re.MatchString(filename) {
			return rule.Target
		}
	}
	return ""
}

func organizeItem(srcPath, targetFolder string) {
	base := filepath.Base(srcPath)
	destDir := filepath.Join("entropy", targetFolder)

	if err := os.MkdirAll(destDir, os.ModePerm); err != nil {
		log.Printf("Failed to create dir %s: %v", destDir, err)
		return
	}

	destPath := filepath.Join(destDir, base)
	if err := os.Rename(srcPath, destPath); err != nil {
		log.Printf("Failed to move %s: %v", base, err)
		return
	}

	log.Printf("Moved %s → %s", base, destDir)
}

func main() {
	os.MkdirAll("entropy", os.ModePerm)

	config := loadConfig("rules.yaml")

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	defer watcher.Close()
	err = watcher.Add("entropy")

	if err != nil {
		log.Fatal(err)
	}

	var client *genai.Client
	if config.Gpt.Enabled {
		client = getGenAIClient(config.Gpt.ApiKey)
		suggestFolderWithGenAI(context.Background(), client, config.Gpt.Model, config.Gpt.Instructions)
	}

	log.Println("Watching 'entropy' folder...")

	for {
		select {
		case event := <-watcher.Events:
			if event.Op&fsnotify.Create == fsnotify.Create {

				// skips directories
				fi, err := os.Stat(event.Name)
				if err == nil && fi.IsDir() {
					// TODO: handle directories as a single unit , you can add some config file int the folder to handle how that folder should be treated
					continue
				}

				if filepath.Dir(event.Name) != "entropy" {
					continue
				}

				time.Sleep(500 * time.Millisecond)
				log.Println("New file detected:", event.Name)

				name := filepath.Base(event.Name)

				if isIgnored(event.Name, config.Ignore) {
					log.Println("Ignored file/folder by config:", name)
					continue
				}
				targetFolder := matchRules(name, config.Rules)

				if targetFolder == "" && config.Gpt.Enabled {
					resultCh := make(chan string, 1)
					jobQueue <- Job{filename: event.Name, resultCh: resultCh}
					targetFolder = <-resultCh
					log.Println("AI suggested folder:", targetFolder)
				}

				if targetFolder == "" {
					targetFolder = "Unsorted"
				}

				targetFolder = strings.TrimSpace(targetFolder)
				organizeItem(event.Name, targetFolder)
			}

		case err := <-watcher.Errors:
			log.Println("Watcher error:", err)
		}
	}

}
