package socket

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/suryatresna/ws-chat/pkg/epoller"
)

type Socket struct {
	receive chan *Receive
	epoll   epoller.Poller
	fdRoom  map[int]string // map[fd]RoomID
	mu      sync.Mutex
}

type Receive struct {
	Message []byte
	Op      ws.OpCode
	RoomID  string
}

type Message struct {
	Message string `json:"message"`
	RoomID  string `json:"room_id"`
}

func NewSocket(poller epoller.Poller) *Socket {
	return &Socket{
		receive: make(chan *Receive),
		epoll:   poller,
		fdRoom:  make(map[int]string),
	}
}

func (s *Socket) Upgrade(w http.ResponseWriter, r *http.Request) (net.Conn, error) {
	conn, _, _, err := ws.UpgradeHTTP(r, w)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (s *Socket) Add(conn net.Conn, roomID string) error {
	if conn == nil {
		return errors.New("connection fail to initialize")
	}

	fd, err := s.epoll.Add(conn)
	if err != nil {
		return errors.New("connection fail to add")
	}
	s.mu.Lock()
	s.fdRoom[fd] = roomID
	s.mu.Unlock()

	return nil
}

func (s *Socket) Start() {
	for {
		connections, err := s.epoll.WaitWithFd(1000)
		if err != nil {

			log.Println("[Error] fail wait epoll " + err.Error())
			return
		}

		for fd, conn := range connections {
			if conn == nil {
				break
			}

			msg, op, err := wsutil.ReadClientData(conn)
			if err != nil {
				// handle error
				if errEpoll := s.epoll.Remove(conn); errEpoll != nil {

					log.Println("[Error] fail remove epoll " + errEpoll.Error())
				}
				conn.Close()
				// log.Println("[INFO] connection break ", err)
				break
			}

			if op == ws.OpClose {
				if errEpoll := s.epoll.Remove(conn); errEpoll != nil {

					log.Println("[Error] fail remove epoll " + errEpoll.Error())
				}
				conn.Close()
				continue
			}

			if op == ws.OpPing {
				err := wsutil.WriteServerMessage(conn, ws.OpPong, []byte("\\pong"))
				if err != nil {

					log.Println("[Error] fail write pong " + err.Error())
				}
				continue
			}

			if roomID, ok := s.fdRoom[fd]; ok {
				recv := &Receive{
					Message: msg,
					Op:      op,
					RoomID:  roomID,
				}

				s.receive <- recv

				// log.Println("[DEBUG] send message ", string(recv.Message))
			}
		}
	}
}

func (s *Socket) Broadcast(ctx context.Context, roomID string, msg []byte) error {
	recv := &Receive{
		Message: msg,
		Op:      ws.OpText,
		RoomID:  roomID,
	}

	s.receive <- recv

	return nil
}

func (s *Socket) Report() map[string][]int {
	var rooms map[string][]int = make(map[string][]int)
	for fd, roomID := range s.fdRoom {
		rooms[roomID] = append(rooms[roomID], fd)
	}

	return rooms
}

func (s *Socket) Listen() {
	for {
		conns := s.epoll.GetMapConnections()
		select {
		case recv, ok := <-s.receive:
			if !ok {
				return
			}

			if recv.Op == ws.OpText {
				for fd, conn := range conns {
					if roomID, ok := s.fdRoom[fd]; ok && roomID == recv.RoomID {
						err := wsutil.WriteServerMessage(conn, ws.OpText, recv.Message)
						if err != nil {

							log.Println("[Error] fail write message " + err.Error())
						}
					}
				}
			}

			// log.Println("[DEBUG] receive message ", string(recv.Message))
		default:
		}
	}
}
