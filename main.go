package main

import (
	"log"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/gorilla/mux"
	"github.com/suryatresna/ws-chat/pkg/epoller"
	"github.com/suryatresna/ws-chat/pkg/socket"
)

func main() {
	epoll, err := epoller.NewPollerWithBuffer(1000)
	if err != nil {
		log.Println("[ERR] fail ini newPoller. err: ", err)
		return
	}

	log.Println("[INFO] initialize NewSocket")
	sock := socket.NewSocket(epoll)

	go sock.Listen()
	go sock.Start()

	startServer(&Handler{sock: sock})
}

var (
	defaultPort = ":8000"
	serviceName = "ws-chat"
)

func startServer(hdl *Handler) {

	router := mux.NewRouter()

	router.HandleFunc("/ws", hdl.WsHandler)

	httpSrv := http.NewServer(http.Address(defaultPort))
	httpSrv.HandlePrefix("/", router)

	app := kratos.New(
		kratos.Name(serviceName),
		kratos.Server(
			httpSrv,
		),
	)

	log.Println("[INFO] connect to HTTP server at ", defaultPort)
	if err := app.Run(); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

type Handler struct {
	sock *socket.Socket
}

func (h *Handler) WsHandler(w http.ResponseWriter, r *http.Request) {
	if h.sock == nil {
		log.Println("[Errr] no socket")
		return
	}

	roomID := r.URL.Query().Get("roomID")

	conn, err := h.sock.Upgrade(w, r)
	if err != nil {
		log.Println("[Errr] ", err)
		return
	}

	if err := h.sock.Add(conn, roomID); err != nil {
		log.Println("[Errr] ", err)
		return
	}
}
