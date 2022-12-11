package main

import (
	"flag"
	"log"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lockval/authn/common"
)

// AuthRequ AuthRequ
type AuthRequ struct {
	Name string
}

func auth(c *gin.Context) {

	resp := common.Platform{}
	requ := AuthRequ{}

	err := c.BindJSON(&requ)
	if err != nil {
		_ = c.Error(err)
		return
	}

	if requ.Name == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	resp.PID = requ.Name
	resp.Platform = "guest"
	resp.TimestampMicro = time.Now().UnixNano() / 1000
	timestampMicro := strconv.FormatInt(resp.TimestampMicro, 10)

	// If you want to change Info, set Info to have a value
	// resp.Info = new(string)

	var token string
	if resp.Info == nil {
		token = common.GetHash(timestampMicro + resp.PID + resp.Platform + *common.VSecretkey)
	} else {
		token = common.GetHash(timestampMicro + resp.PID + resp.Platform + *resp.Info + *common.VSecretkey)
	}
	resp.Token = token
	c.JSON(200, resp)
}

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

	lis, err := net.Listen("tcp", *common.ServiceAddr)
	if err != nil {
		panic(err)
	}

	ipport := strings.Split(*common.ServiceAddr, ":")
	if len(ipport) != 2 || ipport[0] == "" || ipport[1] == "" {
		log.Fatal("please set ip:port")
	}

	common.EtcdInit()

	common.Reg2etcd("http://"+*common.ServiceAddr, "guest")

	gin.SetMode(gin.ReleaseMode)

	r := gin.New()

	// config := cors.DefaultConfig()
	// config.AllowHeaders = []string{"Authorization", "Content-Type", "User-Agent", "Accept"} // CONNECT,OPTIONS,TRACE
	// config.AllowMethods = []string{"GET", "HEAD", "POST", "PUT", "DELETE"}
	// config.AllowCredentials = true
	// config.AllowOriginFunc = func(origin string) bool {
	// 	return true
	// }
	// r.Use(cors.New(config))

	r.POST("/auth", auth)

	log.Println("start guest...")
	httpservice := &http.Server{
		Handler:     r,
		ReadTimeout: 10 * time.Second,
		// WriteTimeout:   10 * time.Second,
		// MaxHeaderBytes: 1 << 20,
	}
	err = httpservice.Serve(lis) // listen and serve on 0.0.0.0:8080
	if err != nil {
		panic(err)
	}
}
