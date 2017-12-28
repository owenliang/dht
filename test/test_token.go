package main

import (
	"github.com/owenliang/dht"
)

func main()  {
	mgr := dht.CreateTokenManager()
	token := mgr.GetToken()
	for {
		if !mgr.ValidateToken(token) {
			break
		}
	}
}
