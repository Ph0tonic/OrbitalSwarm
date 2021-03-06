package gossip

// ========== CS-438 orbitalswarm Skeleton ===========

import (
	"go.dedis.ch/cs438/orbitalswarm/extramessage"
	"gonum.org/v1/gonum/spatial/r3"
)

// GetFactory returns the Gossip factory
func GetFactory() GossipFactory {
	return BaseGossipFactory{}
}

// GossipPacket defines the packet that gets encoded or deserialized from the
// network.
type GossipPacket struct {
	Rumor   *RumorMessage   `json:"rumor"`
	Status  *StatusPacket   `json:"status"`
	Private *PrivateMessage `json:"private"`
}

// Copy performs a deep copy of the GossipPacket. When we use the watcher, it is
// best not to give a pointer to the original packet, as it could create some
// race.
func (g GossipPacket) Copy() GossipPacket {
	var rumor *RumorMessage
	var status *StatusPacket
	var private *PrivateMessage

	if g.Rumor != nil {
		rumor = new(RumorMessage)
		rumor.Origin = g.Rumor.Origin
		rumor.ID = g.Rumor.ID
		rumor.Text = g.Rumor.Text

		if g.Rumor.Extra != nil {
			rumor.Extra = g.Rumor.Extra.Copy()
		}
	}

	if g.Status != nil {
		status = new(StatusPacket)
		status.Want = append([]PeerStatus{}, g.Status.Want...)
	}

	if g.Private != nil {
		private = new(PrivateMessage)
		private.Destination = g.Private.Destination
		private.HopLimit = g.Private.HopLimit
		private.ID = g.Private.ID
		private.Origin = g.Private.Origin
		private.Data = PrivateMessageData{
			Location: g.Private.Data.Location,
			DroneID:  g.Private.Data.DroneID,
		}
	}

	return GossipPacket{
		Rumor:   rumor,
		Status:  status,
		Private: private,
	}
}

// RumorMessage denotes of an actual message originating from a given Peer in the network.
type RumorMessage struct {
	Origin string `json:"origin"`
	ID     uint32 `json:"id"`
	Text   string `json:"text"`

	Extra *extramessage.ExtraMessage `json:"extra"`
}

// StatusPacket is sent as a status of the current local state of messages seen
// so far. It can start a rumormongering process in the network.
type StatusPacket struct {
	Want []PeerStatus `json:"want"`
}

// PeerStatus shows how far have a node see messages coming from a peer in
// the network.
type PeerStatus struct {
	Identifier string `json:"identifier"`
	NextID     uint32 `json:"nextid"`
}

// RouteStruct to hold the routes of other nodes. The Origin (Destination)
// is the key of the routes-map.
type RouteStruct struct {
	// NextHop is the address of the forwarding peer
	NextHop string
	// LastID is the sequence number
	LastID uint32
}

// PrivateMessageData contains the location of a drone and the droneId
type PrivateMessageData struct {
	Location r3.Vec `json:"location"`
	DroneID  uint32 `json:"droneId"`
}

// PrivateMessage is sent privately to one peer
type PrivateMessage struct {
	Origin      string             `json:"origin"`
	ID          uint32             `json:"id"`
	Data        PrivateMessageData `json:"data"`
	Destination string             `json:"destination"`
	HopLimit    int                `json:"hoplimit"`
}

// NewMessageCallback is the type of function that users of the library should
// provide to get a feedback on new messages detected in the gossip network.
type NewMessageCallback func(origin string, message GossipPacket)

// GossipFactory provides the primitive to instantiate a new Gossiper
type GossipFactory interface {
	New(address, identifier string, antiEntropy int, routeTimer int,
		numParticipant int) (*Gossiper, error)
}

// BaseGossiper ...
type BaseGossiper interface {
	BroadcastMessage(GossipPacket)
	RegisterHandler(handler interface{}) error
	// GetNodes returns the list of nodes this gossiper knows currently in the
	// network.
	GetNodes() []string
	// GetDirectNodes returns the list of nodes this gossiper knows  in its routing table
	GetDirectNodes() []string
	// SetIdentifier changes the identifier sent with messages originating from this
	// gossiper.
	SetIdentifier(id string)
	// GetIdentifier returns the currently used identifier for outgoing messages from
	// this gossiper.
	GetIdentifier() string
	// AddMessage takes a text that will be spread through the gossip network
	// with the identifier of g. It returns the ID of the message
	AddMessage(text string) uint32
	// AddPrivateMessage
	AddPrivateMessage(data PrivateMessageData, dest string, origin string, hoplimit int)
	// AddExtraMessage allow to send some extra message via the rumors system
	AddExtraMessage(paxosMsg *extramessage.ExtraMessage) uint32
	// AddAddresses takes any number of node addresses that the gossiper can contact
	// in the gossiping network.
	AddAddresses(addresses ...string) error
	// AddRoute updates the gossiper's routing table by adding a next hop for the given
	// peer node
	AddRoute(peerName, nextHop string)
	// RegisterCallback registers a callback needed by the controller to update
	// the view.
	RegisterCallback(NewMessageCallback)
	// Run creates the UPD connection and starts the gossiper. This function is
	// assumed to be blocking until Stop is called. The ready chan should be
	// closed when the Gossiper is started.
	Run(ready chan struct{})
	// Stop stops the Gossiper
	Stop()
	// GetRoutingTable returns the routing table of the node.
	GetRoutingTable() map[string]*RouteStruct
	// GetLocalAddr returns the local address (ip:port) used for sending and receiving packets to/from the network.
	GetLocalAddr() string
}
