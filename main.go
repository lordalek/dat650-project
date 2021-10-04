package main

import (
	"fmt"
	"math/rand"
	"strings"
)

const (
	BLOCK_REWARD           = 10
	UNCLE_REWARD           = 0
	UNCLE_INCLUSION_REWARD = 0
	UNCLES_LIMIT           = 2
	SELFISH_PUBLISH_DELAY  = 10
)

type Miner interface {
	TickMine(int, int)
	TickCommunicate()
	Mine(int, int) *Block
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
	GetUncles() map[string]*Block

	AppendBlock(*Block)
	AppendUncle(*Block)
	ForgetUncle(*Block)

	ReceiveBlock(*Block)
	EnqueueBlock(*Block)
	PublishBlock(*Block)
}

type HonestMiner struct {
	blockchain   []*Block
	uncles       map[string]*Block
	neighbors    []Miner
	miningPower  int
	id           string
	publishQueue []*Block
	SeenBlocks   map[string]interface{}
}

type SelfishMiner struct {
	miner          Miner
	publishQueue   [][]*Block	//circular array queue
	publishCounter int
}

type Block struct {
	minerID   string
	parent    *Block
	uncles    map[string]*Block
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
		uncles:       make(map[string]*Block),
		neighbors:    neighbors,
		miningPower:  mining_power,
		id:           name,
		SeenBlocks:   make(map[string]interface{}),
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
func NewBlock(minerID string, parent *Block, uncles map[string]*Block, timestamp int) *Block {
	newDepth := -1
	newFees := 0
	if parent != nil {
		newDepth = parent.depth + 1
		newFees = timestamp - parent.timestamp + BLOCK_REWARD
	}

	buncles := make(map[string]*Block)
	for k, v := range uncles {
		buncles[k] = v
	}
	return &Block{
		minerID:   minerID,
		parent:    parent,
		uncles:    buncles,
		timestamp: timestamp,
		fees:      newFees,
		depth:     newDepth,
	}
}

func (b *Block) GetID() string {
	return fmt.Sprintf("%s_%d", b.minerID, b.timestamp)
}

func (b *Block) String() string {
	var lines []string
	lines = append(lines, fmt.Sprintf("============ Block %s ============", b.GetID()))
	lines = append(lines, fmt.Sprintf("Parent: %s", b.parent.GetID()))
	lines = append(lines, fmt.Sprintf("Timestamp: %v", b.timestamp))
	lines = append(lines, fmt.Sprintf("depth: %d", b.depth))
	lines = append(lines, fmt.Sprintf("Uncles: %d", len(b.uncles)))
	for i, _ := range b.uncles {
		lines = append(lines, fmt.Sprintf("\t%s", i))
	}
	return strings.Join(lines, "\n")
}

func (b *Block) Equals(block *Block) bool {
	return b.GetID() == block.GetID()
	//return b.minerID == block.minerID && b.parent.GetID() == block.parent.GetID() && b.timestamp == block.timestamp && b.depth == block.depth
	//TODO: check if uncles match
}

//do things the miner should do each time step
/*func (m *HonestMiner) Tick(totPower, timestamp int) {
	block := m.Mine(totPower, timestamp)
	if block != nil {
		m.AppendBlock(block)
		m.PublishBlock(block)
	}

	return
}

func (s *SelfishMiner) Tick (totPower, timestamp int) {
	block := s.Mine(totPower, timestamp)
	if block != nil {
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
}*/

func (m *HonestMiner) TickMine(totPower, timestamp int) {
	block := m.Mine(totPower, timestamp)
	if block != nil {
		m.AppendBlock(block)
		m.EnqueueBlock(block)
	}
}

func (m *HonestMiner) TickCommunicate() {
	blocks := m.publishQueue
	m.publishQueue = []*Block{}
	for _, b := range blocks {
		m.PublishBlock(b)
	}
}

func (s *SelfishMiner) TickMine(totPower, timestamp int) {
	block := s.Mine(totPower, timestamp)
	if block != nil {
		s.AppendBlock(block)
		s.EnqueueBlock(block)
	}
}

func (s *SelfishMiner) TickCommunicate() {
	//increment queue pointer then publish items at pointer pos.
	s.publishCounter += 1
	blocks := s.publishQueue[s.publishCounter]
	for _, b := range blocks {
		s.PublishBlock(b)
	}
}

//miner makes an attempt to mine a new block
//new block contains fees based on time since last block in chain
//  later: expand with more detailed transaction/fees handling
//new block can reference one or more uncles, gaining extra rewards
//when block is found, trigger BlockFound()
//tot_power and timestamp tracked in main()
func (m *HonestMiner) Mine(totPower, timestamp int) *Block {
	odds := 0.1 * float64(m.miningPower) / float64(totPower)
	if odds > rand.Float64() {
		return m.BlockFound(timestamp)
	}
	return nil
}

//wait x ticks before announcing
func (s *SelfishMiner) Mine(totPower, timestamp int) *Block {
	return s.miner.Mine(totPower, timestamp)
}

func (m *HonestMiner) BlockFound(timestamp int) *Block {
	parent := m.GetLastBlock()
	uncles := m.GetUncles()
	block := NewBlock(m.id, parent,uncles,timestamp + rand.Intn(99))	//timestamp in steps of 100 -> rand up to 99
	//remove uncles
	for _, i := range uncles {
		//TODO: remove uncle
		m.ForgetUncle(i)
	}

	fmt.Println(fmt.Sprintf("%+v\n", block))

	m.SeenBlocks[block.GetID()] = true
	return block
}

func (s *SelfishMiner) BlockFound(timestamp int) *Block {
	return s.miner.BlockFound(timestamp)
}

func (m *HonestMiner) PublishBlock(b *Block) {
	for _, i := range m.neighbors {
		i.ReceiveBlock(b)
	}
}

func (s *SelfishMiner) PublishBlock(b *Block) {
	s.miner.PublishBlock(b)
}

func (m *HonestMiner) EnqueueBlock(b *Block) {
	m.publishQueue = append(m.publishQueue, b)
}

func (s *SelfishMiner) EnqueueBlock(b *Block) {
	if b != nil {
		s.publishQueue[s.publishCounter+1] = append(s.publishQueue[s.publishCounter], b)
	}
}

func (s *SelfishMiner) DelayBlock(b *Block) {
	if b != nil {
		s.publishQueue[s.publishCounter] = append(s.publishQueue[s.publishCounter], b)
	}
}

/*
func (s *SelfishMiner) TickDelayedPublish(b *Block) []*Block {
	emptyBlock := &Block{}
	if b != emptyBlock {
		s.publishQueue[s.publishCounter] = append(s.publishQueue[s.publishCounter], b)
	}
	s.publishCounter = (s.publishCounter + 1) % SELFISH_PUBLISH_DELAY
	return s.publishQueue[s.publishCounter]
}*/

//ignore block if has been seen before
//if block extends current block, append to chain
//if extends previous block, add to uncles
func (m *HonestMiner) ReceiveBlock(b *Block) {
	if _, found := m.SeenBlocks[b.GetID()]; found {
		return
	}
	m.SeenBlocks[b.GetID()] = true

	for _, i := range b.uncles {
		m.ForgetUncle(i)
		m.SeenBlocks[i.GetID()] = true
	}

	m.EnqueueBlock(b)

	//compare latest block to newly received block, set one to latest block and set the other to uncle
	last_block := m.PopBlock()
	if last_block.depth < b.depth {
		//append new block to chain
		m.AppendBlock(last_block)
		m.AppendBlock(b)
	} else if last_block.depth > b.depth {
		//append new block to uncles 
		m.AppendBlock(last_block)
		m.AppendUncle(b)
	} else if last_block.timestamp < b.timestamp {
		//same depth -> move block with lower timestamp to chain and block with higher timestamp to uncles
		m.AppendBlock(last_block)
		m.AppendUncle(b)
	} else {
		m.AppendBlock(b)
		m.AppendUncle(last_block)
	}
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
	m.uncles[b.GetID()] = b
}

func (s *SelfishMiner) AppendUncle(b *Block) {
	s.miner.AppendUncle(b)
}

func (m *HonestMiner) ForgetUncle(b *Block) {
	delete(m.uncles, b.GetID())
}

func (s *SelfishMiner) ForgetUncle(b *Block) {
	s.miner.ForgetUncle(b)
}

func (m *HonestMiner) GetUncles() map[string]*Block {
	return m.uncles
}

func (s *SelfishMiner) GetUncles() map[string]*Block {
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

	rand.Seed(42069)
	fmt.Println("hello_world")
	totalMiningPower := 0

	miners := []Miner{}
	numMiners := 100
	for i := 0; i < numMiners; i++ {
		newMinerPowa := rand.Intn(10)
		miners = append(miners, NewMiner(fmt.Sprintf("m%d", i), nil, newMinerPowa)) //(i+1)%2*(i+1)))
		totalMiningPower += newMinerPowa//(i+1)%2*(i+1)
	}
	for idx, i := range miners {
		i.SetNeighbors([]Miner{miners[(idx-1+numMiners)%numMiners]})
		i.SetNeighbors([]Miner{miners[(idx+1)%numMiners]})
	}
	/*
	miners[0].AddNeighbor(miners[1])
	miners[1].AddNeighbor(miners[0])
	miners[1].AddNeighbor(miners[2])
	miners[2].AddNeighbor(miners[1])
	*/

	time := 0
	for time < 10000 {
		time += 100
		for _, i := range(miners) {
			i.TickMine(totalMiningPower, time)
		}
		for _, i := range(miners) {
			i.TickCommunicate()
		}
	}
	fmt.Printf("total mining power: %d\n", totalMiningPower)
}
