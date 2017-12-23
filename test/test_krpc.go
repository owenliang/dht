package main

import (
	"github.com/owenliang/dht"

	"os"
	"fmt"
)

func main()  {
	var (
		krpc *dht.KRPC
		pingRequest *dht.PingRequest
		pingResponse *dht.PingResponse
		err error
	)
	if krpc, err = dht.CreateKPRC(); err != nil {
		os.Exit(1)
	}

	pingRequest = dht.NewPingRequest()
	if pingResponse, err = krpc.Ping(pingRequest); err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(pingResponse)
	}

}
