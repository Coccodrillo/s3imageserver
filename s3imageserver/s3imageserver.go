package s3imageserver

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"strconv"
	"sync"

	"github.com/julienschmidt/httprouter"
)

//Config ...
type Config struct {
	Handlers     []HandlerConfig `json:"handlers"`
	HTTPPort     int             `json:"http_port"`
	HTTPSEnabled bool            `json:"https_enabled"`
	HTTPSStrict  bool            `json:"https_strict"`
	HTTPSPort    int             `json:"https_port"`
	HTTPSCert    string          `json:"https_cert"`
	HTTPSKey     string          `json:"https_key"`
}

//HandlerConfig ...
type HandlerConfig struct {
	Name   string `json:"name"`
	Prefix string `json:"prefix"`
	AWS    struct {
		AWSAccess  string `json:"aws_access"`
		AWSSecret  string `json:"aws_secret"`
		BucketName string `json:"bucket_name"`
		FilePath   string `json:"file_path"`
	} `json:"aws"`
	ErrorImage   string   `json:"error_image"`
	Allowed      []string `json:"allowed_formats"`
	OutputFormat string   `json:"output_format"`
	CachePath    string   `json:"cache_path"`
	CacheTime    *int     `json:"cache_time"`
}

//HandleVerification ...
type HandleVerification func(string) bool

//Run ...
func Run(verify HandleVerification) {
	envArg := flag.String("c", "config.json", "Configuration")
	flag.Parse()
	content, err := ioutil.ReadFile(*envArg)
	if err != nil {
		fmt.Print("Error:", err)
		os.Exit(1)
	}
	var conf Config
	err = json.Unmarshal(content, &conf)
	if err != nil {
		fmt.Print("Error:", err)
		os.Exit(1)
	}

	r := httprouter.New()
	for _, handler := range conf.Handlers {
		prefix := handler.Name
		if handler.Prefix != "" {
			prefix = handler.Prefix
		}
		r.GET("/"+prefix+"/:param", func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
			i, err := NewImage(r, handler, ps.ByName("param"))
			i.ErrorImage = handler.ErrorImage
			if err == nil && (verify == nil || verify(r.URL.Query().Get("t"))) {
				i.getImage(w, r, handler.AWS.AWSAccess, handler.AWS.AWSSecret)
			} else {
				if err != nil {
					fmt.Println(err.Error())
				}
				i.getErrorImage()
				w.WriteHeader(404)
				w.Header().Set("Content-Length", strconv.Itoa(len(i.Image)))
			}
			i.write(w)
		})
	}

	wg := &sync.WaitGroup{}
	if conf.validateHTTPS() {
		config := tls.Config{
			MinVersion:               tls.VersionTLS10,
			PreferServerCipherSuites: true,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
				tls.TLS_RSA_WITH_AES_128_CBC_SHA,
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA,
				tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA},
		}
		hot := http.Server{
			Addr:      ":" + strconv.Itoa(conf.HTTPSPort),
			Handler:   r,
			TLSConfig: &config,
		}
		wg.Add(1)
		go func() {
			log.Fatal(hot.ListenAndServeTLS(conf.HTTPSCert, conf.HTTPSKey))
			wg.Done()
		}()
	}
	wg.Add(1)
	go func() {
		HTTPPort := ":80"
		if conf.HTTPPort != 0 {
			HTTPPort = ":" + strconv.Itoa(conf.HTTPPort)
		}
		if conf.HTTPSStrict && conf.HTTPSEnabled {
			http.ListenAndServe(HTTPPort, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				http.Redirect(w, req, "https://"+req.Host+req.RequestURI, http.StatusMovedPermanently)
			}))
		} else {
			http.ListenAndServe(HTTPPort, r)
		}
		wg.Done()
	}()
	wg.Wait()
}

func (c *Config) validateHTTPS() bool {
	if c.HTTPSEnabled && c.HTTPSKey != "" && c.HTTPSCert != "" && c.HTTPSPort != 0 && c.HTTPSPort != c.HTTPPort {
		return true
	}
	c.HTTPSEnabled = false
	return false
}
