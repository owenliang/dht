package dht

import (
	"sync"
	"math/rand"
	"time"
)

type TokenManager struct {
	mutex sync.Mutex
	tokens [2]string
}

func genToken() string {
	randBytes := make([]byte, 160)
	for {
		if _, err := rand.Read(randBytes); err == nil {
			return string(randBytes)
		}
	}
}

func (mgr *TokenManager)refreshToken() {
	for {
		time.Sleep(time.Duration(5) * time.Minute)

		mgr.mutex.Lock()
		mgr.tokens[0] = mgr.tokens[1]
		mgr.tokens[1] = genToken()
		mgr.mutex.Unlock()
	}
}

var myTokenMgr *TokenManager
var initTokenMgrOnce sync.Once

// 5分钟刷新一次token, 生成的token10分钟内有效
func GetTokenManager() *TokenManager {
	initTokenMgrOnce.Do(func() {
		myTokenMgr = &TokenManager{}
		myTokenMgr.tokens[0] = genToken()
		myTokenMgr.tokens[1] = genToken()
		go myTokenMgr.refreshToken()
	})
	return myTokenMgr
}

func (mgr *TokenManager) ValidateToken(token string) bool {
	mgr.mutex.Lock()
	defer mgr.mutex.Unlock()

	for _, myToken := range mgr.tokens {
		if token == myToken {
			return true
		}
	}
	return false
}

func (mgr *TokenManager) GetToken() string {
	mgr.mutex.Lock()
	defer mgr.mutex.Unlock()

	return mgr.tokens[1]
}