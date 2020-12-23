// ========== CS-438 orbitalswarm Skeleton ===========
// *** Do not change this file ***

package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"go.dedis.ch/cs438/orbitalswarm/client"
	"go.dedis.ch/cs438/orbitalswarm/gossip"
	"golang.org/x/xerrors"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

type key int

const (
	requestIDKey key = 0
)

// Controller is responsible to be the glue between the gossiping protocol and
// the ui, dispatching responses and messages etc
type Controller struct {
	sync.Mutex
	uiAddress     string
	identifier    string
	gossipAddress string
	gossiper      gossip.BaseGossiper
	cliConn       net.Conn
	messages      []CtrlMessage
	// simpleMode: true if the gossiper should broadcast messages from clients as SimpleMessages
	simpleMode bool

	hookURL *url.URL
}

type CtrlMessage struct {
	Origin string
	ID     uint32
	Text   string
}

// NewController returns the controller that sets up the gossiping state machine
// as well as the web routing. It uses the same gossiping address for the
// identifier.
func NewController(identifier, uiAddress, gossipAddress string, simpleMode bool,
	g gossip.BaseGossiper, addresses ...string) *Controller {

	c := &Controller{
		identifier:    identifier,
		uiAddress:     uiAddress,
		gossipAddress: gossipAddress,
		simpleMode:    simpleMode,
		gossiper:      g,
		searchMatches: make([]*gossip.File, 0),
	}

	g.RegisterCallback(c.NewMessage)
	return c
}

// Run ...
func (c *Controller) Run() {
	logger := Logger.With().Timestamp().Str("role", "http proxy").Logger()

	nextRequestID := func() string {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}

	r := mux.NewRouter()
	r.Methods("GET").Path("/message").HandlerFunc(c.GetMessage)
	r.Methods("POST").Path("/message").HandlerFunc(c.PostMessage)

	r.Methods("GET").Path("/origin").HandlerFunc(c.GetDirectNode)

	r.Methods("GET").Path("/node").HandlerFunc(c.GetNode)
	r.Methods("POST").Path("/node").HandlerFunc(c.PostNode)

	r.Methods("GET").Path("/id").HandlerFunc(c.GetIdentifier)
	r.Methods("POST").Path("/id").HandlerFunc(c.SetIdentifier)

	r.Methods("GET").Path("/routing").HandlerFunc(c.GetRoutingTable)
	r.Methods("POST").Path("/routing").HandlerFunc(c.AddRoute)

	r.Methods("GET").Path("/address").HandlerFunc(c.GetLocalAddr)

	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./static/")))

	server := &http.Server{
		Addr:    c.uiAddress,
		Handler: tracing(nextRequestID)(logging(logger)(r)),
	}

	err := server.ListenAndServe()
	if err != nil {
		panic(err)
	}
}

// GET /message returns all messages seen so far as json encoded Message
// XXX lot of optimizations to be done here
func (c *Controller) GetMessage(w http.ResponseWriter, r *http.Request) {
	c.Lock()
	defer c.Unlock()
	Logger.Info().Msgf("These are the msgs %v", c.messages)
	if err := json.NewEncoder(w).Encode(c.messages); err != nil {
		Logger.Err(err)
		http.Error(w, "could not encode json", http.StatusInternalServerError)
		return
	}
	Logger.Info().Msg("GUI request for the messages received by the gossiper")
}

// POST /message with text in the body as raw string
func (c *Controller) PostMessage(w http.ResponseWriter, r *http.Request) {
	Logger.Info().Msg("POSTING MESSAGE")
	c.Lock()
	defer c.Unlock()
	text, ok := readString(w, r)
	if !ok {
		Logger.Err(xerrors.New("failed to read string"))
		return
	}
	message := client.ClientMessage{}
	err := json.Unmarshal([]byte(text), &message)
	if err != nil {
		Logger.Err(err)
		return
	}
	Logger.Info().Msgf("the controller received a UI message: %+v", message)

	if c.simpleMode {
		c.gossiper.AddSimpleMessage(message.Contents)
		c.messages = append(c.messages, CtrlMessage{c.identifier, 0, message.Contents})
	} else if message.Destination != "" {
		// client message for a private message
		c.gossiper.AddPrivateMessage(message.Contents, message.Destination, c.gossiper.GetIdentifier(), 10)
		c.messages = append(c.messages, CtrlMessage{c.identifier, 0, message.Contents})
	} else {
		// client message for regular text message
		id := c.gossiper.AddMessage(message.Contents)
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, id)
		w.Write(buf)
		c.messages = append(c.messages, CtrlMessage{c.identifier, id, message.Contents})
	}
}

