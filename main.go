package main

import (
	"crypto/tls"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	_ "github.com/go-sql-driver/mysql"
	"github.com/kylelemons/go-gypsy/yaml"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/readium/readium-lcp-server/index"
	"github.com/readium/readium-lcp-server/license"
	"github.com/readium/readium-lcp-server/pack"
	"github.com/readium/readium-lcp-server/server"
	"github.com/readium/readium-lcp-server/storage"
)

func dbFromURI(uri string) (string, string) {
	parts := strings.Split(uri, "://")
	return parts[0], parts[1]
}

func main() {
	var config_file, host, port, publicBaseUrl, dbURI, storagePath, certFile, privKeyFile, static string
	var readonly bool = false
	var err error

	if host = os.Getenv("HOST"); host == "" {
		host, err = os.Hostname()
		if err != nil {
			panic(err)
		}
	}

	if config_file = os.Getenv("READIUM_LCP_CONFIG"); config_file == "" {
		config_file = "config.yaml"
	}

	config, err := yaml.ReadFile(config_file)
	if err != nil {
		panic("can't read config file : " + config_file)
	}

	readonly = os.Getenv("READONLY") != ""

	if port = os.Getenv("PORT"); port == "" {
		port = "8989"
	}

	publicBaseUrl, _ = config.Get("public_base_url")
	if publicBaseUrl == "" {
		publicBaseUrl = "http://" + host + ":" + port
	}

	dbURI, _ = config.Get("database")
	if dbURI == "" {
		if dbURI = os.Getenv("DB"); dbURI == "" {
			dbURI = "sqlite3://file:test.sqlite?cache=shared&mode=rwc"
		}
	}

	storagePath, _ = config.Get("storage.filesystem.storage")
	if storagePath == "" {
		if storagePath = os.Getenv("STORAGE"); storagePath == "" {
			storagePath = "files"
		}
	}

	certFile, _ = config.Get("certificate.cert")
	privKeyFile, _ = config.Get("certificate.private_key")

	if certFile == "" {
		if certFile = os.Getenv("CERT"); certFile == "" {
			panic("Must specify a certificate")
		}
	}

	if privKeyFile == "" {
		if privKeyFile = os.Getenv("PRIVATE_KEY"); privKeyFile == "" {
			panic("Must specify a private key")
		}
	}

	cert, err := tls.LoadX509KeyPair(certFile, privKeyFile)
	if err != nil {
		panic(err)
	}

	driver, cnxn := dbFromURI(dbURI)
	db, err := sql.Open(driver, cnxn)
	if err != nil {
		panic(err)
	}
	if driver == "sqlite3" {
		_, err = db.Exec("PRAGMA journal_mode = WAL")
		if err != nil {
			panic(err)
		}
	}
	idx, err := index.Open(db)
	if err != nil {
		panic(err)
	}

	lst, err := license.NewSqlStore(db)
	if err != nil {
		panic(err)
	}

	var store storage.Store

	if mode, _ := config.Get("storage.mode"); mode == "s3" {
		s3Conf := s3ConfigFromYAML(config)
		store, _ = storage.S3(s3Conf)
	} else {
		os.Mkdir(storagePath, os.ModePerm) //ignore the error, the folder can already exist
		store = storage.NewFileSystem(storagePath, publicBaseUrl+"/files")
	}

	packager := pack.NewPackager(store, idx, 4)

	static, _ = config.Get("static.directory")
	if static == "" {
		_, file, _, _ := runtime.Caller(0)
		here := filepath.Dir(file)
		static = filepath.Join(here, "/static")
	}

	HandleSignals()

	s := server.New(":"+port, static, readonly, &idx, &store, &lst, &cert, packager)
	s.ListenAndServe()

}

func HandleSignals() {
	sigChan := make(chan os.Signal)
	go func() {
		stacktrace := make([]byte, 1<<20)
		for sig := range sigChan {
			switch sig {
			case syscall.SIGQUIT:
				length := runtime.Stack(stacktrace, true)
				fmt.Println(string(stacktrace[:length]))
			case syscall.SIGINT:
				fallthrough
			case syscall.SIGTERM:
				fmt.Println("Shutting down...")
				os.Exit(0)
			}
		}
	}()
	signal.Notify(sigChan, syscall.SIGQUIT, syscall.SIGINT, syscall.SIGTERM)
}

func s3ConfigFromYAML(in *yaml.File) storage.S3Config {
	config := storage.S3Config{}

	config.PublicURL, _ = in.Get("storage.public_url")

	config.Id, _ = in.Get("storage.access_id")
	config.Secret, _ = in.Get("storage.secret")
	config.Token, _ = in.Get("storage.token")

	config.Endpoint, _ = in.Get("storage.endpoint")
	config.Bucket, _ = in.Get("storage.bucket")
	config.Region, _ = in.Get("storage.region")

	ssl, _ := in.GetBool("storage.disable_ssl")
	config.DisableSSL = ssl
	config.ForcePathStyle, _ = in.GetBool("storage.path_style")

	return config
}
