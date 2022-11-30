package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lockval/authn/common"
	"github.com/lockval/authn/db"
)

var (
	httpservice *http.Server
	wg          sync.WaitGroup

	ErrUIDEmpty    = errors.New("UID is empty")
	ErrTokenEmpty  = errors.New("DBToken is empty")
	ErrAuthNoFound = errors.New("auth not found")
	ErrPIDEmpty    = errors.New("PID is empty")
	ErrAuthFail    = errors.New("auth fail")
	ErrCheckTs     = errors.New("bad ts")
)

const (
	// StatusLoginAuthError StatusLoginAuthError
	StatusLoginAuthError = 597
)

func main() {

	rand.Seed(time.Now().UnixNano())

	flag.Parse()

	sAddrPort := strings.Split(*common.ServiceAddr, ":")
	if len(sAddrPort) != 2 {
		log.Fatal("bad serviceAddr")
	}
	sPort, _ := strconv.Atoi(sAddrPort[1])
	if sPort == 0 {
		log.Fatal("bad serviceAddr Port")
	}

	common.EtcdInit()
	db.Init()

	common.Reg2etcd("http://"+*common.ServiceAddr, "login")

	serveGate()

	log.Println("start login...")

	common.HandleExit(func() {

		err := httpservice.Shutdown(context.Background())
		if err != nil {
			log.Println("Shutdown err:", err)
		}

		wg.Wait()

		db.UnInit()

		// time.Sleep(1 * time.Second)
	})
}

func loginbytoken(LorP common.LoginByLoginOrPlatform) (dbtoken, uid, info string, err error) {

	if LorP.UID == "" {
		err = ErrUIDEmpty
		return
	}
	if LorP.DBToken == "" {
		err = ErrTokenEmpty
		return
	}

	info, err = db.DbLoginRequ(LorP.UID, LorP.DBToken)
	uid = LorP.UID
	dbtoken = LorP.DBToken
	return
}

func loginbyhttp(ctx context.Context, LorP common.LoginByLoginOrPlatform) (dbtoken []string, uid, info string, err error) {

	nt := time.Now().UnixNano() / 1000
	if LorP.TimestampMicro+10000000 < nt { // 10s
		err = ErrCheckTs
		return
	}

	timestampMicro := strconv.FormatInt(LorP.TimestampMicro, 10)
	if LorP.PID == "" {
		err = ErrPIDEmpty
		return
	}

	var token string
	if LorP.Info == nil {
		token = common.GetHash(timestampMicro + LorP.PID + LorP.Platform + *common.VSecretkey)
	} else {
		token = common.GetHash(timestampMicro + LorP.PID + LorP.Platform + *LorP.Info + *common.VSecretkey)
	}

	if LorP.Token != token {
		err = ErrAuthFail
		return
	}

	uid, dbtoken, info, err = db.DbGetUIDbyPUID(LorP.Platform, LorP.PID, LorP.Info)
	return
}

func auth(w http.ResponseWriter, r *http.Request) {

	b, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(StatusLoginAuthError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}

	var LorP common.LoginByLoginOrPlatform
	err = json.Unmarshal(b, &LorP)
	if err != nil {
		w.WriteHeader(StatusLoginAuthError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}

	var loginRequ common.LoginRequ2ws
	loginRequ.TimestampMicro = time.Now().UnixNano() / 1000
	timestampMicro := strconv.FormatInt(loginRequ.TimestampMicro, 10)

	var uid, info string
	if LorP.Platform == "" {
		var dbtoken string
		dbtoken, uid, info, err = loginbytoken(LorP)
		loginRequ.DBToken = dbtoken
		// fmt.Println("==token==login==", dbtoken)
	} else {
		var dbtokens []string
		dbtokens, uid, info, err = loginbyhttp(r.Context(), LorP)
		loginRequ.DBToken = dbtokens[len(dbtokens)-1]
		for _, dbtoken := range dbtokens {
			keep := common.GetHash(dbtoken + *common.VSecretkey)
			loginRequ.Keeps = append(loginRequ.Keeps, keep)
		}
		// fmt.Println("==other==login==", dbtokens)
	}
	if err != nil {
		w.WriteHeader(StatusLoginAuthError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}

	loginRequ.UID = uid
	loginRequ.Info = info

	token := common.GetHash(timestampMicro + loginRequ.UID + loginRequ.DBToken + loginRequ.Info + *common.VSecretkey)
	loginRequ.Token = token

	b, err = json.Marshal(&loginRequ)
	if err != nil {
		w.WriteHeader(StatusLoginAuthError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}

	_, _ = w.Write(b)
}

func serveGate() {
	mux := http.NewServeMux()

	mux.HandleFunc("/auth", auth)

	lis, err := net.Listen("tcp", *common.ServiceAddr)
	if err != nil {
		panic(err)
	}

	httpservice = &http.Server{
		Handler:     mux,
		ReadTimeout: 10 * time.Second,
		// WriteTimeout:   10 * time.Second,
		// MaxHeaderBytes: 1 << 20,
	}

	wg.Add(1)
	go func() {
		err := httpservice.Serve(lis)
		if err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				panic(err)
			}
		}

		wg.Done()
	}()
}