// GET /node returns list of nodes as json encoded slice of string
func (c *Controller) GetNode(w http.ResponseWriter, r *http.Request) {
	hosts := c.gossiper.GetNodes()
	if err := json.NewEncoder(w).Encode(hosts); err != nil {
		Logger.Err(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// GET /origin returns list of nodes in the routing table as json encoded slice of string
func (c *Controller) GetDirectNode(w http.ResponseWriter, r *http.Request) {
	hosts := c.gossiper.GetDirectNodes()
	if err := json.NewEncoder(w).Encode(hosts); err != nil {
		Logger.Err(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// POST /node with address of node in the body as a string
func (c *Controller) PostNode(w http.ResponseWriter, r *http.Request) {
	text, ok := readString(w, r)
	if !ok {
		return
	}
	Logger.Info().Msgf("GUI add node %s", text)
	if err := c.gossiper.AddAddresses(text); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

// GET /id returns the identifier as a raw string in the body
func (c *Controller) GetIdentifier(w http.ResponseWriter, r *http.Request) {
	id := c.gossiper.GetIdentifier()
	if _, err := w.Write([]byte(id)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	Logger.Info().Msg("GUI identifier request")
}

// POST /id reads the identifier as a raw string in the body and sets the
// gossiper.
func (c *Controller) SetIdentifier(w http.ResponseWriter, r *http.Request) {
	id, ok := readString(w, r)
	if !ok {
		return
	}

	Logger.Info().Msg("GUI set identifier")

	c.gossiper.SetIdentifier(id)
}

// GET /routing returns the routing table
func (c *Controller) GetRoutingTable(w http.ResponseWriter, r *http.Request) {
	routing := c.gossiper.GetRoutingTable()
	if err := json.NewEncoder(w).Encode(routing); err != nil {
		Logger.Err(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// POST /routing adds a route to the gossiper
func (c *Controller) AddRoute(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		Logger.Err(err).Msg("failed to parse form")
	}

	peerName := r.PostFormValue("peerName")
	if peerName == "" {
		Logger.Error().Msg("peerName is empty")
		return
	}

	nextHop := r.PostFormValue("nextHop")
	if nextHop == "" {
		Logger.Error().Msg("nextHop is empty")
		return
	}

	c.gossiper.AddRoute(peerName, nextHop)
}

// GET /address returns the gossiper's local addr
func (c *Controller) GetLocalAddr(w http.ResponseWriter, r *http.Request) {
	localAddr := c.gossiper.GetLocalAddr()

	_, err := w.Write([]byte(localAddr))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// NewMessage ...
func (c *Controller) NewMessage(origin string, msg gossip.GossipPacket) {
	c.Lock()
	defer c.Unlock()

	if msg.SearchReply != nil {
		results := msg.SearchReply.Results
		if len(results) == 1 {
			Logger.Info().Msg("new search result match received")

			res := results[0]
			fname := res.FileName
			metahash := hex.EncodeToString(res.MetaHash)
			newFile := &gossip.File{Name: fname, MetaHash: metahash}

			c.searchMatchesLock.Lock()
			c.searchMatches = append(c.searchMatches, newFile)
			c.searchMatchesLock.Unlock()
		}
	} else {

		if msg.Rumor != nil {
			c.messages = append(c.messages, CtrlMessage{msg.Rumor.Origin,
				msg.Rumor.ID, msg.Rumor.Text})
		}
		if msg.Simple != nil {
			c.messages = append(c.messages, CtrlMessage{msg.Simple.OriginPeerName,
				0, msg.Simple.Contents})
		}

		Logger.Info().Msgf("messages %v", c.messages)

		if c.hookURL != nil {
			cp := gossip.CallbackPacket{
				Addr: origin,
				Msg:  msg,
			}

			msgBuf, err := json.Marshal(cp)
			if err != nil {
				Logger.Err(err).Msg("failed to marshal packet")
				return
			}

			req := &http.Request{
				Method: "POST",
				URL:    c.hookURL,
				Header: map[string][]string{
					"Content-Type": {"application/json; charset=UTF-8"},
				},
				Body: ioutil.NopCloser(bytes.NewReader(msgBuf)),
			}

			Logger.Info().Msgf("sending a post callback to %s", c.hookURL)
			_, err = http.DefaultClient.Do(req)
			if err != nil {
				Logger.Err(err).Msgf("failed to call callback to %s", c.hookURL)
			}
		}
	}
}

func readString(w http.ResponseWriter, r *http.Request) (string, bool) {
	buff, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "could not read message", http.StatusBadRequest)
		return "", false
	}

	return string(buff), true
}

// logging is a utility function that logs the http server events
func logging(logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				requestID, ok := r.Context().Value(requestIDKey).(string)
				if !ok {
					requestID = "unknown"
				}
				logger.Info().Str("requestID", requestID).
					Str("method", r.Method).
					Str("url", r.URL.Path).
					Str("remoteAddr", r.RemoteAddr).
					Str("agent", r.UserAgent()).Msg("")
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// tracing is a utility function that adds header tracing
func tracing(nextRequestID func() string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-Id")
			if requestID == "" {
				requestID = nextRequestID()
			}
			ctx := context.WithValue(r.Context(), requestIDKey, requestID)
			w.Header().Set("X-Request-Id", requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}