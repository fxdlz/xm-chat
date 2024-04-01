package main

import (
	"fmt"
	"github.com/eatmoreapple/openwechat"
	"github.com/spf13/viper"
	"log"
	"strings"
	"sync"
	"time"
	"xm-chat/llm"
)

var (
	self          *openwechat.Self
	mq            chan *openwechat.Message
	groups        []string
	userChat      map[string][]string
	rwLock        sync.RWMutex
	aiSwitch      bool = true
	bot           *openwechat.Bot
	firstQuestion string = "你好"
)

func contains[T any](slice []T, target T, equal func(a, b T) bool) bool {
	for _, v := range slice {
		if equal(v, target) {
			return true
		}
	}
	return false
}

func initViper() {
	viper.SetConfigName("api")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("config")
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error config file: %w", err))
	}
}

func main() {
	initViper()
	group := viper.GetString("group")
	userChat = make(map[string][]string)
	groups = append(groups, group)
	mq = make(chan *openwechat.Message, 10)
	defer close(mq)
	var err error
	yy := llm.NewYiyanLLM()

	bot = openwechat.DefaultBot(openwechat.Desktop) // 桌面模式

	dispatcher := openwechat.NewMessageMatchDispatcher()
	dispatcher.OnText(func(ctx *openwechat.MessageContext) {
		msg := ctx.Message
		isSelf := msg.IsSendBySelf()
		if msg.IsSendByGroup() {
			if isSelf && msg.Content == "小明闭嘴" {
				fmt.Println(msg.Content)
				aiSwitch = false
			} else if isSelf && msg.Content == "小明说话" {
				fmt.Println(msg.Content)
				aiSwitch = true
			} else if isSelf && msg.Content == "小明退出" {
				er := bot.Logout()
				if er != nil {
					panic(er)
				}
				return
			} else if aiSwitch && strings.HasPrefix(msg.Content, "小明，") {
				mq <- msg
			}
		}
	})

	// 注册消息处理函数
	bot.MessageHandler = dispatcher.AsMessageHandler()
	// 注册登陆二维码回调
	bot.UUIDCallback = openwechat.PrintlnQrcodeUrl

	// 登陆
	reloadStorage := openwechat.NewFileHotReloadStorage("storage.json")
	defer reloadStorage.Close()
	err = bot.PushLogin(reloadStorage, openwechat.NewRetryLoginOption())
	if err != nil {
		fmt.Println(err)
		return
	}

	//// 登陆
	//if err := bot.Login(); err != nil {
	//	fmt.Println(err)
	//	return
	//}

	// 获取登陆的用户
	self, err = bot.GetCurrentUser()
	if err != nil {
		fmt.Println(err)
		return
	}

	for i := 0; i < 10; i++ {
		go chatAi(yy)
	}

	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			rwLock.Lock()
			userChat = make(map[string][]string)
			rwLock.Unlock()
		}
	}()
	//c := make(chan os.Signal, 1)
	//signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	//for {
	//	select {
	//	case <-c:
	//		err1 := bot.Logout()
	//		if err1 != nil {
	//			panic(err1)
	//		}
	//		log.Println("退出成功")
	//	default:
	//
	//	}
	//}

	bot.Block()

}

func chatAi(yy *llm.YiyanLLM) {
	var (
		sendGroup *openwechat.User
		err       error
	)
	for true {
		var msg *openwechat.Message
		select {
		case msg = <-mq:
			//当群里消息的发送者是当前登录者时，使用msg.Receiver()可以获得所在群组名
			//https://github.com/eatmoreapple/openwechat/issues/424
			isSelf := msg.IsSendBySelf()
			if isSelf {
				sendGroup, err = msg.Receiver()
			} else {
				sendGroup, err = msg.Sender()
			}
			if err != nil {
				log.Println(err)
				return
			}
			groupName := sendGroup.NickName
			gp := openwechat.Group{User: sendGroup}
			res := contains[string](groups, groupName, func(a, b string) bool {
				return a == b
			})
			if res {
				sender, er := msg.SenderInGroup()
				if er != nil {
					log.Println(er)
					return
				}
				fmt.Println(sender.NickName + ": " + msg.Content)
				ques := strings.TrimPrefix(msg.Content, "小明，")
				rwLock.Lock()
				chats, ok := userChat[groupName+":"+sender.NickName]
				if ok {
					chats = append(chats, ques)
					userChat[groupName+":"+sender.NickName] = chats
				} else {
					userChat[groupName+":"+sender.NickName] = []string{firstQuestion, ques}
				}
				rwLock.Unlock()
				questions := userChat[groupName+":"+sender.NickName]
				if len(questions)%2 == 0 {
					questions = questions[1:]
				}
				allow, er := yy.Ask(questions)
				if er != nil {
					panic(er)
				}
				_, er = self.SendTextToGroup(&gp, allow)
				if er != nil {
					panic(er)
				}
			}
		default:
		}
	}
}
