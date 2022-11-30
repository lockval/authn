package db

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/segmentio/ksuid"

	"go.etcd.io/bbolt"
)

var (
	mfile = flag.String("mflie", "login.db", "")

	// UIDPrefixPlayer User ID prefix for exposed fields in scripts (init)
	UIDPrefixPlayer = flag.String("UIDPrefixPlayer", "player:", "player uid prefix")

	db *bbolt.DB

	uidBucketName   = []byte("uid")
	tokenBucketName = []byte("Token")
	infoName        = []byte("Info")
	loginTimeName   = []byte("LoginTime")

	platformBucketName = []byte("platform")

	ErrNotFoundMainUID     = errors.New("not found main uid")
	ErrNotFoundUID         = errors.New("not found uid")
	ErrNotFoundTokenBucket = errors.New("not found token bucket")
	ErrNotFoundToken       = errors.New("not found token")

	ErrNotFoundMainPlatform = errors.New("not found main platform")
	ErrCreatePlatformFail   = errors.New("create platform fail")
)

const (
	// ISO8601 ISO8601
	ISO8601 = "2006-01-02T15:04:05.000-07:00"
)

// Init init db
func Init() {
	var err error
	db, err = bbolt.Open(*mfile, 0600, nil)
	if err != nil {
		log.Fatal("bbolt.Open")
	}

	err = db.Update(func(tx *bbolt.Tx) error {

		uidRootB := tx.Bucket(uidBucketName)
		if uidRootB == nil {
			_, err := tx.CreateBucket(uidBucketName)
			if err != nil {
				return err
			}
		}

		platformRootB := tx.Bucket(platformBucketName)
		if platformRootB == nil {
			_, err := tx.CreateBucket(platformBucketName)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		log.Fatal("bolt init db")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/backup", backupHandleFunc)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal("bbolt listen")
	}
	err = os.WriteFile(*mfile+".bakurl", []byte(fmt.Sprintf("http://localhost:%d/backup", listener.Addr().(*net.TCPAddr).Port)), 0600)
	if err != nil {
		log.Fatal("bbolt create .bakurl")
	}

	go func() {
		httpservice := &http.Server{
			Handler:     mux,
			ReadTimeout: 600 * time.Second,
			// WriteTimeout:   10 * time.Second,
			// MaxHeaderBytes: 1 << 20,
		}
		fmt.Println("backup Serve", httpservice.Serve(listener))
	}()

}

func backupHandleFunc(w http.ResponseWriter, req *http.Request) {
	err := db.View(func(tx *bbolt.Tx) error {
		t1 := time.Now()
		t1s := t1.Format("20060102150405")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.%s"`, *mfile, t1s))
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.Itoa(int(tx.Size())))
		_, err := tx.WriteTo(w)
		return err
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// UnInit UnInit
func UnInit() {
	db.Close()
}

// DbGetUIDbyPUID Find user ID by platform ID
func DbGetUIDbyPUID(platform, pUID string, ininfo *string) (string, []string, string, error) {

	var info, uid string
	var tokens, newTokens []string
	err := db.Update(func(tx *bbolt.Tx) error {
		platformRootB := tx.Bucket(platformBucketName)
		if platformRootB == nil {
			return ErrNotFoundMainPlatform
		}
		uidRootB := tx.Bucket(uidBucketName)
		if uidRootB == nil {
			return ErrNotFoundMainUID
		}

		platformB := platformRootB.Bucket([]byte(platform))
		if platformB == nil {
			var err error
			platformB, err = platformRootB.CreateBucket([]byte(platform))
			if err != nil {
				return err
			}
		}

		var uidB *bbolt.Bucket
		var tokenB *bbolt.Bucket
		uidbyte := platformB.Get([]byte(pUID))

		if uidbyte == nil {

			uid = *UIDPrefixPlayer + ksuid.New().String()
			uidbyte = []byte(uid)

			var err error

			err = platformB.Put([]byte(pUID), uidbyte)
			if err != nil {
				return err
			}

			uidB, err = uidRootB.CreateBucket(uidbyte)
			if err != nil {
				return err
			}

			tokenB, err = uidB.CreateBucket(tokenBucketName)
			if err != nil {
				return err
			}

			err = uidB.Put(infoName, []byte(""))
			if err != nil {
				return err
			}

		} else {

			uid = string(uidbyte)

			uidB = uidRootB.Bucket(uidbyte)
			if uidB == nil {
				return ErrNotFoundUID
			}

			tokenB = uidB.Bucket(tokenBucketName)
			if tokenB == nil {
				return ErrNotFoundTokenBucket
			}

			v := uidB.Get(infoName)
			if v != nil {
				info = string(v)
			}

			err := tokenB.ForEach(func(k, _ []byte) error {
				tokens = append(tokens, string(k))
				return nil
			})
			if err != nil {
				return err
			}

		}

		if ininfo != nil {
			err := uidB.Put(infoName, []byte(*ininfo))
			if err != nil {
				return err
			}
			info = *ininfo
		}

		timestr := time.Now().Format(ISO8601)

		DBToken := ksuid.New().String()
		err := tokenB.Put([]byte(DBToken), []byte(timestr))
		if err != nil {
			return err
		}

		tokens = append(tokens, string(DBToken))

		limit := 2
		tokensNum := len(tokens)
		for idx := range tokens {
			key := tokens[tokensNum-1-idx]
			limit--
			if limit < 0 {
				err = tokenB.Delete([]byte(key))
				if err != nil {
					return err
				}
			} else {
				newTokens = append([]string{key}, newTokens...)
			}
		}

		return uidB.Put(loginTimeName, []byte(timestr))
	})

	if err != nil {
		return "", nil, "", err
	}

	// for i, j := 0, len(newTokens)-1; i < j; i, j = i+1, j-1 {
	// 	newTokens[i], newTokens[j] = newTokens[j], newTokens[i]
	// }

	return uid, newTokens, info, nil
}

// DbLoginRequ login by dbToken
func DbLoginRequ(uid, dbToken string) (string, error) {

	var info string
	err := db.Update(func(tx *bbolt.Tx) error {
		uidRootB := tx.Bucket(uidBucketName)
		if uidRootB == nil {
			return ErrNotFoundMainUID
		}

		uidB := uidRootB.Bucket([]byte(uid))
		if uidB == nil {
			return ErrNotFoundUID
		}

		tokenB := uidB.Bucket(tokenBucketName)
		if tokenB == nil {
			return ErrNotFoundTokenBucket
		}

		v := tokenB.Get([]byte(dbToken))
		if v == nil {
			return ErrNotFoundToken
		}

		v = uidB.Get(infoName)
		if v != nil {
			info = string(v)
		}

		timestr := time.Now().Format(ISO8601)
		return uidB.Put(loginTimeName, []byte(timestr))

	})

	if err != nil {
		return "", err
	}

	return info, nil
}
