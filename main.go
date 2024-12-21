package main

import (
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"

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
	config := loadConfig()

	sourceDir := config.Section("").Key("SourceDir").Value()
	outputDir := config.Section("").Key("OutputDir").Value()

	err := filepath.WalkDir(sourceDir, copyWallpapersTo(outputDir))

	if err != nil {
		log.Fatal(nil)
	}
}

// loadConfig loads the configurations that specifies folders
//
// It tries to read configuration file relative to the executable.
// The name of the configuration file is wspotsave.ini.
// If it doesn't exists, returns the default configuration
// and creates the file.
func loadConfig() *ini.File {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	exPath := filepath.Dir(ex)
	cfgFilePath := filepath.Join(exPath, "wspotsave.ini")
	config, err := ini.Load(cfgFilePath)
	if err != nil {
		fmt.Println("[INFO] Restoring default config...")
		config = defaultConfig()
		config.SaveTo(cfgFilePath)
	}
	return config
}

// defaultConfig returns the default configuration
func defaultConfig() *ini.File {
	home := os.Getenv("USERPROFILE")
	configStruct := &config{
		SourceDir:     filepath.Join(home, "AppData", "Local", "Packages", "Microsoft.Windows.ContentDeliveryManager_cw5n1h2txyewy", "LocalState", "Assets"),
		OutputDir:     filepath.Join(home, "Pictures"),
		MinimumWidth:  1080,
		MinimumHeight: 1080,
	}
	config := ini.Empty()
	err := ini.ReflectFrom(config, configStruct)
	if err != nil {
		log.Fatal(err)
	}
	return config
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
	minimumWidth, err := strconv.Atoi(config.Section("").Key("MinimumWidth").Value())
	if err != nil {
		return false, fmt.Errorf("couldn't get MinimumWidth configuration")
	}
	minimumHeight, err := strconv.Atoi(config.Section("").Key("MinimumHeight").Value())
	if err != nil {
		return false, fmt.Errorf("couldn't get MinimumHeight configuration")
	}
	width, height, err := imageSize(imagePath)
	if err != nil {
		return false, fmt.Errorf("couldn't get size of %s", imagePath)
	}
	if width < minimumWidth || height < minimumHeight {
		return false, nil
	}
	return true, nil
}

// copyFile is a utilty function to copy a file
func copyFile(sourceFileName string, targetFileName string) error {
	targetFile, err := os.Create(targetFileName)
	if err != nil {
		return fmt.Errorf("couldn't create file %s", targetFileName)
	}
	defer targetFile.Close()
	sourceFile, err := os.Open(sourceFileName)
	if err != nil {
		return fmt.Errorf("couldn't open %s", sourceFileName)
	}
	defer sourceFile.Close()
	sourceFile.Sync()
	_, err = io.Copy(targetFile, sourceFile)
	if err != nil {
		return fmt.Errorf("couldn't copy file %s", targetFile.Name())
	}
	return nil
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
			log.Println(err)
			return nil
		}
		if !isWallpaper {
			log.Printf("%s size is too small\n", d.Name())
			return nil
		}

		targetFileName := filepath.Join(outputDir, d.Name()+".jpg")
		_, err = os.Stat(targetFileName)
		if os.IsNotExist(err) {
			log.Printf("copying file %s\n", targetFileName)
			err = copyFile(imagePath, targetFileName)
			if err != nil {
				log.Println(err)
			}
		} else {
			log.Printf("File %s already exists\n", targetFileName)
		}
		return nil
	}
	return walkDirFunc
}
