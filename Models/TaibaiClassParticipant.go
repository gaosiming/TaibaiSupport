package Models

import (
	"context"
	"encoding/json"
	"github.com/bitly/go-simplejson"
	"github.com/gorilla/websocket"
	"log"
	"sync"
)

type TaibaiClassRole int

const (
	TeacherRole TaibaiClassRole = iota
	StudentRole
	ObserverRole
)

type TaibaiClassParticipant struct {
	User      *TaibaiUser
	Classroom *TaibaiClassroom
	Role      TaibaiClassRole
	Online    bool
	Index     int
	Rect      TaibaiRect

	Conn     *websocket.Conn
	ConnCtx  context.Context
	ConnStop context.CancelFunc

	operateMutex sync.Mutex
}

func NewTaibaiClassParticipant(classroom *TaibaiClassroom, user *TaibaiUser, role TaibaiClassRole) *TaibaiClassParticipant {
	p := &TaibaiClassParticipant{
		Classroom: classroom,
		User:      user,
		Role:      role,
	}
	p.ConnCtx, p.ConnStop = context.WithCancel(context.Background())
	return p
}

func (this *TaibaiClassParticipant) SetConn(conn *websocket.Conn) {
	this.operateMutex.Lock()
	defer this.operateMutex.Unlock()

	// 保存老的websocket
	oldConn := this.Conn

	this.Conn = conn
	this.Online = true
	go this.ReadLoop(this.Conn)

	// 在最后断掉老的
	if oldConn != nil {
		err := oldConn.Close()
		if err != nil {
			println("close old conn error")
		}
	}
}

func (this *TaibaiClassParticipant) ReadLoop(Conn *websocket.Conn) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("捕获到的错误：%v\n", r)
		}
	}()
	for {
		_, message, err := Conn.ReadMessage()
		if err != nil {
			// 有异常的话 肯定要
			log.Println("read:", err)
			wsEvent := TaibaiUserWsEvent{
				ClassroomId: this.Classroom.ClassroomId,
				UserId:      this.User.UserId,
				Conn:        nil,
			}
			if Conn == this.Conn {
				this.Conn = nil
				this.Online = false
				TaibaiClassroomManagerInstance.LeavingWsChan <- wsEvent
			}
			return
		} else {
			log.Printf("recv: %s", message)


			eventJson, err := simplejson.NewJson(message)
			if err != nil {
				log.Println(err)
				continue
			}
			eventType := eventJson.Get("eventType").MustString()
			eventContent := eventJson.Get("eventContent")

			if eventType == "videoPositionChanged" {
				userId := eventContent.Get("userId").MustInt()

				rect := TaibaiRect{}
				rectstr, _ := json.Marshal(eventContent.Get("rect").Interface())
				json.Unmarshal(rectstr, &rect)

				this.Classroom.participantPositionChanged(userId, rect)
			}

			// {"eventTime": 1557489041, "eventType": "videoPositionChanged", "eventProducer": 0, "eventContent": {"userId": 111, "rect": {"X": 189.0, "Y": 506.99999999999994, "Width": 200.0, "Height": 200.0}}}
		}
	}

}

func (this *TaibaiClassParticipant) SendMessage(message string) {
	defer func() { recover() }()
	if this.Conn != nil {
		this.Conn.WriteMessage(websocket.TextMessage, []byte(message))
	}
}
