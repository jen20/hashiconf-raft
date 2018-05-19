package main

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/raft"
	"github.com/justinas/alice"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
)

type httpServer struct {
	address net.Addr
	node    *node
	logger  *zerolog.Logger
}

func (server *httpServer) Start() {
	server.logger.Info().Str("address", server.address.String()).Msg("Starting server")
	c := alice.New()
	c = c.Append(hlog.NewHandler(*server.logger))
	c = c.Append(hlog.AccessHandler(func(r *http.Request, status, size int, duration time.Duration) {
		hlog.FromRequest(r).Info().
			Str("req.method", r.Method).
			Str("req.url", r.URL.String()).
			Int("req.status", status).
			Int("req.size", size).
			Dur("req.duration", duration).
			Msg("")
	}))
	c = c.Append(hlog.RemoteAddrHandler("req.ip"))
	c = c.Append(hlog.UserAgentHandler("req.useragent"))
	c = c.Append(hlog.RefererHandler("req.referer"))
	c = c.Append(hlog.RequestIDHandler("req.id", "Request-Id"))
	handler := c.Then(server)

	if err := http.ListenAndServe(server.address.String(), handler); err != nil {
		server.logger.Fatal().Err(err).Msg("Error running HTTP server")
	}
}

func (server *httpServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.Contains(r.URL.Path, "/key") {
		server.handleRequest(w, r)
	} else if strings.Contains(r.URL.Path, "/join") {
		server.handleJoin(w, r)
	} else {
		w.WriteHeader(http.StatusBadRequest)
	}
}

func (server *httpServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		server.handleKeyPost(w, r)
		return
	case http.MethodGet:
		server.handleKeyGet(w, r)
		return
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (server *httpServer) handleKeyPost(w http.ResponseWriter, r *http.Request) {
	request := struct {
		NewValue int `json:"newValue"`
	}{}

	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		server.logger.Error().Err(err).Msg("Bad request")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	event := &event{
		Type:  "set",
		Value: request.NewValue,
	}

	eventBytes, err := json.Marshal(event)
	if err != nil {
		server.logger.Error().Err(err).Msg("")
	}

	server.node.raftNode.

	applyFuture := server.node.raftNode.Apply(eventBytes, 5*time.Second)
	if err := applyFuture.Error(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (server *httpServer) handleKeyGet(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)

	response := struct {
		Value int `json:"value"`
	}{
		Value: server.node.fsm.stateValue,
	}

	responseBytes, err := json.Marshal(response)
	if err != nil {
		server.logger.Error().Err(err).Msg("")
	}

	w.Write(responseBytes)
}

func (server *httpServer) handleJoin(w http.ResponseWriter, r *http.Request) {
	peerAddress := r.Header.Get("Peer-Address")
	if peerAddress == "" {
		server.logger.Error().Msg("Peer-Address not set on request")
		w.WriteHeader(http.StatusBadRequest)
	}

	addPeerFuture := server.node.raftNode.AddVoter(
		raft.ServerID(peerAddress), raft.ServerAddress(peerAddress), 0, 0)
	if err := addPeerFuture.Error(); err != nil {
		server.logger.Error().
			Err(err).
			Str("peer.remoteaddr", peerAddress).
			Msg("Error joining peer to Raft")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	server.logger.Info().Str("peer.remoteaddr", peerAddress).Msg("Peer joined Raft")
	w.WriteHeader(http.StatusOK)
}
