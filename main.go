package main

import (
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rwcarlsen/goexif/exif"
	"gopkg.in/ini.v1"
)

type config struct {
	SourceDir     string `comment:"Windows Spotlight's content delivery manager folder"`
	OutputDir     string `comment:"Folder to save images"`
	MinimumWidth  int    `comment:"Minimum image width to be considered as a wallpaper"`
	MinimumHeight int    `comment:"Minimum image height to be considered as a wallpaper"`
}

func main() {
	args := os.Args[1:]
	if len(args) == 1 && args[0] == "restore" {
		fmt.Println("restoring default configuration")
		restoreConfig(configPath())
		os.Exit(0)
	}
	if len(args) != 0 {
		fmt.Printf("Unknown arguments %s\n", strings.Join(args, " "))
		os.Exit(1)
	}

	logFilePath := filepath.Join(executablePath(), "logs.txt")
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		log.Fatalln(err)
	}
	log.SetOutput(logFile)

	log.Default().Println()

	config := loadConfig()

	sourceDir := config.SourceDir
	if err := checkDirectory(sourceDir); err != nil {
		log.Fatalln(err)
	}
	outputDir := config.OutputDir
	if err := checkDirectory(outputDir); err != nil {
		log.Fatalln(err)
	}
	err = filepath.WalkDir(sourceDir, copyWallpapersTo(outputDir))
	if err != nil {
		log.Fatalln(err)
	}
}

// executablePath returns the path of the directory of the executable
func executablePath() string {
	ex, err := os.Executable()
	if err != nil {
		log.Fatalln(err)
	}
	exPath := filepath.Dir(ex)
	return exPath
}

// configPath returns the path of the configuration
func configPath() string {
	cfgFilePath := filepath.Join(executablePath(), "wspotsave.ini")
	return cfgFilePath
}

// copyWallpapersTo returns a lambda function of type fs.WalkDirFunc
// that copies a file from sourceDir to the outputDir
//
// It validates if the file can be a wallpapers and if it doesn't
// already exists in the output directory
func copyWallpapersTo(outputDir string) fs.WalkDirFunc {
	walkDirFunc := func(imagePath string, d fs.DirEntry, _ error) error {
		if d.IsDir() {
			return nil
		}
		isWallpaper, err := isImageWallpaper(imagePath)
		if err != nil {
			log.Print(err)
			return nil
		}
		if !isWallpaper {
			log.Printf("%s size is too small\n", d.Name())
			return nil
		}
		targetPath := filepath.Join(outputDir, d.Name()+".jpg")
		_, err = os.Stat(targetPath)
		if os.IsNotExist(err) {
			log.Printf("copying file %s\n", targetPath)
			err = copyFile(imagePath, targetPath)
			if err != nil {
				log.Println(err)
			}
		} else {
			log.Printf("File %s already exists\n", targetPath)
		}
		return nil
	}
	return walkDirFunc
}

// loadConfig loads the configurations that specifies folders
//
// It tries to read configuration file relative to the executable.
// The name of the configuration file is wspotsave.ini.
// If it doesn't exists, returns the default configuration
// and creates the file.
func loadConfig() *config {
	cfgFilePath := configPath()
	iniConfig, err := ini.Load(cfgFilePath)
	if err != nil {
		fmt.Println("restoring default config")
		iniConfig = restoreConfig(cfgFilePath)
	}
	config := new(config)
	err = iniConfig.MapTo(config)
	if err != nil {
		log.Fatal(err)
	}
	return config
}

// restoreConfig returns the default configuration
// and save to the file
func restoreConfig(filepath string) *ini.File {
	iniConfig := defaultIniConfig()
	err := iniConfig.SaveTo(filepath)
	if err != nil {
		log.Fatal(err)
	}
	return iniConfig
}

// defaultIniConfig returns the default configuration
func defaultIniConfig() *ini.File {
	home := os.Getenv("USERPROFILE")
	defaultConfig := &config{
		SourceDir:     filepath.Join(home, "AppData", "Local", "Packages", "Microsoft.Windows.ContentDeliveryManager_cw5n1h2txyewy", "LocalState", "Assets"),
		OutputDir:     filepath.Join(home, "Pictures"),
		MinimumWidth:  1080,
		MinimumHeight: 1080,
	}
	iniConfig := ini.Empty()
	err := ini.ReflectFrom(iniConfig, defaultConfig)
	if err != nil {
		log.Fatal(err)
	}
	return iniConfig
}

// imageSize returns the width and the height of
// a given image path
func imageSize(imagePath string) (int, int, error) {
	imageFile, err := os.Open(imagePath)
	if err != nil {
		return 0, 0, fmt.Errorf("couldn't open %s", imagePath)
	}
	defer imageFile.Close()
	info, err := exif.Decode(imageFile)
	if err != nil {
		return 0, 0, fmt.Errorf("couldn't extract metadata of %s", imageFile.Name())
	}
	widthTag, err := info.Get(exif.PixelXDimension)
	if err != nil {
		return 0, 0, fmt.Errorf("couldn't get width of %s", imageFile.Name())
	}
	heightTag, err := info.Get(exif.PixelYDimension)
	if err != nil {
		return 0, 0, fmt.Errorf("couldn't get height of %s", imageFile.Name())
	}
	width, err := strconv.Atoi(widthTag.String())
	if err != nil {
		return 0, 0, err
	}
	height, err := strconv.Atoi(heightTag.String())
	if err != nil {
		return 0, 0, err
	}
	return width, height, nil
}

// isImageWallpaper tells whether the image in the given path
// fulfills the requirements of minimum width and the
// minimum height in the configuration
func isImageWallpaper(imagePath string) (bool, error) {
	config := loadConfig()
	minimumWidth := config.MinimumWidth
	minimumHeight := config.MinimumHeight

	width, height, err := imageSize(imagePath)
	if err != nil {
		return false, fmt.Errorf("couldn't get size of %s", imagePath)
	}
	if width < minimumWidth || height < minimumHeight {
		return false, nil
	}
	return true, nil
}

// checkDirectory checks if a path is a directory and exists
func checkDirectory(dirPath string) error {
	stat, err := os.Stat(dirPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("%s doesn't exist", dirPath)
	} else if err != nil {
		return err
	}
	if !stat.IsDir() {
		return fmt.Errorf("%s is not a directory", dirPath)
	}
	return nil
}

// copyFile is a utilty function to copy a file
func copyFile(sourcePath string, targetPath string) error {
	targetFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("couldn't create file %s", targetPath)
	}
	defer targetFile.Close()
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("couldn't open %s", sourcePath)
	}
	defer sourceFile.Close()
	_, err = io.Copy(targetFile, sourceFile)
	if err != nil {
		return fmt.Errorf("couldn't copy file %s", targetFile.Name())
	}
	return nil
}
