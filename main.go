package main

import (
	"fmt"
	"math/rand"
)

const (
	BLOCK_REWARD           = 10
	UNCLE_REWARD           = 0
	UNCLE_INCLUSION_REWARD = 0
	SELFISH_PUBLISH_DELAY  = 10
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

	GetBlockchain() []*Block
}

type HonestMiner struct {
	blockchain   []*Block
	uncles       []*Block
	neighbors    []*Miner
	mining_power int
	id           string
}

type SelfishMiner struct {
	miner          Miner
	publishQueue   [][]*Block	//circular array queue
	publishCounter int
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
	bc := neighbors[0].GetBlockchain()
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
	queue := [][]*Block{}
	return &SelfishMiner{miner: miner, publishQueue: queue, publish_counter: 0}
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

//do things the miner should do each time step
func (m *HonestMiner) Tick() {
	blockFound := m.Mine()
	if blockFound {
		block := m.BlockFound()
		m.AppendBlock(block)
		m.PublishBlock(block)
	}

	return
}

func (s *SelfishMiner) Tick () {
	blockFound := s.Mine()
	if blockFound {
		s.BlockFound()
		s.AppendBlock()
		//enqueue block for delayed publishing
	}

	return
}

//miner makes an attempt to mine a new block
//new block contains fees based on time since last block in chain
//  later: expand with more detailed transaction/fees handling
//new block can reference one or more uncles, gaining extra rewards
//when block is found, trigger BlockFound()
//tot_power and timestamp tracked in main()
func (m *HonestMiner) Mine(totPower, timestamp int) bool {
	odds := m.miningPower / tot_power * 0.01
	if odds > rand.Float64() {
		return true
	}
	return false
}

//wait x ticks before announcing
func (s *SelfishMiner) Mine() {
	return s.miner.Mine()
}

func (m *HonestMiner) BlockFound(timestamp int) *Block {
	parent := m.GetLastBlock()
	uncles := m.GetUncles()
	block := NewBlock(parent,uncles,timestamp + rand.Intn(99))	//timestamp in steps of 100 -> rand up to 99
	//m.blockchain = append(m.blockchain, block)
	//m.PublishBlock(block)
	return block
}

func (s *SelfishMiner) BlockFound(timestamp int) {
	return s.miner.BlockFound(timestamp)
}

func (m *HonestMiner) PublishBlock(b *Block) {
	for _, i := range m.neighbors {
		neighbor.ReceiveBlock(b)
	}
}

func (s *SelfishMiner) PublishBlock(b *Block) {
	s.miner.PublishBlock()
	return
}

func (m *HonestMiner) ReceiveBlock(b *Block) {
	last_block := m.GetLastBlock()
	if b.depth > last_block.depth {
		//append new block to chain
		m.AppendBlock(b)
	} else if b.depth < last_block.depth {
		//append new block to uncles
		m.AppendUncle(b)
	} else if b.timestamp > last_block.timestamp {
		//same depth -> move block with lower timestamp to chain and block with higher timestamp to uncles
		m.AppendUncle(b)
	} else {
		m.uncles = append(m.uncles, m.PopBlock())
		m.AppendBlock(b)
	}
	return
}

func (s *SelfishMiner) ReceiveBlock(b *Block) {
	return
}

func (m *HonestMiner) GetBlockchain() []*Block {
	return m.blockchain
}

func (s *SelfishMiner) GetBlockchain() []*Block {
	return s.miner.GetBlockchain()
}

func (m *HonestMiner) GetLastBlock() *Block {
	chain := m.GetBlockchain()
	return chain[len(chain)-1]
}

func (s *SelfishMiner) GetLastBlock() *Block {
	return s.miner.GetLastBlock()
}

func (m *HonestMiner) PopBlock() *Block {
	chain := m.GetBlockChain()
	m.blockchain, block = m.blockchain[:len(chain)-1], b.blockchain[len(chain)-1]
	return block
}

func (s *SelfishMiner) PopBlock() *Block {
	return s.miner.PopBlock()
}

func (m *HonestMiner) AppendBlock(b *Block) {
	m.blockchain = append(m.blockchain, b)
	return
}

func (s *SelfishMiner) AppendBlock(b *Block) {
	s.miner.AppendBlock(b)
	return
}

func (m *HonestMiner) AppendUncle(b *Block) {
	m.uncles = append(m.uncles, b)
	return
}

func (s *SelfishMiner) AppendUncle(b *Block) {
	s.miner.AppendUncle(b)
	return
}
func (m *HonestMiner) GetUncles() []*Block {
	return m.uncles
}

func (s *SelfishMiner) GetUncles() []*Block {
	return s.miner.GetUncles()
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
