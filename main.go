package main

import (
	"fmt"
	"math/rand"
)

const (
	BLOCK_REWARD           = 10
	UNCLE_REWARD           = 0
	UNCLE_INCLUSION_REWARD = 0
	UNCLES_LIMIT           = 2
	SELFISH_PUBLISH_DELAY  = 10
)

type Miner interface {
	Tick(int, int)
	Mine(int, int) bool
	SetNeighbors([]Miner)
	AddNeighbor(Miner)

	//triggered when miner finds block or is informed of a new block
	//broadcasts the new block to the miner's neighbors
	//update miner's blockchain/uncle lists
	//  if new block has same parent but later timestamp than miner's current block, new block is uncle
	//  if new block has same parent but earlier timestamp than miner's current block, current block becomes uncle
	//  and new block becomes current block
	BlockFound(int) *Block

	//method to inform other miner of a new block

	GetBlockchain() []*Block
	GetLastBlock() *Block
	PopBlock() *Block
	GetUncles() []*Block

	AppendBlock(*Block)
	AppendUncle(*Block)

	ReceiveBlock(*Block)
	PublishBlock(*Block)
}

type HonestMiner struct {
	blockchain   []*Block
	uncles       []*Block
	neighbors    []Miner
	miningPower int
	id           string
}

type SelfishMiner struct {
	miner          Miner
	publishQueue   [][]*Block	//circular array queue
	publishCounter int
}

type Block struct {
	minerID   string
	parent    *Block
	uncles    []*Block
	timestamp int
	fees      int
	depth     int
}

//initializes new miner with the first neighbor's blockchain and uncles, and the list of neighbors as neighbors
func NewMiner(name string, neighbors []Miner, mining_power int) Miner {
	genesisBlock := NewBlock("genesis", nil,nil,0)
	bc := []*Block{}
	if len(neighbors) != 0 {
		bc = neighbors[0].GetBlockchain()
	} else {
		bc = []*Block{genesisBlock}
	}
	return &HonestMiner{
		blockchain:   bc,
		uncles:       []*Block{},
		neighbors:    neighbors,
		miningPower: mining_power,
		id:           name,
	}
}

func NewSelfishMiner(name string, neighbors []Miner, mining_power int) Miner {
	miner := NewMiner(name, neighbors, mining_power)
	queue := [][]*Block{}
	for i := 0; i < SELFISH_PUBLISH_DELAY; i++ {
		queue = append(queue, []*Block{})
	}
	return &SelfishMiner{miner: miner, publishQueue: queue, publishCounter: 0}
}

func (m *HonestMiner) SetNeighbors(n []Miner) {
	m.neighbors = n
}

func (s *SelfishMiner) SetNeighbors(n []Miner) {
	s.miner.SetNeighbors(n)
}

func (m *HonestMiner) AddNeighbor(n Miner) {
	m.neighbors = append(m.neighbors, n)
}

func (s *SelfishMiner) AddNeighbor(n Miner) {
	s.miner.AddNeighbor(n)
}

//initializes new block with a given parent, list of uncles and a timestamp
func NewBlock(minerID string, parent *Block, uncles []*Block, timestamp int) *Block {
	newDepth := -1
	newFees := 0
	if parent != nil {
		newDepth = parent.depth + 1
		newFees = timestamp - parent.timestamp + BLOCK_REWARD
	}
	return &Block{
		minerID:   minerID,
		parent:    parent,
		uncles:    uncles,
		timestamp: timestamp,
		fees:      newFees,
		depth:     newDepth,
	}
}

//do things the miner should do each time step
func (m *HonestMiner) Tick(totPower, timestamp int) {
	blockFound := m.Mine(totPower, timestamp)
	if blockFound {
		block := m.BlockFound(timestamp)
		m.AppendBlock(block)
		m.PublishBlock(block)
	}

	return
}

