package paxos

import (
	"encoding/hex"

	"go.dedis.ch/cs438/orbitalswarm/extramessage"
	"go.dedis.ch/cs438/orbitalswarm/gossip"
	"go.dedis.ch/cs438/orbitalswarm/paxos/blk"
	"go.dedis.ch/onet/v3/log"
)

// BlockChain allow to handle HandlingPackets
type BlockChain struct {
	numParticipant int
	nodeIndex      int
	paxosRetry     int

	tail   *blk.BlockContainer
	blocks map[string]*blk.BlockContainer
	tlc    *TLC

	blockFactory blk.BlockFactory
}

func NewBlockchain(numParticipant int, nodeIndex int, paxosRetry int, blockFactory blk.BlockFactory) *BlockChain {
	blocks := make(map[string]*blk.BlockContainer)

	return &BlockChain{
		numParticipant: numParticipant,
		nodeIndex:      nodeIndex,
		paxosRetry:     paxosRetry,

		tlc:          NewTLC(numParticipant, nodeIndex, paxosRetry, 0, blockFactory),
		tail:         nil,
		blocks:       blocks,
		blockFactory: blockFactory,
	}
}

func (b *BlockChain) Propose(g *gossip.Gossiper, blockContent blk.BlockContent) {
	if b.tail == nil {
		// First block
		log.Printf("Block type of propose : %s", blockContent.BlockType())
		b.tlc.propose(g, b.blockFactory.NewGenesisBlock(blockContent.BlockType(), 0, blockContent))
	} else {
		b.tlc.propose(g, b.blockFactory.NewBlock(blockContent.BlockType(), b.tail.BlockNumber()+1, b.tail.Hash(), blockContent))
	}
}

// GetBlocks returns all the blocks added so far. Key should be hexadecimal
// representation of the block's hash. The first return is the hexadecimal
// hash of the last block.
func (b *BlockChain) GetBlocks() (string, map[string]*blk.BlockContainer) {
	if b.tail == nil {
		return hex.EncodeToString(make([]byte, 32)), b.blocks
	}
	return hex.EncodeToString(b.tail.Hash()), b.blocks
}

func (b *BlockChain) HandleExtraMessage(g *gossip.Gossiper, msg *extramessage.ExtraMessage) *blk.BlockContainer {
	block := b.tlc.handleExtraMessage(g, msg)
	if block != nil {
		b.blocks[hex.EncodeToString(block.Hash())] = block
		b.tail = block
		b.tlc.stop()
		b.tlc = NewTLC(b.numParticipant, b.nodeIndex, b.paxosRetry, b.tail.BlockNumber()+1, b.blockFactory)
	}
	return block
}
