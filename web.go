package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/eaigner/s3"
)

const (
	kPort     = "PORT"
	kGhToken  = "GH_TOKEN"
	kGhRepo   = "GH_REPO"
	kS3Bucket = "S3_BUCKET"
	kS3Key    = "S3_KEY"
	kS3Secret = "S3_SECRET"
	kS3Path   = "S3_PATH"
)

var conf = map[string]string{
	kPort:     os.Getenv(kPort),
	kGhToken:  os.Getenv(kGhToken),
	kGhRepo:   os.Getenv(kGhRepo),
	kS3Bucket: os.Getenv(kS3Bucket),
	kS3Key:    os.Getenv(kS3Key),
	kS3Secret: os.Getenv(kS3Secret),
	kS3Path:   os.Getenv(kS3Path),
}

const issueFmt = "Reported By: [%s](mailto:%s)\n" +
	"Version: %s\n" +
	"Platform: %s\n" +
	"Attachment: [%s](%s)\n" +
	"Customer: %s\n" +
	"Description:\n\n```\n%s\n```\n\n" +
	"How to reproduce:\n\n```\n%s\n```\n\n"

func init() {
	http.HandleFunc("/", Static)
	http.HandleFunc("/create", Create)
}

func main() {
	if conf[kPort] == "" {
		conf[kPort] = "8080"
	}

	for k, v := range conf {
		if v == "" {
			panic(`missing environment var ` + k)
		}
	}

	err := http.ListenAndServe(`:`+conf[kPort], nil)
	if err != nil {
		log.Fatal(err)
	}
}

func Static(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Path
	if file == "" || file == "/" {
		file = "index.html"
	}
	http.ServeFile(w, r, `static/`+file)
}

func randomAlphaId() (string, error) {
	var b [8]byte
	_, err := rand.Read(b[:])
	if err != nil {
		return "", err
	}
	id := base64.StdEncoding.EncodeToString(b[:])
	id = strings.NewReplacer(`/`, ``, `+`, ``, `=`, ``).Replace(id)

	return id, nil
}

func Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.NotFound(w, r)
		return
	}

	err := r.ParseMultipartForm(1024 * 10)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, `could not parse multipart form`, 400)
		return
	}

	var (
		honey       = r.FormValue("honey")
		name        = r.FormValue("name")
		email       = r.FormValue("email")
		prodVer     = r.FormValue("prodver")
		platform    = r.FormValue("platform")
		summary     = r.FormValue("summary")
		desc        = r.FormValue("desc")
		repro       = r.FormValue("repro")
		info        = r.FormValue("info")
		file, fh, _ = r.FormFile("file")
	)

	// honeypot for bots
	if honey != "" {
		http.NotFound(w, r)
		return
	}

	// default values
	for _, v := range []*string{&desc, &info, &repro} {
		if *v == "" {
			*v = "-"
		}
	}
	if summary == "" {
		summary = "New Customer Issue"
	}

	// upload file to S3
	fileURL := "#"
	fileName := "No Attachment"

	if file != nil && fh != nil {
		id, err := randomAlphaId()
		if err != nil {
			log.Println(err.Error())
			http.Error(w, `could not generate random id`, 400)
			return
		}

		fileExt := path.Ext(fh.Filename)
		fileName = fmt.Sprintf("upload-%d-%s%s", time.Now().Unix(), id, fileExt)

		log.Printf("uploading %s ...\n", fileName)

		s3c := &s3.S3{
			Bucket:    conf[kS3Bucket],
			AccessKey: conf[kS3Key],
			Secret:    conf[kS3Secret],
			Path:      conf[kS3Path],
		}
		s3o := s3c.Object(fileName)
		s3w := s3o.Writer()

		_, err = io.Copy(s3w, file)
		if err != nil {
			s3w.Abort()
			log.Println(err.Error())
			http.Error(w, `could not upload file to S3`, 400)
			return
		}
		s3w.Close()

		fileURL = fmt.Sprintf("http://s3.amazonaws.com/%s/%s", s3c.Bucket, s3o.Key())
	}

	// create new issue on GitHub
	text := fmt.Sprintf(issueFmt, name, email, prodVer, platform, fileName, fileURL, info, desc, repro)

	v := map[string]interface{}{
		"title":  summary,
		"body":   text,
		"labels": []string{"customer-issue"},
	}
	b, err := json.Marshal(v)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, `could not create issue`, 500)
		return
	}

	url := fmt.Sprintf(`https://api.github.com/repos/%s/issues`, conf[kGhRepo])
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(b))
	if err != nil {
		log.Println(err.Error())
		http.Error(w, `could not create issue`, 500)
		return
	}
	req.SetBasicAuth(conf[kGhToken], ``)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, `could not create issue`, 500)
		return
	}
	if resp.StatusCode != 201 {
		log.Println("invalid status", resp.StatusCode)
		http.Error(w, `could not create issue`, 500)
	}

	// https://developer.github.com/v3/issues/#response-3
	var issue struct {
		Number int
	}
	err = json.NewDecoder(resp.Body).Decode(&issue)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, `could not create issue`, 500)
		return
	}

	log.Printf("created issue #%d", issue.Number)

	path := fmt.Sprintf(`/success.html?id=%d`, issue.Number)

	http.Redirect(w, r, path, http.StatusTemporaryRedirect)
}
