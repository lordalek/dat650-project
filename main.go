package main

import (
	"fmt"
)

const (
	BLOCK_REWARD           = 10
	UNCLE_REWARD           = 0
	UNCLE_INCLUSION_REWARD = 0
)

type Miner interface {
	Mine()

	//triggered when miner finds block or is informed of a new block
	//broadcasts the new block to the miner's neighbors
	//update miner's blockchain/uncle lists
	//  if new block has same parent but later timestamp than miner's current block, new block is uncle
	//  if new block has same parent but earlier timestamp than miner's current block, current block becomes uncle
	//  and new block becomes current block
	BlockFound()

	//method to inform other miner of a new block
	ReceiveBlock(b *Block)
}

type HonestMiner struct {
	blockchain   []*Block
	uncles       []*Block
	neighbors    []*Miner
	mining_power int
	id           string
}

type SelfishMiner struct {
	miner Miner
}

type Block struct {
	parent    *Block
	uncles    []*Block
	timestamp int
	fees      int
	depth     int
}

//initializes new miner with the first neighbor's blockchain and uncles, and the list of neighbors as neighbors
func NewMiner(name string, neighbors []*Miner, mining_power int) Miner {
	bc := neighbors[0].blockchain
	return &HonestMiner{
		blockchain:   bc,
		uncles:       []*Block{},
		neighbors:    neighbors,
		mining_power: mining_power,
		id:           name,
	}
}

func NewSelfishMiner(name string, neighbors []*Miner, mining_power int) Miner {
	miner := NewMiner(name, neighbors, mining_power)
	return &SelfishMiner{miner: miner}
}

//initializes new block with a given parent, list of uncles and a timestamp
func NewBlock(parent *Block, uncles []*Block, timestamp int) *Block {
	return &Block{
		parent:    parent,
		uncles:    uncles,
		timestamp: timestamp,
		fees:      timestamp - parent.timestamp + BLOCK_REWARD,
		depth:     parent.depth + 1,
	}
}

//miner makes an attempt to mine a new block
//new block contains fees based on time since last block in chain
//  later: expand with more detailed transaction/fees handling
//new block can reference one or more uncles, gaining extra rewards
//when block is found, trigger BlockFound()
func (m *HonestMiner) Mine() {
	return
}

//wait x ticks before announcing
func (s *SelfishMiner) Mine() {
	s.miner.Mine()
}

func (m *HonestMiner) BlockFound() {
	return
}

func (s *SelfishMiner) BlockFound() {
	return
}

func (m *HonestMiner) ReceiveBlock(b *Block) {
	return
}

func (s *SelfishMiner) ReceiveBlock(b *Block) {
	return
}

//calculate fairness
//	- kinds of fairness
//  - with/without uncles
func CalculateFairness( /*TODO*/ ) float64 {
	return 0
}

func main() {
	m := HonestMiner{id: "hello world"}
	fmt.Println(m.id)
}
