package s3imageserver

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (i *Image) getFromCache(r *http.Request) (err error) {
	newFileName := i.getCachedFileName(r)
	info, err := os.Stat(newFileName)
	if err != nil {
		return err
	}
	if (time.Duration(i.CacheTime))*time.Second > time.Since(info.ModTime()) {
		f, err := os.Open(newFileName)
		if err != nil {
			return err
		}
		file, err := ioutil.ReadAll(f)
		if err != nil {
			fmt.Println(err)
			return err
		}
		i.Image = file
		if i.Debug {
			fmt.Println("from cache")
		}
		return nil
	}
	go removeExpiredImage(newFileName)
	return errors.New("The file has expired")
}

func (i *Image) writeCache(r *http.Request) {
	err := ioutil.WriteFile(i.getCachedFileName(r), i.Image, 0644)
	if err != nil {
		fmt.Println(err)
	}
}

func removeExpiredImage(fileName string) {
	err := os.Remove(fileName)
	if err != nil {
		fmt.Println(err)
	}
}

func (i *Image) getCachedFileName(r *http.Request) (fileName string) {
	var pathPrefix string
	u, err := url.Parse(r.URL.String())
	if err != nil {
		panic(err)
	}
	h := strings.Split(u.Path, "/")
	if len(h) > 1 {
		pathPrefix = h[1]
	}
	fileNameOnly := i.FileName[0 : len(i.FileName)-len(filepath.Ext(i.FileName))]
	return fmt.Sprintf("%v/%v_w%v_h%v_c%v_%v%v", i.CachePath, pathPrefix, i.Width, i.Height, i.Crop, fileNameOnly, allowedMap[i.OutputFormat])
}

// TODO: add garbage colection
// TODO: add documentation