func (s *SelfishMiner) Tick (totPower, timestamp int) {
	blockFound := s.Mine(totPower, timestamp)
	block := &Block{}
	if blockFound {
		block = s.BlockFound(timestamp)
		s.AppendBlock(block)
		//enqueue block for delayed publishing
	}
	publishedBlocks := s.TickDelayedPublish(block)
	if len(publishedBlocks) != 0 {
		for _, b := range publishedBlocks {
			s.PublishBlock(b)
		}
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
	odds := 1 * float64(m.miningPower) / float64(totPower)
	if odds > rand.Float64() {
		return true
	}
	return false
}

//wait x ticks before announcing
func (s *SelfishMiner) Mine(totPower, timestamp int) bool {
	return s.miner.Mine(totPower, timestamp)
}

func (m *HonestMiner) BlockFound(timestamp int) *Block {
	parent := m.GetLastBlock()
	uncles := m.GetUncles()
	block := NewBlock(m.id, parent,uncles,timestamp + rand.Intn(99))	//timestamp in steps of 100 -> rand up to 99
	//m.blockchain = append(m.blockchain, block)
	//m.PublishBlock(block)
	return block
}

func (s *SelfishMiner) BlockFound(timestamp int) *Block {
	return s.miner.BlockFound(timestamp)
}

func (m *HonestMiner) PublishBlock(b *Block) {
	for _, i := range m.neighbors {
		i.ReceiveBlock(b)
	}
	fmt.Println(fmt.Sprintf("%+v\n", b))
}

func (s *SelfishMiner) PublishBlock(b *Block) {
	s.miner.PublishBlock(b)
	return
}

func (s *SelfishMiner) TickDelayedPublish(b *Block) []*Block {
	emptyBlock := &Block{}
	if b != emptyBlock {
		s.publishQueue[s.publishCounter] = append(s.publishQueue[s.publishCounter], b)
	}
	s.publishCounter = (s.publishCounter + 1) % SELFISH_PUBLISH_DELAY
	return s.publishQueue[s.publishCounter]
}

func (m *HonestMiner) ReceiveBlock(b *Block) {
	for _, i := range m.GetBlockchain() {
		if i == b {
			//TODO: write block.Equals(block) function instead of relying on pointers being the same
			return
		}
	}
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
	s.miner.ReceiveBlock(b)
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
	chain := m.GetBlockchain()
	block := &Block{}
	m.blockchain, block = m.blockchain[:len(chain)-1], m.blockchain[len(chain)-1]
	return block
}

func (s *SelfishMiner) PopBlock() *Block {
	return s.miner.PopBlock()
}

func (m *HonestMiner) AppendBlock(b *Block) {
	m.blockchain = append(m.blockchain, b)
}

func (s *SelfishMiner) AppendBlock(b *Block) {
	s.miner.AppendBlock(b)
}

func (m *HonestMiner) AppendUncle(b *Block) {
	m.uncles = append(m.uncles, b)
}

func (s *SelfishMiner) AppendUncle(b *Block) {
	s.miner.AppendUncle(b)
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
	//TODOs:
	/*
	*) let received blocks propagate through network
	*) separate mining and communicating activities
	*) uncles limit
	*/

	rand.Seed(1230)
	m := HonestMiner{id: "hello world"}
	fmt.Println(m.id)
	totalMiningPower := 0

	miners := []Miner{}
	numMiners := 3
	for i := 0; i < numMiners; i++ {
		miners = append(miners, NewMiner(fmt.Sprintf("m%d", i), nil, (i+1)%2*(i+1)))
		totalMiningPower += (i+1)%2*(i+1)
	}
	/*
	for _, i := range miners {
		i.SetNeighbors(miners)
	}
	*/
	miners[0].AddNeighbor(miners[1])
	miners[1].AddNeighbor(miners[0])
	miners[1].AddNeighbor(miners[2])
	miners[2].AddNeighbor(miners[1])
	time := 0
	for time < 10000 {
		time += 100
		for _, i := range(miners) {
			i.Tick(totalMiningPower, time)
		}
	}
}
