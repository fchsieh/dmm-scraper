package main

import (
	"fmt"
	"image"
	"os"
	"path"
	"path/filepath"
	"strings"

	"dmm-scraper/pkg/config"
	"dmm-scraper/pkg/img"
	"dmm-scraper/pkg/logger"
	"dmm-scraper/pkg/metadata"
	"dmm-scraper/pkg/scraper"
)

var (
	ratio = 1.42
)

func isValidVideo(ext string) bool {
	switch strings.ToLower(ext) {
	case
		".wmv",
		".mp4",
		".avi",
		".mkv":
		return true
	}
	return false
}

func MyProgress(l logger.Logger, sType, filename string) func(current, total int64) {
	return func(current, total int64) {
		l.Infof(fmt.Sprintf("%s downloading %s ... %f%%", sType, filename, float32(current)/float32(total)*100))
	}
}

func main() {
	var err error
	log := logger.New()

	conf, err := config.NewLoader().LoadFile("config")
	if err != nil {
		log.Errorf("Error reading config file, %s", err)
		log.Warnf("Loading default config")
		conf = config.Default()
	}

	scraper.Setup(conf)

	files, err := os.ReadDir(conf.Input.Path)
	if err != nil {
		log.Fatal(err)
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		ext := filepath.Ext(f.Name())
		if !isValidVideo(ext) {
			continue
		}
		log.Infof("Check file: %s", f.Name())
		name := strings.TrimSuffix(f.Name(), ext)

		// 用正则处理文件名
		if query, scrapers := scraper.GetQuery(name); query != "" {

			for _, s := range scrapers {
				log.Infof("%s capturing query: %s", s.GetType(), query)

				// fetch
				err = s.FetchDoc(query)
				if err != nil {
					log.Error(err)
					continue
				}

				if s.GetNumber() == "" {
					log.Errorf("%s get num empty", s.GetType())
					continue
				}

				num := s.GetFormatNumber()
				log.Infof("%s get num %s format: %s", s.GetType(), s.GetNumber(), num)

				// mkdir
				outputPath := scraper.GetOutputPath(s, conf.Output.Path)
				log.Infof("%s making output path: %s", s.GetType(), outputPath)
				err = os.MkdirAll(outputPath, 0700)
				if err != nil && !os.IsExist(err) {
					log.Error(err)
					break
				}

				// build nfo
				movieNfo := metadata.NewMovieNfo(s)
				cover := fmt.Sprintf("%s-fanart.jpg", num)
				poster := fmt.Sprintf("%s-poster.jpg", num)
				// movieNfo.SetPoster(poster)
				movieNfo.SetTitle(num)

				coverPath := path.Join(outputPath, cover)
				posterPath := path.Join(outputPath, poster)
				err = scraper.Download(s.GetCover(), coverPath, MyProgress(log, s.GetType(), cover))
				if err != nil {
					log.Error(err)
					break
				}

				imgOperation := img.NewOperation()
				// calculate posterWidth based on cover width
				posterWidth, _ := getPosterWidth(coverPath, ratio)
				err = imgOperation.CropAndSave(coverPath, posterPath, posterWidth, 0)
				if err != nil {
					log.Errorf("Crop image failed: %v", err)
					break
				}

				nfo := path.Join(outputPath, fmt.Sprintf("%s.nfo", num))
				log.Infof("%s writing nfo file: %s", s.GetType(), nfo)
				err = movieNfo.Save(nfo)
				if err != nil {
					log.Error(err)
					break
				}

				log.Infof("%s moving video file to: %s", s.GetType(), outputPath)
				// if file exist no overwrite
				filePath := path.Join(conf.Input.Path, f.Name())
				err = MoveFile(filePath, outputPath, num, 1)
				if err != nil {
					log.Error(err)
					break
				}

				log.Infof("-------- Done: %s --------", num)
				break
			}
		}
	}
}

func MoveFile(oldPath, outputPath, num string, index int) error {
	var filename string
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		return err
	}

	if index != 1 {
		filename = fmt.Sprintf("%s-cd%d%s", num, index, filepath.Ext(oldPath))
	} else {
		filename = fmt.Sprintf("%s%s", num, filepath.Ext(oldPath))
	}
	newPath := path.Join(outputPath, filename)
	if _, err := os.Stat(newPath); err == nil {
		index += 1
		return MoveFile(oldPath, outputPath, num, index)
	}
	return os.Rename(oldPath, newPath)
}

func getPosterWidth(fanartPath string, ratio float64) (height int, width int) {
	if reader, err := os.Open(fanartPath); err == nil {
		defer reader.Close()
		im, _, err := image.Decode(reader)
		if err != nil {
			return 378, 0
		}

		var posterH, posterW int

		fanartH, fanartW := im.Bounds().Dy(), im.Bounds().Dx()
		if float64(fanartH)/float64(fanartW) < ratio {
			posterW = int(float64(fanartH) / ratio)
			posterH = fanartH
		} else {
			posterW = fanartW
			posterH = int(float64(fanartW) * ratio)
		}
		return posterW, posterH
	}
	return 378, 0
}