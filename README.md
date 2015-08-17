# S3imageserver package for Go

The S3imageserver package for Go provides with a scalable API server that fetches images from a S3 bucket, resizes them, displays then and caches them on the instance. In case of errors it optionally displays a fallback image, but still returns a 404 response, thus preventing caching.

I built it in early 2015 for social network Her, where I was working as a tech lead, to replace the old image serving solution which became inadequate. It is been in use since and was last seen serving over 50 million images per day quite reliably.

I have tried a dozen of image resizers, but none of them fit specific needs of the project. It uses [libvips](https://github.com/jcupitt/libvips) for resizing, which is up to [12 times faster than ImageMagick](https://github.com/fawick/speedtest-resize) or any other image resizing solution. For HTTP routing I have used Julien's [httprouter](https://github.com/julienschmidt/httprouter), the most performant and reliable Go router out there.

It is made to be highly configurable, and you can pass a JSON file with configurations and with a goal of zero maintenance.

There are still outstanding things, like the fact that we use computationally intensive ECDHE cipher suites, which while offering perfect security, cause a noticeable performance hit if there is nothing in front of them. If there are put into a load balancer or reverse proxy (as they should be), it gets a lot faster.

There is no periodical cache expiration checks, it could use a handler forcing a refresh and since we were trying to move fast and break things, it lacks unit tests. Probably somethng else as well and I am always happy to accept suggestions and / or pull requests.

### Usage

Run the server, pass the optional configuration parameter:

	./s3imageserver -c=config.json


You can use it as a package like this:

	package main

	import (
		"fmt"

		"github.com/coccodrillo/s3imageserver/s3imageserver"
		"github.com/dgrijalva/jwt-go"
	)

	func main() {
		s3imageserver.Run(nil)
	}


There is also an option to pass a handler for validation, so it's easy to implement JWT client verification:

	package main

	import (
		"fmt"
		"io/ioutil"

		"github.com/coccodrillo/s3imageserver/s3imageserver"
		"github.com/dgrijalva/jwt-go"
	)

	func main() {
		s3imageserver.Run(verifyToken)
	}

	func verifyToken(tokenString string) bool {
		publicKey, err := ioutil.ReadFile("verification.key")
		if err != nil {
			fmt.Print("Error:", err)
			return false
		}
		_, err = jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return publicKey, nil
		})
		if err != nil {
			errType := err.(*jwt.ValidationError)
			switch errType.Errors {
			case jwt.ValidationErrorMalformed:
				fmt.Println("malformed")
			case jwt.ValidationErrorUnverifiable:
				fmt.Println("unverifiable")
			case jwt.ValidationErrorSignatureInvalid:
				fmt.Println("signature invalid")
			case jwt.ValidationErrorExpired:
				fmt.Println("expired")
			case jwt.ValidationErrorNotValidYet:
				fmt.Println("not valid yet")
			}
		} else {
			return true
		}
		return false
	}

This is a sample of how configuration looks like:

	{
	  "handlers": [
	    {
	      "name": "handlerName",
	      "prefix": "h",
	      "aws": {
	        "bucket_name": "bucketName",
	      	"file_path": "imagesLarge", 	// folder in the bucket
	        "aws_access": "aws_access_key",
	        "aws_secret": "aws_secret_key"
	      },
	      "error_image": "error_image.jpg",	// image to be displayed in case of an error
	      "output_format": "jpg", 			// default output format
	      "cache_enabled": true,
	      "cache_path": "./cache",
	      "cache_time": 604800 				// max cache time in seconds, defaults to a week
	    },
	    {
	      "name": "secondHandlerName",
	      "prefix": "h2",
	      "aws": {
	        "bucket_name": "bucketName2",
	      	"file_path": "",
	        "aws_access": "aws_access_key",
	        "aws_secret": "aws_secret_key"
	      },
	      "error_image": "error_image.jpg",
	      "output_format": "jpg",
	      "cache_enabled": true,
	      "cache_path": "./cache",
	    }
	  ],
	  "http_port": 80,
	  "https_enabled": true,
	  "https_strict": false, 				// redirects http responses to https
	  "https_port": 443,
	  "https_cert": "bundle.crt",
	  "https_key": "cert.key"
	}


- handlers bind to specific endpoints, that carry handler prefix or if there is none, then handler name
- http / https settings are optional, it defaults to port 80 on http if nothing is set

### Running

http://example.com/h/my_image_name.jpg?w=300&h=200&c=true&f=png

	w = width
	h = height
	c = crop
	f = output format

If you enabled validation, you just pass parameter the desired token as a URL parameter t:

http://example.com/h/my_image_name.jpg?w=300&h=200&c=true&f=png&t=verification_token

### Install

Install the package:

	go get github.com/coccodrillo/s3imageserver