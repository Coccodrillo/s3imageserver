package s3imageserver

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/coccodrillo/vips"
	"github.com/gosexy/to"
	"github.com/kr/s3"
)

type Image struct {
	Path            string
	FileName        string
	Bucket          string
	Crop            bool
	Debug           bool
	Height          int
	Width           int
	Image           []byte
	CacheTime       int
	CachePath       string
	ErrorImage      string
	ErrorResizeCrop bool
	OutputFormat    vips.ImageType
}

var allowedTypes = []string{".png", ".jpg", ".jpeg", ".gif", ".webp"}
var allowedMap = map[vips.ImageType]string{vips.WEBP: ".webp", vips.JPEG: ".jpg", vips.PNG: ".png"}

func NewImage(r *http.Request, config HandlerConfig, fileName string) (image *Image, err error) {
	maxDimension := 3064
	height := int(to.Float64(r.URL.Query().Get("h")))
	width := int(to.Float64(r.URL.Query().Get("w")))
	if height > maxDimension {
		height = maxDimension
	}
	if width > maxDimension {
		width = maxDimension
	}
	crop := true
	if r.URL.Query().Get("c") != "" {
		crop = to.Bool(r.URL.Query().Get("c"))
	}
	image = &Image{
		Path:            config.AWS.FilePath,
		Bucket:          config.AWS.BucketName,
		Height:          height,
		Crop:            crop,
		Width:           width,
		CacheTime:       604800, // cache time in seconds, set 0 to infinite and -1 for disabled
		CachePath:       config.CachePath,
		ErrorImage:      "",
		ErrorResizeCrop: true,
		OutputFormat:    vips.WEBP,
	}
	if config.CacheTime != nil {
		image.CacheTime = *config.CacheTime
	}
	image.isFormatSupported(r.URL.Query().Get("f"))
	acceptedTypes := allowedTypes
	if config.Allowed != nil && len(config.Allowed) > 0 {
		acceptedTypes = config.Allowed
	}
	for _, allowed := range acceptedTypes {
		if allowed == filepath.Ext(fileName) {
			image.FileName = filepath.FromSlash(fileName)
		}
	}
	if image.FileName == "" {
		err = errors.New("File name cannot be an empty string")
	}
	if image.Bucket == "" {
		err = errors.New("Bucket cannot be an empty string")
	}
	return image, err
}

func (i *Image) getImage(w http.ResponseWriter, r *http.Request, AWSAccess string, AWSSecret string) {
	var err error
	if i.CacheTime > -1 {
		err = i.getFromCache(r)
	} else {
		err = errors.New("Caching disabled")
	}
	if err != nil {
		fmt.Println(err)
		err := i.getImageFromS3(AWSAccess, AWSSecret)
		if err != nil {
			fmt.Println(err)
			err = i.getErrorImage()
			w.WriteHeader(404)
		} else {
			i.resizeCrop()
			go i.writeCache(r)
		}
	}
	i.write(w)
}

func (i *Image) isFormatSupported(format string) {
	if format != "" {
		format = "." + format
		for v, k := range allowedMap {
			if k == format {
				i.OutputFormat = v
			}
		}
	}
}

func (i *Image) write(w http.ResponseWriter) {
	w.Header().Set("Content-Length", strconv.Itoa(len(i.Image)))
	w.Write(i.Image)
}

func (i *Image) getErrorImage() (err error) {
	if i.ErrorImage != "" {
		i.Image, err = ioutil.ReadFile(i.ErrorImage)
		if err != nil {
			return err
		}
		if i.ErrorResizeCrop {
			i.resizeCrop()
		}
		return nil
	}
	return errors.New("Error image not specified")
}

func (i *Image) getImageFromS3(AWSAccess string, AWSSecret string) (err error) {
	req, _ := http.NewRequest("GET", fmt.Sprintf("https://%v.s3.amazonaws.com/%v%v", i.Bucket, i.Path, i.FileName), nil)
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	req.Header.Set("X-Amz-Acl", "public-read")
	s3.Sign(req, s3.Keys{
		AccessKey: AWSAccess,
		SecretKey: AWSSecret,
	})
	resp, err := http.DefaultClient.Do(req)
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		i.Image, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Println(err)
		} else if i.Debug {
			fmt.Println("Retrieved image from from S3")
		}
		return nil
	} else if resp.StatusCode != http.StatusOK {
		err = errors.New("Error while making request")
	}
	return err
}

func (i *Image) resizeCrop() {
	options := vips.Options{
		Width:        i.Width,
		Height:       i.Height,
		Crop:         i.Crop,
		Extend:       vips.EXTEND_WHITE,
		Interpolator: vips.BICUBIC,
		Gravity:      vips.CENTRE,
		Quality:      75,
		Format:       i.OutputFormat,
	}
	buf, err := vips.Resize(i.Image, options)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	i.Image = buf
}
