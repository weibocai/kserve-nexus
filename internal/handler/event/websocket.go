package event

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/kserve-nexus/internal/middleware"
)

// websocket 连接器
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// WebsocketClientManager websocket 连接管理器
type WebsocketClientManager struct {
	Name   string                     // 连接端名称
	Client map[string]*websocket.Conn // 客户端
	Lock   sync.Mutex                 // 锁，并发安全
}

// NewWebsocketClientManager 初始化websocket管理容器
func NewWebsocketClientManager(name string) *WebsocketClientManager {
	return &WebsocketClientManager{
		Name:   name,
		Client: make(map[string]*websocket.Conn),
		Lock:   sync.Mutex{},
	}
}

// AddClient 添加新的连接器
func (m *WebsocketClientManager) AddClient(c *gin.Context, name string) (*websocket.Conn, error) {
	m.Lock.Lock()
	defer m.Lock.Unlock()
	if _, ok := m.Client[name]; ok {
		return nil, fmt.Errorf("该链接名已经存在对应的连接，请切换后重新尝试")
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return nil, err
	}

	m.Client[name] = conn
	return conn, nil
}

// RemoveClient 移除新的前端
func (m *WebsocketClientManager) RemoveClient(name string) {
	m.Lock.Lock()
	defer m.Lock.Unlock()
	if _, ok := m.Client[name]; ok {
		delete(m.Client, name)
	}
}

// Broadcast 消息广播
func (m *WebsocketClientManager) Broadcast(c *gin.Context, msg string) {
	goOffline := make([]string, 0)
	for name, conn := range m.Client {
		if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			goOffline = append(goOffline, name)
		}
	}
	for _, name := range goOffline {
		m.RemoveClient(name)
	}
}

type HandleEvent struct {
	ws *WebsocketClientManager
}

func NewHandleEvent(ws *WebsocketClientManager) *HandleEvent {
	return &HandleEvent{ws}
}

// Event 创建事件websocket连接器
func (h *HandleEvent) Event(c *gin.Context) {
	name := c.Param("name")
	conn, err := h.ws.AddClient(c, name)
	if err != nil {
		middleware.CodeJson(c, middleware.WEBSOCKET_ERROR, err, "创建连接失败")
		return
	}
	data := `{
  "status": "success",
  "code": 1,
  "data": {
    "type": "event",
    "event": {
      "namespace": "default",
      "name": "isvc-name",
      "apiVersion": "serving.kserve.io/v1beta1",
      "kind": "InferenceService",
      "reason": "Succeeded",
      "message": "InferenceService default/isvc-name is ready.",
      "count": 1,
      "lastTime": "2021-09-07T09:09:09Z"
    }
  }
}`
	sendData := []byte(data)

	// 心跳检测
	ticker := time.NewTicker(time.Second * 1)
	defer ticker.Stop()
	defer h.ws.RemoveClient(name)
	select {
	case <-c.Done():
		return
	case <-ticker.C:
		if err = conn.WriteMessage(websocket.PingMessage, nil); err != nil {
			return
		}
		if err = conn.WriteMessage(websocket.TextMessage, sendData); err != nil {
			return
		}
	}
}

// GetEventHost 获取websocket的访问路由
func (h *HandleEvent) GetEventHost(c *gin.Context) {
	u1 := strings.ReplaceAll(uuid.New().String(), "-", "")
	middleware.SuccessJson(c, map[string]string{"url": "ws://" + c.Request.Host + "/websocket/event/" + u1})
	return
}
